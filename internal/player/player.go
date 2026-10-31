// Package player runs mpv as a separate borderless picture-in-picture window
// and controls it over mpv's JSON IPC (Windows named pipe).
package player

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

type Status string

const (
	StatusIdle     Status = "idle"
	StatusStarting Status = "starting"
	StatusPlaying  Status = "playing"
	StatusStopping Status = "stopping"
	StatusError    Status = "error"
)

// State is the snapshot pushed to the UI. Volume, Muted, OnTop and Size are
// also remembered as preferences while no player is running.
type State struct {
	Status    Status `json:"status"`
	URL       string `json:"url"`
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Paused    bool   `json:"paused"`
	Buffering bool   `json:"buffering"`
	Volume    int    `json:"volume"`
	Muted     bool   `json:"muted"`
	OnTop     bool   `json:"onTop"`
	Size      string `json:"size"`
	Error     string `json:"error"`
	// Position and Duration are in seconds; Duration is 0 for live streams.
	Position float64 `json:"position"`
	Duration float64 `json:"duration"`
	Seekable bool    `json:"seekable"`
	// Live is reported by yt-dlp; mpv alone shows live HLS as a short,
	// seekable buffer window.
	Live bool `json:"live"`

	Speed     float64    `json:"speed"`
	SubTracks []SubTrack `json:"subTracks"`
	Sub       int        `json:"sub"` // selected subtitle track id, 0 = off

	Quality      string `json:"quality"` // see qualityHeights
	Opacity      int    `json:"opacity"` // percent
	ClickThrough bool   `json:"clickThrough"`
	Hidden       bool   `json:"hidden"`

	Recent []RecentItem `json:"recent"`
}

// SubTrack is a subtitle track offered in the UI.
type SubTrack struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

// resetMedia clears everything that belongs to the current file.
func (st *State) resetMedia() {
	st.Title, st.Paused, st.Buffering = "", false, false
	st.Position, st.Duration, st.Seekable, st.Live = 0, 0, false, false
	st.SubTracks, st.Sub = nil, 0
}

// qualityHeights maps the quality setting to a maximum video height for
// yt-dlp; 0 means no limit.
var qualityHeights = map[string]int{"480": 480, "720": 720, "1080": 1080, "best": 0}

const minOpacity = 20

// ytdlFormat prefers HLS: YouTube often answers direct DASH links of longer
// videos with HTTP 403, while its HLS variants stream reliably. AV1 is
// skipped because it often lacks hardware decoding.
func ytdlFormat(quality string) string {
	h := ""
	if limit := qualityHeights[quality]; limit > 0 {
		h = fmt.Sprintf("[height<=?%d]", limit)
	}
	return "bv*" + h + "[vcodec!*=av01][protocol^=m3u8]+ba[protocol^=m3u8]" +
		"/b" + h + "[protocol^=m3u8]" +
		"/bv*" + h + "[vcodec!*=av01]+ba/b" + h + "/b"
}

// Size is a window size in logical (96 DPI) pixels.
type Size struct{ W, H int }

var Sizes = map[string]Size{
	"small":  {320, 180},
	"medium": {480, 270},
	"large":  {640, 360},
}

var ErrNotRunning = errors.New("Nothing is playing right now.")

type session struct {
	cmd  *exec.Cmd
	job  windows.Handle
	pipe string
	tail *tailBuffer

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{} // closed after the process has exited

	ipcReady chan struct{} // closed once ipc is set (or connecting failed)
	ipc      *ipcClient

	// guarded by Manager.mu
	stopping bool
	lastErr  string
	lastLog  string
	hwnd     windows.HWND
}

