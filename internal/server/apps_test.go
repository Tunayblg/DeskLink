package server

import (
	"testing"

	"github.com/Tunayblg/DeskLink/internal/winlist"
)

func TestFocusAppOnlyFromTheOpenList(t *testing.T) {
	s, did, front := liveServer(t)
	s.windows = func() []winlist.Window {
		return []winlist.Window{{ID: 5, App: "vlc", Title: "Movie"}, {ID: 6, App: "chrome", Title: "News"}}
	}
	s.focus = func(id uint64) error { *front = id; *did = append(*did, "focus"); return nil }

	if code := s.focusApp(5); code != typeOK || *front != 5 {
		t.Fatalf("listed window: %q (front %d)", code, *front)
	}
	for _, id := range []uint64{0, 99} {
		if code := s.focusApp(id); code != typeGone {
			t.Errorf("window %d: %q, want %q", id, code, typeGone)
		}
	}
	if n := len(*did); n != 1 {
		t.Fatalf("an unlisted window was focused: %v", *did)
	}
	// Windows says yes but another window stays in front.
	s.focus = func(id uint64) error { return nil }
	*front = 1
	if code := s.focusApp(6); code != typeNoFocus {
		t.Fatalf("got %q, want %q", code, typeNoFocus)
	}
	s.focus = func(id uint64) error { return winlist.ErrNotFocused }
	if code := s.focusApp(6); code != typeNoFocus {
		t.Fatalf("got %q, want %q", code, typeNoFocus)
	}
}

func TestHelperWindowsAreLeftOutUnlessAsked(t *testing.T) {
	s, _, _ := liveServer(t)
	s.windows = func() []winlist.Window {
		return []winlist.Window{
			{ID: 1, App: "WhatsApp.Root", Title: "WhatsApp"},
			{ID: 2, App: "steamwebhelper", Title: "Friends"},
			{ID: 3, App: "NZXT CAM", Title: "NZXT CAM"},
			{ID: 4, App: "msedge", Title: "DeskLink"},
			{ID: 5, App: "msedge", Title: "Some page - Microsoft Edge"},
		}
	}
	if got := s.openWindows(false); len(got) != 2 || got[0].ID != 1 || got[1].ID != 5 {
		t.Fatalf("tidy list: %v", got)
	}
	if got := s.openWindows(true); len(got) != 5 {
		t.Fatalf("full list: %v", got)
	}
}
