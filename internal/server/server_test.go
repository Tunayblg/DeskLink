package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/Tunayblg/DeskLink/internal/actions"
	"github.com/Tunayblg/DeskLink/internal/auth"
	"github.com/Tunayblg/DeskLink/internal/config"
	"github.com/Tunayblg/DeskLink/internal/media"
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
	s.nowPlaying = func() media.Info { return media.Info{Title: "Song", Playing: true, Volume: 50} }

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
			if m.Media == nil || m.Media.Title != "Song" {
				t.Fatalf("media: %+v", m.Media)
			}
			break
		}
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
	if st.Code != store.Current().Digits || !strings.HasPrefix(st.PairURL, "http://192.168.1.20:8765/#pair=") ||
		!strings.HasPrefix(st.QR, "<svg") || st.CodeLeft <= 0 || st.CodeLeft > 30 {
		t.Fatalf("pairing info: code=%q url=%q qr=%.20q left=%v", st.Code, st.PairURL, st.QR, st.CodeLeft)
	}
}

func TestStaticRevalidates(t *testing.T) {
	cfg, _ := config.Parse([]byte(config.Default))
	store, _ := auth.Open(filepath.Join(t.TempDir(), "devices.json"))
	ts := httptest.NewServer(New(cfg, store, fstest.MapFS{"index.html": {Data: []byte("<p>v1</p>")}}, nil).Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	tag := resp.Header.Get("ETag")
	if tag == "" || resp.Header.Get("Cache-Control") != "no-cache" {
		t.Fatalf("missing cache headers: ETag=%q Cache-Control=%q", tag, resp.Header.Get("Cache-Control"))
	}
	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
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
	code, got, raw := post(`{"games":[{"name":"My Game","target":` + string(quoted) + `,"pinned":true},{"name":"Steam One","steam_appid":730}]}`)
	if code != http.StatusOK || len(got.Games) != 2 || got.Games[0].ID != "my-game" || !got.Games[0].Pinned || got.Games[0].Target != exe {
		t.Fatalf("save: %d %s", code, raw)
	}
	saved, err := config.Load(cfgPath)
	if err != nil || len(saved.Games) != 2 || !saved.Pinned()["my-game"] {
		t.Fatalf("config.json not updated: %v", err)
	}
	if a, ok := s.cfg.Find("q_my-game"); !ok || a.Target != exe {
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
