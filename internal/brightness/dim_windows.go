package brightness

import (
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// Software dimming for screens that cannot be controlled any other way: a black,
// see-through window over the whole desktop. It ignores the mouse and never
// takes focus, and it goes away when DeskLink quits. It dims every monitor,
// does not depend on the graphics card, and can never make the screen fully
// black (see minLevel).

const (
	minLevel = 10 // percent; the darkest the screen can be made

	wsPopup          = 0x80000000
	wsExTopmost      = 0x00000008
	wsExTransparent  = 0x00000020
	wsExToolWindow   = 0x00000080
	wsExLayered      = 0x00080000
	wsExNoActivate   = 0x08000000
	lwaAlpha         = 0x2
	swHide           = 0
	swShowNoActivate = 4
	swpNoSize        = 0x1
	swpNoMove        = 0x2
	swpNoActivate    = 0x10
	wmApp            = 0x8000
	smXVirtual       = 76
	smYVirtual       = 77
	smCXVirtual      = 78
	smCYVirtual      = 79
	blackBrush       = 4
)

var (
	procRegisterClass   = user32.NewProc("RegisterClassExW")
	procCreateWindowEx  = user32.NewProc("CreateWindowExW")
	procDefWindowProc   = user32.NewProc("DefWindowProcW")
	procGetMessage      = user32.NewProc("GetMessageW")
	procDispatchMessage = user32.NewProc("DispatchMessageW")
	procPostMessage     = user32.NewProc("PostMessageW")
	procShowWindow      = user32.NewProc("ShowWindow")
	procSetWindowPos    = user32.NewProc("SetWindowPos")
	procSetLayeredAttrs = user32.NewProc("SetLayeredWindowAttributes")
	procSetTimer        = user32.NewProc("SetTimer")
	procGetSystemMetric = user32.NewProc("GetSystemMetrics")
	procIsWindowVisible = user32.NewProc("IsWindowVisible")
	procGetLayeredAttrs = user32.NewProc("GetLayeredWindowAttributes")
	procGetStockObject  = syscall.NewLazyDLL("gdi32.dll").NewProc("GetStockObject")
	procGetModuleHandle = syscall.NewLazyDLL("kernel32.dll").NewProc("GetModuleHandleW")
)

const wmTimer = 0x0113

type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   uintptr
	icon       uintptr
	cursor     uintptr
	background uintptr
	menuName   *uint16
	className  *uint16
	iconSm     uintptr
}

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	ptX     int32
	ptY     int32
}

// dimmer owns the overlay window and the thread that runs it.
type dimmer struct {
	mu    sync.Mutex
	level int // 100 = no dimming
	hwnd  uintptr
	once  sync.Once
	ready chan struct{}
}

var soft = &dimmer{level: 100, ready: make(chan struct{})}

func softGet() int {
	soft.mu.Lock()
	defer soft.mu.Unlock()
	return soft.level
}

func softSet(level int) {
	level = min(max(level, minLevel), 100)
	soft.once.Do(func() {
		go soft.run()
	})
	<-soft.ready
	soft.mu.Lock()
	soft.level = level
	h := soft.hwnd
	soft.mu.Unlock()
	if h != 0 {
		procPostMessage.Call(h, wmApp, uintptr(level), 0)
	}
}

// alphaFor is how opaque the black window is for a brightness level.
func alphaFor(level int) uint8 {
	level = min(max(level, minLevel), 100)
	return uint8((100 - level) * 255 / 100)
}

func (d *dimmer) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	instance, _, _ := procGetModuleHandle.Call(0)
	brush, _, _ := procGetStockObject.Call(blackBrush)
	name, _ := syscall.UTF16PtrFromString("DeskLinkDim")
	proc := syscall.NewCallback(d.wndProc)
	wc := wndClassEx{size: uint32(unsafe.Sizeof(wndClassEx{})), wndProc: proc, instance: instance, background: brush, className: name}
	procRegisterClass.Call(uintptr(unsafe.Pointer(&wc)))
	hwnd, _, _ := procCreateWindowEx.Call(wsExLayered|wsExTransparent|wsExTopmost|wsExToolWindow|wsExNoActivate,
		uintptr(unsafe.Pointer(name)), 0, wsPopup, 0, 0, 1, 1, 0, 0, instance, 0)
	d.mu.Lock()
	d.hwnd = hwnd
	d.mu.Unlock()
	close(d.ready)
	if hwnd == 0 {
		return
	}
	// A game or another always-on-top window can end up above the overlay; claim the top again now and then.
	procSetTimer.Call(hwnd, 1, 2000, 0)
	var m msg
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func (d *dimmer) wndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case wmApp:
		level := int(wParam)
		if level >= 100 {
			procShowWindow.Call(hwnd, swHide)
			return 0
		}
		// Cover the whole desktop, every monitor; displays can come and go between two calls.
		x, _, _ := procGetSystemMetric.Call(smXVirtual)
		y, _, _ := procGetSystemMetric.Call(smYVirtual)
		w, _, _ := procGetSystemMetric.Call(smCXVirtual)
		h, _, _ := procGetSystemMetric.Call(smCYVirtual)
		procSetLayeredAttrs.Call(hwnd, 0, uintptr(alphaFor(level)), lwaAlpha)
		procSetWindowPos.Call(hwnd, ^uintptr(0), uintptr(int32(x)), uintptr(int32(y)), w, h, swpNoActivate) // HWND_TOPMOST
		procShowWindow.Call(hwnd, swShowNoActivate)
		return 0
	case wmTimer:
		if v, _, _ := procIsWindowVisible.Call(hwnd); v != 0 {
			procSetWindowPos.Call(hwnd, ^uintptr(0), 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
		}
		return 0
	}
	r, _, _ := procDefWindowProc.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

// dimState reports what the overlay looks like now, for tests.
func dimState() (visible bool, alpha uint8) {
	soft.mu.Lock()
	h := soft.hwnd
	soft.mu.Unlock()
	if h == 0 {
		return false, 0
	}
	v, _, _ := procIsWindowVisible.Call(h)
	var a uint8
	var key, flags uint32
	procGetLayeredAttrs.Call(h, uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&a)), uintptr(unsafe.Pointer(&flags)))
	return v != 0, a
}
