package player

import (
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procRegisterHotKey     = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey   = user32.NewProc("UnregisterHotKey")
	procGetMessageW        = user32.NewProc("GetMessageW")
	procPostThreadMessageW = user32.NewProc("PostThreadMessageW")
)

const (
	modAlt      = 0x0001
	modControl  = 0x0002
	modNoRepeat = 0x4000
	wmHotkey    = 0x0312
	wmQuit      = 0x0012
)

// Hotkey describes a global shortcut for the UI. Active is false when
// another application already owns the combination.
type Hotkey struct {
	Keys   string `json:"keys"`
	Action string `json:"action"`
	Active bool   `json:"active"`
}

type hotkeyDef struct {
	keys, action string
	vk           uintptr
	run          func(m *Manager)
}

// All shortcuts use Ctrl+Alt so they rarely collide with other apps.
var hotkeyDefs = []hotkeyDef{
	{"Ctrl+Alt+P", "Pause", 0x50, func(m *Manager) { _ = m.TogglePause() }},
	{"Ctrl+Alt+M", "Mute", 0x4D, func(m *Manager) { _ = m.SetMuted(!m.State().Muted) }},
	{"Ctrl+Alt+H", "Hide", 0x48, func(m *Manager) { _ = m.SetHidden(!m.State().Hidden) }},
	{"Ctrl+Alt+T", "Click-through", 0x54, func(m *Manager) { _ = m.SetClickThrough(!m.State().ClickThrough) }},
	{"Ctrl+Alt+←", "−10 s", 0x25, func(m *Manager) { _ = m.SeekRelative(-10) }},
	{"Ctrl+Alt+→", "+10 s", 0x27, func(m *Manager) { _ = m.SeekRelative(10) }},
}

// hotkeyLoop is the OS thread that owns the registrations; WM_HOTKEY is
// posted to that thread's message queue.
type hotkeyLoop struct {
	threadID uint32
	done     chan struct{}
}

type winMsg struct {
	hwnd     uintptr
	message  uint32
	wParam   uintptr
	lParam   uintptr
	time     uint32
	pt       [2]int32
	lPrivate uint32
}

// StartHotkeys registers the global shortcuts and reports which ones are
// available.
func (m *Manager) StartHotkeys() []Hotkey {
	result := make([]Hotkey, len(hotkeyDefs))
	loop := &hotkeyLoop{done: make(chan struct{})}
	ready := make(chan struct{})
	go func() {
		// Registrations belong to this thread; it ends with the goroutine.
		runtime.LockOSThread()
		loop.threadID = windows.GetCurrentThreadId()
		for i, d := range hotkeyDefs {
			ok, _, _ := procRegisterHotKey.Call(0, uintptr(i+1), modControl|modAlt|modNoRepeat, d.vk)
			result[i] = Hotkey{Keys: d.keys, Action: d.action, Active: ok != 0}
		}
		close(ready)

		var msg winMsg
		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if int32(r) <= 0 { // WM_QUIT or error
				break
			}
			if i := int(msg.wParam) - 1; msg.message == wmHotkey && i >= 0 && i < len(hotkeyDefs) {
				go hotkeyDefs[i].run(m)
			}
		}
		for i := range hotkeyDefs {
			procUnregisterHotKey.Call(0, uintptr(i+1))
		}
		close(loop.done)
	}()
	<-ready

	m.mu.Lock()
	m.hotkeys = loop
	m.mu.Unlock()
	return result
}

func (m *Manager) stopHotkeys() {
	m.mu.Lock()
	loop := m.hotkeys
	m.hotkeys = nil
	m.mu.Unlock()
	if loop == nil {
		return
	}
	procPostThreadMessageW.Call(uintptr(loop.threadID), wmQuit, 0, 0)
	select {
	case <-loop.done:
	case <-time.After(time.Second):
	}
}