type Manager struct {
	bins    Binaries
	notify  func(State)
	changed chan struct{}

	mu      sync.Mutex
	state   State
	sess    *session
	pipeSeq int
	closed  bool
	// last window bounds in physical pixels, reused for the next start
	lastPos  *[2]int32
	lastSize *[2]int32

	prefsPath    string
	saveMu       sync.Mutex
	saveTimer    *time.Timer
	savedPrefs   Prefs
	pendingPrefs *Prefs

	hotkeys *hotkeyLoop
}

// NewManager creates a manager with the preferences stored at prefsPath ("" to
// keep nothing); notify receives coalesced state snapshots on a dedicated
// goroutine.
func NewManager(bins Binaries, prefsPath string, notify func(State)) *Manager {
	p := loadPrefs(prefsPath)
	m := &Manager{
		bins:    bins,
		notify:  notify,
		changed: make(chan struct{}, 1),
		state: State{
			Status: StatusIdle, Speed: 1,
			Volume: p.Volume, Muted: p.Muted, OnTop: p.OnTop, Size: p.Size,
			Quality: p.Quality, Opacity: p.Opacity, Recent: p.Recent,
		},
		lastPos:    p.WindowPos,
		lastSize:   p.WindowSize,
		prefsPath:  prefsPath,
		savedPrefs: p,
	}
	go m.notifyLoop()
	return m
}

func (m *Manager) notifyLoop() {
	for range m.changed {
		m.mu.Lock()
		st := m.state
		p := m.prefsLocked()
		m.mu.Unlock()
		if m.notify != nil {
			m.notify(st)
		}
		m.queueSave(p)
	}
}

func (m *Manager) touch() {
	select {
	case m.changed <- struct{}{}:
	default:
	}
}

func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

func (m *Manager) Binaries() Binaries { return m.bins }

// Play starts mpv, or switches the stream of the running player in place so
// that there is never more than one mpv process.
func (m *Manager) Play(src Source) error {
	if m.bins.MPV == "" {
		return errors.New("mpv was not found. Please install mpv or set PERCH_MPV.")
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return errors.New("The application is shutting down.")
	}
	if s := m.sess; s != nil && s.stopping {
		// A concurrent Stop is in progress; let it finish first.
		m.mu.Unlock()
		select {
		case <-s.done:
		case <-time.After(6 * time.Second):
			return errors.New("The previous player is not responding.")
		}
		m.mu.Lock()
	}
	if s := m.sess; s != nil {
		m.mu.Unlock()
		return m.switchStream(s, src)
	}
	defer m.mu.Unlock()
	return m.spawnLocked(src)
}

func (m *Manager) switchStream(s *session, src Source) error {
	c, err := waitIPC(s)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.state.Status, m.state.URL, m.state.Kind = StatusStarting, src.URL, src.Kind
	m.state.Error = ""
	m.state.resetMedia()
	s.lastErr, s.lastLog = "", ""
	m.mu.Unlock()
	m.touch()

	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()
	if _, err := c.command(ctx, "loadfile", src.URL, "replace"); err != nil {
		return fmt.Errorf("Could not switch stream: %w", err)
	}
	return nil
}

