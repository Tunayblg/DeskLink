package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/Tunayblg/DeskLink/internal/auth"
	"github.com/Tunayblg/DeskLink/internal/battery"
	"github.com/Tunayblg/DeskLink/internal/config"
)

// The phone asks for the batteries and gets the list back, empty or not.
func TestBatteriesReply(t *testing.T) {
	cfg, err := config.Parse([]byte(config.Default))
	if err != nil {
		t.Fatal(err)
	}
	store, err := auth.Open(filepath.Join(t.TempDir(), "devices.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg, store, fstest.MapFS{"index.html": {Data: []byte("ok")}}, nil)
	devices := []battery.Device{{Name: "Keyboard", Percent: 71}}
	s.batteries = func() []battery.Device { return devices }

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

	wsjson.Write(ctx, c, clientMsg{Type: "batteries"})
	m = serverMsg{}
	if err := wsjson.Read(ctx, c, &m); err != nil || m.Type != "batteries" || m.Batteries == nil || len(*m.Batteries) != 1 || (*m.Batteries)[0].Percent != 71 {
		t.Fatalf("batteries: %v %+v", err, m)
	}

	// Nothing connected is an empty list, not a missing one, so the phone can tell.
	devices = nil
	wsjson.Write(ctx, c, clientMsg{Type: "batteries"})
	m = serverMsg{}
	if err := wsjson.Read(ctx, c, &m); err != nil || m.Type != "batteries" || m.Batteries == nil || len(*m.Batteries) != 0 {
		t.Fatalf("empty batteries: %v %+v", err, m)
	}
}
