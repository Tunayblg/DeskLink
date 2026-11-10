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
	"github.com/Tunayblg/DeskLink/internal/winlist"
)

func TestTypingAndClaudeKeys(t *testing.T) {
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
	var typed []string
	var pressed []string
	s.typer = func(text string) bool { mu.Lock(); typed = append(typed, text); mu.Unlock(); return true }
	// Claude is the window in front.
	s.windows = func() []winlist.Window { return []winlist.Window{{ID: 7, App: "claude", Title: "Claude"}} }
	s.foreground = func() uint64 { return 7 }
	s.run = func(a actions.Action) error {
		mu.Lock()
		pressed = append(pressed, strings.Join(a.Keys, "+"))
		mu.Unlock()
		return nil
	}

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

	// Typed and not sent; typed then Enter; empty (not even Enter); long.
	wsjson.Write(ctx, c, clientMsg{Type: "type", Text: "  merhaba dünya  "})
	wsjson.Write(ctx, c, clientMsg{Type: "type", Text: "bitir\x07", Send: true})
	wsjson.Write(ctx, c, clientMsg{Type: "type", Text: " \n ", Send: true})
	wsjson.Write(ctx, c, clientMsg{Type: "type", Text: strings.Repeat("ü", 1500)})
	for _, k := range []string{"esc", "shifttab", "format-c", "ctrl", "1", ""} {
		wsjson.Write(ctx, c, clientMsg{Type: "key", Action: k})
	}
	// A ping is answered after everything before it was handled.
	wsjson.Write(ctx, c, clientMsg{Type: "ping"})
	var pong serverMsg
	if err := wsjson.Read(ctx, c, &pong); err != nil || pong.Type != "pong" {
		t.Fatalf("pong: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(typed) != 3 || typed[0] != "merhaba dünya" || typed[1] != "bitir" || len([]rune(typed[2])) != 1500 {
		t.Errorf("typed %q", typed)
	}
	if got, want := strings.Join(pressed, ","), "enter,esc,shift+tab"; got != want {
		t.Errorf("pressed %q, want %q", got, want)
	}
}
