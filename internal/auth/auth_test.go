package auth

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func newStore(t *testing.T) (*Store, *time.Time) {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "devices.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	s.now = func() time.Time { return now }
	return s, &now
}

func TestPairWithCodes(t *testing.T) {
	s, now := newStore(t)

	c := s.Current()
	if len(c.Digits) != 6 || len(c.Long) != 16 {
		t.Fatalf("unexpected code shape %+v", c)
	}
	token, handoff, err := s.Pair(c.Digits, "iPhone", "10.0.0.2")
	if err != nil || token == "" || handoff == "" {
		t.Fatalf("pair with digits: %v", err)
	}
	if d, ok := s.Verify(token); !ok || d.Name != "iPhone" {
		t.Fatalf("Verify = %+v, %v", d, ok)
	}

	// The long (QR) form works, and the previous window is still accepted.
	*now = now.Add(Window)
	if _, _, err := s.Pair(c.Long, "iPad", "10.0.0.2"); err != nil {
		t.Fatalf("previous window long code: %v", err)
	}
	// Two windows later it is expired.
	*now = now.Add(Window)
	if _, _, err := s.Pair(c.Digits, "x", "10.0.0.2"); err != ErrBadCode {
		t.Fatalf("expired code: got %v", err)
	}
	if s.Current().Digits == c.Digits && s.Current().Long == c.Long {
		t.Fatal("code did not change between windows")
	}
}

func TestHandoff(t *testing.T) {
	s, now := newStore(t)
	token, handoff, _ := s.Pair(s.Current().Digits, "iPhone", "10.0.0.2")

	got, err := s.Redeem(handoff)
	if err != nil || got != token {
		t.Fatalf("Redeem = %q, %v", got, err)
	}
	if _, err := s.Redeem(handoff); err == nil {
		t.Fatal("handoff worked twice")
	}

	_, h2, _ := s.Pair(s.Current().Digits, "iPhone", "10.0.0.2")
	*now = now.Add(HandoffTTL + time.Second)
	if _, err := s.Redeem(h2); err == nil {
		t.Fatal("expired handoff accepted")
	}
}

func TestTooManyWrongCodes(t *testing.T) {
	s, now := newStore(t)
	for i := 0; i < MaxAttempts; i++ {
		if _, _, err := s.Pair("abcdef", "x", "10.0.0.2"); err != ErrBadCode {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	// Locked for the rest of the window, even with the right code.
	if _, _, err := s.Pair(s.Current().Digits, "x", "10.0.0.2"); err != ErrTooMany {
		t.Fatalf("got %v, want ErrTooMany", err)
	}
	*now = now.Add(Window)
	if _, _, err := s.Pair(s.Current().Digits, "x", "10.0.0.2"); err != nil {
		t.Fatalf("next window: %v", err)
	}
}

func TestWrongCodesAreCountedPerDevice(t *testing.T) {
	s, _ := newStore(t)
	// A stranger burns its own limit...
	for i := 0; i < MaxAttempts; i++ {
		s.Pair("abcdef", "x", "10.0.0.99")
	}
	if _, _, err := s.Pair("abcdef", "x", "10.0.0.99"); err != ErrTooMany {
		t.Fatalf("stranger: got %v, want ErrTooMany", err)
	}
	// ...but the owner's tablet can still pair.
	if _, _, err := s.Pair(s.Current().Digits, "iPad", "10.0.0.2"); err != nil {
		t.Fatalf("owner locked out by a stranger: %v", err)
	}
}

func TestWrongCodesFromManyAddressesHaveACeiling(t *testing.T) {
	s, _ := newStore(t)
	for i := 0; i < MaxAttemptsAll; i++ {
		s.Pair("abcdef", "x", fmt.Sprintf("10.0.1.%d", i))
	}
	if _, _, err := s.Pair(s.Current().Digits, "x", "10.0.0.2"); err != ErrTooMany {
		t.Fatalf("got %v, want ErrTooMany", err)
	}
}

func TestRotateAndRevoke(t *testing.T) {
	s, _ := newStore(t)
	old := s.Current()
	s.Rotate()
	if _, _, err := s.Pair(old.Digits, "x", "10.0.0.2"); err != ErrBadCode {
		t.Fatalf("code survived Rotate: %v", err)
	}

	token, handoff, _ := s.Pair(s.Current().Digits, "iPhone", "10.0.0.2")
	d, _ := s.Verify(token)
	if ok, err := s.Revoke(d.ID()); !ok || err != nil {
		t.Fatalf("Revoke = %v, %v", ok, err)
	}
	if _, ok := s.Verify(token); ok {
		t.Fatal("revoked token still accepted")
	}
	if _, err := s.Redeem(handoff); err == nil {
		t.Fatal("handoff for a removed device worked")
	}
	if s2, _ := Open(s.path); len(s2.Devices()) != 0 {
		t.Fatal("revocation not saved")
	}
}
