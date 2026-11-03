package tray

import (
	"errors"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

var (
	user32  = syscall.NewLazyDLL("user32.dll")
	shell32 = syscall.NewLazyDLL("shell32.dll")
	kernel  = syscall.NewLazyDLL("kernel32.dll")

	registerClass   = user32.NewProc("RegisterClassExW")
	createWindow    = user32.NewProc("CreateWindowExW")
	defWindowProc   = user32.NewProc("DefWindowProcW")
	destroyWindow   = user32.NewProc("DestroyWindow")
	getMessage      = user32.NewProc("GetMessageW")
	translateMsg    = user32.NewProc("TranslateMessage")
	dispatchMsg     = user32.NewProc("DispatchMessageW")
	postMessage     = user32.NewProc("PostMessageW")
	postQuit        = user32.NewProc("PostQuitMessage")
	registerMsg     = user32.NewProc("RegisterWindowMessageW")
	createMenu      = user32.NewProc("CreatePopupMenu")
	appendMenu      = user32.NewProc("AppendMenuW")
	trackMenu       = user32.NewProc("TrackPopupMenu")
	destroyMenu     = user32.NewProc("DestroyMenu")
	cursorPos       = user32.NewProc("GetCursorPos")
	setForeground   = user32.NewProc("SetForegroundWindow")
	iconFromRes     = user32.NewProc("CreateIconFromResourceEx")
	destroyIcon     = user32.NewProc("DestroyIcon")
	notifyIcon      = shell32.NewProc("Shell_NotifyIconW")
	getModuleHandle = kernel.NewProc("GetModuleHandleW")
)

const (
	wmNull       = 0x0000
	wmDestroy    = 0x0002
	wmClose      = 0x0010
	wmLButtonUp  = 0x0202
	wmRButtonUp  = 0x0205
	wmTray       = 0x8000 + 1 // WM_APP + 1
	nimAdd       = 0
	nimDelete    = 2
	nifMessage   = 0x1
	nifIcon      = 0x2
	nifTip       = 0x4
	mfString     = 0x0
	mfChecked    = 0x8
	mfSeparator  = 0x800
	tpmReturnCmd = 0x100
	tpmRightBtn  = 0x2
)

type notifyIconData struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         [16]byte
	hBalloonIcon     uintptr
}

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
	private uint32
}

// One tray icon per process.
var (
	mu             sync.Mutex
	hwnd           uintptr
	opts           Options
	nid            notifyIconData
	taskbarCreated uintptr
	wndProc        = syscall.NewCallback(proc)
)

func u16(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }

// Run shows the icon and blocks, running the window message loop, until Quit.
func Run(o Options) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	inst, _, _ := getModuleHandle.Call(0)
	class := u16("DeskLinkTray")
	wc := wndClassEx{lpfnWndProc: wndProc, hInstance: inst, lpszClassName: class}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	if r, _, err := registerClass.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return err
	}
	// Explorer broadcasts this after it restarts; the icon has to be added again.
	taskbarCreated, _, _ = registerMsg.Call(uintptr(unsafe.Pointer(u16("TaskbarCreated"))))

	// A hidden top-level window (not message-only, so it gets the broadcast above).
	h, _, err := createWindow.Call(0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(u16("DeskLink"))),
		0, 0, 0, 0, 0, 0, 0, inst, 0)
	if h == 0 {
		return err
	}

	mu.Lock()
	hwnd, opts = h, o
	nid = notifyIconData{hWnd: h, uID: 1, uFlags: nifMessage | nifIcon | nifTip, uCallbackMessage: wmTray}
	nid.cbSize = uint32(unsafe.Sizeof(nid))
	nid.hIcon = loadIcon(o.Icon)
	tip, _ := syscall.UTF16FromString(o.Tooltip)
	copy(nid.szTip[:len(nid.szTip)-1], tip)
	mu.Unlock()

	if r, _, _ := notifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&nid))); r == 0 {
		destroyWindow.Call(h)
		return errors.New("could not add the tray icon")
	}

	var m msg
	for {
		r, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		translateMsg.Call(uintptr(unsafe.Pointer(&m)))
		dispatchMsg.Call(uintptr(unsafe.Pointer(&m)))
	}
	if nid.hIcon != 0 {
		destroyIcon.Call(nid.hIcon)
	}
	return nil
}

// Quit removes the icon and makes Run return. Safe to call from any goroutine.
func Quit() {
	mu.Lock()
	h := hwnd
	mu.Unlock()
	if h != 0 {
		postMessage.Call(h, wmClose, 0, 0)
	}
}

// loadIcon turns PNG bytes into an HICON (Windows accepts PNG icon data directly).
func loadIcon(png []byte) uintptr {
	if len(png) == 0 {
		return 0
	}
	const iconVersion = 0x00030000
	h, _, _ := iconFromRes.Call(uintptr(unsafe.Pointer(&png[0])), uintptr(len(png)), 1, iconVersion, 0, 0, 0)
	return h
}

func proc(h, message, wParam, lParam uintptr) uintptr {
	switch {
	case message == wmTray:
		switch lParam & 0xffff {
		case wmLButtonUp:
			if opts.OnOpen != nil {
				go opts.OnOpen()
			}
		case wmRButtonUp:
			showMenu(h)
		}
		return 0
	case message == taskbarCreated && taskbarCreated != 0:
		notifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
		return 0
	case message == wmClose:
		notifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
		destroyWindow.Call(h)
		return 0
	case message == wmDestroy:
		postQuit.Call(0)
		return 0
	}
	r, _, _ := defWindowProc.Call(h, message, wParam, lParam)
	return r
}

func showMenu(h uintptr) {
	menu, _, _ := createMenu.Call()
	defer destroyMenu.Call(menu)
	for i, it := range opts.Items {
		if it.Label == "" {
			appendMenu.Call(menu, mfSeparator, 0, 0)
			continue
		}
		flags := uintptr(mfString)
		if it.Checked != nil && it.Checked() {
			flags |= mfChecked
		}
		appendMenu.Call(menu, flags, uintptr(i+1), uintptr(unsafe.Pointer(u16(it.Label))))
	}
	var pt struct{ x, y int32 }
	cursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	// Needed so the menu closes when the user clicks elsewhere.
	setForeground.Call(h)
	cmd, _, _ := trackMenu.Call(menu, tpmReturnCmd|tpmRightBtn, uintptr(pt.x), uintptr(pt.y), 0, h, 0)
	postMessage.Call(h, wmNull, 0, 0)
	if cmd > 0 && int(cmd) <= len(opts.Items) {
		if f := opts.Items[cmd-1].OnClick; f != nil {
			go f()
		}
	}
}