func (m *Manager) spawnLocked(src Source) error {
	m.pipeSeq++
	pipe := fmt.Sprintf(`\\.\pipe\perch-mpv-%d-%d`, os.Getpid(), m.pipeSeq)

	cmd := exec.Command(m.bins.MPV, m.argsLocked(src, pipe)...)
	cmd.Env = m.childEnv()
	tail := &tailBuffer{max: 8 << 10}
	cmd.Stdout, cmd.Stderr = tail, tail
	// Don't let a grandchild that inherited the output pipes delay Wait.
	cmd.WaitDelay = 2 * time.Second

	job, jobErr := newKillOnCloseJob()
	if err := cmd.Start(); err != nil {
		if jobErr == nil {
			windows.CloseHandle(job)
		}
		return fmt.Errorf("Could not start mpv: %w", err)
	}
	if jobErr == nil {
		if err := assignToJob(job, cmd.Process.Pid); err != nil {
			windows.CloseHandle(job)
			job = 0
		}
	} else {
		job = 0
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &session{
		cmd: cmd, job: job, pipe: pipe, tail: tail,
		ctx: ctx, cancel: cancel,
		done: make(chan struct{}), ipcReady: make(chan struct{}),
	}
	m.sess = s
	m.state.Status, m.state.URL, m.state.Kind = StatusStarting, src.URL, src.Kind
	m.state.Error = ""
	m.state.resetMedia()
	m.state.Speed, m.state.ClickThrough, m.state.Hidden = 1, false, false
	m.touch()

	go m.waitExit(s)
	go m.connect(s)
	go m.adoptWindow(s)
	return nil
}

// adoptWindow waits for mpv's window, then shows it under Perch's taskbar
// icon (shared AppID) and with Perch's window icon.
func (m *Manager) adoptWindow(s *session) {
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	deadline := time.After(15 * time.Second)
	for {
		if hwnd := m.window(s); hwnd != 0 {
			_ = setWindowAppID(hwnd, AppID)
			copyAppIcon(hwnd)
			if st := m.State(); st.Opacity < 100 {
				_ = setTransparency(hwnd, st.Opacity, false)
			}
			return
		}
		select {
		case <-tick.C:
		case <-s.done:
			return
		case <-deadline:
			return
		}
	}
}

func (m *Manager) argsLocked(src Source, pipe string) []string {
	dpi := systemDPI()
	size := Sizes[m.state.Size]
	w, h := size.W*dpi/96, size.H*dpi/96
	if m.lastSize != nil {
		w, h = int(m.lastSize[0]), int(m.lastSize[1])
	}
	pos := fmt.Sprintf("+%d+%d", 40*dpi/96, 40*dpi/96)
	if m.lastPos != nil && m.lastPos[0] >= 0 && m.lastPos[1] >= 0 {
		pos = fmt.Sprintf("+%d+%d", m.lastPos[0], m.lastPos[1])
	}

	scriptOpts := []string{
		"osc-layout=bottombar",
		"osc-deadzonesize=0",
		"osc-minmousemove=1",
		"osc-hidetimeout=1200",
		"osc-fadeduration=150",
		"osc-windowcontrols=auto",
	}
	if p := m.bins.YTDLP; p != "" && !strings.ContainsAny(p, ",=") {
		scriptOpts = append(scriptOpts, "ytdl_hook-ytdl_path="+p)
	}

	onTop := "--no-ontop"
	if m.state.OnTop {
		onTop = "--ontop"
	}
	mute := "--mute=no"
	if m.state.Muted {
		mute = "--mute=yes"
	}

	return []string{
		"--no-config",
		onTop,
		"--border=no",
		"--force-window=yes",
		"--idle=no",
		"--keep-open=no",
		fmt.Sprintf("--geometry=%dx%d%s", w, h, pos),
		"--auto-window-resize=no",
		"--keepaspect-window=yes",
		"--snap-window=yes",
		"--cursor-autohide=1000",
		"--title=${media-title} – Perch",
		// Hardware decoding and a capped resolution keep CPU/RAM low,
		// which matters for an x64 mpv running under ARM64 emulation.
		"--hwdec=auto-safe",
		// The flip presentation model shows nothing in a layered
		// (semi-transparent) window, so the opacity setting needs bitblt.
		"--d3d11-flip=no",
		"--ytdl-format=" + ytdlFormat(m.state.Quality),
		"--demuxer-max-bytes=64MiB",
		"--demuxer-max-back-bytes=16MiB",
		"--volume=" + strconv.Itoa(m.state.Volume),
		mute,
		"--osc=yes",
		"--script-opts=" + strings.Join(scriptOpts, ","),
		"--msg-level=all=error",
		"--input-ipc-server=" + pipe,
		"--",
		src.URL,
	}
}

// childEnv puts yt-dlp's folder in front of PATH as a fallback for mpv's
// ytdl_hook lookup.
func (m *Manager) childEnv() []string {
	env := os.Environ()
	if m.bins.YTDLP == "" {
		return env
	}
	dir := filepath.Dir(m.bins.YTDLP)
	for i, kv := range env {
		if len(kv) > 5 && strings.EqualFold(kv[:5], "PATH=") {
			env[i] = kv[:5] + dir + string(os.PathListSeparator) + kv[5:]
			return env
		}
	}
	return append(env, "PATH="+dir)
}

func (m *Manager) connect(s *session) {
	ctx, cancel := context.WithTimeout(s.ctx, 15*time.Second)
	defer cancel()
	c, err := dialIPC(ctx, s.pipe, func(msg ipcMessage) { m.handleEvent(s, msg) })
	if err != nil {
		close(s.ipcReady)
		select {
		case <-s.done:
		default:
			// Without IPC the player can't be controlled; don't leave it behind.
			m.mu.Lock()
			s.lastErr = "Could not establish a control connection to mpv."
			m.mu.Unlock()
			_ = s.cmd.Process.Kill()
		}
		return
	}
	s.ipc = c
	close(s.ipcReady)

	setup := [][]any{
		{"request_log_messages", "error"},
		{"observe_property", 1, "pause"},
		{"observe_property", 2, "volume"},
		{"observe_property", 3, "mute"},
		{"observe_property", 4, "ontop"},
		{"observe_property", 5, "media-title"},
		{"observe_property", 6, "paused-for-cache"},
		{"observe_property", 7, "duration"},
		{"observe_property", 8, "seekable"},
		{"observe_property", 9, "time-pos"},
		{"observe_property", 10, "speed"},
		{"observe_property", 11, "track-list"},
		{"observe_property", 12, "sid"},
	}
	for _, cmd := range setup {
		cctx, ccancel := context.WithTimeout(s.ctx, 2*time.Second)
		_, err := c.command(cctx, cmd...)
		ccancel()
		if errors.Is(err, errIPCClosed) {
			return
		}
	}
}

func (m *Manager) handleEvent(s *session, msg ipcMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sess != s {
		return
	}
	st := &m.state
	switch msg.Event {
	case "property-change":
		switch msg.Name {
		case "pause":
			_ = json.Unmarshal(msg.Data, &st.Paused)
		case "volume":
			var v float64
			if json.Unmarshal(msg.Data, &v) == nil {
				st.Volume = int(v + 0.5)
			}
		case "mute":
			_ = json.Unmarshal(msg.Data, &st.Muted)
		case "ontop":
			_ = json.Unmarshal(msg.Data, &st.OnTop)
		case "media-title":
			var t string
			if json.Unmarshal(msg.Data, &t) == nil {
				st.Title = t
				if st.Status == StatusPlaying && len(st.Recent) > 0 && st.Recent[0].URL == st.URL {
					m.addRecentLocked(RecentItem{URL: st.URL, Kind: st.Kind, Title: t})
				}
			}
		case "paused-for-cache":
			_ = json.Unmarshal(msg.Data, &st.Buffering)
		case "duration":
			st.Duration = 0 // stays 0 (null) for live streams
			_ = json.Unmarshal(msg.Data, &st.Duration)
		case "seekable":
			_ = json.Unmarshal(msg.Data, &st.Seekable)
		case "time-pos":
			var pos float64
			_ = json.Unmarshal(msg.Data, &pos)
			sameSecond := int(pos) == int(st.Position)
			st.Position = pos
			if sameSecond {
				return // mpv reports every frame; the UI needs whole seconds
			}
		case "speed":
			_ = json.Unmarshal(msg.Data, &st.Speed)
		case "track-list":
			st.SubTracks = parseSubTracks(msg.Data)
		case "sid":
			var id float64 // false or "no" when off
			st.Sub = 0
			if json.Unmarshal(msg.Data, &id) == nil {
				st.Sub = int(id)
			}
		default:
			return
		}
	case "start-file":
		if !s.stopping {
			st.Status = StatusStarting
		}
	case "file-loaded":
		if !s.stopping {
			st.Status, st.Error = StatusPlaying, ""
			s.lastErr = ""
			m.addRecentLocked(RecentItem{URL: st.URL, Kind: st.Kind, Title: st.Title})
			if st.Kind == KindYouTube || st.Kind == KindTwitch {
				// Not on the reader goroutine: it has to receive the reply.
				go m.detectLive(s, st.URL)
			}
		}
	case "end-file":
		if msg.Reason != "error" {
			return
		}
		s.lastErr = friendlyLoadError(msg.FileError, s.lastLog)
		st.Error = s.lastErr
	case "log-message":
		text := strings.TrimSpace(msg.Text)
		// Keep the specific cause (yt-dlp "ERROR: ...", HTTP 403) instead of
		// the generic summary mpv logs after it.
		if (msg.Level == "error" || msg.Level == "fatal") && text != "" &&
			!(isGenericError(text) && s.lastLog != "") {
			s.lastLog = text
		}
		return
	default:
		return
	}
	m.touch()
}

func isGenericError(text string) bool {
	return strings.HasPrefix(text, "youtube-dl failed") ||
		strings.HasPrefix(text, "No video or audio streams selected")
}

var ytdlpPrefix = regexp.MustCompile(`^(?:ERROR:\s*)?(?:\[[^\]]+\]\s*[^:\s]*:\s*)?`)

// detectLive reads yt-dlp's JSON result, which mpv's ytdl_hook keeps in
// user-data, and checks its is_live flag.
func (m *Manager) detectLive(s *session, url string) {
	c, err := waitIPCTimeout(s, 2*time.Second)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()
	data, err := c.command(ctx, "get_property", "user-data/mpv/ytdl/json-subprocess-result")
	if err != nil {
		return
	}
	var result struct {
		Stdout string `json:"stdout"`
	}
	var info struct {
		IsLive bool `json:"is_live"`
	}
	if json.Unmarshal(data, &result) != nil || json.Unmarshal([]byte(result.Stdout), &info) != nil {
		return
	}
	m.mu.Lock()
	if m.sess == s && m.state.URL == url && m.state.Live != info.IsLive {
		m.state.Live = info.IsLive
		m.touch()
	}
	m.mu.Unlock()
}

func friendlyLoadError(fileErr, log string) string {
	const msg = "Could not load the stream"
	lower := strings.ToLower(log)
	switch {
	case strings.Contains(lower, "failed to resolve") || strings.Contains(lower, "getaddrinfo"):
		return msg + ": server not found. Check the address and your internet connection."
	case strings.Contains(lower, "is offline") || strings.Contains(lower, "not currently live"):
		return "This channel is not live right now."
	case strings.Contains(lower, "video unavailable") || strings.Contains(lower, "private video"):
		return "This video is unavailable or private."
	case strings.Contains(lower, "http error 403") || strings.Contains(lower, "http error 401"):
		return msg + ": the server denied access (HTTP 403). " +
			"For YouTube, updating yt-dlp usually helps: winget upgrade yt-dlp"
	case strings.Contains(lower, "http error 404"):
		return msg + ": not found (HTTP 404)."
	case log != "":
		detail := ytdlpPrefix.ReplaceAllString(log, "")
		if i := strings.Index(detail, " (caused by"); i > 0 {
			detail = detail[:i]
		}
		if r := []rune(detail); len(r) > 160 {
			detail = string(r[:160]) + "…"
		}
		return msg + ": " + detail
	case fileErr != "":
		return msg + " (" + fileErr + ")."
	}
	return msg + "."
}

func (m *Manager) waitExit(s *session) {
	_ = s.cmd.Wait()
	close(s.done)
	s.cancel()
	<-s.ipcReady
	if s.ipc != nil {
		s.ipc.Close()
	}
	if s.job != 0 {
		// Kills anything mpv left behind (e.g. a running yt-dlp).
		windows.CloseHandle(s.job)
	}

	code := -1
	if s.cmd.ProcessState != nil {
		code = s.cmd.ProcessState.ExitCode()
	}
	m.mu.Lock()
	if m.sess == s {
		m.sess = nil
		st := &m.state
		title := st.Title
		st.resetMedia()
		st.Title = title
		st.ClickThrough, st.Hidden = false, false
		if s.stopping || code == 0 {
			st.Status, st.Error = StatusIdle, ""
		} else {
			st.Status = StatusError
			switch {
			case s.lastErr != "":
				st.Error = s.lastErr
			case s.tail.lastError() != "":
				st.Error = "mpv: " + s.tail.lastError()
			default:
				st.Error = fmt.Sprintf("Playback stopped unexpectedly (code %d).", code)
			}
		}
	}
	m.mu.Unlock()
	m.touch()
}

// Stop ends playback and waits until the mpv process is gone.
func (m *Manager) Stop() error {
	m.mu.Lock()
	s := m.sess
	if s == nil {
		m.mu.Unlock()
		return nil
	}
	alreadyStopping := s.stopping
	s.stopping = true
	m.state.Status = StatusStopping
	m.mu.Unlock()
	m.touch()

	if !alreadyStopping {
		m.rememberWindow(s)
		if c, err := waitIPCTimeout(s, time.Second); err == nil {
			ctx, cancel := context.WithTimeout(s.ctx, 1500*time.Millisecond)
			_, _ = c.command(ctx, "quit")
			cancel()
		}
	}

	select {
	case <-s.done:
		return nil
	case <-time.After(3 * time.Second):
	}
	_ = s.cmd.Process.Kill()
	select {
	case <-s.done:
		return nil
	case <-time.After(3 * time.Second):
		return errors.New("Could not stop mpv.")
	}
}

// Shutdown stops the player for good; later Play calls fail.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	m.stopHotkeys()
	_ = m.Stop()

	m.mu.Lock()
	p := m.prefsLocked()
	m.mu.Unlock()
	m.queueSave(p)
	m.flushPrefs()
}

