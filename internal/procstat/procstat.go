// Package procstat measures DeskLink's own resource use for the PC dashboard.
package procstat

import (
	"runtime"
	"sync"
	"time"
)

// Usage is one reading of this process.
type Usage struct {
	PrivateMB    float64 `json:"private_mb"`     // private working set (Task Manager's "Memory")
	WorkingSetMB float64 `json:"working_set_mb"` // includes shared/image pages
	HeapMB       float64 `json:"heap_mb"`        // live Go heap
	CPU          float64 `json:"cpu"`            // percent of all cores since the last reading
	Goroutines   int     `json:"goroutines"`
}

// Meter keeps the previous CPU times so it can report usage between readings.
type Meter struct {
	mu       sync.Mutex
	lastAt   time.Time
	lastBusy time.Duration
}

func (m *Meter) Read() Usage {
	m.mu.Lock()
	defer m.mu.Unlock()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	u := Usage{HeapMB: mb(ms.HeapAlloc), Goroutines: runtime.NumGoroutine()}
	private, ws := memory()
	u.PrivateMB, u.WorkingSetMB = mb(private), mb(ws)

	now, busy := time.Now(), cpuTime()
	if !m.lastAt.IsZero() {
		if wall := now.Sub(m.lastAt); wall > 0 {
			u.CPU = round1(100 * float64(busy-m.lastBusy) / float64(wall) / float64(runtime.NumCPU()))
		}
	}
	m.lastAt, m.lastBusy = now, busy
	return u
}

func mb(b uint64) float64 { return round1(float64(b) / (1 << 20)) }

func round1(f float64) float64 { return float64(int(f*10+0.5)) / 10 }
