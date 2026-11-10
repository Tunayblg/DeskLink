package server

import (
	"github.com/Tunayblg/DeskLink/internal/keyboard"
	"github.com/Tunayblg/DeskLink/internal/winlist"
)

// Live typing: while the person dictates or writes on the phone, the text
// appears in the chosen program as it comes, instead of all at once on Send.
//
// The phone sends the whole text each time it changes. The PC compares it with
// what it has typed so far, presses Backspace for the part that changed (the
// speech recognizer often rewrites its last words) and types the rest. Each
// phone has its own session, and only the newest one may type: an older one
// (another phone) is stopped, so two phones never delete each other's text. A
// session is tied to the window it started in: if another window comes to the
// front, nothing more is typed or deleted, because Backspace there would eat
// somebody else's text.

// typeLost is the answer when the window in front is no longer the one the
// live session started in (or another phone took over).
const typeLost = "lost"

// liveSession is what one phone has typed live so far.
type liveSession struct {
	active bool
	text   string // what is in the program now, as typed by this session
	front  uint64 // the window the session started in
	kind   string // what kind of window that is (a terminal gets no line breaks)
}

func (l *liveSession) reset() { *l = liveSession{} }

// commonRunes is how many characters two texts share at the start.
func commonRunes(a, b []rune) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

// liveUpdate makes the program's text equal the phone's. An empty answer
// means it worked.
func (s *Server) liveUpdate(l *liveSession, text string, window uint64) string {
	text = keyboard.Sanitize(text)
	s.typeMu.Lock()
	defer s.typeMu.Unlock()
	if !l.active {
		if text == "" {
			return typeOK
		}
		w, code := s.toWindow(window)
		if code != typeOK {
			return code
		}
		l.active, l.text, l.front, l.kind = true, "", w.ID, w.Kind
		s.liveOwner = l
	} else if s.liveOwner != l || s.frontWindow() != l.front {
		if s.liveOwner == l {
			s.liveOwner = nil
		}
		l.reset()
		return typeLost
	}
	if text == l.text {
		return typeOK
	}
	old, now := []rune(l.text), []rune(text)
	keep := commonRunes(old, now)
	if back := len(old) - keep; back > 0 && !s.backspace(back) {
		s.dropLive(l)
		return typeRefused
	}
	l.text = string(old[:keep]) // the part that is still there
	if rest := string(now[keep:]); rest != "" {
		if !s.liveTyper(forTarget(rest, winlist.Window{Kind: l.kind})) {
			s.dropLive(l)
			return typeRefused
		}
	}
	l.text = text
	return typeOK
}

// dropLive ends a session after a failure; the caller holds typeMu.
func (s *Server) dropLive(l *liveSession) {
	if s.liveOwner == l {
		s.liveOwner = nil
	}
	l.reset()
}

// liveEnd closes the session. With enter, it presses Enter first, if the
// session still owns the typing and its window is still in front.
func (s *Server) liveEnd(l *liveSession, enter bool) string {
	s.typeMu.Lock()
	defer s.typeMu.Unlock()
	owner := s.liveOwner == l
	defer s.dropLive(l)
	if !l.active {
		return typeOK
	}
	if enter {
		if !owner || s.frontWindow() != l.front {
			return typeLost
		}
		s.pressLocked("enter")
	}
	return typeOK
}

func (s *Server) frontWindow() uint64 {
	if s.foreground == nil {
		return 0
	}
	return s.foreground()
}
