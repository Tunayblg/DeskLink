package brightness

import (
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"unsafe"
)

var (
	user32 = syscall.NewLazyDLL("user32.dll")
	dxva2  = syscall.NewLazyDLL("dxva2.dll")

	procEnumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")
	procCount               = dxva2.NewProc("GetNumberOfPhysicalMonitorsFromHMONITOR")
	procPhysical            = dxva2.NewProc("GetPhysicalMonitorsFromHMONITOR")
	procGet                 = dxva2.NewProc("GetMonitorBrightness")
	procSet                 = dxva2.NewProc("SetMonitorBrightness")
	procDestroy             = dxva2.NewProc("DestroyPhysicalMonitors")
)

// physicalMonitor is PHYSICAL_MONITOR from physicalmonitorenumerationapi.h.
type physicalMonitor struct {
	handle uintptr
	desc   [128]uint16
}

// each calls f for every physical monitor Windows knows about.
func each(f func(h uintptr)) {
	cb := syscall.NewCallback(func(mon, _, _, _ uintptr) uintptr {
		var n uint32
		if r, _, _ := procCount.Call(mon, uintptr(unsafe.Pointer(&n))); r == 0 || n == 0 {
			return 1
		}
		list := make([]physicalMonitor, n)
		if r, _, _ := procPhysical.Call(mon, uintptr(n), uintptr(unsafe.Pointer(&list[0]))); r == 0 {
			return 1
		}
		for _, m := range list {
			f(m.handle)
		}
		procDestroy.Call(uintptr(n), uintptr(unsafe.Pointer(&list[0])))
		return 1
	})
	procEnumDisplayMonitors.Call(0, 0, cb, 0)
}

func ddcGet() (int, bool) {
	level, ok := 0, false
	each(func(h uintptr) {
		if ok {
			return
		}
		var lo, cur, hi uint32
		if r, _, _ := procGet.Call(h, uintptr(unsafe.Pointer(&lo)), uintptr(unsafe.Pointer(&cur)), uintptr(unsafe.Pointer(&hi))); r != 0 {
			level, ok = percent(cur, lo, hi), true
		}
	})
	return level, ok
}

func ddcSet(level int) bool {
	any := false
	each(func(h uintptr) {
		var lo, cur, hi uint32
		if r, _, _ := procGet.Call(h, uintptr(unsafe.Pointer(&lo)), uintptr(unsafe.Pointer(&cur)), uintptr(unsafe.Pointer(&hi))); r == 0 {
			return
		}
		if r, _, _ := procSet.Call(h, uintptr(scale(level, lo, hi))); r != 0 {
			any = true
		}
	})
	return any
}

// powershell runs a short script without a console window and returns what it printed.
func powershell(script string) (string, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	out, err := cmd.Output()
	return string(out), err
}

// Laptop panels do not speak DDC/CI; Windows has its own control for them.
func wmiGet() (int, bool) {
	out, err := powershell("(Get-CimInstance -Namespace root/WMI -ClassName WmiMonitorBrightness | Select-Object -First 1).CurrentBrightness")
	if err != nil {
		return 0, false
	}
	n, err := strconv.Atoi(trim(out))
	return n, err == nil
}

func wmiSet(level int) bool {
	_, err := powershell("Get-CimInstance -Namespace root/WMI -ClassName WmiMonitorBrightnessMethods | Invoke-CimMethod -MethodName WmiSetBrightness -Arguments @{Timeout=0;Brightness=" + strconv.Itoa(level) + "} | Out-Null; if (-not $?) { exit 1 }")
	return err == nil
}

func trim(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\r' || s[len(s)-1] == '\n' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}

// How the screen is controlled is found out once and then remembered: the
// monitor itself (DDC/CI), a laptop panel (Windows' own control), or, when
// neither works, dimming in software. The last one works on every screen.
const (
	modeNone = ""
	modeDDC  = "ddc"
	modeWMI  = "wmi"
	modeSoft = "soft"
)

var (
	modeMu sync.Mutex
	mode   = modeNone

	// hwMu lets one call at a time talk to the screens: monitors answer DDC/CI
	// slowly and do not like overlapping requests, and on a laptop every call
	// starts PowerShell.
	hwMu sync.Mutex
)

func setMode(m string) { modeMu.Lock(); mode = m; modeMu.Unlock() }
func getMode() string  { modeMu.Lock(); defer modeMu.Unlock(); return mode }

// detect finds the best way to control the screen and returns its level.
func detect() int {
	if l, ok := ddcGet(); ok {
		setMode(modeDDC)
		return l
	}
	if l, ok := wmiGet(); ok {
		setMode(modeWMI)
		return l
	}
	setMode(modeSoft)
	return softGet()
}

func get() (int, bool) {
	hwMu.Lock()
	defer hwMu.Unlock()
	switch getMode() {
	case modeDDC:
		if l, ok := ddcGet(); ok {
			return l, true
		}
	case modeWMI:
		if l, ok := wmiGet(); ok {
			return l, true
		}
	case modeSoft:
		return softGet(), true
	}
	return detect(), true // the first time, or the screen stopped answering
}

func set(level int) {
	hwMu.Lock()
	defer hwMu.Unlock()
	if getMode() == modeNone {
		detect()
	}
	if getMode() == modeDDC {
		if ddcSet(level) {
			return
		}
		setMode(modeNone) // the monitor stopped answering: look again
		detect()
	}
	if getMode() == modeWMI {
		if wmiSet(level) {
			return
		}
		setMode(modeSoft)
	}
	softSet(level)
}
