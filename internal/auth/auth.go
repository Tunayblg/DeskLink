// Package auth handles pairing phones with a PIN and verifying their tokens.
//
// Flow: the server shows a 6-digit PIN on the PC. The phone sends that PIN
// once and receives a random 256-bit token, which it keeps in localStorage.
// Only the SHA-256 hash of each token is written to disk.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"
)

// MaxAttempts is how many wrong PINs are accepted before the PIN is rotated.
const MaxAttempts = 5

var (
	ErrBadPIN    = errors.New("wrong PIN")
	ErrPINRotate = errors.New("too many wrong PINs, a new PIN was generated")
)

// Device is a paired phone or tablet.
type Device struct {
	Name      string    `json:"name"`
	TokenHash string    `json:"token_hash"`
	PairedAt  time.Time `json:"paired_at"`
}

// Store keeps the current PIN and the list of paired devices.
type Store struct {
	mu       sync.Mutex
	path     string
	pin      string
	attempts int
	devices  []Device
	// OnPIN is called whenever a new PIN is generated so it can be shown to the user.
	OnPIN func(pin string)
}

// Open loads paired devices from path (a missing file means none).
func Open(path string, onPIN func(string)) (*Store, error) {
	s := &Store{path: path, OnPIN: onPIN}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &s.devices); err != nil {
			return nil, fmt.Errorf("%s is corrupted: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	s.rotatePIN()
	return s, nil
}

// Pair exchanges a correct PIN for a new device token.
func (s *Store) Pair(pin, name string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if subtle.ConstantTimeCompare([]byte(pin), []byte(s.pin)) != 1 {
		s.attempts++
		if s.attempts >= MaxAttempts {
			s.rotatePIN()
			return "", ErrPINRotate
		}
		return "", ErrBadPIN
	}

	token := randomHex(32)
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 40 {
		name = "Device"
	}
	s.devices = append(s.devices, Device{Name: name, TokenHash: hashToken(token), PairedAt: time.Now().UTC()})
	if err := s.save(); err != nil {
		s.devices = s.devices[:len(s.devices)-1]
		return "", err
	}
	// A PIN is single use; the next phone gets a fresh one.
	s.rotatePIN()
	return token, nil
}

// Verify reports whether token belongs to a paired device and returns it.
func (s *Store) Verify(token string) (Device, bool) {
	if len(token) != 64 {
		return Device{}, false
	}
	h := []byte(hashToken(token))
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.devices {
		if subtle.ConstantTimeCompare(h, []byte(d.TokenHash)) == 1 {
			return d, true
		}
	}
	return Device{}, false
}

// ID is a short, stable identifier for a device, safe to show and to pass
// around (it is a prefix of the token hash, not of the token).
func (d Device) ID() string { return d.TokenHash[:12] }

// Devices returns a copy of the paired devices.
func (s *Store) Devices() []Device {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Device(nil), s.devices...)
}

// Revoke unpairs the device with the given ID.
func (s *Store) Revoke(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, d := range s.devices {
		if d.ID() == id {
			s.devices = append(s.devices[:i:i], s.devices[i+1:]...)
			return true, s.save()
		}
	}
	return false, nil
}

// NewPIN replaces the pairing PIN.
func (s *Store) NewPIN() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rotatePIN()
}

// PIN returns the current pairing PIN.
func (s *Store) PIN() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pin
}

// rotatePIN must be called with mu held (or before the store is shared).
func (s *Store) rotatePIN() {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		panic(err) // crypto/rand failing means the OS is broken
	}
	s.pin = fmt.Sprintf("%06d", n.Int64())
	s.attempts = 0
	if s.OnPIN != nil {
		s.OnPIN(s.pin)
	}
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.devices, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
