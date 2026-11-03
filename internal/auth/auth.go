// Package auth handles pairing phones and verifying their tokens.
//
// Pairing uses a code that changes every 30 seconds, derived from a random
// secret that exists only in memory (TOTP-style). The dashboard shows the
// 6-digit code and a QR code carrying a longer code for the same window. A
// phone that sends a valid code receives a random 256-bit token, kept in its
// localStorage; only the SHA-256 hash of each token is written to disk.
//
// Because iOS keeps a home screen web app's storage apart from Safari's, a
// successful pairing also returns a short-lived, single-use handoff code. The
// page puts it in its URL, so "Add to Home Screen" carries it over and the
// home screen app can fetch the same token without pairing again.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	// Window is how long a pairing code is shown. The previous window's code
	// is accepted too, so a code typed just as it changes still works.
	Window = 30 * time.Second
	// MaxAttempts is how many wrong codes are accepted per window; after that,
	// pairing is refused until the next code appears.
	MaxAttempts = 5
	// HandoffTTL is how long a handoff code can be redeemed.
	HandoffTTL = 15 * time.Minute
)

var (
	ErrBadCode    = errors.New("wrong or expired code")
	ErrTooMany    = errors.New("too many wrong codes, wait for the next one")
	ErrBadHandoff = errors.New("this pairing link has expired, scan the QR code again")
)

// Device is a paired phone or tablet.
type Device struct {
	Name      string    `json:"name"`
	TokenHash string    `json:"token_hash"`
	PairedAt  time.Time `json:"paired_at"`
}

// Code is the pairing code for the current window.
type Code struct {
	Digits  string    // 6 digits, to type in
	Long    string    // 16 characters, carried by the QR code
	Expires time.Time // when the next code appears
}

type handoff struct {
	token   string
	expires time.Time
}

// Store keeps the pairing secret and the list of paired devices.
type Store struct {
	mu         sync.Mutex
	path       string
	secret     []byte
	devices    []Device
	failWindow int64
	fails      int
	handoffs   map[string]handoff
	now        func() time.Time // for tests
}

// Open loads paired devices from path (a missing file means none).
func Open(path string) (*Store, error) {
	s := &Store{path: path, handoffs: map[string]handoff{}, now: time.Now}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &s.devices); err != nil {
			return nil, fmt.Errorf("%s is corrupted: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	s.secret = randomBytes(32)
	return s, nil
}

func (s *Store) window(t time.Time) int64 { return t.Unix() / int64(Window/time.Second) }

// codes derives the short and long code for a window.
func (s *Store) codes(w int64) (digits, long string) {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(w))
	m := hmac.New(sha256.New, s.secret)
	m.Write(msg[:])
	sum := m.Sum(nil)
	n := binary.BigEndian.Uint32(sum[:4]) & 0x7fffffff
	digits = fmt.Sprintf("%06d", n%1_000_000)
	long = strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[4:14]))
	return digits, long
}

// Current returns the code to show right now.
func (s *Store) Current() Code {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	w := s.window(now)
	d, l := s.codes(w)
	return Code{Digits: d, Long: l, Expires: time.Unix((w+1)*int64(Window/time.Second), 0)}
}

// Rotate replaces the secret, so every code shown so far stops working.
func (s *Store) Rotate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secret = randomBytes(32)
}

// Pair exchanges a valid code (6 digits or the QR code's long form) for a new
// device token. It also returns a handoff code for the home screen app.
func (s *Store) Pair(code, name string) (token, handoffCode string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	w := s.window(now)
	if s.failWindow != w {
		s.failWindow, s.fails = w, 0
	}
	if s.fails >= MaxAttempts {
		return "", "", ErrTooMany
	}
	code = strings.ToLower(strings.TrimSpace(code))
	ok := false
	for _, cw := range []int64{w, w - 1} {
		d, l := s.codes(cw)
		if subtle.ConstantTimeCompare([]byte(code), []byte(d)) == 1 || subtle.ConstantTimeCompare([]byte(code), []byte(l)) == 1 {
			ok = true
		}
	}
	if !ok {
		s.fails++
		return "", "", ErrBadCode
	}

	token = randomHex(32)
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 40 {
		name = "Device"
	}
	s.devices = append(s.devices, Device{Name: name, TokenHash: hashToken(token), PairedAt: now.UTC()})
	if err := s.save(); err != nil {
		s.devices = s.devices[:len(s.devices)-1]
		return "", "", err
	}
	handoffCode = s.newHandoff(token, now)
	return token, handoffCode, nil
}

// newHandoff must be called with mu held.
func (s *Store) newHandoff(token string, now time.Time) string {
	for k, h := range s.handoffs {
		if now.After(h.expires) {
			delete(s.handoffs, k)
		}
	}
	code := randomHex(16)
	s.handoffs[code] = handoff{token: token, expires: now.Add(HandoffTTL)}
	return code
}

// Redeem returns the token behind a handoff code. Each code works once.
func (s *Store) Redeem(code string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, ok := s.handoffs[code]
	delete(s.handoffs, code)
	if !ok || s.now().After(h.expires) {
		return "", ErrBadHandoff
	}
	// The device may have been removed in the meantime.
	for _, d := range s.devices {
		if d.TokenHash == hashToken(h.token) {
			return h.token, nil
		}
	}
	return "", ErrBadHandoff
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

func randomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand failing means the OS is broken
	}
	return b
}

func randomHex(n int) string { return hex.EncodeToString(randomBytes(n)) }
