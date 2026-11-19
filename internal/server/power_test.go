package server

import (
	"path/filepath"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Tunayblg/DeskLink/internal/actions"
	"github.com/Tunayblg/DeskLink/internal/auth"
	"github.com/Tunayblg/DeskLink/internal/config"
)

func newPowerServer(t *testing.T) (*Server, *atomic.Int32) {
	t.Helper()
	cfg, err := config.Parse([]byte(config.Default))
	if err != nil {
		t.Fatal(err)
	}
	store, err := auth.Open(filepath.Join(t.TempDir(), "devices.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg, store, fstest.MapFS{"index.html": {Data: []byte("ok")}}, nil)
	var runs atomic.Int32
	s.run = func(actions.Action) error { runs.Add(1); return nil }
	return s, &runs
}

// A sleep that waits runs once when its time comes, and shows as a plan until then.
func TestPowerWaitsThenRuns(t *testing.T) {
	s, runs := newPowerServer(t)
	if err := s.schedulePower("sleep", 40*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if m := s.powerMsg(); m.Power == nil || m.Power.Command != "sleep" {
		t.Fatalf("plan not shown: %+v", m.Power)
	}
	time.Sleep(200 * time.Millisecond)
	if runs.Load() != 1 {
		t.Fatalf("ran %d times, want 1", runs.Load())
	}
	if m := s.powerMsg(); m.Power != nil {
		t.Fatalf("plan should be gone after it ran: %+v", m.Power)
	}
}

// A cancelled plan never runs, and a new plan replaces the old one.
func TestPowerCancelAndReplace(t *testing.T) {
	s, runs := newPowerServer(t)
	if err := s.schedulePower("shutdown", 60*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	s.cancelPower()
	time.Sleep(150 * time.Millisecond)
	if runs.Load() != 0 {
		t.Fatalf("a cancelled plan ran %d times", runs.Load())
	}
	if err := s.schedulePower("shutdown", 60*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := s.schedulePower("sleep", 60*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if runs.Load() != 1 {
		t.Fatalf("replaced plan: ran %d times, want 1", runs.Load())
	}
}

// Only sleep and shut down can wait, and not for longer than two hours.
func TestPowerRefusals(t *testing.T) {
	s, _ := newPowerServer(t)
	if err := s.schedulePower("lock", time.Minute); err == nil {
		t.Fatal("lock must not wait")
	}
	if err := s.schedulePower("nope", time.Minute); err == nil {
		t.Fatal("an unknown button must not wait")
	}
	if err := s.schedulePower("sleep", 3*time.Hour); err == nil {
		t.Fatal("three hours is too long")
	}
	if err := s.schedulePower("sleep", 0); err == nil {
		t.Fatal("no delay is not a wait")
	}
}