func (m *Manager) rememberWindow(s *session) {
	hwnd := m.window(s)
	if hwnd == 0 {
		return
	}
	r, err := windowRect(hwnd)
	if err != nil {
		return
	}
	m.mu.Lock()
	if r.Left >= 0 && r.Top >= 0 {
		m.lastPos = &[2]int32{r.Left, r.Top}
	}
	m.lastSize = &[2]int32{r.Right - r.Left, r.Bottom - r.Top}
	m.mu.Unlock()
	m.touch() // persists the new bounds
}

func (m *Manager) window(s *session) windows.HWND {
	m.mu.Lock()
	hwnd := s.hwnd
	m.mu.Unlock()
	if hwnd != 0 && isWindow(hwnd) {
		return hwnd
	}
	hwnd = findPlayerWindow(s.cmd.Process.Pid)
	m.mu.Lock()
	s.hwnd = hwnd
	m.mu.Unlock()
	return hwnd
}

func waitIPC(s *session) (*ipcClient, error) { return waitIPCTimeout(s, 10*time.Second) }

func waitIPCTimeout(s *session, d time.Duration) (*ipcClient, error) {
	select {
	case <-s.ipcReady:
	case <-s.done:
		return nil, ErrNotRunning
	case <-time.After(d):
		return nil, errors.New("mpv is not ready yet. Please wait a moment.")
	}
	if s.ipc == nil {
		return nil, errors.New("No control connection to mpv.")
	}
	return s.ipc, nil
}

