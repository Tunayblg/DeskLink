// Package server serves the web panel and the WebSocket control channel.
package server

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/Tunayblg/DeskLink/internal/actions"
	"github.com/Tunayblg/DeskLink/internal/art"
	"github.com/Tunayblg/DeskLink/internal/auth"
	"github.com/Tunayblg/DeskLink/internal/brightness"
	"github.com/Tunayblg/DeskLink/internal/config"
	"github.com/Tunayblg/DeskLink/internal/media"
	"github.com/Tunayblg/DeskLink/internal/pointer"
	"github.com/Tunayblg/DeskLink/internal/procstat"
	"github.com/Tunayblg/DeskLink/internal/running"
	"github.com/Tunayblg/DeskLink/internal/stats"
)

// errAlreadyRunning is the answer to a press on a game that is already open.
// The phone shows it in the user's language.
var errAlreadyRunning = errors.New("already_running")

const (
	authTimeout = 5 * time.Second
	// idleTimeout closes a connection that has been silent this long; the
	// phone sends a ping every 15 s.
	idleTimeout = 45 * time.Second
	// heldTimeout lets go of a held mouse button when the phone has said
	// nothing for this long (it pings every 15 s), so a dropped Wi-Fi link in
	// the middle of a drag cannot leave the button stuck down.
	heldTimeout = 20 * time.Second
	// A phone may send this many messages per second (a trackpad sends about
	// 60), with room for a burst; more are dropped.
	msgRate  = 300
	msgBurst = 600
	// tokenCookie holds the device token as a backup to localStorage.
	tokenCookie = "desklink_token"
	// minPressGap stops a stuck finger or a flood of messages from firing an action repeatedly.
	minPressGap   = 120 * time.Millisecond
	statsInterval = time.Second
	mediaResync   = 5 * time.Second
)

