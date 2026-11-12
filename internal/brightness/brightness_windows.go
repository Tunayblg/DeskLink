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

// ddcMon is one physical monitor that answered DDC/CI, with its own range.
type ddcMon struct {
	h           uintptr
	key         string // position and name: the handle changes on every enumeration
	lo, cur, hi uint32
}

func (m ddcMon) pct() int { return percent(m.cur, m.lo, m.hi) }

// enumCallback is created once: Go allows only a limited number of callbacks
// per process and never frees them, so one per call would run out after a
// few thousand slider moves. It collects into enumInto, which hwMu protects.
var (
	enumInto     *[][]physicalMonitor
	enumCallback = syscall.NewCallback(func(mon, _, _, _ uintptr) uintptr {
		var n uint32
		if r, _, _ := procCount.Call(mon, uintptr(unsafe.Pointer(&n))); r == 0 || n == 0 {
			return 1
		}
		list := make([]physicalMonitor, n)
		if r, _, _ := procPhysical.Call(mon, uintptr(n), uintptr(unsafe.Pointer(&list[0]))); r == 0 {
			return 1
		}
		*enumInto = append(*enumInto, list)
		return 1
	})
)

// withMonitors reads every monitor that speaks DDC/CI and hands the list to f.
// Monitors that do not answer, or report a range that is not a range, are left
// alone: writing to them could only guess. The caller must hold hwMu.
func withMonitors(f func([]ddcMon)) {
	var groups [][]physicalMonitor
	enumInto = &groups
	procEnumDisplayMonitors.Call(0, 0, enumCallback, 0)
	enumInto = nil

	var mons []ddcMon
	idx := 0
	for _, list := range groups {
		for _, m := range list {
			var lo, cur, hi uint32
			r, _, _ := procGet.Call(m.handle, uintptr(unsafe.Pointer(&lo)), uintptr(unsafe.Pointer(&cur)), uintptr(unsafe.Pointer(&hi)))
			if r != 0 && hi > lo {
				mons = append(mons, ddcMon{h: m.handle, key: strconv.Itoa(idx) + ":" + syscall.UTF16ToString(m.desc[:]), lo: lo, cur: cur, hi: hi})
			}
			idx++
		}
	}
	f(mons)
	for _, list := range groups {
		procDestroy.Call(uintptr(len(list)), uintptr(unsafe.Pointer(&list[0])))
	}
}

// Monitors are usually not set to the same brightness (a screen next to a
// window, a TV, two different panels). Writing the same value to all of them
// would flatten what the user set up, so DeskLink keeps each monitor's own
// setting as its "full" brightness and the slider scales all of them together.
// The calibration is dropped whenever a monitor is no longer where DeskLink
// left it, because then someone used its own buttons, and what it shows now
// becomes the new starting point.
type calibration struct {
	ref   int               // the slider level the bases belong to
	level int               // the slider level last set
	base  map[string]int    // each monitor's own brightness (percent) at ref
	last  map[string]uint32 // the raw value DeskLink last wrote to each monitor
}

var cal *calibration

// calibrated reports whether every monitor is still where DeskLink left it.
func (c *calibration) calibrated(mons []ddcMon) bool {
	if c == nil || len(c.base) != len(mons) {
		return false
	}
	for _, m := range mons {
		last, ok := c.last[m.key]
		if !ok {
			return false
		}
		tol := max(1, (m.hi-m.lo)/50)
		if m.cur+tol < last || last+tol < m.cur {
			return false
		}
	}
	return true
}

// calibrate takes the monitors as they are now. At level ref, they are all at
// their own brightness.
func calibrate(mons []ddcMon, ref int) {
	c := &calibration{ref: max(ref, 1), level: ref, base: map[string]int{}, last: map[string]uint32{}}
	for _, m := range mons {
		c.base[m.key] = max(m.pct(), 1)
		c.last[m.key] = m.cur
	}
	cal = c
}

func brightest(mons []ddcMon) int {
	b := 0
	for _, m := range mons {
		b = max(b, m.pct())
	}
	return b
}

// targetFor is the brightness (percent of the monitor's own range) for a
// slider level, given the monitor's own brightness at the reference level.
func targetFor(base, level, ref int) int {
	return min(100, (base*level+ref/2)/max(ref, 1))
}

func ddcGet() (int, bool) {
	level, ok := 0, false
	withMonitors(func(mons []ddcMon) {
		if len(mons) == 0 {
			return
		}
		if !cal.calibrated(mons) {
			calibrate(mons, brightest(mons))
		}
		level, ok = cal.level, true
	})
	return level, ok
}

func ddcSet(level int) bool {
	any := false
	withMonitors(func(mons []ddcMon) {
		if len(mons) == 0 {
			return
		}
		if !cal.calibrated(mons) {
			ref := brightest(mons)
			if cal != nil {
				ref = cal.level // moved by hand: it is at this level now
			}
			calibrate(mons, ref)
		}
		for _, m := range mons {
			raw := scale(targetFor(cal.base[m.key], level, cal.ref), m.lo, m.hi)
			if raw != m.cur { // do not write what is already there: it wears the monitor's memory and is slow
				if r, _, _ := procSet.Call(m.h, uintptr(raw)); r == 0 {
					continue
				}
			}
			cal.last[m.key] = raw
			any = true
		}
		cal.level = level
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
