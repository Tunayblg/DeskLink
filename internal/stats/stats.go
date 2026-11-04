// Package stats samples CPU, RAM and GPU usage for the game mode panel.
//
// Nothing is sampled unless a phone has the game mode page open, and samples
// are cached briefly so several phones share one reading.
package stats

import (
	"sync"
	"time"
)

// Snapshot is one reading. GPU is nil when no supported GPU is found.
type Snapshot struct {
	CPU CPU  `json:"cpu"`
	RAM RAM  `json:"ram"`
	GPU *GPU `json:"gpu,omitempty"`
}

type CPU struct {
	Usage float64 `json:"usage"` // percent, all cores
}

type RAM struct {
	UsedMB  uint64  `json:"used_mb"`
	TotalMB uint64  `json:"total_mb"`
	Usage   float64 `json:"usage"`
}

type GPU struct {
	Name        string  `json:"name"`
	Usage       float64 `json:"usage"`
	Temp        float64 `json:"temp"` // °C
	Fan         float64 `json:"fan"`  // percent of max
	VRAMUsedMB  uint64  `json:"vram_used_mb"`
	VRAMTotalMB uint64  `json:"vram_total_mb"`
	PowerW      float64 `json:"power_w"`
	PowerLimitW float64 `json:"power_limit_w"`
	ClockMHz    uint32  `json:"clock_mhz"`
}

const cacheFor = 800 * time.Millisecond

// Collector takes readings. The zero value is ready to use.
type Collector struct {
	mu    sync.Mutex
	at    time.Time // when CPU and RAM were last read
	gpuAt time.Time // when the GPU was last read
	last  Snapshot
	cpu   cpuSampler
	gpu   gpuSampler
}

// refresh reads CPU and RAM unless that was done very recently.
func (c *Collector) refresh() {
	if time.Since(c.at) < cacheFor {
		return
	}
	c.last.CPU, c.last.RAM = c.cpu.sample(), sampleRAM()
	c.at = time.Now()
}

// Sample returns a fresh reading of everything, or the cached one if it is
// very recent.
func (c *Collector) Sample() Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.refresh()
	if time.Since(c.gpuAt) >= cacheFor {
		c.last.GPU = c.gpu.sample()
		c.gpuAt = time.Now()
	}
	return c.last
}

// System returns only CPU and RAM, for the PC dashboard. It shares the cache
// with Sample, and never touches the GPU.
func (c *Collector) System() Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.refresh()
	return Snapshot{CPU: c.last.CPU, RAM: c.last.RAM}
}
