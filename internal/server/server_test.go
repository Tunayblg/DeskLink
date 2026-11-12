package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/Tunayblg/DeskLink/internal/actions"
	"github.com/Tunayblg/DeskLink/internal/auth"
	"github.com/Tunayblg/DeskLink/internal/config"
	"github.com/Tunayblg/DeskLink/internal/media"
	"github.com/Tunayblg/DeskLink/internal/pointer"
	"github.com/Tunayblg/DeskLink/internal/stats"
)

func TestIsLocal(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1": true, "192.168.1.20": true, "10.0.0.5": true, "172.16.3.4": true,
		"169.254.1.1": true, "::1": true, "fd00::1": true,
		"8.8.8.8": false, "172.32.0.1": false, "2001:4860::1": false,
	} {
		if got := IsLocal(net.ParseIP(addr)); got != want {
			t.Errorf("IsLocal(%s) = %v, want %v", addr, got, want)
		}
	}
}

func TestPairThenPress(t *testing.T) {
	cfg, err := config.Parse([]byte(config.Default))
	if err != nil {
		t.Fatal(err)
	}
	store, err := auth.Open(filepath.Join(t.TempDir(), "devices.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg, store, fstest.MapFS{"index.html": {Data: []byte("ok")}}, nil)
	var ran []actions.Action
	s.run = func(a actions.Action) error { ran = append(ran, a); return nil }
	s.sample = func() stats.Snapshot { return stats.Snapshot{CPU: stats.CPU{Usage: 42}} }
	s.nowPlaying = func() media.State {
		return media.State{Sources: []media.Source{{ID: "Spotify.exe", App: "Spotify", Title: "Song", Playing: true, Volume: 50}}, Volume: 80}
	}
	var controls []string
	s.control = func(source, action string) { controls = append(controls, source+" "+action) }
	s.setVolume = func(source string, level int) {
		controls = append(controls, fmt.Sprintf("%s volume %d", source, level))
	}
	s.readMixer = func() media.Mixer {
		return media.Mixer{Volume: 70, Apps: []media.MixerApp{
			{ID: "testgame", Name: "TestGame", Volume: 90, Active: true},
			{ID: "shooter-win64-shipping", Name: "SHOOTER", Path: `C:\Launcher Games\Shooter\live\ShooterGame\Binaries\Win64\Shooter-Win64-Shipping.exe`, Volume: 80},
			{ID: "spotify", Name: "Spotify", Volume: 40},
			{ID: "steam", Name: "Steam", Volume: 100, Active: true},
			{ID: "discord", Name: "Discord", Volume: 60, Active: true},
			{ID: "chrome", Name: "Google Chrome", Volume: 100},
		}}
	}
	s.gameExes = []gameRef{{exe: "testgame", name: "Test Game"}, {exe: "shooter", dir: `c:\launcher games\shooter\live\`, name: "Shooter"}}
	s.setMixerVolume = func(app string, level int) { controls = append(controls, fmt.Sprintf("mixer %s %d", app, level)) }
	s.toggleMute = func(app string) { controls = append(controls, "mute "+app) }
	s.setMic = func(muted bool) { controls = append(controls, fmt.Sprintf("mic %v", muted)) }
	var pads []*fakePad
	s.newPad = func() padder { p := &fakePad{}; pads = append(pads, p); return p }

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/pair", "application/json", strings.NewReader(`{"code":"nope00","name":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong code: status %d", resp.StatusCode)
	}

	resp, err = http.Post(ts.URL+"/api/pair", "application/json",
		bytes.NewBufferString(`{"code":"`+store.Current().Digits+`","name":"Test phone"}`))
	if err != nil {
		t.Fatal(err)
	}
	var paired struct{ Token, Handoff string }
	if err := readJSON(resp, &paired); err != nil || paired.Token == "" || paired.Handoff == "" {
		t.Fatalf("pair: %v %+v", err, paired)
	}

	// The home screen app trades the handoff for the same token, once.
	for i, want := range []int{http.StatusOK, http.StatusUnauthorized} {
		resp, err = http.Post(ts.URL+"/api/pair", "application/json", strings.NewReader(`{"handoff":"`+paired.Handoff+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		var h struct{ Token string }
		readJSON(resp, &h)
		if resp.StatusCode != want || (i == 0 && h.Token != paired.Token) {
			t.Fatalf("handoff try %d: status %d token match %v", i, resp.StatusCode, h.Token == paired.Token)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()

	wsjson.Write(ctx, c, clientMsg{Type: "auth", Token: paired.Token})
	var layout serverMsg
	if err := wsjson.Read(ctx, c, &layout); err != nil || layout.Type != "layout" || len(layout.Groups) == 0 {
		t.Fatalf("layout: %v %+v", err, layout)
	}

	wsjson.Write(ctx, c, clientMsg{Type: "press", ID: "mute"})
	var res serverMsg
	if err := wsjson.Read(ctx, c, &res); err != nil || !res.OK {
		t.Fatalf("press: %v %+v", err, res)
	}
	if len(ran) != 1 || ran[0].Keys[0] != "volume_mute" {
		t.Fatalf("ran %+v", ran)
	}

	wsjson.Write(ctx, c, clientMsg{Type: "press", ID: "does_not_exist"})
	res = serverMsg{}
	if err := wsjson.Read(ctx, c, &res); err != nil || res.OK {
		t.Fatalf("unknown button should fail: %v %+v", err, res)
	}

	wsjson.Write(ctx, c, clientMsg{Type: "stats", On: true})
	var st serverMsg
	if err := wsjson.Read(ctx, c, &st); err != nil || st.Type != "stats" || st.Stats == nil || st.Stats.CPU.Usage != 42 {
		t.Fatalf("stats: %v %+v", err, st)
	}

	wsjson.Write(ctx, c, clientMsg{Type: "stats", On: false})
	wsjson.Write(ctx, c, clientMsg{Type: "media", On: true})
	for i := 0; ; i++ { // skip stats messages that were already in flight
		var m serverMsg
		if err := wsjson.Read(ctx, c, &m); err != nil || i > 5 {
			t.Fatalf("media: %v", err)
		}
		if m.Type == "media" {
			if m.Media == nil || len(m.Media.Sources) != 1 || m.Media.Sources[0].Title != "Song" {
				t.Fatalf("media: %+v", m.Media)
			}
			break
		}
	}

	// Controls only reach sources that exist, with valid actions and levels.
	wsjson.Write(ctx, c, clientMsg{Type: "mediactl", Source: "Spotify.exe", Action: "next"})
	wsjson.Write(ctx, c, clientMsg{Type: "mediactl", Source: "Spotify.exe", Action: "shutdown"})
	wsjson.Write(ctx, c, clientMsg{Type: "mediactl", Source: "evil.exe", Action: "toggle"})
	level, high := 30, 101
	wsjson.Write(ctx, c, clientMsg{Type: "volume", Source: "Spotify.exe", Level: &level})
	wsjson.Write(ctx, c, clientMsg{Type: "volume", Level: &level})
	wsjson.Write(ctx, c, clientMsg{Type: "volume", Source: "Spotify.exe", Level: &high})
	wsjson.Write(ctx, c, clientMsg{Type: "volume", App: "spotify", Level: &level})
	wsjson.Write(ctx, c, clientMsg{Type: "volume", App: "notepad", Level: &level})
	wsjson.Write(ctx, c, clientMsg{Type: "mute", App: "testgame"})
	wsjson.Write(ctx, c, clientMsg{Type: "mute"})
	wsjson.Write(ctx, c, clientMsg{Type: "mute", App: "notepad"})
	wsjson.Write(ctx, c, clientMsg{Type: "micmute", On: true})
	wsjson.Write(ctx, c, clientMsg{Type: "micmute", On: false})
	wsjson.Write(ctx, c, clientMsg{Type: "mouse", Action: "move", DX: 4, DY: -2})
	wsjson.Write(ctx, c, clientMsg{Type: "mouse", Action: "down", Button: "left"})
	wsjson.Write(ctx, c, clientMsg{Type: "ping"})
	acks := 0
	for i := 0; ; i++ {
		var m serverMsg
		if err := wsjson.Read(ctx, c, &m); err != nil || i > 10 {
			t.Fatalf("pong: %v", err)
		}
		if m.Type == "micack" {
			acks++
		}
		if m.Type == "pong" {
			break
		}
	}
	if acks != 2 {
		t.Errorf("micack answers: %d, want 2 (one per micmute)", acks)
	}
	want := []string{"Spotify.exe next", "Spotify.exe volume 30", " volume 30", "mixer spotify 30", "mute testgame", "mute ", "mic true", "mic false"}
	if fmt.Sprint(controls) != fmt.Sprint(want) {
		t.Fatalf("controls %q, want %q", controls, want)
	}

	// Trackpad events reach this phone's pad, and its buttons are let go on disconnect.
	if len(pads) != 1 {
		t.Fatalf("pads: %d", len(pads))
	}
	if n, _ := pads[0].state(); n != 2 {
		t.Fatalf("pad events: %d, want 2", n)
	}

	// The mixer stream names configured games after the game.
	wsjson.Write(ctx, c, clientMsg{Type: "media", On: false})
	wsjson.Write(ctx, c, clientMsg{Type: "mixer", On: true})
	for i := 0; ; i++ {
		var m serverMsg
		if err := wsjson.Read(ctx, c, &m); err != nil || i > 10 {
			t.Fatalf("mixer: %v", err)
		}
		if m.Type == "mixer" {
			// Games by name, Spotify (it has media controls), Discord (playing
			// sound); not Steam, nor a silent Chrome.
			var got []string
			for _, a := range m.Mixer.Apps {
				got = append(got, fmt.Sprintf("%s/%v", a.Name, a.Game))
			}
			want := "[Test Game/true Shooter/true Spotify/false Discord/false]"
			if m.Mixer.Volume != 70 || fmt.Sprint(got) != want {
				t.Fatalf("mixer: %v, want %s", got, want)
			}
			break
		}
	}

	// A flood of messages is cut off at the per-second budget.
	before, _ := pads[0].state()
	for i := 0; i < 1500; i++ {
		wsjson.Write(ctx, c, clientMsg{Type: "mouse", Action: "move", DX: 1})
	}
	wsjson.Write(ctx, c, clientMsg{Type: "ping"})
	for i := 0; ; i++ {
		var m serverMsg
		if err := wsjson.Read(ctx, c, &m); err != nil || i > 10 {
			t.Fatalf("pong after flood: %v", err)
		}
		if m.Type == "pong" {
			break
		}
	}
	if after, _ := pads[0].state(); after-before > msgBurst+100 || after-before < 100 {
		t.Fatalf("flood delivered %d of 1500 messages, want about %d", after-before, msgBurst)
	}

	c.Close(websocket.StatusNormalClosure, "")
	for i := 0; ; i++ {
		if _, released := pads[0].state(); released {
			break
		}
		if i > 100 {
			t.Fatal("pad was not released on disconnect")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestAdmin(t *testing.T) {
	cfg, _ := config.Parse([]byte(config.Default))
	store, _ := auth.Open(filepath.Join(t.TempDir(), "devices.json"))
	s := New(cfg, store, fstest.MapFS{}, nil)
	s.AdminPage = []byte(`<script nonce="{{NONCE}}">var T="{{ADMIN_TOKEN}}"</script>`)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/admin")
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(page), s.adminToken) || strings.Contains(string(page), "{{NONCE}}") {
		t.Fatalf("admin page not filled in: %s", page)
	}

	get := func(host, token string) int {
		req, _ := http.NewRequest("GET", ts.URL+"/admin/api/status", nil)
		if host != "" {
			req.Host = host
		}
		if token != "" {
			req.Header.Set("X-Admin-Token", token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if code := get("", s.adminToken); code != http.StatusOK {
		t.Errorf("status with token: %d", code)
	}
	if code := get("", ""); code != http.StatusForbidden {
		t.Errorf("status without token: %d, want 403", code)
	}
	if code := get("evil.example:80", s.adminToken); code != http.StatusMisdirectedRequest {
		t.Errorf("status via foreign Host: %d, want 421 (DNS rebinding guard)", code)
	}

	// The usage reading (the dashboard's CPU and RAM charts) is admin-only too.
	s.system = func() stats.Snapshot {
		return stats.Snapshot{CPU: stats.CPU{Usage: 12.5}, RAM: stats.RAM{UsedMB: 9000, TotalMB: 16000}}
	}
	for _, tc := range []struct {
		token string
		want  int
	}{{s.adminToken, http.StatusOK}, {"", http.StatusForbidden}} {
		req, _ := http.NewRequest("GET", ts.URL+"/admin/api/usage", nil)
		if tc.token != "" {
			req.Header.Set("X-Admin-Token", tc.token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var u struct {
			CPU        float64 `json:"cpu"`
			RAMUsedMB  uint64  `json:"ram_used_mb"`
			RAMTotalMB uint64  `json:"ram_total_mb"`
			Process    struct {
				PrivateMB float64 `json:"private_mb"`
			} `json:"process"`
		}
		if tc.want == http.StatusOK {
			if err := readJSON(resp, &u); err != nil || u.CPU != 12.5 || u.RAMUsedMB != 9000 || u.RAMTotalMB != 16000 {
				t.Errorf("usage: %v %+v", err, u)
			}
		} else {
			resp.Body.Close()
		}
		if resp.StatusCode != tc.want {
			t.Errorf("usage with token %q: %d, want %d", tc.token, resp.StatusCode, tc.want)
		}
	}

	// The status carries the current code and a QR code for the first address.
	s.Addresses = []string{"http://192.168.1.20:8765"}
	req, _ := http.NewRequest("GET", ts.URL+"/admin/api/status", nil)
	req.Header.Set("X-Admin-Token", s.adminToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var st adminStatus
	if err := readJSON(resp, &st); err != nil {
		t.Fatal(err)
	}
	if st.Code != store.Current().Digits || !strings.HasPrefix(st.PairURL, "http://192.168.1.20:8765/?v=") || !strings.Contains(st.PairURL, "#pair=") ||
		!strings.HasPrefix(st.QR, "<svg") || st.CodeLeft <= 0 || st.CodeLeft > 30 {
		t.Fatalf("pairing info: code=%q url=%q qr=%.20q left=%v", st.Code, st.PairURL, st.QR, st.CodeLeft)
	}
}

func TestStaticRevalidates(t *testing.T) {
	cfg, _ := config.Parse([]byte(config.Default))
	store, _ := auth.Open(filepath.Join(t.TempDir(), "devices.json"))
	files := fstest.MapFS{
		"index.html": {Data: []byte(`<script src="app.js"></script>`)},
		"app.js":     {Data: []byte("var v = 1;")},
	}
	ts := httptest.NewServer(New(cfg, store, files, nil).Handler())
	defer ts.Close()

	// The page is never cached and points at the script by content hash.
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.Header.Get("Cache-Control") != "no-store" || !strings.Contains(string(page), `src="app.js?v=`) {
		t.Fatalf("index: Cache-Control=%q page=%s", resp.Header.Get("Cache-Control"), page)
	}

	// The script itself is revalidated with its ETag.
	resp, err = http.Get(ts.URL + "/app.js?v=x")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	tag := resp.Header.Get("ETag")
	if tag == "" || resp.Header.Get("Cache-Control") != "no-cache" {
		t.Fatalf("missing cache headers: ETag=%q Cache-Control=%q", tag, resp.Header.Get("Cache-Control"))
	}
	req, _ := http.NewRequest("GET", ts.URL+"/app.js", nil)
	req.Header.Set("If-None-Match", tag)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotModified {
		t.Fatalf("revalidation: status %d, want 304", resp.StatusCode)
	}
}

// A phone whose localStorage was cleared still connects with the cookie set
// at pairing, and app-level pings are answered.
func TestCookieAuthAndPing(t *testing.T) {
	cfg, _ := config.Parse([]byte(config.Default))
	store, _ := auth.Open(filepath.Join(t.TempDir(), "devices.json"))
	ts := httptest.NewServer(New(cfg, store, fstest.MapFS{}, nil).Handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/pair", "application/json", strings.NewReader(`{"code":"`+store.Current().Digits+`","name":"iPhone"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == tokenCookie {
			cookie = c
		}
	}
	if cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.MaxAge < 3600 {
		t.Fatalf("token cookie missing or weak: %+v", cookie)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h := http.Header{}
	h.Set("Cookie", tokenCookie+"="+cookie.Value)
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/ws", &websocket.DialOptions{HTTPHeader: h})
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	wsjson.Write(ctx, c, clientMsg{Type: "auth"}) // no token: the cookie is used
	var m serverMsg
	if err := wsjson.Read(ctx, c, &m); err != nil || m.Type != "layout" {
		t.Fatalf("cookie auth: %v %+v", err, m)
	}
	wsjson.Write(ctx, c, clientMsg{Type: "ping"})
	m = serverMsg{}
	if err := wsjson.Read(ctx, c, &m); err != nil || m.Type != "pong" {
		t.Fatalf("ping: %v %+v", err, m)
	}
}

func TestHostAllowed(t *testing.T) {
	name, _ := os.Hostname()
	for host, want := range map[string]bool{
		"192.168.1.20:8765": true, "[fd00::1]:8765": true, "localhost:8765": true, name + ":8765": true,
		"evil.example:8765": false, "desklink.attacker.com": false,
	} {
		if got := hostAllowed(host); got != want {
			t.Errorf("hostAllowed(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestEditGames(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	cfg, err := config.Load(cfgPath) // writes the default config
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "My Game.exe")
	os.WriteFile(exe, []byte("x"), 0o600)

	store, _ := auth.Open(filepath.Join(dir, "devices.json"))
	s := New(cfg, store, fstest.MapFS{}, nil)
	s.ConfigPath = cfgPath
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	post := func(body string) (int, gamesResponse, string) {
		req, _ := http.NewRequest("POST", ts.URL+"/admin/api/games", strings.NewReader(body))
		req.Header.Set("X-Admin-Token", s.adminToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var g gamesResponse
		json.Unmarshal(b, &g)
		return resp.StatusCode, g, string(b)
	}

	// A path pasted with quotes, as Explorer's "Copy as path" gives it.
	quoted, _ := json.Marshal(`"` + exe + `"`)
	code, got, raw := post(`{"games":[{"name":"My Game","target":` + string(quoted) + `},{"name":"Steam One","steam_appid":730}]}`)
	if code != http.StatusOK || len(got.Games) != 2 || got.Games[0].ID != "my-game" || got.Games[0].Target != exe {
		t.Fatalf("save: %d %s", code, raw)
	}
	saved, err := config.Load(cfgPath)
	if err != nil || len(saved.Games) != 2 {
		t.Fatalf("config.json not updated: %v", err)
	}
	if a, ok := s.cfg.Find("my-game"); !ok || a.Target != exe {
		t.Fatalf("running config not swapped: %+v %v", a, ok)
	}

	// A missing file is refused and nothing changes.
	code, _, raw = post(`{"games":[{"name":"Gone","target":"C:\\nope\\gone.exe"}]}`)
	if code != http.StatusBadRequest || !strings.Contains(raw, "not found") {
		t.Fatalf("missing file: %d %s", code, raw)
	}
	if again, _ := config.Load(cfgPath); len(again.Games) != 2 {
		t.Fatal("config changed after a refused save")
	}
}

func readJSON(resp *http.Response, v any) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(v)
}

func TestBadTokenRejected(t *testing.T) {
	cfg, _ := config.Parse([]byte(config.Default))
	store, _ := auth.Open(filepath.Join(t.TempDir(), "devices.json"))
	ts := httptest.NewServer(New(cfg, store, fstest.MapFS{}, nil).Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	wsjson.Write(ctx, c, clientMsg{Type: "auth", Token: strings.Repeat("a", 64)})
	var msg serverMsg
	if err := wsjson.Read(ctx, c, &msg); err != nil || msg.Type != "unauthorized" {
		t.Fatalf("got %v %+v, want unauthorized", err, msg)
	}
}

type fakePad struct {
	mu       sync.Mutex
	events   []pointer.Event
	released bool
}

func (p *fakePad) Do(e pointer.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, e)
}

func (p *fakePad) Release() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.released = true
}

func (p *fakePad) Holding() bool { return false }

func (p *fakePad) state() (int, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.events), p.released
}

func TestGameNotStartedTwice(t *testing.T) {
	cfg, err := config.Parse([]byte(`{"port":8765,
		"games":[
			{"id":"testgame","name":"Test Game","target":"C:/Games/Game.lnk","icon_from":"C:/Games/TestGame.exe"},
			{"id":"cs2","name":"CS2","steam_appid":730},
			{"id":"only","name":"Only","target":"C:/Games/Only.lnk"}],
		"groups":[{"title":"Q","style":"quick","buttons":[
			{"id":"q_testgame","label":"FC","icon":"gamepad","action":{"type":"game","target":"testgame"}},
			{"id":"q_cs2","label":"CS2","icon":"gamepad","action":{"type":"game","target":"cs2"}}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	store, err := auth.Open(filepath.Join(t.TempDir(), "devices.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg, store, fstest.MapFS{"index.html": {Data: []byte("ok")}}, nil)
	launched := 0
	s.run = func(actions.Action) error { launched++; return nil }

	open := map[string]bool{}
	s.running = func() map[string]bool { return open }
	press := func(id string) error {
		// A fresh "last press" table: only the running check is under test.
		return s.press(id, &sync.Map{})
	}

	// Nothing is running: the game starts, from the game id or its button.
	if err := press("q_testgame"); err != nil || launched != 1 {
		t.Fatalf("first press: err %v, launched %d", err, launched)
	}
	// The game's program (the icon_from .exe, not the shortcut) is open now.
	open["testgame"] = true
	for _, id := range []string{"q_testgame", "testgame"} {
		if err := press(id); err != errAlreadyRunning {
			t.Errorf("press %s while running: err %v, want already running", id, err)
		}
	}
	if launched != 1 {
		t.Errorf("a running game was started again (%d launches)", launched)
	}
	// Another program with another name does not block it.
	delete(open, "testgame")
	open["otherprogram"] = true
	if err := press("q_testgame"); err != nil || launched != 2 {
		t.Errorf("press with another program open: err %v, launched %d", err, launched)
	}
	// Steam games are left to Steam, and a game whose program is unknown
	// (a .lnk with no icon_from) can always be started.
	open["cs2"], open["only"] = true, true
	if err := press("q_cs2"); err != nil || launched != 3 {
		t.Errorf("steam game: err %v, launched %d", err, launched)
	}
	if err := press("only"); err != nil || launched != 4 {
		t.Errorf("game with an unknown program: err %v, launched %d", err, launched)
	}
}

func TestSourceKeyGroupsAnIPv6Prefix(t *testing.T) {
	if got := sourceKey("192.168.1.20:5000"); got != "192.168.1.20" {
		t.Errorf("IPv4: %q", got)
	}
	a := sourceKey("[fd00:1:2:3:aaaa::1]:5000")
	b := sourceKey("[fd00:1:2:3:bbbb::9]:6000")
	if a != b {
		t.Errorf("one /64 counted as two devices: %q, %q", a, b)
	}
	if c := sourceKey("[fd00:1:2:4::1]:5000"); c == a {
		t.Errorf("two different /64s counted as one: %q", c)
	}
	if got := sourceKey("[fe80::1%eth0]:5000"); got != sourceKey("[fe80::1]:5000") {
		t.Errorf("zone changes the key: %q", got)
	}
}

func TestSameOrigin(t *testing.T) {
	for _, tc := range []struct {
		origin, host string
		want         bool
	}{
		{"", "192.168.1.5:8765", true},
		{"http://192.168.1.5:8765", "192.168.1.5:8765", true},
		{"http://evil.example", "192.168.1.5:8765", false},
		{"http://192.168.1.5:9999", "192.168.1.5:8765", false},
		{"null", "192.168.1.5:8765", false},
	} {
		r := httptest.NewRequest("POST", "/api/pair", nil)
		r.Host = tc.host
		if tc.origin != "" {
			r.Header.Set("Origin", tc.origin)
		}
		if got := sameOrigin(r); got != tc.want {
			t.Errorf("origin %q host %q: got %v", tc.origin, tc.host, got)
		}
	}
}
