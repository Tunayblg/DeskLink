package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32     = syscall.NewLazyDLL("user32.dll")
	kernel32   = syscall.NewLazyDLL("kernel32.dll")
	advapi32   = syscall.NewLazyDLL("advapi32.dll")
	shell32    = syscall.NewLazyDLL("shell32.dll")
	messageBox = user32.NewProc("MessageBoxW")
	uiLanguage = kernel32.NewProc("GetUserDefaultUILanguage")
	regOpen    = advapi32.NewProc("RegOpenKeyExW")
	regCreate  = advapi32.NewProc("RegCreateKeyExW")
	regSet     = advapi32.NewProc("RegSetValueExW")
	regDelete  = advapi32.NewProc("RegDeleteValueW")
	regQuery   = advapi32.NewProc("RegQueryValueExW")
	regGet     = advapi32.NewProc("RegGetValueW")
	regClose   = advapi32.NewProc("RegCloseKey")
	shellExecW = shell32.NewProc("ShellExecuteW")
)

const (
	hkeyCurrentUser  = 0x80000001
	hkeyLocalMachine = 0x80000002
	keyQuery         = 0x0001
	keySet           = 0x0002
	regSZ            = 1
	runKey           = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValue         = "DeskLink"
)

func u16(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }

// Text returns the tray menu strings for the Windows display language.
func Text() Strings {
	lang, _, _ := uiLanguage.Call()
	if lang&0x3ff == 0x1f { // LANG_TURKISH
		return turkish
	}
	return english
}

// Alert shows an error dialog. Used when there is no console to print to.
func Alert(msg string) {
	const mbIconError = 0x10
	messageBox.Call(0, uintptr(unsafe.Pointer(u16(msg))), uintptr(unsafe.Pointer(u16("DeskLink"))), mbIconError)
}

var (
	enumWindows     = user32.NewProc("EnumWindows")
	getWindowText   = user32.NewProc("GetWindowTextW")
	getClassName    = user32.NewProc("GetClassNameW")
	isWindowVisible = user32.NewProc("IsWindowVisible")
	isIconic        = user32.NewProc("IsIconic")
	isZoomed        = user32.NewProc("IsZoomed")
	showWindow      = user32.NewProc("ShowWindow")
	setForeground   = user32.NewProc("SetForegroundWindow")
)

const (
	// The dashboard page's <title>, which an Edge app window shows as its own.
	dashboardTitle = "DeskLink"
	edgeWindowType = "Chrome_WidgetWin_1" // Chromium's top-level window class
	swMaximize     = 3
	swRestore      = 9
	// A window takes a moment to appear; a second request in that time must not
	// open another one.
	openGrace = 6 * time.Second
)

var (
	openMu     sync.Mutex
	openedAt   time.Time
	foundWnd   uintptr
	enumDeskLk = syscall.NewCallback(func(h, _ uintptr) uintptr {
		if v, _, _ := isWindowVisible.Call(h); v == 0 {
			return 1
		}
		var class [64]uint16
		getClassName.Call(h, uintptr(unsafe.Pointer(&class[0])), uintptr(len(class)))
		if syscall.UTF16ToString(class[:]) != edgeWindowType {
			return 1
		}
		var title [128]uint16
		getWindowText.Call(h, uintptr(unsafe.Pointer(&title[0])), uintptr(len(title)))
		if syscall.UTF16ToString(title[:]) != dashboardTitle {
			return 1
		}
		foundWnd = h
		return 0 // stop looking
	})
)

// focusDashboard brings an already open dashboard window to the front (also
// from the taskbar, if it was minimized). It reports whether there was one.
// The caller holds openMu.
func focusDashboard() bool {
	foundWnd = 0
	enumWindows.Call(enumDeskLk, 0)
	if foundWnd == 0 {
		return false
	}
	if iconic, _, _ := isIconic.Call(foundWnd); iconic != 0 {
		showWindow.Call(foundWnd, swRestore)
	}
	setForeground.Call(foundWnd)
	return true
}

// MaximizeDashboard makes the dashboard window fill the screen, as the middle
// button of its title bar does, or puts it back to its size. It reports whether
// it changed anything, so a caller can restore only what it enlarged itself.
func MaximizeDashboard(on bool) bool {
	openMu.Lock()
	defer openMu.Unlock()
	foundWnd = 0
	enumWindows.Call(enumDeskLk, 0)
	if foundWnd == 0 {
		return false
	}
	zoomed, _, _ := isZoomed.Call(foundWnd)
	switch {
	case on && zoomed == 0:
		showWindow.Call(foundWnd, swMaximize)
		return true
	case !on && zoomed != 0:
		showWindow.Call(foundWnd, swRestore)
		return true
	}
	return false
}

// OpenApp shows url as a standalone app window (no tabs or address bar) using
// Microsoft Edge, which ships with Windows. There is only ever one: if the
// dashboard is already open it is brought to the front instead. Without Edge
// it falls back to the default browser.
func OpenApp(url string) {
	openMu.Lock()
	defer openMu.Unlock()
	if focusDashboard() || time.Since(openedAt) < openGrace {
		return
	}
	if edge := edgePath(); edge != "" {
		cmd := exec.Command(edge, "--app="+url, "--window-size=1120,800")
		if cmd.Start() == nil {
			openedAt = time.Now()
			go cmd.Wait()
			return
		}
	}
	const swShowNormal = 1
	shellExecW.Call(0, uintptr(unsafe.Pointer(u16("open"))), uintptr(unsafe.Pointer(u16(url))), 0, 0, swShowNormal)
}

