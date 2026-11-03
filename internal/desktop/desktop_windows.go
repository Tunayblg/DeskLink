package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
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

// OpenApp opens url as a standalone app window (no tabs or address bar) using
// Microsoft Edge, which ships with Windows. Without Edge it falls back to the
// default browser.
func OpenApp(url string) {
	if edge := edgePath(); edge != "" {
		cmd := exec.Command(edge, "--app="+url, "--window-size=1120,800")
		if cmd.Start() == nil {
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