// Server wires config, auth and actions to HTTP.
type Server struct {
	// The config can be edited from the dashboard while phones are connected;
	// cfgMu guards cfg and games, which are replaced together.
	cfgMu     sync.RWMutex
	cfg       *config.Config
	games     []publicGame
	shortcuts []publicShortcut
	auth      *auth.Store
	web       fs.FS
	// assetVersion changes whenever the panel's files do (see AssetVersion).
	assetVersion string
	// Rate limit for /api/report.
	reportMu sync.Mutex
	reportAt time.Time
	// LogRequests logs every request from the network (not this PC), for
	// diagnosing a phone that will not load. Set by main from -debug.
	LogRequests bool
	run         func(actions.Action) error
	sample      func() stats.Snapshot
	system      func() stats.Snapshot // CPU and RAM only, for the dashboard

	media      *media.Watcher
	nowPlaying func() media.State
	setVolume  func(source string, level int) // source "" is the system volume
	control    func(source, action string)
	// The volume mixer (see mixer.go).
	readMixer      func() media.Mixer
	setMixerVolume func(app string, level int)
	toggleMute     func(app string) // "" mutes the whole PC
	setMic         func(muted bool)
	// Screen brightness (see package brightness).
	getBrightness func() (int, bool)
	setBrightness func(level int)
	// newPad makes one phone's trackpad (see package pointer).
	newPad   func() padder
	gameExes []gameRef
	// running lists the programs that are open (lowercase, no ".exe"); a game
	// whose program is among them is not started again.
	running func() map[string]bool
	iconMu  sync.Mutex
	icons   map[string]string // executable path -> icon data URI

	// Dashboard (see admin.go). Version, Addresses and AdminPage are set by main.
	Version   string
	Addresses []string
	AdminPage []byte
	// ConfigPath is where dashboard edits are saved. ChooseProgram shows a
	// file picker on the PC. Both are set by main; nil/empty disables them.
	ConfigPath    string
	ChooseProgram func() (string, bool)
	// MaximizeWindow makes the dashboard window fill the screen (true) or puts
	// it back (false) and reports whether it changed. Set by main.
	MaximizeWindow func(on bool) bool
	adminToken     string
	started        time.Time
	meter          procstat.Meter
	sessMu         sync.Mutex
	sessions       map[*session]struct{}
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
	bright := brightness.NewSetter()
	return &Server{cfg: cfg, auth: store, web: web, assetVersion: AssetVersion(web), games: publicGames(cfg, pictures), shortcuts: buildShortcuts(cfg), run: actions.Run, sample: c.Sample, system: c.System,
		media: w, nowPlaying: w.Sample, setVolume: setVolume(w), control: w.Control,
		readMixer: w.Mixer, setMixerVolume: w.SetMixerVolume, toggleMute: w.ToggleMute, setMic: w.SetMic, getBrightness: brightness.Get, setBrightness: bright.Set, newPad: func() padder { return pointer.NewPad() }, gameExes: gameExes(cfg.Games), running: running.Names, icons: map[string]string{},
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
	s.cfgMu.RLock()
	cfg, games, shortcuts := s.cfg, s.games, s.shortcuts
	s.cfgMu.RUnlock()
	return serverMsg{Type: "layout", Groups: cfg.Public(), Games: games, Shortcuts: shortcuts}
}

// apply switches to a new config and sends the new layout to every phone.
func (s *Server) apply(cfg *config.Config, pictures map[string]art.Art) {
	s.cfgMu.Lock()
	s.cfg, s.games, s.gameExes, s.shortcuts = cfg, publicGames(cfg, pictures), gameExes(cfg.Games), buildShortcuts(cfg)
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
	mux.HandleFunc("POST /api/session", s.handleSession)
	mux.HandleFunc("POST /api/report", s.handleReport)
	mux.HandleFunc("GET /ws", s.handleWS)
	mux.HandleFunc("GET /admin", s.localOnly(s.handleAdminPage))
	mux.HandleFunc("GET /admin/api/status", s.adminAPI(s.handleAdminStatus))
	mux.HandleFunc("GET /admin/api/usage", s.adminAPI(s.handleAdminUsage))
	mux.HandleFunc("POST /admin/api/revoke", s.adminAPI(s.handleAdminRevoke))
	mux.HandleFunc("POST /admin/api/rotate", s.adminAPI(s.handleAdminRotate))
	mux.HandleFunc("GET /admin/api/games", s.adminAPI(s.handleGamesGet))
	mux.HandleFunc("POST /admin/api/games", s.adminAPI(s.handleGamesSave))
	mux.HandleFunc("GET /admin/api/shortcuts", s.adminAPI(s.handleShortcutsGet))
	mux.HandleFunc("POST /admin/api/shortcuts", s.adminAPI(s.handleShortcutsSave))
	mux.HandleFunc("GET /admin/api/steam", s.adminAPI(s.handleSteamGames))
	mux.HandleFunc("POST /admin/api/browse", s.adminAPI(s.handleBrowse))
	mux.HandleFunc("POST /admin/api/window", s.adminAPI(s.handleWindow))
	var h http.Handler = securityHeaders(mux)
	if s.LogRequests {
		h = logRequests(h)
	}
	return lanOnly(h)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) { r.status = code; r.ResponseWriter.WriteHeader(code) }

// Hijack lets the WebSocket upgrade through the recorder.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijacking not supported")
	}
	return hj.Hijack()
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			next.ServeHTTP(w, r)
			return
		}
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rec, r)
		log.Printf("http %s %s %s -> %d (%s) %q", host, r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond), r.UserAgent())
	})
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
		setTokenCookie(w, token)
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
		setTokenCookie(w, token)
		writeJSON(w, http.StatusOK, map[string]string{"token": token, "handoff": handoff})
	}
}

// Messages on the WebSocket. The phone only ever sends "auth", "press",
// "volume" (set the level of the PC, a media source or a program in the
// mixer), "mute" (toggle the PC's or a program's mute), "micmute" (set the microphone's mute to "on", answered with "micack"), "mouse", "mediactl"
// (play/pause, skip or mute one media source), and "stats"/"media" (turn a
// live stream on or off), "mixer" (the same, for the volume mixer).
type clientMsg struct {
	Type   string `json:"type"`
	Token  string `json:"token,omitempty"`
	ID     string `json:"id,omitempty"`
	On     bool   `json:"on,omitempty"`
	Level  *int   `json:"level,omitempty"`  // "volume": 0-100
	Source string `json:"source,omitempty"` // "volume", "mediactl": a media source ID
	App    string `json:"app,omitempty"`    // "volume", "mute": a program in the mixer
	Action string `json:"action,omitempty"` // "mediactl": toggle, next, prev, mute; "mouse": move, down, up, click, scroll, hscroll
	Button string `json:"button,omitempty"` // "mouse": left, right, middle
	DX     int    `json:"dx,omitempty"`     // "mouse": pixels (move) or wheel units (hscroll uses dy)
	DY     int    `json:"dy,omitempty"`
}