// running returns the active session, or nil if none is controllable.
func (m *Manager) running() *session {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sess == nil || m.sess.stopping {
		return nil
	}
	return m.sess
}

func (m *Manager) setProperty(name string, value any) error {
	s := m.running()
	if s == nil {
		return nil // stored as preference only
	}
	c, err := waitIPC(s)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
	defer cancel()
	_, err = c.command(ctx, "set_property", name, value)
	return err
}

func (m *Manager) SetPaused(paused bool) error {
	if m.running() == nil {
		return ErrNotRunning
	}
	return m.setProperty("pause", paused)
}

func (m *Manager) TogglePause() error {
	return m.SetPaused(!m.State().Paused)
}

// Seek jumps to an absolute position in seconds.
func (m *Manager) Seek(seconds float64) error {
	return m.seek(max(seconds, 0), "absolute")
}

// SeekRelative skips forward (positive) or backward (negative).
func (m *Manager) SeekRelative(delta float64) error {
	// exact: otherwise mpv snaps to the next keyframe, often several seconds off
	return m.seek(delta, "relative+exact")
}

func (m *Manager) seek(value float64, mode string) error {
	s := m.running()
	if s == nil {
		return ErrNotRunning
	}
	if !m.State().Seekable {
		return errors.New("Seeking is not possible in this stream.")
	}
	c, err := waitIPC(s)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
	defer cancel()
	_, err = c.command(ctx, "seek", value, mode)
	return err
}

