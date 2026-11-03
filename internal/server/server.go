// Package server serves the web panel and the WebSocket control channel.
package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/Tunayblg/DeskLink/internal/actions"
	"github.com/Tunayblg/DeskLink/internal/art"
	"github.com/Tunayblg/DeskLink/internal/auth"
	"github.com/Tunayblg/DeskLink/internal/config"
	"github.com/Tunayblg/DeskLink/internal/media"
	"github.com/Tunayblg/DeskLink/internal/procstat"
	"github.com/Tunayblg/DeskLink/internal/stats"
)

const (
	authTimeout  = 5 * time.Second
	pingInterval = 30 * time.Second
	// minPressGap stops a stuck finger or a flood of messages from firing an action repeatedly.
	minPressGap   = 120 * time.Millisecond
	statsInterval = time.Second
	mediaResync   = 5 * time.Second
)

// Server wires config, auth and actions to HTTP.
type Server struct {
	cfg    *config.Config
	auth   *auth.Store
	web    fs.FS
	games  []publicGame
	run    func(actions.Action) error
	sample func() stats.Snapshot

	media      *media.Watcher
	nowPlaying func() media.Info
	setVolume  func(int)

	// Dashboard (see admin.go). Version, Addresses and AdminPage are set by main.
	Version    string
	Addresses  []string
	AdminPage  []byte
	adminToken string
	started    time.Time
	meter      procstat.Meter
	sessMu     sync.Mutex
	sessions   map[*session]struct{}
}

type publicGame struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	art.Art
}

// New creates a server. web holds the static panel files and pictures holds
// the art for each game ID (see package art).
func New(cfg *config.Config, store *auth.Store, web fs.FS, pictures map[string]art.Art) *Server {
	var c stats.Collector
	games := make([]publicGame, len(cfg.Games))
	for i, g := range cfg.Games {
		games[i] = publicGame{ID: g.ID, Name: g.Name, Art: pictures[g.ID]}
	}
	w := &media.Watcher{}
	return &Server{cfg: cfg, auth: store, web: web, games: games, run: actions.Run, sample: c.Sample,
		media: w, nowPlaying: w.Sample, setVolume: w.SetVolume,
		adminToken: newAdminToken(), started: time.Now(), sessions: map[*session]struct{}{}}
}

// Handler returns the full HTTP handler, including the LAN-only guard.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(s.web))
	mux.HandleFunc("POST /api/pair", s.handlePair)
	mux.HandleFunc("GET /ws", s.handleWS)
	mux.HandleFunc("GET /admin", s.localOnly(s.handleAdminPage))
	mux.HandleFunc("GET /admin/api/status", s.adminAPI(s.handleAdminStatus))
	mux.HandleFunc("POST /admin/api/revoke", s.adminAPI(s.handleAdminRevoke))
	mux.HandleFunc("POST /admin/api/pin", s.adminAPI(s.handleAdminPIN))
	return lanOnly(securityHeaders(mux))
}

type pairRequest struct {
	PIN  string `json:"pin"`
	Name string `json:"name"`
}

func (s *Server) handlePair(w http.ResponseWriter, r *http.Request) {
	var req pairRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	token, err := s.auth.Pair(req.PIN, req.Name)
	switch {
	case errors.Is(err, auth.ErrBadPIN):
		log.Printf("pairing: wrong PIN from %s", r.RemoteAddr)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Wrong PIN"})
	case errors.Is(err, auth.ErrPINRotate):
		log.Printf("pairing: too many wrong PINs from %s, PIN rotated", r.RemoteAddr)
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Too many attempts. Use the new PIN shown on the PC."})
	case err != nil:
		log.Printf("pairing: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Could not save device"})
	default:
		log.Printf("pairing: paired %q from %s", req.Name, r.RemoteAddr)
		writeJSON(w, http.StatusOK, map[string]string{"token": token})
	}
}

// Messages on the WebSocket. The phone only ever sends "auth", "press",
// "volume" (set the level), and "stats"/"media" (turn a live stream on or off).
type clientMsg struct {
	Type  string `json:"type"`
	Token string `json:"token,omitempty"`
	ID    string `json:"id,omitempty"`
	On    bool   `json:"on,omitempty"`
	Level *int   `json:"level,omitempty"` // "volume": 0-100
}

