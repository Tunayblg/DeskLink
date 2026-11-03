package actions

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32            = syscall.NewLazyDLL("user32.dll")
	shell32           = syscall.NewLazyDLL("shell32.dll")
	procKeybdEvent    = user32.NewProc("keybd_event")
	procLockWorkStn   = user32.NewProc("LockWorkStation")
	procShellExecuteW = shell32.NewProc("ShellExecuteW")

	procSetSuspendState = syscall.NewLazyDLL("powrprof.dll").NewProc("SetSuspendState")
)

const (
	keyeventfExtendedKey = 0x0001
	keyeventfKeyUp       = 0x0002
	swShowNormal         = 1
)

// Run executes a validated action.
func Run(a Action) error {
	switch a.Type {
	case "launch":
		return shellOpen(expandEnv(a.Target), expandEnv(a.Args))
	case "keys":
		pressCombo(a.Keys)
		return nil
	case "system":
		switch a.Command {
		case "lock":
			if r, _, err := procLockWorkStn.Call(); r == 0 {
				return fmt.Errorf("LockWorkStation: %v", err)
			}
		case "sleep":
			// Reply to the phone first; the PC is asleep a moment later.
			go func() {
				time.Sleep(500 * time.Millisecond)
				procSetSuspendState.Call(0, 0, 0) // sleep (not hibernate), no forced suspend
			}()
		}
		return nil
	}
	return fmt.Errorf("unsupported action %q", a.Type)
}

func shellOpen(target, args string) error {
	verb, _ := syscall.UTF16PtrFromString("open")
	file, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	var params, dir *uint16
	if args != "" {
		if params, err = syscall.UTF16PtrFromString(args); err != nil {
			return err
		}
	}
	// Programs (games especially) expect to start in their own folder.
	if st, err := os.Stat(target); err == nil && !st.IsDir() {
		if dir, err = syscall.UTF16PtrFromString(filepath.Dir(target)); err != nil {
			return err
		}
	}
	r, _, _ := procShellExecuteW.Call(0,
		uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(params)), uintptr(unsafe.Pointer(dir)), swShowNormal)
	// ShellExecute returns a value greater than 32 on success.
	if r <= 32 {
		return fmt.Errorf("could not open %q (ShellExecute error %d)", target, r)
	}
	return nil
}

// pressCombo holds keys down in order and releases them in reverse.
func pressCombo(keys []string) {
	for _, k := range keys {
		keyEvent(keyCodes[k], 0)
	}
	time.Sleep(10 * time.Millisecond)
	for i := len(keys) - 1; i >= 0; i-- {
		keyEvent(keyCodes[keys[i]], keyeventfKeyUp)
	}
}

func keyEvent(vk uint8, flags uintptr) {
	if vk >= 0xA6 && vk <= 0xB7 || vk >= 0x21 && vk <= 0x2E || vk == 0x5B {
		flags |= keyeventfExtendedKey
	}
	procKeybdEvent.Call(uintptr(vk), 0, flags, 0)
}
