package pointer

import (
	"fmt"
	"testing"
)

func testPad() (*Pad, *[]string) {
	var log []string
	p := &Pad{held: map[string]bool{},
		move:   func(dx, dy int) { log = append(log, fmt.Sprintf("move %d %d", dx, dy)) },
		button: func(n string, d bool) { log = append(log, fmt.Sprintf("%s %v", n, d)) },
		wheel:  func(h bool, d int) { log = append(log, fmt.Sprintf("wheel h=%v %d", h, d)) }}
	return p, &log
}

func TestEvents(t *testing.T) {
	p, log := testPad()
	for _, e := range []Event{
		{Action: "move", DX: 5, DY: -3},
		{Action: "move", DX: 100000, DY: -100000}, // clamped
		{Action: "move"},                          // nothing to do
		{Action: "click", Button: "right"},
		{Action: "click", Button: "evil"},
		{Action: "down", Button: "left"},
		{Action: "down", Button: "left"},  // already held
		{Action: "click", Button: "left"}, // ignored while held
		{Action: "up", Button: "left"},
		{Action: "up", Button: "left"}, // not held
		{Action: "scroll", DY: -50000},
		{Action: "hscroll", DY: 120},
		{Action: "format-c"},
	} {
		p.Do(e)
	}
	want := "[move 5 -3 move 400 -400 right true right false left true left false wheel h=false -1200 wheel h=true 120]"
	if got := fmt.Sprint(*log); got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestReleaseLetsGoOfHeldButtons(t *testing.T) {
	p, log := testPad()
	p.Do(Event{Action: "down", Button: "left"})
	p.Do(Event{Action: "down", Button: "right"})
	p.Release()
	p.Release()
	released := 0
	for _, l := range *log {
		if l == "left false" || l == "right false" {
			released++
		}
	}
	if released != 2 || len(p.held) != 0 {
		t.Fatalf("log %v held %v", *log, p.held)
	}
}

func TestHolding(t *testing.T) {
	p, _ := testPad()
	if p.Holding() {
		t.Fatal("nothing held yet")
	}
	p.Do(Event{Action: "down", Button: "left"})
	if !p.Holding() {
		t.Fatal("left should be held")
	}
	p.Release()
	if p.Holding() {
		t.Fatal("release should let go")
	}
}
