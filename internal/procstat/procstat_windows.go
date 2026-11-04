package procstat

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	procQueryWorkingSet = kernel32.NewProc("K32QueryWorkingSet")
	procGetProcessTimes = kernel32.NewProc("GetProcessTimes")
	procGetCurrent      = kernel32.NewProc("GetCurrentProcess")
	procMemoryInfo      = kernel32.NewProc("K32GetProcessMemoryInfo")
)

const pageSize = 4096

// memory counts the pages in our working set, and how many of them are not
// shared with other processes. This is how Task Manager computes "Memory".
func memory() (private, total uint64) {
	h, _, _ := procGetCurrent.Call()
	n := 16384
	for tries := 0; tries < 4; tries++ {
		// PSAPI_WORKING_SET_INFORMATION: a count followed by one word per page.
		buf := make([]uintptr, n+1)
		r, _, _ := procQueryWorkingSet.Call(h, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))*unsafe.Sizeof(buf[0]))
		count := int(buf[0])
		if r == 0 && count > n { // buffer too small
			n = count + 1024
			continue
		}
		if r == 0 {
			return 0, 0
		}
		for _, e := range buf[1 : count+1] {
			if e&(1<<8) == 0 { // Shared bit
				private++
			}
		}
		return private * pageSize, uint64(count) * pageSize
	}
	return 0, 0
}

func cpuTime() time.Duration {
	h, _, _ := procGetCurrent.Call()
	var creation, exit, kernel, user syscall.Filetime
	procGetProcessTimes.Call(h, uintptr(unsafe.Pointer(&creation)), uintptr(unsafe.Pointer(&exit)),
		uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)))
	ticks := (uint64(kernel.HighDateTime)<<32 | uint64(kernel.LowDateTime)) +
		(uint64(user.HighDateTime)<<32 | uint64(user.LowDateTime))
	return time.Duration(ticks * 100)
}

// processMemoryCounters is PROCESS_MEMORY_COUNTERS.
type processMemoryCounters struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
}

// peakWorkingSet is the most memory this process has had resident since it
// started, as tracked by Windows (so it also covers moments nobody looked).
func peakWorkingSet() uint64 {
	h, _, _ := procGetCurrent.Call()
	c := processMemoryCounters{cb: uint32(unsafe.Sizeof(processMemoryCounters{}))}
	if r, _, _ := procMemoryInfo.Call(h, uintptr(unsafe.Pointer(&c)), uintptr(c.cb)); r == 0 {
		return 0
	}
	return uint64(c.peakWorkingSetSize)
}
