package server

import (
	"strings"
	"testing"

	"github.com/Tunayblg/DeskLink/internal/winlist"
)

// "The window in front" is only typed into when it is a developer window.
func TestFrontWindowMustBeADeveloperWindow(t *testing.T) {
	s, did, front := liveServer(t)
	var typed []string
	s.typer = func(text string) bool { typed = append(typed, text); return true }
	s.windows = func() []winlist.Window {
		return []winlist.Window{{ID: 7, App: "claude", Title: "Claude"}, {ID: 9, App: "WhatsApp", Title: "WhatsApp"}, {ID: 10, App: "cmd", Title: "cmd"}}
	}
	*front = 9
	if code := s.typeText("merhaba", true, 0); code != typeNotDev {
		t.Fatalf("WhatsApp in front: %q, want %q", code, typeNotDev)
	}
	if code := s.pressNamed("enter", 0); code != typeNotDev {
		t.Fatalf("key with WhatsApp in front: %q", code)
	}
	if code := s.liveUpdate(&liveSession{}, "selam", 0); code != typeNotDev {
		t.Fatalf("live with WhatsApp in front: %q", code)
	}
	if len(typed) != 0 || len(*did) != 0 {
		t.Fatalf("something reached WhatsApp: %v %v", typed, *did)
	}
	*front = 7
	if code := s.typeText("merhaba", false, 0); code != "" || len(typed) != 1 {
		t.Fatalf("Claude in front: %q %v", code, typed)
	}
	// A terminal runs a line at a line break: the lines are joined instead.
	*front = 10
	if code := s.typeText("echo bir\necho iki", false, 0); code != "" || typed[1] != "echo bir echo iki" {
		t.Fatalf("terminal got %q (%q)", typed[len(typed)-1], code)
	}
}

// A window that only seemed to come forward gets nothing.
func TestFocusIsCheckedAgain(t *testing.T) {
	s, did, front := liveServer(t)
	*front = 1 // focus "succeeds" but another window stays in front
	if code := s.typeText("x", true, 7); code != typeNoFocus {
		t.Fatalf("got %q, want %q", code, typeNoFocus)
	}
	if strings.Contains(strings.Join(*did, ","), "key:") {
		t.Fatalf("a key was pressed: %v", *did)
	}
}

// When a second phone starts typing live, the first one stops.
func TestNewestLiveSessionOwnsTheTyping(t *testing.T) {
	s, did, _ := liveServer(t)
	a, b := &liveSession{}, &liveSession{}
	if code := s.liveUpdate(a, "abc", 7); code != "" {
		t.Fatal(code)
	}
	if code := s.liveUpdate(b, "XYZ", 7); code != "" {
		t.Fatal(code)
	}
	n := len(*did)
	if code := s.liveUpdate(a, "ab", 7); code != typeLost {
		t.Fatalf("old phone: %q, want %q", code, typeLost)
	}
	if len(*did) != n {
		t.Fatalf("the old phone still typed: %v", (*did)[n:])
	}
	if code := s.liveEnd(a, true); code != "" {
		t.Fatalf("closing the old session: %q", code)
	}
	if code := s.liveEnd(b, true); code != "" || (*did)[len(*did)-1] != "key:enter" {
		t.Fatalf("new phone's Enter: %q %v", code, *did)
	}
}

func TestBrowserTitlesNeedTheSiteName(t *testing.T) {
	cases := map[string]bool{
		"Kod incelemesi - Claude - Google Chrome": true,
		"Claude - Google Chrome":                  true,
		"ChatGPT - Profil 1 - Microsoft Edge":     true,
		"Google Gemini - Google Chrome":           true,
		"Claude hakkında — Gmail - Google Chrome": false,
		"Inbox - Gmail - Google Chrome":           false,
		"claude code tutorial - YouTube - Chrome": false,
	}
	for title, want := range cases {
		if _, got := webAIName(title); got != want {
			t.Errorf("%q: %v, want %v", title, got, want)
		}
	}
}
