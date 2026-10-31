package player

import (
	"errors"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32              = windows.NewLazySystemDLL("user32.dll")
	procSetWindowPos    = user32.NewProc("SetWindowPos")
	procGetWindowRect   = user32.NewProc("GetWindowRect")
	procGetDpiForWindow = user32.NewProc("GetDpiForWindow")
	procGetDpiForSystem = user32.NewProc("GetDpiForSystem")
	procGetClassNameW   = user32.NewProc("GetClassNameW")
	procIsWindow        = user32.NewProc("IsWindow")
	procGetWindowLong   = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLong   = user32.NewProc("SetWindowLongPtrW")
	procSetLayeredAttrs = user32.NewProc("SetLayeredWindowAttributes")
	procShowWindow      = user32.NewProc("ShowWindow")
	procMonitorFromWnd  = user32.NewProc("MonitorFromWindow")
	procMonitorFromPt   = user32.NewProc("MonitorFromPoint")
	procGetMonitorInfo  = user32.NewProc("GetMonitorInfoW")
)

const (
	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010

	gwlExStyle       = ^uintptr(19) // -20
	wsExLayered      = 0x00080000
	wsExTransparent  = 0x00000020
	lwaAlpha         = 0x2
	swHide           = 0
	swShowNoActivate = 4

	monitorDefaultToNull    = 0
	monitorDefaultToNearest = 2
)

type rect struct{ Left, Top, Right, Bottom int32 }

// newKillOnCloseJob creates a job object that terminates every assigned
// process as soon as the last handle is closed. Because the handle lives in
// this process, Windows also kills mpv if Perch crashes.
func newKillOnCloseJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

func assignToJob(job windows.Handle, pid int) error {
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.AssignProcessToJobObject(job, h)
}

var (
	enumMu     sync.Mutex
	enumPID    uint32
	enumClass  string
	enumResult windows.HWND
	// syscall.NewCallback slots are limited, so the callback is created once.
	enumCallback = syscall.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		var pid uint32
		if _, err := windows.GetWindowThreadProcessId(hwnd, &pid); err != nil || pid != enumPID {
			return 1
		}
		if !windows.IsWindowVisible(hwnd) || (enumClass != "" && windowClass(hwnd) != enumClass) {
			return 1
		}
		enumResult = hwnd
		return 0
	})
)

func windowClass(hwnd windows.HWND) string {
	buf := make([]uint16, 64)
	n, _, _ := procGetClassNameW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf[:n])
}

// findPlayerWindow returns the visible mpv video window of the given process.
func findPlayerWindow(pid int) windows.HWND {
	return findWindow(pid, "mpv")
}

// findWindow returns a visible top-level window of the process; an empty
// class matches any window.
func findWindow(pid int, class string) windows.HWND {
	enumMu.Lock()
	defer enumMu.Unlock()
	enumPID, enumClass, enumResult = uint32(pid), class, 0
	_ = windows.EnumWindows(enumCallback, nil) // returns an error when the callback stops early
	return enumResult
}

func isWindow(hwnd windows.HWND) bool {
	r, _, _ := procIsWindow.Call(uintptr(hwnd))
	return r != 0
}

func dpiForWindow(hwnd windows.HWND) int {
	if procGetDpiForWindow.Find() == nil {
		if d, _, _ := procGetDpiForWindow.Call(uintptr(hwnd)); d != 0 {
			return int(d)
		}
	}
	return systemDPI()
}

func systemDPI() int {
	if procGetDpiForSystem.Find() == nil {
		if d, _, _ := procGetDpiForSystem.Call(); d != 0 {
			return int(d)
		}
	}
	return 96
}

// resizeWindow sets the outer size in logical (96 DPI) pixels, keeping the
// top-left corner in place.
func resizeWindow(hwnd windows.HWND, width, height int) error {
	dpi := dpiForWindow(hwnd)
	w, h := width*dpi/96, height*dpi/96
	r, _, err := procSetWindowPos.Call(uintptr(hwnd), 0, 0, 0, uintptr(w), uintptr(h),
		swpNoMove|swpNoZOrder|swpNoActivate)
	if r == 0 {
		return err
	}
	return nil
}

// windowRect returns the window bounds in physical pixels.
func windowRect(hwnd windows.HWND) (rect, error) {
	var r rect
	ok, _, err := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&r)))
	if ok == 0 {
		return r, err
	}
	if r.Right <= r.Left || r.Bottom <= r.Top {
		return r, errors.New("invalid window size")
	}
	return r, nil
}

// setTransparency applies opacity (percent) and click-through, where mouse
// input passes to the window behind. Both need WS_EX_LAYERED.
func setTransparency(hwnd windows.HWND, opacity int, clickThrough bool) error {
	ex, _, _ := procGetWindowLong.Call(uintptr(hwnd), gwlExStyle)
	if opacity >= 100 && !clickThrough {
		procSetWindowLong.Call(uintptr(hwnd), gwlExStyle, ex&^(wsExLayered|wsExTransparent))
		return nil
	}
	ex |= wsExLayered
	if clickThrough {
		ex |= wsExTransparent
	} else {
		ex &^= wsExTransparent
	}
	procSetWindowLong.Call(uintptr(hwnd), gwlExStyle, ex)
	alpha := max(min(opacity, 100), 10) * 255 / 100
	if r, _, err := procSetLayeredAttrs.Call(uintptr(hwnd), 0, uintptr(alpha), lwaAlpha); r == 0 {
		return err
	}
	return nil
}

func showWindow(hwnd windows.HWND, visible bool) {
	cmd := uintptr(swHide)
	if visible {
		cmd = swShowNoActivate
	}
	procShowWindow.Call(uintptr(hwnd), cmd)
}

type monitorInfo struct {
	size    uint32
	monitor rect
	work    rect
	flags   uint32
}

// workArea returns the work area (screen minus taskbar) of the monitor the
// window is on, in physical pixels.
func workArea(hwnd windows.HWND) (rect, error) {
	mon, _, _ := procMonitorFromWnd.Call(uintptr(hwnd), monitorDefaultToNearest)
	mi := monitorInfo{size: uint32(unsafe.Sizeof(monitorInfo{}))}
	if r, _, err := procGetMonitorInfo.Call(mon, uintptr(unsafe.Pointer(&mi))); r == 0 {
		return rect{}, err
	}
	return mi.work, nil
}

// onScreen reports whether a point lies on any connected monitor, so a
// saved position from a since-removed monitor is not restored.
func onScreen(x, y int32) bool {
	pt := uintptr(uint32(x)) | uintptr(uint32(y))<<32
	mon, _, _ := procMonitorFromPt.Call(pt, monitorDefaultToNull)
	return mon != 0
}

// snapWindow moves the window into a corner of its monitor's work area,
// leaving a small margin. corner is "tl", "tr", "bl" or "br".
func snapWindow(hwnd windows.HWND, corner string) error {
	r, err := windowRect(hwnd)
	if err != nil {
		return err
	}
	work, err := workArea(hwnd)
	if err != nil {
		return err
	}
	margin := int32(16 * dpiForWindow(hwnd) / 96)
	w, h := r.Right-r.Left, r.Bottom-r.Top
	x, y := work.Left+margin, work.Top+margin
	if corner == "tr" || corner == "br" {
		x = work.Right - margin - w
	}
	if corner == "bl" || corner == "br" {
		y = work.Bottom - margin - h
	}
	ok, _, err := procSetWindowPos.Call(uintptr(hwnd), 0, uintptr(x), uintptr(y), 0, 0,
		swpNoSize|swpNoZOrder|swpNoActivate)
	if ok == 0 {
		return err
	}
	return nil
}
