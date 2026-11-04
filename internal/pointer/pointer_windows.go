package pointer

import (
	"syscall"
	"unsafe"
)

var (
	user32    = syscall.NewLazyDLL("user32.dll")
	sendInput = user32.NewProc("SendInput")
)

const (
	inputMouse = 0

	eventMove       = 0x0001
	eventLeftDown   = 0x0002
	eventLeftUp     = 0x0004
	eventRightDown  = 0x0008
	eventRightUp    = 0x0010
	eventMiddleDown = 0x0020
	eventMiddleUp   = 0x0040
	eventWheel      = 0x0800
	eventHWheel     = 0x1000
)

// input is the Win32 INPUT structure for a mouse event (type, then the union
// padded to the size of the largest member).
type input struct {
	typ  uint32
	_    uint32 // padding on 64-bit so the union is 8-byte aligned
	dx   int32
	dy   int32
	data uint32
	flag uint32
	time uint32
	_    uint32
	info uintptr
}

func send(flags uint32, dx, dy int32, data int32) {
	in := input{typ: inputMouse, dx: dx, dy: dy, data: uint32(data), flag: flags}
	sendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
}

func move(dx, dy int) { send(eventMove, int32(dx), int32(dy), 0) }

func button(name string, down bool) {
	var down_, up uint32
	switch name {
	case "left":
		down_, up = eventLeftDown, eventLeftUp
	case "right":
		down_, up = eventRightDown, eventRightUp
	case "middle":
		down_, up = eventMiddleDown, eventMiddleUp
	default:
		return
	}
	if down {
		send(down_, 0, 0, 0)
	} else {
		send(up, 0, 0, 0)
	}
}

func wheel(horizontal bool, delta int) {
	if horizontal {
		send(eventHWheel, 0, 0, int32(delta))
	} else {
		send(eventWheel, 0, 0, int32(delta))
	}
}
