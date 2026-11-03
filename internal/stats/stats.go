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
	mu   sync.Mutex
	at   time.Time
	last Snapshot
	cpu  cpuSampler
	gpu  gpuSampler
}

// Sample returns a fresh reading, or the cached one if it is very recent.
func (c *Collector) Sample() Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.at) < cacheFor {
		return c.last
	}
	c.last = Snapshot{CPU: c.cpu.sample(), RAM: sampleRAM(), GPU: c.gpu.sample()}
	c.at = time.Now()
	return c.last
}