func (m *Manager) SetVolume(v int) error {
	v = max(0, min(v, 130))
	m.mu.Lock()
	m.state.Volume = v
	m.mu.Unlock()
	m.touch()
	return m.setProperty("volume", v)
}

func (m *Manager) SetMuted(muted bool) error {
	m.mu.Lock()
	m.state.Muted = muted
	m.mu.Unlock()
	m.touch()
	return m.setProperty("mute", muted)
}

func (m *Manager) SetOnTop(onTop bool) error {
	m.mu.Lock()
	m.state.OnTop = onTop
	m.mu.Unlock()
	m.touch()
	return m.setProperty("ontop", onTop)
}

// SetSize applies a preset size; the user can still resize freely afterwards.
func (m *Manager) SetSize(name string) error {
	size, ok := Sizes[name]
	if !ok {
		return errors.New("Unknown player size.")
	}
	m.mu.Lock()
	m.state.Size = name
	m.lastSize = nil
	m.mu.Unlock()
	m.touch()

	s := m.running()
	if s == nil {
		return nil
	}
	hwnd := m.window(s)
	if hwnd == 0 {
		return errors.New("The player window was not found.")
	}
	if err := resizeWindow(hwnd, size.W, size.H); err != nil {
		return fmt.Errorf("Could not resize the window: %w", err)
	}
	return nil
}

