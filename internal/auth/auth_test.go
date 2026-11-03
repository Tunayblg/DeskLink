package auth

import (
	"path/filepath"
	"testing"
)

func TestPairAndVerify(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices.json")
	s, err := Open(path, nil)
	if err != nil {
		t.Fatal(err)
	}

	pin := s.PIN()
	token, err := s.Pair(pin, "iPhone")
	if err != nil {
		t.Fatalf("pair with correct PIN: %v", err)
	}
	if d, ok := s.Verify(token); !ok || d.Name != "iPhone" {
		t.Fatalf("Verify(token) = %+v, %v", d, ok)
	}
	tampered := token[:63] + "0"
	if token[63] == '0' {
		tampered = token[:63] + "1"
	}
	if _, ok := s.Verify(tampered); ok {
		t.Fatal("tampered token accepted")
	}

	// Tokens survive a restart.
	s2, err := Open(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := s2.Verify(token)
	if !ok {
		t.Fatal("token not accepted after reload")
	}

	if ok, err := s2.Revoke(d.ID()); !ok || err != nil {
		t.Fatalf("Revoke = %v, %v", ok, err)
	}
	if _, ok := s2.Verify(token); ok {
		t.Fatal("revoked token still accepted")
	}
	if s3, _ := Open(path, nil); len(s3.Devices()) != 0 {
		t.Fatal("revocation not saved")
	}
}

func TestWrongPINRotates(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "devices.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	rotated := 0
	s.OnPIN = func(string) { rotated++ }

	wrong := "abcdef" // never a valid PIN
	for i := 1; i < MaxAttempts; i++ {
		if _, err := s.Pair(wrong, "x"); err != ErrBadPIN {
			t.Fatalf("attempt %d: got %v, want ErrBadPIN", i, err)
		}
	}
	if _, err := s.Pair(wrong, "x"); err != ErrPINRotate {
		t.Fatalf("got %v, want ErrPINRotate", err)
	}
	if rotated != 1 {
		t.Fatalf("PIN rotated %d times, want 1", rotated)
	}
}