type serverMsg struct {
	Type       string               `json:"type"`
	Groups     []config.PublicGroup `json:"groups,omitempty"`
	Games      []publicGame         `json:"games,omitempty"`
	ID         string               `json:"id,omitempty"`
	OK         bool                 `json:"ok,omitempty"`
	Error      string               `json:"error,omitempty"`
	Stats      *stats.Snapshot      `json:"stats,omitempty"`
	Media      *media.State         `json:"media,omitempty"`
	Art        map[string]string    `json:"art,omitempty"` // art ID -> data: URI, each cover sent once
	Mixer      *mixerView           `json:"mixer,omitempty"`
	Mic        *media.Mic           `json:"mic,omitempty"`        // "micack": the microphone now (absent: none)
	Icons      map[string]string    `json:"icons,omitempty"`      // mixer app ID -> data: URI, each sent once
	Shortcuts  []publicShortcut     `json:"shortcuts,omitempty"`  // "layout": the three home screen shortcut tiles
	Brightness *int                 `json:"brightness,omitempty"` // "brightness": the screen now, in percent (absent: cannot be controlled)
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	// Accept rejects cross-origin requests by default, so other websites
	// open on the phone cannot talk to DeskLink.
	// Compression is off: Safari on iOS 15 has known problems with
	// permessage-deflate, and on a LAN it saves nothing worth the CPU.
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return
	}
	defer c.CloseNow()
	c.SetReadLimit(4096)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// The token comes in the first message, or else from the cookie set at
	// pairing (iOS can drop a home screen app's localStorage).
	cookieToken := ""
	if ck, err := r.Cookie(tokenCookie); err == nil {
		cookieToken = ck.Value
	}
	device, ok := s.authenticate(ctx, c, cookieToken)
	if !ok {
		c.Close(websocket.StatusPolicyViolation, "unauthorized")
		return
	}
	name := device.Name
	log.Printf("connected: %s (%s)", name, r.RemoteAddr)
	sess := &session{device: device, addr: r.RemoteAddr, since: time.Now(), cancel: cancel, ctx: ctx, conn: c, streams: map[string]bool{}}
	s.addSession(sess)
	defer s.removeSession(sess)

	if err := wsjson.Write(ctx, c, s.layout()); err != nil {
		log.Printf("disconnected: %s (%s, while sending the layout)", name, closeReason(ctx, err))
		return
	}

	var last sync.Map // button id -> time.Time of last press
	// The trackpad: buttons the phone holds are let go when the connection ends.
	pad := s.newPad()
	defer pad.Release()
	var lastMsg atomic.Int64 // unix nanoseconds of the last message from the phone
	lastMsg.Store(time.Now().UnixNano())
	go func() {
		t := time.NewTicker(heldTimeout / 4)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if pad.Holding() && time.Since(time.Unix(0, lastMsg.Load())) > heldTimeout {
					pad.Release()
				}
			}
		}
	}()
	budget, budgetAt := float64(msgBurst), time.Now()
	// Live streams the phone has switched on, keyed by message type.
	streams := map[string]func(context.Context, *websocket.Conn){"stats": s.streamStats, "media": s.streamMedia, "mixer": s.streamMixer}
	var brightAsking atomic.Bool // a brightness question from this phone is being answered
	stop := map[string]context.CancelFunc{}
	defer func() {
		for _, f := range stop {
			f()
		}
	}()
	for {
		// The phone sends a ping every 15 s. WebSocket protocol pings are not
		// used: Safari on iOS 15 did not answer them, so connections were cut
		// after 30-40 s. Silence for longer than idleTimeout means a dead link
		// (phone asleep, Wi-Fi gone).
		var msg clientMsg
		rctx, rcancel := context.WithTimeout(ctx, idleTimeout)
		err := wsjson.Read(rctx, c, &msg)
		rcancel()
		if err != nil {
			log.Printf("disconnected: %s (%s)", name, closeReason(ctx, err))
			return
		}
		lastMsg.Store(time.Now().UnixNano())
		if msg.Type == "ping" {
			if err := wsjson.Write(ctx, c, serverMsg{Type: "pong"}); err != nil {
				log.Printf("disconnected: %s (%s)", name, closeReason(ctx, err))
				return
			}
			continue
		}
		// Pings are exempt, so a flood cannot make the phone think the link died.
		now := time.Now()
		budget = min(float64(msgBurst), budget+now.Sub(budgetAt).Seconds()*msgRate) - 1
		budgetAt = now
		if budget < 0 {
			budget = 0
			continue // too many messages; drop this one
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
			if msg.Level == nil || *msg.Level < 0 || *msg.Level > 100 {
				continue
			}
			if msg.App != "" {
				if s.knownApp(msg.App) {
					s.setMixerVolume(msg.App, *msg.Level)
				}
			} else if s.knownSource(msg.Source) {
				s.setVolume(msg.Source, *msg.Level)
			}
			continue
		}
		if msg.Type == "mouse" {
			pad.Do(pointer.Event{Action: msg.Action, Button: msg.Button, DX: msg.DX, DY: msg.DY})
			continue
		}
		if msg.Type == "micmute" {
			s.setMic(msg.On)
			// Answer with what the microphone really is now, so the phone does
			// not have to guess (its own picture can be stale on a slow link).
			if err := wsjson.Write(ctx, c, serverMsg{Type: "micack", Mic: s.readMixer().Mic}); err != nil {
				log.Printf("disconnected: %s (%s)", name, closeReason(ctx, err))
				return
			}
			continue
		}
		if msg.Type == "mute" {
			if s.knownApp(msg.App) {
				s.toggleMute(msg.App)
			}
			continue
		}
		if msg.Type == "watch" {
			s.watch(msg.Action)
			continue
		}
		if msg.Type == "brightness" {
			// Without a level the phone is asking; with one, it is setting.
			if msg.Level != nil {
				if *msg.Level >= 0 && *msg.Level <= 100 {
					s.setBrightness(*msg.Level)
				}
				continue
			}
			// Reading a monitor takes a moment, so do not hold up the phone's other
			// messages. One question at a time per phone: more would only queue up.
			if !brightAsking.CompareAndSwap(false, true) {
				continue
			}
			go func() {
				defer brightAsking.Store(false)
				if err := s.answerBrightness(ctx, c); err != nil {
					log.Printf("brightness reply to %s: %v", name, err)
				}
			}()
			continue
		}
		if msg.Type == "mediactl" {
			if media.ValidAction(msg.Action) && msg.Source != "" && s.knownSource(msg.Source) {
				s.control(msg.Source, msg.Action)
			}
			continue
		}
		if msg.Type != "press" {
			continue
		}
		reply := serverMsg{Type: "result", ID: msg.ID}
		if err := s.press(msg.ID, &last); err != nil {
			reply.Error = err.Error()
			if !errors.Is(err, errAlreadyRunning) {
				log.Printf("%s pressed %q: %v", name, msg.ID, err)
			}
		} else {
			reply.OK = true
		}
		if err := wsjson.Write(ctx, c, reply); err != nil {
			return
		}
	}
}