type serverMsg struct {
	Type   string               `json:"type"`
	Groups []config.PublicGroup `json:"groups,omitempty"`
	Games  []publicGame         `json:"games,omitempty"`
	ID     string               `json:"id,omitempty"`
	OK     bool                 `json:"ok,omitempty"`
	Error  string               `json:"error,omitempty"`
	Stats  *stats.Snapshot      `json:"stats,omitempty"`
	Media  *media.Info          `json:"media,omitempty"`
	Art    string               `json:"art,omitempty"` // data: URI, only when the cover changes
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	// Accept rejects cross-origin requests by default, so other websites
	// open on the phone cannot talk to DeskLink.
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer c.CloseNow()
	c.SetReadLimit(4096)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	device, ok := s.authenticate(ctx, c)
	if !ok {
		c.Close(websocket.StatusPolicyViolation, "unauthorized")
		return
	}
	name := device.Name
	log.Printf("connected: %s (%s)", name, r.RemoteAddr)
	defer log.Printf("disconnected: %s", name)
	sess := &session{device: device, addr: r.RemoteAddr, since: time.Now(), cancel: cancel, streams: map[string]bool{}}
	s.addSession(sess)
	defer s.removeSession(sess)

	if err := wsjson.Write(ctx, c, serverMsg{Type: "layout", Groups: s.cfg.Public(), Games: s.games}); err != nil {
		return
	}

	go keepAlive(ctx, c, cancel)

	var last sync.Map // button id -> time.Time of last press
	// Live streams the phone has switched on, keyed by message type.
	streams := map[string]func(context.Context, *websocket.Conn){"stats": s.streamStats, "media": s.streamMedia}
	stop := map[string]context.CancelFunc{}
	defer func() {
		for _, f := range stop {
			f()
		}
	}()
	for {
		var msg clientMsg
		if err := wsjson.Read(ctx, c, &msg); err != nil {
			return
		}
		if stream, ok := streams[msg.Type]; ok {
			if f := stop[msg.Type]; f != nil {
				f()
				delete(stop, msg.Type)
			}
			if msg.On {
				sctx, scancel := context.WithCancel(ctx)
				stop[msg.Type] = scancel
				go stream(sctx, c)
			}
			sess.setStream(msg.Type, msg.On)
			continue
		}
		if msg.Type == "volume" {
			if msg.Level != nil && *msg.Level >= 0 && *msg.Level <= 100 {
				s.setVolume(*msg.Level)
			}
			continue
		}
		if msg.Type != "press" {
			continue
		}
		reply := serverMsg{Type: "result", ID: msg.ID}
		if err := s.press(msg.ID, &last); err != nil {
			reply.Error = err.Error()
			log.Printf("%s pressed %q: %v", name, msg.ID, err)
		} else {
			reply.OK = true
		}
		if err := wsjson.Write(ctx, c, reply); err != nil {
			return
		}
	}
}

func (s *Server) authenticate(ctx context.Context, c *websocket.Conn) (auth.Device, bool) {
	ctx, cancel := context.WithTimeout(ctx, authTimeout)
	defer cancel()
	var msg clientMsg
	if err := wsjson.Read(ctx, c, &msg); err != nil || msg.Type != "auth" {
		return auth.Device{}, false
	}
	d, ok := s.auth.Verify(msg.Token)
	if !ok {
		wsjson.Write(ctx, c, serverMsg{Type: "unauthorized"})
	}
	return d, ok
}

func (s *Server) press(id string, last *sync.Map) error {
	a, ok := s.cfg.Find(id)
	if !ok || a.Type == "page" {
		return errors.New("unknown button")
	}
	now := time.Now()
	if prev, ok := last.Load(id); ok && now.Sub(prev.(time.Time)) < minPressGap {
		return nil
	}
	last.Store(id, now)
	return s.run(a)
}

// streamStats sends a reading every second until ctx is cancelled.
func (s *Server) streamStats(ctx context.Context, c *websocket.Conn) {
	t := time.NewTicker(statsInterval)
	defer t.Stop()
	for {
		snap := s.sample()
		if err := wsjson.Write(ctx, c, serverMsg{Type: "stats", Stats: &snap}); err != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// streamMedia sends the now-playing state when it changes (and every few
// seconds to resync the progress bar). Cover art is only sent for a new track.
func (s *Server) streamMedia(ctx context.Context, c *websocket.Conn) {
	t := time.NewTicker(statsInterval)
	defer t.Stop()
	var prev media.Info
	var sentAt time.Time
	sentArt := ""
	for first := true; ; first = false {
		info := s.nowPlaying()
		if first || !info.Same(prev) || time.Since(sentAt) >= mediaResync {
			msg := serverMsg{Type: "media", Media: &info}
			if info.ArtID != sentArt {
				if b := s.media.Art(info.ArtID); b != nil {
					msg.Art = "data:" + http.DetectContentType(b) + ";base64," + base64.StdEncoding.EncodeToString(b)
				}
				sentArt = info.ArtID
			}
			if err := wsjson.Write(ctx, c, msg); err != nil {
				return
			}
			prev, sentAt = info, time.Now()
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// keepAlive pings so dead connections (phone locked, Wi-Fi dropped) are noticed.
func keepAlive(ctx context.Context, c *websocket.Conn, cancel context.CancelFunc) {
	t := time.NewTicker(pingInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			pctx, pcancel := context.WithTimeout(ctx, 10*time.Second)
			err := c.Ping(pctx)
			pcancel()
			if err != nil {
				cancel()
				return
			}
		}
	}
}

// lanOnly rejects requests that do not come from loopback or a private network.
func lanOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		ip := net.ParseIP(host)
		if err != nil || !IsLocal(ip) {
			http.Error(w, "DeskLink only accepts connections from the local network", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// IsLocal reports whether ip is loopback, private (RFC 1918 / ULA) or link-local.
func IsLocal(ip net.IP) bool {
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast())
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'self'; connect-src 'self' ws:; img-src 'self' data:; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
