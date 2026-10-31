package player

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// AppID is the AppUserModelID shared by Perch and its mpv window, so the
// taskbar shows the video under the Perch icon instead of a separate mpv one.
const AppID = "Perch.Player"

var (
	shell32                                     = windows.NewLazySystemDLL("shell32.dll")
	procSetCurrentProcessExplicitAppUserModelID = shell32.NewProc("SetCurrentProcessExplicitAppUserModelID")
	procSHGetPropertyStoreForWindow             = shell32.NewProc("SHGetPropertyStoreForWindow")
	procSendMessageTimeoutW                     = user32.NewProc("SendMessageTimeoutW")
	procGetClassLongPtrW                        = user32.NewProc("GetClassLongPtrW")
)

const (
	wmGetIcon       = 0x007F
	wmSetIcon       = 0x0080
	iconSmall       = 0
	iconBig         = 1
	gclpHIcon       = ^uintptr(13) // -14
	gclpHIconSm     = ^uintptr(33) // -34
	smtoAbortIfHung = 0x0002
	vtLPWSTR        = 31
)

type propertyKey struct {
	fmtid windows.GUID
	pid   uint32
}

// propVariant matches the 64-bit PROPVARIANT layout for a VT_LPWSTR value.
type propVariant struct {
	vt       uint16
	reserved [3]uint16
	val      *uint16
	_        uintptr
}

var (
	pkeyAppUserModelID = propertyKey{
		fmtid: windows.GUID{Data1: 0x9F4C2855, Data2: 0x9F79, Data3: 0x4B39,
			Data4: [8]byte{0xA8, 0xD0, 0xE1, 0xD4, 0x2D, 0xE1, 0xD5, 0xF3}},
		pid: 5,
	}
	iidPropertyStore = windows.GUID{Data1: 0x886D8EEB, Data2: 0x8CF2, Data3: 0x4446,
		Data4: [8]byte{0x8D, 0x02, 0xCD, 0xBA, 0x1D, 0xBD, 0xCF, 0x99}}
)

// SetProcessAppID gives Perch's own windows the shared AppID. It must run
// before the first window is created.
func SetProcessAppID() error {
	id, err := windows.UTF16PtrFromString(AppID)
	if err != nil {
		return err
	}
	if hr, _, _ := procSetCurrentProcessExplicitAppUserModelID.Call(uintptr(unsafe.Pointer(id))); hr != 0 {
		return fmt.Errorf("SetCurrentProcessExplicitAppUserModelID: 0x%x", hr)
	}
	return nil
}

// setWindowAppID moves a window (of any process) into the AppID's taskbar
// group via its shell property store.
func setWindowAppID(hwnd windows.HWND, appID string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); err == nil || err == syscall.Errno(1) {
		defer windows.CoUninitialize() // S_OK and S_FALSE both need balancing
	}

	var store unsafe.Pointer
	hr, _, _ := procSHGetPropertyStoreForWindow.Call(uintptr(hwnd),
		uintptr(unsafe.Pointer(&iidPropertyStore)), uintptr(unsafe.Pointer(&store)))
	if hr != 0 || store == nil {
		return fmt.Errorf("SHGetPropertyStoreForWindow: 0x%x", hr)
	}
	// IPropertyStore vtable: QueryInterface, AddRef, Release, GetCount,
	// GetAt, GetValue, SetValue, Commit.
	vtbl := *(**[8]uintptr)(store)
	defer syscall.SyscallN(vtbl[2], uintptr(store))

	id, err := windows.UTF16PtrFromString(appID)
	if err != nil {
		return err
	}
	pv := propVariant{vt: vtLPWSTR, val: id}
	if hr, _, _ := syscall.SyscallN(vtbl[6], uintptr(store),
		uintptr(unsafe.Pointer(&pkeyAppUserModelID)), uintptr(unsafe.Pointer(&pv))); hr != 0 {
		return fmt.Errorf("IPropertyStore.SetValue: 0x%x", hr)
	}
	if hr, _, _ := syscall.SyscallN(vtbl[7], uintptr(store)); hr != 0 {
		return fmt.Errorf("IPropertyStore.Commit: 0x%x", hr)
	}
	runtime.KeepAlive(id)
	return nil
}

// copyAppIcon gives the player window Perch's window icon. HICONs are
// session-wide, so the handle can be passed to another process.
func copyAppIcon(dst windows.HWND) {
	src := findWindow(os.Getpid(), "")
	if src == 0 {
		return
	}
	for _, size := range []uintptr{iconBig, iconSmall} {
		icon := sendMessage(src, wmGetIcon, size, 0)
		if icon == 0 {
			index := gclpHIcon
			if size == iconSmall {
				index = gclpHIconSm
			}
			icon, _, _ = procGetClassLongPtrW.Call(uintptr(src), index)
		}
		if icon != 0 {
			sendMessage(dst, wmSetIcon, size, icon)
		}
	}
}

// sendMessage gives up after 500 ms instead of blocking on a hung window.
func sendMessage(hwnd windows.HWND, msg uint32, wparam, lparam uintptr) uintptr {
	var result uintptr
	procSendMessageTimeoutW.Call(uintptr(hwnd), uintptr(msg), wparam, lparam,
		smtoAbortIfHung, 500, uintptr(unsafe.Pointer(&result)))
	return result
}