// closeReason describes why a connection ended, for the log.
func closeReason(ctx context.Context, err error) string {
	if ctx.Err() != nil {
		return "closed by DeskLink"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "no message for " + idleTimeout.String() + ", phone asleep or out of Wi-Fi"
	}
	if st := websocket.CloseStatus(err); st != -1 {
		return "closed by phone, " + st.String()
	}
	// The phone vanished without a goodbye: page killed, Wi-Fi dropped.
	return "connection lost"
}

func (s *Server) authenticate(ctx context.Context, c *websocket.Conn, cookieToken string) (auth.Device, bool) {
	ctx, cancel := context.WithTimeout(ctx, authTimeout)
	defer cancel()
	var msg clientMsg
	if err := wsjson.Read(ctx, c, &msg); err != nil || msg.Type != "auth" {
		return auth.Device{}, false
	}
	if msg.Token == "" {
		msg.Token = cookieToken
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
	if s.gameOpen(cfg, id) {
		return errAlreadyRunning
	}
	return s.run(a)
}

// gameOpen reports whether the game behind a game id or a game button is
// already running, found by the name of its program (the program in
// "icon_from", or the target). Steam games are left to Steam, which does not
// start a second copy itself, and so are games whose program is unknown (a
// shortcut or launcher): those can always be started.
func (s *Server) gameOpen(cfg *config.Config, id string) bool {
	g, ok := cfg.GameOf(id)
	if !ok || g.SteamAppID > 0 || s.running == nil {
		return false
	}
	s.cfgMu.RLock()
	refs := s.gameExes
	s.cfgMu.RUnlock()
	for _, r := range refs {
		if r.id == g.ID {
			return s.running()[r.exe]
		}
	}
	return false
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

// setVolume picks the system or the per-app volume.
func setVolume(w *media.Watcher) func(string, int) {
	return func(source string, level int) {
		if source == "" {
			w.SetVolume(level)
		} else {
			w.SetAppVolume(source, level)
		}
	}
}

// knownSource reports whether id is "" (the system) or a source that is playing.
func (s *Server) knownSource(id string) bool {
	if id == "" {
		return true
	}
	_, ok := s.nowPlaying().Find(id)
	return ok
}

// streamMedia sends the now-playing state when it changes (and every few
// seconds to resync the progress bars). Each cover is sent once per connection.
func (s *Server) streamMedia(ctx context.Context, c *websocket.Conn) {
	t := time.NewTicker(statsInterval)
	defer t.Stop()
	var prev media.State
	var sentAt time.Time
	sentArt := map[string]bool{}
	for first := true; ; first = false {
		st := s.nowPlaying()
		if first || !st.Same(prev) || time.Since(sentAt) >= mediaResync {
			msg := serverMsg{Type: "media", Media: &st}
			current := map[string]bool{}
			for _, src := range st.Sources {
				if src.ArtID == "" {
					continue
				}
				if sentArt[src.ArtID] {
					current[src.ArtID] = true
					continue
				}
				if b := s.media.Art(src.ArtID); b != nil {
					if msg.Art == nil {
						msg.Art = map[string]string{}
					}
					msg.Art[src.ArtID] = "data:" + http.DetectContentType(b) + ";base64," + base64.StdEncoding.EncodeToString(b)
					current[src.ArtID] = true
				}
			}
			if err := wsjson.Write(ctx, c, msg); err != nil {
				return
			}
			sentArt = current
			prev, sentAt = st, time.Now()
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// setTokenCookie stores the device token in a long-lived cookie as a second
// copy next to the page's localStorage, which iOS may clear for home screen
// apps. HttpOnly keeps scripts from reading it; SameSite=Strict keeps other
// sites from sending it.
func setTokenCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: tokenCookie, Value: token, Path: "/", MaxAge: 10 * 365 * 24 * 3600,
		HttpOnly: true, SameSite: http.SameSiteStrictMode,
	})
}

// handleReport writes script errors from the phone into the log, so problems
// that only happen on the device can be diagnosed from the PC. Reports are
// short, rate limited, and need no pairing (a broken page may not have paired).
func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message string `json:"message"`
		Where   string `json:"where"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	s.reportMu.Lock()
	allowed := time.Since(s.reportAt) > 2*time.Second
	if allowed {
		s.reportAt = time.Now()
	}
	s.reportMu.Unlock()
	if allowed {
		clip := func(v string, n int) string {
			v = strings.Map(func(c rune) rune {
				if c < 32 {
					return ' '
				}
				return c
			}, v)
			if len(v) > n {
				v = v[:n]
			}
			return v
		}
		log.Printf("phone %s reports: %s (%s)", r.RemoteAddr, clip(req.Message, 300), clip(req.Where, 200))
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSession lets an already paired phone (from before the cookie
// existed) store its token in the cookie as well.
func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if _, ok := s.auth.Verify(req.Token); !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unknown"})
		return
	}
	setTokenCookie(w, req.Token)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// staticFiles serves the panel with an ETag per file and Cache-Control:
// no-cache, so the phone revalidates on every load and gets a 304 when nothing
// changed. Embedded files have no modification time, and without any
// validator iOS kept serving a stale index.html next to a new app.js after an
// update, which broke the panel.
//
// On top of that, index.html is never cached and refers to every script and
// stylesheet with ?v=<content hash>, so a phone holding an old copy of a file
// from an earlier version cannot use it: the URL itself is new.
func staticFiles(files fs.FS) http.Handler {
	hashes := map[string]string{}
	fs.WalkDir(files, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if b, err := fs.ReadFile(files, p); err == nil {
			sum := sha256.Sum256(b)
			hashes[p] = hex.EncodeToString(sum[:6])
		}
		return nil
	})
	index, _ := fs.ReadFile(files, "index.html")
	page := string(index)
	for p, h := range hashes {
		for _, attr := range []string{`src="`, `href="`} {
			page = strings.ReplaceAll(page, attr+p+`"`, attr+p+"?v="+h+`"`)
		}
	}
	fileServer := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			h := w.Header()
			h.Set("Content-Type", "text/html; charset=utf-8")
			h.Set("Cache-Control", "no-store")
			io.WriteString(w, page)
			return
		}
		if h, ok := hashes[strings.TrimPrefix(r.URL.Path, "/")]; ok {
			w.Header().Set("ETag", `"`+h+`"`)
		}
		w.Header().Set("Cache-Control", "no-cache")
		fileServer.ServeHTTP(w, r)
	})
}

// AssetVersion is a short hash of the panel's files, used in the QR code's
// URL so that scanning it always loads the current version of the panel.
func AssetVersion(files fs.FS) string {
	sum := sha256.New()
	fs.WalkDir(files, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if b, err := fs.ReadFile(files, p); err == nil {
				sum.Write([]byte(p))
				sum.Write(b)
			}
		}
		return nil
	})
	return hex.EncodeToString(sum.Sum(nil)[:4])
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

// padder is one phone's trackpad.
type padder interface {
	Do(pointer.Event)
	Release()
	Holding() bool
}
