// Package server serves the web panel and the WebSocket control channel.
package server

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
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
	// The config can be edited from the dashboard while phones are connected;
	// cfgMu guards cfg and games, which are replaced together.
	cfgMu  sync.RWMutex
	cfg    *config.Config
	games  []publicGame
	auth   *auth.Store
	web    fs.FS
	run    func(actions.Action) error
	sample func() stats.Snapshot

	media      *media.Watcher
	nowPlaying func() media.Info
	setVolume  func(int)

	// Dashboard (see admin.go). Version, Addresses and AdminPage are set by main.
	Version   string
	Addresses []string
	AdminPage []byte
	// ConfigPath is where dashboard edits are saved. ChooseProgram shows a
	// file picker on the PC. Both are set by main; nil/empty disables them.
	ConfigPath    string
	ChooseProgram func() (string, bool)
	adminToken    string
	started       time.Time
	meter         procstat.Meter
	sessMu        sync.Mutex
	sessions      map[*session]struct{}
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
	w := &media.Watcher{}
	return &Server{cfg: cfg, auth: store, web: web, games: publicGames(cfg, pictures), run: actions.Run, sample: c.Sample,
		media: w, nowPlaying: w.Sample, setVolume: w.SetVolume,
		adminToken: newAdminToken(), started: time.Now(), sessions: map[*session]struct{}{}}
}

func publicGames(cfg *config.Config, pictures map[string]art.Art) []publicGame {
	games := make([]publicGame, len(cfg.Games))
	for i, g := range cfg.Games {
		games[i] = publicGame{ID: g.ID, Name: g.Name, Art: pictures[g.ID]}
	}
	return games
}

// current returns the config and game list in use.
func (s *Server) current() (*config.Config, []publicGame) {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.cfg, s.games
}

func (s *Server) layout() serverMsg {
	cfg, games := s.current()
	return serverMsg{Type: "layout", Groups: cfg.Public(), Games: games}
}

// apply switches to a new config and sends the new layout to every phone.
func (s *Server) apply(cfg *config.Config, pictures map[string]art.Art) {
	s.cfgMu.Lock()
	s.cfg, s.games = cfg, publicGames(cfg, pictures)
	s.cfgMu.Unlock()
	msg := s.layout()
	s.sessMu.Lock()
	defer s.sessMu.Unlock()
	for ss := range s.sessions {
		go func(ss *session) {
			ctx, cancel := context.WithTimeout(ss.ctx, 5*time.Second)
			defer cancel()
			wsjson.Write(ctx, ss.conn, msg)
		}(ss)
	}
}

// Handler returns the full HTTP handler, including the LAN-only guard.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /", staticFiles(s.web))
	mux.HandleFunc("POST /api/pair", s.handlePair)
	mux.HandleFunc("GET /ws", s.handleWS)
	mux.HandleFunc("GET /admin", s.localOnly(s.handleAdminPage))
	mux.HandleFunc("GET /admin/api/status", s.adminAPI(s.handleAdminStatus))
	mux.HandleFunc("POST /admin/api/revoke", s.adminAPI(s.handleAdminRevoke))
	mux.HandleFunc("POST /admin/api/rotate", s.adminAPI(s.handleAdminRotate))
	mux.HandleFunc("GET /admin/api/games", s.adminAPI(s.handleGamesGet))
	mux.HandleFunc("POST /admin/api/games", s.adminAPI(s.handleGamesSave))
	mux.HandleFunc("GET /admin/api/steam", s.adminAPI(s.handleSteamGames))
	mux.HandleFunc("POST /admin/api/browse", s.adminAPI(s.handleBrowse))
	return lanOnly(securityHeaders(mux))
}

// pairRequest carries either a pairing code (typed, or from the QR code) or a
// handoff code (from the URL of a freshly added home screen app).
type pairRequest struct {
	Code    string `json:"code"`
	Handoff string `json:"handoff"`
	Name    string `json:"name"`
}

// Pairing errors are sent as codes; the phone shows them in its own language.
func (s *Server) handlePair(w http.ResponseWriter, r *http.Request) {
	var req pairRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.Handoff != "" {
		token, err := s.auth.Redeem(req.Handoff)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "expired_link"})
			return
		}
		log.Printf("pairing: handoff to home screen app from %s", r.RemoteAddr)
		writeJSON(w, http.StatusOK, map[string]string{"token": token})
		return
	}
	token, handoff, err := s.auth.Pair(req.Code, req.Name)
	switch {
	case errors.Is(err, auth.ErrBadCode):
		log.Printf("pairing: wrong code from %s", r.RemoteAddr)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "bad_code"})
	case errors.Is(err, auth.ErrTooMany):
		log.Printf("pairing: too many wrong codes from %s", r.RemoteAddr)
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too_many"})
	case err != nil:
		log.Printf("pairing: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "save_failed"})
	default:
		log.Printf("pairing: paired %q from %s", req.Name, r.RemoteAddr)
		writeJSON(w, http.StatusOK, map[string]string{"token": token, "handoff": handoff})
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
	sess := &session{device: device, addr: r.RemoteAddr, since: time.Now(), cancel: cancel, ctx: ctx, conn: c, streams: map[string]bool{}}
	s.addSession(sess)
	defer s.removeSession(sess)

	if err := wsjson.Write(ctx, c, s.layout()); err != nil {
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
	cfg, _ := s.current()
	a, ok := cfg.Find(id)
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

// staticFiles serves the panel with an ETag per file and Cache-Control:
// no-cache, so the phone revalidates on every load and gets a 304 when nothing
// changed. Embedded files have no modification time, and without any
// validator iOS kept serving a stale index.html next to a new app.js after an
// update, which broke the panel.
func staticFiles(files fs.FS) http.Handler {
	etags := map[string]string{}
	fs.WalkDir(files, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if b, err := fs.ReadFile(files, p); err == nil {
			sum := sha256.Sum256(b)
			etags["/"+p] = `"` + hex.EncodeToString(sum[:8]) + `"`
		}
		return nil
	})
	etags["/"] = etags["/index.html"]
	fileServer := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if tag, ok := etags[r.URL.Path]; ok {
			w.Header().Set("ETag", tag)
		}
		w.Header().Set("Cache-Control", "no-cache")
		fileServer.ServeHTTP(w, r)
	})
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
		if !hostAllowed(r.Host) {
			http.Error(w, "Open DeskLink by its IP address", http.StatusMisdirectedRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// hostAllowed accepts requests addressed to an IP, localhost or this PC's own
// name. It rejects other names, which is what a DNS-rebinding attack uses: a
// web page on some domain that later resolves to this PC would otherwise be
// treated as same-origin by the browser.
func hostAllowed(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.ToLower(strings.Trim(host, "[]"))
	if net.ParseIP(host) != nil || host == "localhost" {
		return true
	}
	name, _ := os.Hostname()
	name = strings.ToLower(name)
	return name != "" && (host == name || host == name+".local" || host == name+".lan" || host == name+".home")
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
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
