package procstat

import (
	"runtime"
	"testing"
)

func TestRead(t *testing.T) {
	var m Meter
	m.Read()
	u := m.Read()
	if runtime.GOOS == "windows" && (u.PrivateMB <= 0 || u.WorkingSetMB < u.PrivateMB) {
		t.Fatalf("implausible memory reading %+v", u)
	}
	if runtime.GOOS == "windows" && u.PeakWSMB < u.WorkingSetMB {
		t.Fatalf("peak working set %v below the current %v", u.PeakWSMB, u.WorkingSetMB)
	}
	if u.CPU < 0 || u.Goroutines < 1 {
		t.Fatalf("implausible reading %+v", u)
	}
	t.Logf("%+v", u)
}
