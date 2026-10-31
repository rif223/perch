package main

import (
	"context"
	"errors"

	"perch/internal/player"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// stateEvent carries player.State snapshots to the frontend.
const stateEvent = "player:state"

// App exposes the player to the frontend. All methods are safe for
// concurrent calls; Wails invokes bindings on separate goroutines.
type App struct {
	ctx     context.Context
	player  *player.Manager
	hotkeys []player.Hotkey
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.player = player.NewManager(player.LocateBinaries(), player.DefaultPrefsPath(), func(st player.State) {
		runtime.EventsEmit(ctx, stateEvent, st)
	})
	a.hotkeys = a.player.StartHotkeys()
}

// shutdown makes sure no mpv process outlives the application.
func (a *App) shutdown(ctx context.Context) {
	if a.player != nil {
		a.player.Shutdown()
	}
}

func (a *App) onSecondInstance() {
	if a.ctx == nil {
		return
	}
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
}

// Environment reports the resolved tool paths for the UI.
func (a *App) Environment() player.Binaries {
	return a.player.Binaries()
}

func (a *App) GetState() player.State {
	return a.player.State()
}

// ValidateURL checks input without starting playback.
func (a *App) ValidateURL(raw string) (player.Source, error) {
	return player.ParseSource(raw)
}

// StartStream starts the player or switches the running one to a new source.
func (a *App) StartStream(raw string) error {
	src, err := player.ParseSource(raw)
	if err != nil {
		return err
	}
	if src.NeedsYTDLP() && a.player.Binaries().YTDLP == "" {
		return errors.New("yt-dlp was not found – it is required for YouTube and Twitch.")
	}
	return a.player.Play(src)
}

func (a *App) StopStream() error {
	return a.player.Stop()
}

func (a *App) TogglePause() error {
	return a.player.TogglePause()
}

// Seek jumps to an absolute position in seconds (videos only).
func (a *App) Seek(seconds float64) error {
	return a.player.Seek(seconds)
}

// SeekRelative skips by the given number of seconds.
func (a *App) SeekRelative(delta float64) error {
	return a.player.SeekRelative(delta)
}

// OpenVideoFile shows the Windows file picker and returns the chosen path,
// or "" if the dialog was cancelled.
func (a *App) OpenVideoFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Open video",
		Filters: []runtime.FileFilter{
			{DisplayName: "Videos", Pattern: "*.mp4;*.mkv;*.webm;*.mov;*.m4v;*.avi;*.wmv;*.flv;*.ts;*.m2ts;*.mpg;*.mpeg;*.3gp;*.ogv"},
			{DisplayName: "Audio", Pattern: "*.mp3;*.m4a;*.aac;*.flac;*.wav;*.ogg;*.opus"},
			{DisplayName: "Playlists", Pattern: "*.m3u;*.m3u8;*.pls"},
			{DisplayName: "All files", Pattern: "*.*"},
		},
	})
}

func (a *App) SetVolume(volume int) error {
	return a.player.SetVolume(volume)
}

func (a *App) SetMuted(muted bool) error {
	return a.player.SetMuted(muted)
}

func (a *App) SetAlwaysOnTop(enabled bool) error {
	return a.player.SetOnTop(enabled)
}

// SetPlayerSize accepts "small", "medium" or "large".
func (a *App) SetPlayerSize(size string) error {
	return a.player.SetSize(size)
}

// SnapPlayer moves the video window into a corner: "tl", "tr", "bl", "br".
func (a *App) SnapPlayer(corner string) error {
	return a.player.SnapTo(corner)
}

// SetOpacity sets the video window's opacity in percent (20–100).
func (a *App) SetOpacity(percent int) error {
	return a.player.SetOpacity(percent)
}

// SetClickThrough lets clicks pass through the video window.
func (a *App) SetClickThrough(enabled bool) error {
	return a.player.SetClickThrough(enabled)
}

// SetHidden hides or shows the video window while playback continues.
func (a *App) SetHidden(hidden bool) error {
	return a.player.SetHidden(hidden)
}

// SetQuality accepts "480", "720", "1080" or "best".
func (a *App) SetQuality(quality string) error {
	return a.player.SetQuality(quality)
}

func (a *App) SetSpeed(speed float64) error {
	return a.player.SetSpeed(speed)
}

// SetSubtitle selects a subtitle track id; 0 turns subtitles off.
func (a *App) SetSubtitle(id int) error {
	return a.player.SetSubtitle(id)
}

// RemoveRecent deletes one history entry; "" clears the list.
func (a *App) RemoveRecent(url string) {
	a.player.RemoveRecent(url)
}

// Hotkeys lists the global shortcuts and whether they could be registered.
func (a *App) Hotkeys() []player.Hotkey {
	return a.hotkeys
}

func (a *App) YTDLPVersion() string {
	return a.player.YTDLPVersion()
}

// UpdateYTDLP updates yt-dlp and returns a short result message.
func (a *App) UpdateYTDLP() (string, error) {
	return a.player.UpdateYTDLP()
}
