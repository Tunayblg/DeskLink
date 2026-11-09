package server

import (
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
	"github.com/Tunayblg/DeskLink/internal/config"
	"github.com/Tunayblg/DeskLink/internal/plan"
)

func TestPlanEditsOverWebSocket(t *testing.T) {
	cfg, _ := config.Parse([]byte(config.Default))
	store, _ := auth.Open(filepath.Join(t.TempDir(), "devices.json"))
	s := New(cfg, store, fstest.MapFS{}, nil)
	pl, err := plan.Open(filepath.Join(t.TempDir(), "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	s.SetPlan(pl)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/pair", "application/json", strings.NewReader(`{"code":"`+store.Current().Digits+`","name":"iPhone"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	var token string
	for _, c := range resp.Cookies() {
		if c.Name == tokenCookie {
			token = c.Value
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	wsjson.Write(ctx, c, clientMsg{Type: "auth", Token: token})
	read := func(want string) serverMsg {
		t.Helper()
		var m serverMsg
		if err := wsjson.Read(ctx, c, &m); err != nil || m.Type != want {
			t.Fatalf("want %q: %v %+v", want, err, m)
		}
		return m
	}
	read("layout")
	if m := read("plan"); m.Plan == nil || len(*m.Plan) != 0 {
		t.Fatalf("empty plan should be sent as an empty list: %+v", m)
	}

	wsjson.Write(ctx, c, clientMsg{Type: "planadd", Title: "Study", Time: "14:00"})
	m := read("plan")
	if m.Plan == nil || len(*m.Plan) != 1 || (*m.Plan)[0].Title != "Study" {
		t.Fatalf("after add: %+v", m)
	}
	id := (*m.Plan)[0].ID

	wsjson.Write(ctx, c, clientMsg{Type: "planadd", Title: "", Time: ""})
	if m := read("planerr"); m.Error == "" {
		t.Fatalf("a bad item should be refused: %+v", m)
	}

	for _, bad := range []clientMsg{
		{Type: "planadd", Title: "Bad time", Time: "25:00"},
		{Type: "planadd", Title: "Bad date", Date: "2026-02-30"},
		{Type: "plantoggle", ID: id},
		{Type: "plantoggle", ID: id, Date: "2026-2-01"},
		{Type: "plantoggle", ID: "unknown", Date: "2026-11-08"},
	} {
		wsjson.Write(ctx, c, bad)
		if read("planerr").Error == "" {
			t.Fatal("missing validation error")
		}
	}
	for i, date := range []string{"2026-11-08", "2026-11-09", "2026-11-09"} {
		wsjson.Write(ctx, c, clientMsg{Type: "plantoggle", ID: id, Date: date})
		m = read("plan")
		want := date
		if i == 2 {
			want = ""
		}
		if (*m.Plan)[0].DoneOn != want {
			t.Fatalf("toggle: %+v", m)
		}
	}
	if pl.List()[0].DoneOn != "" {
		t.Fatal("undo not persisted")
	}

	wsjson.Write(ctx, c, clientMsg{Type: "plandel", ID: id})
	if m := read("plan"); m.Plan == nil || len(*m.Plan) != 0 {
		t.Fatalf("after delete: %+v", m)
	}
}
