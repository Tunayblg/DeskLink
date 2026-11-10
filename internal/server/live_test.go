package server

import (
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Tunayblg/DeskLink/internal/actions"
	"github.com/Tunayblg/DeskLink/internal/auth"
	"github.com/Tunayblg/DeskLink/internal/config"
	"github.com/Tunayblg/DeskLink/internal/winlist"
)

func liveServer(t *testing.T) (*Server, *[]string, *uint64) {
	t.Helper()
	cfg, err := config.Parse([]byte(config.Default))
	if err != nil {
		t.Fatal(err)
	}
	store, err := auth.Open(filepath.Join(t.TempDir(), "devices.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg, store, fstest.MapFS{"index.html": {Data: []byte("ok")}}, nil)
	var did []string
	front := uint64(7)
	s.windows = func() []winlist.Window { return []winlist.Window{{ID: 7, App: "claude", Title: "Claude"}} }
	s.focus = func(id uint64) error { did = append(did, "focus"); return nil }
	s.foreground = func() uint64 { return front }
	s.liveTyper = func(text string) bool { did = append(did, "type:"+text); return true }
	s.backspace = func(n int) bool { did = append(did, "bs:"+strings.Repeat("x", n)); return true }
	s.run = func(a actions.Action) error { did = append(did, "key:"+strings.Join(a.Keys, "+")); return nil }
	return s, &did, &front
}

// Dictation arrives as a growing, sometimes rewritten text: only the change is typed.
func TestLiveTypingSendsOnlyTheChange(t *testing.T) {
	s, did, _ := liveServer(t)
	l := &liveSession{}
	for _, text := range []string{"Merhaba", "Merhaba ", "Merhaba dünya", "Merhaba dün", "Merhaba dün", "Merhaba dünyayı"} {
		if code := s.liveUpdate(l, text, 7); code != "" {
			t.Fatalf("live %q: %s", text, code)
		}
	}
	if code := s.liveEnd(l, true); code != "" {
		t.Fatal(code)
	}
	want := "focus,type:Merhaba,type: ,type:dünya,bs:xx,type:yayı,key:enter"
	// "Merhaba dünya" -> "Merhaba dün" deletes "ya" (2); the same text again does nothing;
	// "Merhaba dün" -> "Merhaba dünyayı" types "yayı".
	if got := strings.Join(*did, ","); got != want {
		t.Errorf("did\n %q\nwant\n %q", got, want)
	}
	if l.active {
		t.Error("session still open after liveEnd")
	}
}

// If another window comes to the front, nothing more is typed or deleted.
func TestLiveTypingStopsWhenTheWindowChanges(t *testing.T) {
	s, did, front := liveServer(t)
	l := &liveSession{}
	if code := s.liveUpdate(l, "ilk", 7); code != "" {
		t.Fatal(code)
	}
	*front = 99
	if code := s.liveUpdate(l, "ilk kelime", 7); code != typeLost {
		t.Fatalf("answer %q, want %q", code, typeLost)
	}
	if code := s.liveEnd(l, true); code != "" {
		t.Fatal(code)
	}
	if got, want := strings.Join(*did, ","), "focus,type:ilk"; got != want {
		t.Errorf("did %q, want %q", got, want)
	}
}

// An empty text starts nothing, and a session that was never started has nothing to end.
func TestLiveTypingNeedsText(t *testing.T) {
	s, did, _ := liveServer(t)
	l := &liveSession{}
	if code := s.liveUpdate(l, "", 7); code != "" {
		t.Fatal(code)
	}
	if code := s.liveEnd(l, true); code != "" {
		t.Fatal(code)
	}
	if len(*did) != 0 {
		t.Errorf("did %q for nothing", *did)
	}
}
