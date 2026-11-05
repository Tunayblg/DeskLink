package brightness

import (
	"sync"
	"testing"
	"time"
)

func TestScaleAndPercent(t *testing.T) {
	if got := scale(50, 0, 100); got != 50 {
		t.Errorf("scale(50, 0, 100) = %d", got)
	}
	if got := scale(100, 10, 90); got != 90 {
		t.Errorf("scale(100, 10, 90) = %d", got)
	}
	if got := scale(0, 10, 90); got != 10 {
		t.Errorf("scale(0, 10, 90) = %d", got)
	}
	if got := percent(50, 10, 90); got != 50 {
		t.Errorf("percent(50, 10, 90) = %d", got)
	}
	if got := percent(5, 10, 90); got != 0 {
		t.Errorf("percent below the range = %d", got)
	}
	if got := scale(40, 5, 5); got != 5 {
		t.Errorf("flat range = %d", got)
	}
}

// A burst of requests ends on the last level, and the ones in between may be skipped.
func TestSetterKeepsTheLatest(t *testing.T) {
	var mu sync.Mutex
	var seen []int
	s := &Setter{apply: func(l int) {
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		seen = append(seen, l)
		mu.Unlock()
	}}
	for i := 0; i <= 100; i += 10 {
		s.Set(i)
	}
	s.Set(250) // clamped
	time.Sleep(300 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(seen) == 0 || seen[len(seen)-1] != 100 {
		t.Fatalf("applied %v, want it to end on 100", seen)
	}
	if len(seen) > 4 {
		t.Errorf("applied %d times, expected the burst to be coalesced", len(seen))
	}
}
