package brightness

import "testing"

// Two monitors set up differently keep their balance when the slider moves.
func TestTargetKeepsEachMonitorsOwnSetting(t *testing.T) {
	// Slider at 70 (the brightest monitor); the other one is at 30.
	if got := targetFor(70, 70, 70); got != 70 {
		t.Errorf("at the reference: %d", got)
	}
	if got := targetFor(30, 70, 70); got != 30 {
		t.Errorf("other monitor at the reference: %d", got)
	}
	if got := targetFor(30, 35, 70); got != 15 {
		t.Errorf("halved: %d", got)
	}
	if got := targetFor(70, 100, 70); got != 100 {
		t.Errorf("full: %d", got)
	}
	if got := targetFor(70, 200, 70); got != 100 {
		t.Errorf("never above the monitor's maximum: %d", got)
	}
}

func TestCalibrationNoticesButtonsOnTheMonitor(t *testing.T) {
	mons := []ddcMon{{key: "0:A", lo: 0, cur: 40, hi: 100}, {key: "1:B", lo: 0, cur: 70, hi: 100}}
	calibrate(mons, 70)
	if !cal.calibrated(mons) {
		t.Fatal("not calibrated right after calibrating")
	}
	mons[0].cur = 80 // someone used the monitor's own buttons
	if cal.calibrated(mons) {
		t.Fatal("a change made on the monitor went unnoticed")
	}
	if cal.calibrated(mons[:1]) {
		t.Fatal("a monitor going away went unnoticed")
	}
	if got := brightest(mons); got != 80 {
		t.Errorf("brightest = %d", got)
	}
}
