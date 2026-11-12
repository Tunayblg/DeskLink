package winlist

import (
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")

	procEnumWindows         = user32.NewProc("EnumWindows")
	procIsWindow            = user32.NewProc("IsWindow")
	procIsWindowVisible     = user32.NewProc("IsWindowVisible")
	procIsIconic            = user32.NewProc("IsIconic")
	procShowWindow          = user32.NewProc("ShowWindow")
	procGetWindowTextW      = user32.NewProc("GetWindowTextW")
	procGetWindowLongW      = user32.NewProc("GetWindowLongW")
	procGetWindowThreadPID  = user32.NewProc("GetWindowThreadProcessId")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procGetClassNameW       = user32.NewProc("GetClassNameW")
	procGetClientRect       = user32.NewProc("GetClientRect")
	procSetForeground       = user32.NewProc("SetForegroundWindow")
	procKeybdEvent          = user32.NewProc("keybd_event")

	procGetCurrentProcessID = kernel32.NewProc("GetCurrentProcessId")
	procOpenProcess         = kernel32.NewProc("OpenProcess")
	procCloseHandle         = kernel32.NewProc("CloseHandle")
	procQueryImageName      = kernel32.NewProc("QueryFullProcessImageNameW")

	procDwmGetWindowAttribute = dwmapi.NewProc("DwmGetWindowAttribute")
)

const (
	gwlExStyle        = -20
	wsExToolWindow    = 0x00000080
	wsExAppWindow     = 0x00040000
	wsExNoActivate    = 0x08000000
	dwmaCloaked       = 14
	swRestore         = 9
	vkMenu            = 0x12
	keyeventfKeyUp    = 0x0002
	processQueryLimit = 0x1000
)

// EnumWindows calls back into Go; the callback is made once, because Windows
// callbacks created by syscall.NewCallback are never freed.
var (
	enumMu  sync.Mutex
	enumOut []uintptr
	enumCB  = syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		enumOut = append(enumOut, hwnd)
		return 1 // keep going
	})
)

func title(hwnd uintptr) string {
	buf := make([]uint16, 256)
	n, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf[:n])
}

func processID(hwnd uintptr) uint32 {
	var pid uint32
	procGetWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return pid
}

// exeName is the program's file name without ".exe", or "" if it cannot be read.
func exeName(pid uint32) string {
	h, _, _ := procOpenProcess.Call(processQueryLimit, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer procCloseHandle.Call(h)
	buf := make([]uint16, 512)
	size := uint32(len(buf))
	if r, _, _ := procQueryImageName.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size))); r == 0 {
		return ""
	}
	base := filepath.Base(syscall.UTF16ToString(buf[:size]))
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func className(hwnd uintptr) string {
	buf := make([]uint16, 128)
	n, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf[:n])
}

// shellClasses are windows of Windows itself that carry a title but are no
// program a person switches to: the desktop, the taskbar, and the invisible
// core windows of Store apps.
var shellClasses = map[string]bool{
	"Progman": true, "WorkerW": true, "Shell_TrayWnd": true, "Shell_SecondaryTrayWnd": true,
	"NotifyIconOverflowWindow": true, "Windows.UI.Core.CoreWindow": true,
}

// emptyClient reports whether a window that is not minimized has no area:
// a titled window with nothing to show is not something to switch to.
func emptyClient(hwnd uintptr) bool {
	if min, _, _ := procIsIconic.Call(hwnd); min != 0 {
		return false // a minimized window has no area, and is the point of the list
	}
	var r struct{ left, top, right, bottom int32 }
	if ok, _, _ := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); ok == 0 {
		return false
	}
	return r.right-r.left <= 0 || r.bottom-r.top <= 0
}

func cloaked(hwnd uintptr) bool {
	var v uint32
	r, _, _ := procDwmGetWindowAttribute.Call(hwnd, dwmaCloaked, uintptr(unsafe.Pointer(&v)), unsafe.Sizeof(v))
	return r == 0 && v != 0
}

// List returns the windows a person would call open programs, in front-to-back
// order (about the order of Alt+Tab): visible, titled, not a tool window or a
// helper that never takes focus, not a piece of the desktop or taskbar, not
// hidden on another virtual desktop, not empty, and not DeskLink's own.
func List() []Window {
	enumMu.Lock()
	enumOut = enumOut[:0]
	procEnumWindows.Call(enumCB, 0)
	handles := append([]uintptr(nil), enumOut...)
	enumMu.Unlock()

	self, _, _ := procGetCurrentProcessID.Call()
	var out []Window
	for _, h := range handles {
		if vis, _, _ := procIsWindowVisible.Call(h); vis == 0 {
			continue
		}
		t := strings.TrimSpace(title(h))
		if t == "" || t == "Program Manager" {
			continue
		}
		if shellClasses[className(h)] {
			continue
		}
		ex, _, _ := procGetWindowLongW.Call(h, uintptr(0xFFFFFFEC)) // GWL_EXSTYLE as a 32-bit index
		if (ex&wsExToolWindow != 0 || ex&wsExNoActivate != 0) && ex&wsExAppWindow == 0 {
			continue
		}
		if cloaked(h) || emptyClient(h) {
			continue
		}
		pid := processID(h)
		if uintptr(pid) == self {
			continue
		}
		app := exeName(pid)
		if app == "" || strings.EqualFold(app, "ApplicationFrameHost") {
			app = shorten(t) // a Store app: its title is the best name there is
		}
		out = append(out, Window{ID: uint64(h), App: app, Title: shorten(t)})
		if len(out) >= MaxWindows {
			break
		}
	}
	return out
}

// Focus brings the window to the front and checks that it got there. Windows
// refuses to move a window forward for a program that is not in front; a tap
// on Alt first lets the request through.
func Focus(id uint64) error {
	h := uintptr(id)
	if ok, _, _ := procIsWindow.Call(h); ok == 0 {
		return ErrNotFocused
	}
	if min, _, _ := procIsIconic.Call(h); min != 0 {
		procShowWindow.Call(h, swRestore)
	}
	procKeybdEvent.Call(vkMenu, 0, 0, 0)
	procKeybdEvent.Call(vkMenu, 0, keyeventfKeyUp, 0)
	procSetForeground.Call(h)
	for i := 0; i < 10; i++ {
		if fg, _, _ := procGetForegroundWindow.Call(); fg == h {
			time.Sleep(150 * time.Millisecond) // let it settle before typing
			return nil
		}
		time.Sleep(40 * time.Millisecond)
	}
	return ErrNotFocused
}

// Foreground returns the window in front, or 0 if there is none.
func Foreground() uint64 {
	fg, _, _ := procGetForegroundWindow.Call()
	return uint64(fg)
}
