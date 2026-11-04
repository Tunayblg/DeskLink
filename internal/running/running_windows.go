package running

import (
	"syscall"
	"unsafe"
)

func names() map[string]bool {
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer syscall.CloseHandle(snap)

	out := map[string]bool{}
	var e syscall.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	if err := syscall.Process32First(snap, &e); err != nil {
		return nil
	}
	for {
		out[Name(syscall.UTF16ToString(e.ExeFile[:]))] = true
		if err := syscall.Process32Next(snap, &e); err != nil {
			break
		}
	}
	return out
}
