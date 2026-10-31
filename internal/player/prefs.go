package player

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"time"
)

const maxRecent = 10

// RecentItem is one entry of the "recently played" list.
type RecentItem struct {
	URL   string `json:"url"`
	Kind  string `json:"kind"`
	Title string `json:"title"`
}

// Prefs is what survives a restart, stored as JSON in the user's
// AppData folder.
type Prefs struct {
	Volume     int          `json:"volume"`
	Muted      bool         `json:"muted"`
	OnTop      bool         `json:"onTop"`
	Size       string       `json:"size"`
	Quality    string       `json:"quality"`
	Opacity    int          `json:"opacity"`
	WindowPos  *[2]int32    `json:"windowPos,omitempty"`
	WindowSize *[2]int32    `json:"windowSize,omitempty"`
	Recent     []RecentItem `json:"recent"`
}

func defaultPrefs() Prefs {
	return Prefs{Volume: 80, OnTop: true, Size: "medium", Quality: "720", Opacity: 100}
}

// DefaultPrefsPath returns %APPDATA%\Perch\settings.json.
func DefaultPrefsPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "Perch", "settings.json")
}

// loadPrefs falls back to defaults for a missing or damaged file and for
// out-of-range values.
func loadPrefs(path string) Prefs {
	p := defaultPrefs()
	if path == "" {
		return p
	}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &p)
	}
	def := defaultPrefs()
	if p.Volume < 0 || p.Volume > 130 {
		p.Volume = def.Volume
	}
	if _, ok := Sizes[p.Size]; !ok {
		p.Size = def.Size
	}
	if _, ok := qualityHeights[p.Quality]; !ok {
		p.Quality = def.Quality
	}
	if p.Opacity < minOpacity || p.Opacity > 100 {
		p.Opacity = def.Opacity
	}
	if p.WindowPos != nil && !onScreen(p.WindowPos[0], p.WindowPos[1]) {
		p.WindowPos = nil // the monitor it was on is gone
	}
	if p.WindowSize != nil && (p.WindowSize[0] < 100 || p.WindowSize[1] < 60) {
		p.WindowSize = nil
	}
	if len(p.Recent) > maxRecent {
		p.Recent = p.Recent[:maxRecent]
	}
	return p
}

// writePrefs replaces the file atomically so a crash never leaves it half
// written.
func writePrefs(path string, p Prefs) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (m *Manager) prefsLocked() Prefs {
	st := m.state
	return Prefs{
		Volume: st.Volume, Muted: st.Muted, OnTop: st.OnTop, Size: st.Size,
		Quality: st.Quality, Opacity: st.Opacity,
		WindowPos: m.lastPos, WindowSize: m.lastSize,
		Recent: st.Recent,
	}
}

// queueSave writes changed preferences after a short pause, so dragging the
// volume slider doesn't write the file dozens of times.
func (m *Manager) queueSave(p Prefs) {
	m.saveMu.Lock()
	defer m.saveMu.Unlock()
	if reflect.DeepEqual(p, m.savedPrefs) {
		m.pendingPrefs = nil
		return
	}
	m.pendingPrefs = &p
	if m.saveTimer == nil {
		m.saveTimer = time.AfterFunc(500*time.Millisecond, m.flushPrefs)
	} else {
		m.saveTimer.Reset(500 * time.Millisecond)
	}
}

func (m *Manager) flushPrefs() {
	m.saveMu.Lock()
	defer m.saveMu.Unlock()
	if m.pendingPrefs == nil {
		return
	}
	if writePrefs(m.prefsPath, *m.pendingPrefs) == nil {
		m.savedPrefs = *m.pendingPrefs
	}
	m.pendingPrefs = nil
}

// addRecentLocked puts the item first, dropping an older entry for the same
// URL. The slice is replaced, never modified, because State copies share it.
func (m *Manager) addRecentLocked(item RecentItem) {
	list := []RecentItem{item}
	for _, r := range m.state.Recent {
		if r.URL != item.URL && len(list) < maxRecent {
			list = append(list, r)
		}
	}
	m.state.Recent = list
}

// RemoveRecent deletes one entry; an empty URL clears the whole list.
func (m *Manager) RemoveRecent(url string) {
	m.mu.Lock()
	var list []RecentItem
	if url != "" {
		for _, r := range m.state.Recent {
			if r.URL != url {
				list = append(list, r)
			}
		}
	}
	m.state.Recent = list
	m.mu.Unlock()
	m.touch()
}