func parseSubTracks(data []byte) []SubTrack {
	var tracks []struct {
		ID    int    `json:"id"`
		Type  string `json:"type"`
		Lang  string `json:"lang"`
		Title string `json:"title"`
	}
	if json.Unmarshal(data, &tracks) != nil {
		return nil
	}
	var subs []SubTrack
	for _, t := range tracks {
		if t.Type != "sub" {
			continue
		}
		label := t.Title
		switch {
		case label == "" && t.Lang != "":
			label = t.Lang
		case label == "":
			label = fmt.Sprintf("Track %d", t.ID)
		case t.Lang != "" && !strings.Contains(strings.ToLower(label), strings.ToLower(t.Lang)):
			label += " (" + t.Lang + ")"
		}
		subs = append(subs, SubTrack{ID: t.ID, Label: label})
	}
	return subs
}

// command sends one command to the running player.
func (m *Manager) command(args ...any) error {
	s := m.running()
	if s == nil {
		return ErrNotRunning
	}
	c, err := waitIPC(s)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()
	_, err = c.command(ctx, args...)
	return err
}

// SetSpeed sets the playback speed (0.25x to 4x).
func (m *Manager) SetSpeed(speed float64) error {
	return m.command("set_property", "speed", max(0.25, min(speed, 4)))
}

