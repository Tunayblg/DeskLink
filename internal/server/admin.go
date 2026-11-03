package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Tunayblg/DeskLink/internal/auth"
)

// The PC dashboard at /admin. It is only served to this PC (loopback), and
// its API needs a token that is embedded in the page, so other websites open
// in the PC's browser cannot use it: they can send requests but cannot read
// the page to learn the token.

// session is one connected phone.
type session struct {
	device auth.Device
	addr   string
	since  time.Time
	cancel context.CancelFunc

	mu      sync.Mutex
	streams map[string]bool
}

func (s *session) setStream(name string, on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if on {
		s.streams[name] = true
	} else {
		delete(s.streams, name)
	}
}

func (s *Server) addSession(ss *session) {
	s.sessMu.Lock()
	defer s.sessMu.Unlock()
	s.sessions[ss] = struct{}{}
}

func (s *Server) removeSession(ss *session) {
	s.sessMu.Lock()
	defer s.sessMu.Unlock()
	delete(s.sessions, ss)
}

func newAdminToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// localOnly allows only requests from this PC that address it by a loopback
// name (the Host check stops DNS-rebinding pages from reaching the dashboard).
func (s *Server) localOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		ip := net.ParseIP(host)
		hostname := r.Host
		if h, _, err := net.SplitHostPort(r.Host); err == nil {
			hostname = h
		}
		hostname = strings.Trim(hostname, "[]")
		if err != nil || ip == nil || !ip.IsLoopback() ||
			(hostname != "localhost" && hostname != "127.0.0.1" && hostname != "::1") {
			http.NotFound(w, r)
			return
		}
		next(w, r)
	}
}

func (s *Server) adminAPI(next http.HandlerFunc) http.HandlerFunc {
	return s.localOnly(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Admin-Token")), []byte(s.adminToken)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	})
}

func (s *Server) handleAdminPage(w http.ResponseWriter, r *http.Request) {
	if s.AdminPage == nil {
		http.NotFound(w, r)
		return
	}
	nonce := newAdminToken()[:24]
	page := bytes.Replace(s.AdminPage, []byte("{{ADMIN_TOKEN}}"), []byte(s.adminToken), 1)
	page = bytes.ReplaceAll(page, []byte("{{NONCE}}"), []byte(nonce))
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("Content-Security-Policy", "default-src 'self'; script-src 'nonce-"+nonce+"'; style-src 'nonce-"+nonce+"'; img-src 'self' data:; frame-ancestors 'none'")
	w.Write(page)
}

type adminStatus struct {
	Version   string          `json:"version"`
	UptimeS   int64           `json:"uptime_s"`
	Port      int             `json:"port"`
	Addresses []string        `json:"addresses"`
	PIN       string          `json:"pin"`
	Process   any             `json:"process"`
	Sessions  []sessionStatus `json:"sessions"`
	Devices   []deviceStatus  `json:"devices"`
}

type sessionStatus struct {
	DeviceID string   `json:"device_id"`
	Name     string   `json:"name"`
	Addr     string   `json:"addr"`
	SinceS   int64    `json:"since_s"`
	Streams  []string `json:"streams"`
}

type deviceStatus struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	PairedAt time.Time `json:"paired_at"`
	Online   bool      `json:"online"`
}

func (s *Server) handleAdminStatus(w http.ResponseWriter, r *http.Request) {
	st := adminStatus{
		Version:   s.Version,
		UptimeS:   int64(time.Since(s.started).Seconds()),
		Port:      s.cfg.Port,
		Addresses: s.Addresses,
		PIN:       s.auth.PIN(),
		Process:   s.meter.Read(),
		Sessions:  []sessionStatus{},
		Devices:   []deviceStatus{},
	}
	online := map[string]bool{}
	s.sessMu.Lock()
	for ss := range s.sessions {
		ss.mu.Lock()
		streams := make([]string, 0, len(ss.streams))
		for k := range ss.streams {
			streams = append(streams, k)
		}
		ss.mu.Unlock()
		sort.Strings(streams)
		st.Sessions = append(st.Sessions, sessionStatus{
			DeviceID: ss.device.ID(), Name: ss.device.Name, Addr: ss.addr,
			SinceS: int64(time.Since(ss.since).Seconds()), Streams: streams,
		})
		online[ss.device.ID()] = true
	}
	s.sessMu.Unlock()
	sort.Slice(st.Sessions, func(i, j int) bool { return st.Sessions[i].SinceS > st.Sessions[j].SinceS })
	for _, d := range s.auth.Devices() {
		st.Devices = append(st.Devices, deviceStatus{ID: d.ID(), Name: d.Name, PairedAt: d.PairedAt, Online: online[d.ID()]})
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleAdminRevoke(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ok, err := s.auth.Revoke(req.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such device"})
		return
	}
	// Disconnect the device right away.
	s.sessMu.Lock()
	for ss := range s.sessions {
		if ss.device.ID() == req.ID {
			ss.cancel()
		}
	}
	s.sessMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleAdminPIN(w http.ResponseWriter, r *http.Request) {
	s.auth.NewPIN()
	writeJSON(w, http.StatusOK, map[string]string{"pin": s.auth.PIN()})
}
