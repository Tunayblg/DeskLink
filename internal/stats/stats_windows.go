package stats

import (
	"log"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemTimes       = kernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
)

// cpuSampler turns the cumulative idle/kernel/user times into a usage percent
// since the previous call.
type cpuSampler struct {
	idle, total uint64
}

func (s *cpuSampler) sample() CPU {
	var idle, kernel, user syscall.Filetime
	r, _, _ := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)))
	if r == 0 {
		return CPU{}
	}
	i := ft(idle)
	t := ft(kernel) + ft(user) // kernel time includes idle time
	di, dt := i-s.idle, t-s.total
	s.idle, s.total = i, t
	if dt == 0 {
		return CPU{}
	}
	return CPU{Usage: round1(100 * float64(dt-di) / float64(dt))}
}

func ft(f syscall.Filetime) uint64 { return uint64(f.HighDateTime)<<32 | uint64(f.LowDateTime) }

type memoryStatusEx struct {
	length, memoryLoad                 uint32
	totalPhys, availPhys               uint64
	totalPageFile, availPageFile       uint64
	totalVirtual, availVirtual, availX uint64
}

func sampleRAM() RAM {
	m := memoryStatusEx{length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	if r, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&m))); r == 0 {
		return RAM{}
	}
	used := m.totalPhys - m.availPhys
	return RAM{
		UsedMB:  used >> 20,
		TotalMB: m.totalPhys >> 20,
		Usage:   round1(100 * float64(used) / float64(m.totalPhys)),
	}
}

// gpuSampler reads the first NVIDIA GPU through NVML, which ships with the
// NVIDIA driver. The DLL is only loaded the first time game mode is opened.
type gpuSampler struct {
	tried  bool
	ok     bool
	handle uintptr
	name   string
	procs  map[string]*syscall.LazyProc
}

func (g *gpuSampler) init() {
	g.tried = true
	dll := syscall.NewLazyDLL("nvml.dll")
	if dll.Load() != nil {
		log.Printf("stats: nvml.dll not found, GPU stats disabled (only NVIDIA GPUs are supported for now)")
		return
	}
	g.procs = map[string]*syscall.LazyProc{}
	for _, n := range []string{"nvmlInit_v2", "nvmlDeviceGetHandleByIndex_v2", "nvmlDeviceGetName",
		"nvmlDeviceGetUtilizationRates", "nvmlDeviceGetTemperature", "nvmlDeviceGetFanSpeed",
		"nvmlDeviceGetMemoryInfo", "nvmlDeviceGetPowerUsage", "nvmlDeviceGetEnforcedPowerLimit",
		"nvmlDeviceGetClockInfo"} {
		g.procs[n] = dll.NewProc(n)
	}
	if g.call("nvmlInit_v2") != 0 || g.call("nvmlDeviceGetHandleByIndex_v2", 0, ptr(&g.handle)) != 0 {
		log.Printf("stats: NVML init failed, GPU stats disabled")
		return
	}
	name := make([]byte, 96)
	g.call("nvmlDeviceGetName", g.handle, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
	g.name = strings.TrimRight(string(name), "\x00 ")
	g.ok = true
}

// call returns the NVML status code (0 is success).
func (g *gpuSampler) call(name string, args ...uintptr) uintptr {
	p := g.procs[name]
	if p.Find() != nil {
		return 1
	}
	r, _, _ := p.Call(args...)
	return r
}

func (g *gpuSampler) sample() *GPU {
	if !g.tried {
		g.init()
	}
	if !g.ok {
		return nil
	}
	var (
		util         [2]uint32 // gpu, memory
		temp, fan    uint32
		mem          [3]uint64 // total, free, used
		power, limit uint32    // milliwatts
		clock        uint32
		h            = g.handle
	)
	g.call("nvmlDeviceGetUtilizationRates", h, ptr(&util))
	g.call("nvmlDeviceGetTemperature", h, 0, ptr(&temp)) // 0 = GPU core sensor
	g.call("nvmlDeviceGetFanSpeed", h, ptr(&fan))
	g.call("nvmlDeviceGetMemoryInfo", h, ptr(&mem))
	g.call("nvmlDeviceGetPowerUsage", h, ptr(&power))
	g.call("nvmlDeviceGetEnforcedPowerLimit", h, ptr(&limit))
	g.call("nvmlDeviceGetClockInfo", h, 0, ptr(&clock)) // 0 = graphics clock
	return &GPU{
		Name:        g.name,
		Usage:       float64(util[0]),
		Temp:        float64(temp),
		Fan:         float64(fan),
		VRAMUsedMB:  mem[2] >> 20,
		VRAMTotalMB: mem[0] >> 20,
		PowerW:      round1(float64(power) / 1000),
		PowerLimitW: round1(float64(limit) / 1000),
		ClockMHz:    clock,
	}
}

func ptr[T any](v *T) uintptr { return uintptr(unsafe.Pointer(v)) }

func round1(f float64) float64 { return float64(int(f*10+0.5)) / 10 }
