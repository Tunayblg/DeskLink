package stats

import (
	"testing"
	"time"
)

func TestNetSamplerReadsAdapters(t *testing.T) {
	var s netSampler
	s.sample() // the first reading only stores the counters
	time.Sleep(300 * time.Millisecond)
	n := s.sample()
	if n == nil {
		t.Skip("no active Ethernet or Wi-Fi adapter on this machine")
	}
	if n.DownBps < 0 || n.UpBps < 0 {
		t.Fatalf("negative speed: %+v", n)
	}
	t.Logf("down %d B/s, up %d B/s", n.DownBps, n.UpBps)
}