// SetSubtitle selects a subtitle track; 0 turns subtitles off.
func (m *Manager) SetSubtitle(id int) error {
	if id <= 0 {
		return m.command("set_property", "sid", "no")
	}
	return m.command("set_property", "sid", id)
}

// SetQuality changes the maximum resolution for YouTube and Twitch. A
// running stream is reloaded at its current position.
func (m *Manager) SetQuality(quality string) error {
	if _, ok := qualityHeights[quality]; !ok {
		return errors.New("Unknown quality.")
	}
	m.mu.Lock()
	m.state.Quality = quality
	st := m.state
	m.mu.Unlock()
	m.touch()

	if m.running() == nil {
		return nil
	}
	if err := m.command("set_property", "ytdl-format", ytdlFormat(quality)); err != nil {
		return err
	}
	if st.Kind != KindYouTube && st.Kind != KindTwitch || st.Status != StatusPlaying {
		return nil
	}
	opts := ""
	if !st.Live && st.Duration > 0 && st.Position > 1 {
		opts = fmt.Sprintf("start=%.1f", st.Position)
	}
	return m.command("loadfile", st.URL, "replace", -1, opts)
}

// SetOpacity sets the video window's opacity in percent.
func (m *Manager) SetOpacity(percent int) error {
	percent = max(minOpacity, min(percent, 100))
	m.mu.Lock()
	m.state.Opacity = percent
	click := m.state.ClickThrough
	m.mu.Unlock()
	m.touch()
	return m.applyWindowStyle(percent, click)
}

// SetClickThrough lets mouse clicks pass through the video to the window
// behind it.
func (m *Manager) SetClickThrough(on bool) error {
	if m.running() == nil {
		return ErrNotRunning
	}
	m.mu.Lock()
	m.state.ClickThrough = on
	opacity := m.state.Opacity
	m.mu.Unlock()
	m.touch()
	return m.applyWindowStyle(opacity, on)
}

func (m *Manager) applyWindowStyle(opacity int, clickThrough bool) error {
	s := m.running()
	if s == nil {
		return nil
	}
	hwnd := m.window(s)
	if hwnd == 0 {
		return errors.New("The player window was not found.")
	}
	return setTransparency(hwnd, opacity, clickThrough)
}

// SetHidden hides or shows the video window; playback continues.
func (m *Manager) SetHidden(hidden bool) error {
	s := m.running()
	if s == nil {
		return ErrNotRunning
	}
	hwnd := m.window(s)
	if hwnd == 0 {
		return errors.New("The player window was not found.")
	}
	showWindow(hwnd, !hidden)
	m.mu.Lock()
	m.state.Hidden = hidden
	m.mu.Unlock()
	m.touch()
	return nil
}

// SnapTo moves the video window into a screen corner: "tl", "tr", "bl", "br".
func (m *Manager) SnapTo(corner string) error {
	switch corner {
	case "tl", "tr", "bl", "br":
	default:
		return errors.New("Unknown corner.")
	}
	s := m.running()
	if s == nil {
		return ErrNotRunning
	}
	hwnd := m.window(s)
	if hwnd == 0 {
		return errors.New("The player window was not found.")
	}
	return snapWindow(hwnd, corner)
}

// tailBuffer keeps the last bytes of mpv's console output for diagnostics.
type tailBuffer struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if over := len(t.buf) - t.max; over > 0 {
		t.buf = append(t.buf[:0], t.buf[over:]...)
	}
	return len(p), nil
}

func (t *tailBuffer) lastError() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	lines := strings.Split(strings.ReplaceAll(string(t.buf), "\r", "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if ll := strings.ToLower(l); strings.Contains(ll, "error") || strings.Contains(ll, "failed") {
			return l
		}
	}
	return ""
}
