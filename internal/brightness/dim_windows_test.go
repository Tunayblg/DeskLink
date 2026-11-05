package brightness

import (
	"os"
	"testing"
	"time"
)

func TestAlphaFor(t *testing.T) {
	for _, c := range []struct {
		level int
		want  uint8
	}{{100, 0}, {50, 127}, {10, 229}, {0, 229}, {-5, 229}, {250, 0}} {
		if got := alphaFor(c.level); got != c.want {
			t.Errorf("alphaFor(%d) = %d, want %d", c.level, got, c.want)
		}
	}
}

// The overlay really appears, is only as dark as asked, and goes away again.
// It is dimmed by 10 percent only, so it is barely visible on the machine running the tests.
func TestOverlayShowsAndHides(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("needs a desktop session; CI runners have none")
	}
	softSet(90)
	deadline := time.Now().Add(2 * time.Second)
	var visible bool
	var alpha uint8
	for time.Now().Before(deadline) {
		if visible, alpha = dimState(); visible {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !visible || alpha != alphaFor(90) {
		t.Fatalf("overlay: visible=%v alpha=%d, want visible with alpha %d", visible, alpha, alphaFor(90))
	}
	if softGet() != 90 {
		t.Errorf("softGet = %d", softGet())
	}
	softSet(100)
	for time.Now().Before(deadline.Add(2 * time.Second)) {
		if visible, _ = dimState(); !visible {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("overlay is still showing at 100%")
}

// Set DESKLINK_VISUAL=1 to dim the real screen to 50% for six seconds, e.g. to take a screenshot.
func TestOverlayVisual(t *testing.T) {
	if os.Getenv("DESKLINK_VISUAL") == "" {
		t.Skip("set DESKLINK_VISUAL=1 to run")
	}
	softSet(50)
	time.Sleep(6 * time.Second)
	softSet(100)
	time.Sleep(300 * time.Millisecond)
}
