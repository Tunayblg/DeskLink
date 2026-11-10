package keyboard

import (
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

var (
	user32    = syscall.NewLazyDLL("user32.dll")
	sendInput = user32.NewProc("SendInput")
)

const (
	inputKeyboard   = 1
	keyeventUnicode = 0x0004
	keyeventKeyUp   = 0x0002
	vkBack          = 0x08
	vkReturn        = 0x0D
	vkShift         = 0x10
	flushEvery      = 64 // UTF-16 units per SendInput call
)

// kbInput is the Win32 INPUT structure for a keyboard event: type, padding,
// then KEYBDINPUT padded to the size of the union (MOUSEINPUT is the largest).
type kbInput struct {
	typ   uint32
	_     uint32
	vk    uint16
	scan  uint16
	flags uint32
	time  uint32
	_     uint32
	extra uintptr
	_     [8]byte
}

// send gives the events to Windows and reports whether it took all of them.
// It takes none for a window that runs as administrator while DeskLink does not.
func send(events []kbInput) bool {
	if len(events) == 0 {
		return true
	}
	n, _, _ := sendInput.Call(uintptr(len(events)), uintptr(unsafe.Pointer(&events[0])), unsafe.Sizeof(events[0]))
	return int(n) == len(events)
}

func backspace(n int) bool {
	ok := true
	var batch []kbInput
	for i := 0; i < n; i++ {
		batch = append(batch,
			kbInput{typ: inputKeyboard, vk: vkBack},
			kbInput{typ: inputKeyboard, vk: vkBack, flags: keyeventKeyUp})
		if len(batch) >= flushEvery*2 {
			ok = send(batch) && ok
			batch = batch[:0]
			time.Sleep(3 * time.Millisecond)
		}
	}
	return send(batch) && ok
}

func typeText(text string) bool {
	ok := true
	var batch []kbInput
	flush := func() {
		ok = send(batch) && ok
		batch = batch[:0]
		time.Sleep(3 * time.Millisecond) // let the window keep up
	}
	for _, r := range text {
		if r == '\n' { // a new line inside the prompt, not a send
			flush()
			ok = send([]kbInput{
				{typ: inputKeyboard, vk: vkShift},
				{typ: inputKeyboard, vk: vkReturn},
				{typ: inputKeyboard, vk: vkReturn, flags: keyeventKeyUp},
				{typ: inputKeyboard, vk: vkShift, flags: keyeventKeyUp},
			}) && ok
			continue
		}
		// A character outside the first plane is two UTF-16 units.
		for _, u := range utf16.Encode([]rune{r}) {
			batch = append(batch,
				kbInput{typ: inputKeyboard, scan: u, flags: keyeventUnicode},
				kbInput{typ: inputKeyboard, scan: u, flags: keyeventUnicode | keyeventKeyUp})
		}
		if len(batch) >= flushEvery*2 {
			flush()
		}
	}
	flush()
	return ok
}
