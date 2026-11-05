package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
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
)

func TestWatchKeysAndBrightness(t *testing.T) {
	cfg, err := config.Parse([]byte(config.Default))
	if err != nil {
		t.Fatal(err)
	}
	store, err := auth.Open(filepath.Join(t.TempDir(), "devices.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg, store, fstest.MapFS{"index.html": {Data: []byte("ok")}}, nil)
	var mu sync.Mutex
	var ran []actions.Action
	s.run = func(a actions.Action) error { mu.Lock(); ran = append(ran, a); mu.Unlock(); return nil }
	s.getBrightness = func() (int, bool) { return 37, true }
	var set []int
	s.setBrightness = func(l int) { mu.Lock(); set = append(set, l); mu.Unlock() }

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	resp, err := http.Post(ts.URL+"/api/pair", "application/json", bytes.NewBufferString(`{"code":"`+store.Current().Digits+`","name":"Test phone"}`))
	if err != nil {
		t.Fatal(err)
	}
	var paired struct{ Token string }
	if err := readJSON(resp, &paired); err != nil || paired.Token == "" {
		t.Fatalf("pair: %v", err)
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
	if err := wsjson.Read(ctx, c, &layout); err != nil || layout.Type != "layout" {
		t.Fatalf("layout: %v", err)
	}

	// Only the listed watch buttons do anything, and each presses exactly its keys.
	for _, a := range []string{"playpause", "back", "forward", "fullscreen", "f", "format-c", ""} {
		wsjson.Write(ctx, c, clientMsg{Type: "watch", Action: a})
	}
	good, bad, over := 50, -1, 101
	wsjson.Write(ctx, c, clientMsg{Type: "brightness", Level: &good})
	wsjson.Write(ctx, c, clientMsg{Type: "brightness", Level: &bad})
	wsjson.Write(ctx, c, clientMsg{Type: "brightness", Level: &over})

	// A brightness message without a level is a question; the answer comes back.
	wsjson.Write(ctx, c, clientMsg{Type: "brightness"})
	var m serverMsg
	if err := wsjson.Read(ctx, c, &m); err != nil || m.Type != "brightness" || m.Brightness == nil || *m.Brightness != 37 {
		t.Fatalf("brightness answer: %v %+v", err, m)
	}

	mu.Lock()
	defer mu.Unlock()
	want := [][]string{{"media_play_pause"}, {"left"}, {"right"}} // fullscreen is not a watch button: it would press F in whatever window is in front
	if len(ran) != len(want) {
		t.Fatalf("ran %d actions, want %d: %+v", len(ran), len(want), ran)
	}
	for i, w := range want {
		if ran[i].Type != "keys" || strings.Join(ran[i].Keys, "+") != strings.Join(w, "+") {
			t.Errorf("watch %d pressed %+v, want %v", i, ran[i], w)
		}
	}
	if len(set) != 1 || set[0] != 50 {
		t.Errorf("brightness set %v, want only [50]", set)
	}

	// A screen that cannot be controlled answers without a level.
	s.getBrightness = func() (int, bool) { return 0, false }
	wsjson.Write(ctx, c, clientMsg{Type: "brightness"})
	m = serverMsg{}
	if err := wsjson.Read(ctx, c, &m); err != nil || m.Type != "brightness" || m.Brightness != nil {
		t.Fatalf("unsupported answer: %v %+v", err, m)
	}
}