func edgePath() string {
	// Edge registers its location under App Paths; fall back to the usual folders.
	buf := make([]uint16, 520)
	size := uint32(len(buf) * 2)
	const rrfRtRegSz = 0x2
	if r, _, _ := regGet.Call(hkeyLocalMachine, uintptr(unsafe.Pointer(u16(`SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\msedge.exe`))),
		0, rrfRtRegSz, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size))); r == 0 {
		if p := syscall.UTF16ToString(buf); fileExists(p) {
			return p
		}
	}
	for _, dir := range []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles")} {
		if p := filepath.Join(dir, "Microsoft", "Edge", "Application", "msedge.exe"); dir != "" && fileExists(p) {
			return p
		}
	}
	return ""
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

var (
	comdlg32      = syscall.NewLazyDLL("comdlg32.dll")
	ole32         = syscall.NewLazyDLL("ole32.dll")
	getOpenFile   = comdlg32.NewProc("GetOpenFileNameW")
	coInit        = ole32.NewProc("CoInitializeEx")
	coUninit      = ole32.NewProc("CoUninitialize")
	foregroundWnd = user32.NewProc("GetForegroundWindow")
)

type openFileName struct {
	structSize    uint32
	owner         uintptr
	instance      uintptr
	filter        *uint16
	customFilter  *uint16
	maxCustFilter uint32
	filterIndex   uint32
	file          *uint16
	maxFile       uint32
	fileTitle     *uint16
	maxFileTitle  uint32
	initialDir    *uint16
	title         *uint16
	flags         uint32
	fileOffset    uint16
	fileExtension uint16
	defExt        *uint16
	custData      uintptr
	hook          uintptr
	templateName  *uint16
	reserved      uintptr
	reservedDW    uint32
	flagsEx       uint32
}

// ChooseProgram shows the Windows "Open" dialog for picking a program and
// returns the chosen path. Shortcuts (.lnk) resolve to their target.
func ChooseProgram(title string) (string, bool) {
	type result struct {
		path string
		ok   bool
	}
	done := make(chan result, 1)
	go func() {
		// The dialog needs a single-threaded COM apartment on its own OS thread.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		const coinitApartmentThreaded = 0x2
		coInit.Call(0, coinitApartmentThreaded)
		defer coUninit.Call()

		filter := utf16z("Programs (*.exe, *.lnk, *.bat)", "*.exe;*.lnk;*.bat;*.cmd", "All files", "*.*")
		buf := make([]uint16, 1024)
		owner, _, _ := foregroundWnd.Call()
		ofn := openFileName{owner: owner, filter: &filter[0], filterIndex: 1, file: &buf[0], maxFile: uint32(len(buf)), title: u16(title)}
		ofn.structSize = uint32(unsafe.Sizeof(ofn))
		const (
			ofnNoChangeDir     = 0x00000008
			ofnPathMustExist   = 0x00000800
			ofnFileMustExist   = 0x00001000
			ofnExplorer        = 0x00080000
			ofnDontAddToRecent = 0x02000000
		)
		ofn.flags = ofnNoChangeDir | ofnPathMustExist | ofnFileMustExist | ofnExplorer | ofnDontAddToRecent
		r, _, _ := getOpenFile.Call(uintptr(unsafe.Pointer(&ofn)))
		if r == 0 {
			done <- result{}
			return
		}
		done <- result{syscall.UTF16ToString(buf), true}
	}()
	res := <-done
	return res.path, res.ok
}

// utf16z builds a double-NUL-terminated list of strings, as dialog filters need.
func utf16z(parts ...string) []uint16 {
	var out []uint16
	for _, p := range parts {
		u, _ := syscall.UTF16FromString(p)
		out = append(out, u...)
	}
	return append(out, 0)
}

// Autostart reports whether DeskLink is registered to start with Windows.
func Autostart() bool {
	var h uintptr
	if r, _, _ := regOpen.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(u16(runKey))), 0, keyQuery, uintptr(unsafe.Pointer(&h))); r != 0 {
		return false
	}
	defer regClose.Call(h)
	r, _, _ := regQuery.Call(h, uintptr(unsafe.Pointer(u16(runValue))), 0, 0, 0, 0)
	return r == 0
}

// SetAutostart registers (or removes) command to run when the user signs in.
// It uses the per-user Run key, so no administrator rights are needed.
func SetAutostart(on bool, command string) error {
	var h uintptr
	if r, _, _ := regCreate.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(u16(runKey))), 0, 0, 0, keySet|keyQuery, 0,
		uintptr(unsafe.Pointer(&h)), 0); r != 0 {
		return syscall.Errno(r)
	}
	defer regClose.Call(h)
	if !on {
		if r, _, _ := regDelete.Call(h, uintptr(unsafe.Pointer(u16(runValue)))); r != 0 && r != 2 { // 2: not found
			return syscall.Errno(r)
		}
		return nil
	}
	data, _ := syscall.UTF16FromString(command)
	if r, _, _ := regSet.Call(h, uintptr(unsafe.Pointer(u16(runValue))), 0, regSZ,
		uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)*2)); r != 0 {
		return syscall.Errno(r)
	}
	return nil
}
