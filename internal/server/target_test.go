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

// Sending to a chosen program: only windows from the list, and only if the
// window really came to the front.
func TestSendToAChosenWindow(t *testing.T) {
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
	var log []string
	s.windows = func() []winlist.Window {
		return []winlist.Window{{ID: 7, App: "claude", Title: "Claude"}, {ID: 8, App: "cursor", Title: "Won't come forward"}}
	}
	var front uint64
	s.focus = func(id uint64) error {
		mu.Lock()
		defer mu.Unlock()
		log = append(log, "focus")
		if id == 8 {
			return winlist.ErrNotFocused
		}
		front = id
		return nil
	}
	s.foreground = func() uint64 { mu.Lock(); defer mu.Unlock(); return front }
	s.typer = func(text string) bool { mu.Lock(); log = append(log, "type:"+text); mu.Unlock(); return true }
	s.run = func(a actions.Action) error {
		mu.Lock()
		log = append(log, "key:"+strings.Join(a.Keys, "+"))
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
	var m serverMsg
	if err := wsjson.Read(ctx, c, &m); err != nil || m.Type != "layout" {
		t.Fatalf("layout: %v", err)
	}

	// The list comes back on request.
	wsjson.Write(ctx, c, clientMsg{Type: "windows"})
	m = serverMsg{}
	if err := wsjson.Read(ctx, c, &m); err != nil || m.Type != "windows" || len(m.Windows) != 2 || m.Windows[0].App != "Claude" {
		t.Fatalf("windows: %v %+v", err, m)
	}

	// A listed window: focused, typed into, then Enter.
	wsjson.Write(ctx, c, clientMsg{Type: "type", Text: "selam", Send: true, Window: 7})
	// A window that is not in the list: nothing happens but an answer.
	wsjson.Write(ctx, c, clientMsg{Type: "type", Text: "nope", Send: true, Window: 99})
	m = serverMsg{}
	if err := wsjson.Read(ctx, c, &m); err != nil || m.Type != "typeres" || m.Error != "gone" {
		t.Fatalf("gone answer: %v %+v", err, m)
	}
	// A window that will not come to the front: no text, no Enter.
	wsjson.Write(ctx, c, clientMsg{Type: "type", Text: "nope", Send: true, Window: 8})
	m = serverMsg{}
	if err := wsjson.Read(ctx, c, &m); err != nil || m.Type != "typeres" || m.Error != "focus" {
		t.Fatalf("focus answer: %v %+v", err, m)
	}
	// A key goes to the chosen window too.
	wsjson.Write(ctx, c, clientMsg{Type: "key", Action: "esc", Window: 7})
	wsjson.Write(ctx, c, clientMsg{Type: "ping"})
	m = serverMsg{}
	if err := wsjson.Read(ctx, c, &m); err != nil || m.Type != "pong" {
		t.Fatalf("pong: %v %+v", err, m)
	}

	mu.Lock()
	defer mu.Unlock()
	if got, want := strings.Join(log, ","), "focus,type:selam,key:enter,focus,focus,key:esc"; got != want {
		t.Errorf("did %q, want %q", got, want)
	}
}
