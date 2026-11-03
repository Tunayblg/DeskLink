package stats

import (
	"runtime"
	"testing"
	"time"
)

func TestSample(t *testing.T) {
	var c Collector
	c.Sample()
	time.Sleep(cacheFor + 50*time.Millisecond)
	s := c.Sample()
	if s.CPU.Usage < 0 || s.CPU.Usage > 100 {
		t.Errorf("CPU usage %v out of range", s.CPU.Usage)
	}
	if runtime.GOOS == "windows" && (s.RAM.TotalMB == 0 || s.RAM.UsedMB > s.RAM.TotalMB) {
		t.Errorf("bad RAM reading %+v", s.RAM)
	}
	t.Logf("%+v gpu=%+v", s, s.GPU)
}
