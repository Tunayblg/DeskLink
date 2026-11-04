// Package pointer moves the PC's mouse for the phone's trackpad screen.
//
// The phone sends small relative events (move, button down/up, click, scroll).
// Everything is clamped here, and a Pad remembers which buttons the phone is
// holding so they are released if the connection drops mid-drag.
package pointer

import "sync"

// Event is one message from the phone's trackpad.
type Event struct {
	Action string // move, down, up, click, scroll, hscroll
	Button string // left, right, middle (for down, up, click)
	DX, DY int    // move: pixels; scroll/hscroll: wheel units (120 = one notch)
}

const (
	maxMove   = 400  // pixels per event
	maxScroll = 1200 // wheel units per event
)

var buttons = map[string]bool{"left": true, "right": true, "middle": true}

// Pad is one phone's trackpad. The WebSocket loop drives it, and a watchdog
// may Release it from another goroutine, so it locks.
type Pad struct {
	mu   sync.Mutex
	held map[string]bool

	// The platform calls; replaced in tests.
	move   func(dx, dy int)
	button func(name string, down bool)
	wheel  func(horizontal bool, delta int)
}

// NewPad returns a pad that drives the real mouse.
func NewPad() *Pad {
	return &Pad{held: map[string]bool{}, move: move, button: button, wheel: wheel}
}

func clamp(v, limit int) int { return min(max(v, -limit), limit) }

// Do applies one event. Unknown actions and buttons are ignored.
func (p *Pad) Do(e Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch e.Action {
	case "move":
		if e.DX != 0 || e.DY != 0 {
			p.move(clamp(e.DX, maxMove), clamp(e.DY, maxMove))
		}
	case "down":
		if buttons[e.Button] && !p.held[e.Button] {
			p.held[e.Button] = true
			p.button(e.Button, true)
		}
	case "up":
		if p.held[e.Button] {
			delete(p.held, e.Button)
			p.button(e.Button, false)
		}
	case "click":
		if buttons[e.Button] && !p.held[e.Button] {
			p.button(e.Button, true)
			p.button(e.Button, false)
		}
	case "scroll":
		if e.DY != 0 {
			p.wheel(false, clamp(e.DY, maxScroll))
		}
	case "hscroll":
		if e.DY != 0 {
			p.wheel(true, clamp(e.DY, maxScroll))
		}
	}
}

// Release lets go of every button the phone is holding.
func (p *Pad) Release() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for b := range p.held {
		p.button(b, false)
		delete(p.held, b)
	}
}

// Holding reports whether the phone is holding a button down.
func (p *Pad) Holding() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.held) > 0
}
