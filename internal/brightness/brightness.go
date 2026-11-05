// Package brightness reads and sets the PC's screen brightness for the phone.
//
// It works on any screen, using the best way available: an external monitor is
// driven over DDC/CI (the same thing its own buttons do), a laptop panel
// through Windows' built-in brightness control, and anything else is dimmed in
// software with a see-through black window (see dim_windows.go). On other
// systems nothing can be controlled and the phone hides the slider.
package brightness

import "sync"

// Get returns the brightness as a percentage, and false when no screen can be read.
func Get() (int, bool) { return get() }

// A Setter applies the latest level asked for and drops the ones in between,
// because talking to a monitor takes a good fraction of a second.
type Setter struct {
	mu      sync.Mutex
	pending int
	has     bool
	running bool
	apply   func(level int)
}

// NewSetter returns a setter that drives the real screens.
func NewSetter() *Setter { return &Setter{apply: set} }

// Set asks for a level (0-100). It returns at once.
func (s *Setter) Set(level int) {
	level = min(max(level, 0), 100)
	s.mu.Lock()
	s.pending, s.has = level, true
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()
	go s.loop()
}

func (s *Setter) loop() {
	for {
		s.mu.Lock()
		if !s.has {
			s.running = false
			s.mu.Unlock()
			return
		}
		level := s.pending
		s.has = false
		s.mu.Unlock()
		s.apply(level)
	}
}

// scale maps a percentage onto a monitor's own range.
func scale(percent int, lo, hi uint32) uint32 {
	if hi <= lo {
		return lo
	}
	return lo + uint32(percent)*(hi-lo)/100
}

// percent is the opposite of scale.
func percent(cur, lo, hi uint32) int {
	if hi <= lo || cur <= lo {
		return 0
	}
	return int((cur - lo) * 100 / (hi - lo))
}
