package server

import (
	"log"
	"strings"

	"github.com/Tunayblg/DeskLink/internal/actions"
	"github.com/Tunayblg/DeskLink/internal/keyboard"
	"github.com/Tunayblg/DeskLink/internal/winlist"
)

// The phone's developer screen types what was dictated or written into a
// program on the PC (the one in front, or one picked from the list of open
// windows) and can press a few fixed keys there. The phone sends text and a
// window from that list, never a command. Either way the window must be a
// developer window (see targets.go): the one in front is checked too.

// Answers for the phone when a text or key could not be delivered. Success is
// the empty string.
const (
	typeGone    = "gone"    // the chosen window is no longer open
	typeNoFocus = "focus"   // Windows would not bring it to the front
	typeRefused = "refused" // Windows did not accept the input (an administrator window)
	typeNotDev  = "notdev"  // the window in front is not a developer window
	typeOK      = ""
)

// listWindows is the list the phone may choose from: developer windows only.
func (s *Server) listWindows() []winlist.Window {
	if s.windows == nil {
		return nil
	}
	return devTargets(s.windows())
}

// toWindow brings the chosen window to the front and returns it. Nothing is
// typed unless it really is in front: text sent to the wrong program is worse
// than none. Window 0 means "the one in front", which must be a developer
// window as well.
func (s *Server) toWindow(id uint64) (winlist.Window, string) {
	if id == 0 {
		front := s.frontWindow()
		for _, w := range s.listWindows() {
			if front != 0 && w.ID == front {
				return w, typeOK
			}
		}
		return winlist.Window{}, typeNotDev
	}
	var target winlist.Window
	for _, w := range s.listWindows() {
		if w.ID == id {
			target = w
			break
		}
	}
	if target.ID == 0 {
		return target, typeGone
	}
	if s.focus == nil || s.focus(id) != nil {
		return target, typeNoFocus
	}
	// Focus waits a moment for the window to settle: look once more.
	if s.frontWindow() != id {
		return target, typeNoFocus
	}
	return target, typeOK
}

// forTarget fits a text to the window: in a terminal a line break would run
// the line (Shift+Enter is Enter there), so lines are joined with spaces.
func forTarget(text string, w winlist.Window) string {
	if w.Kind == kindTerm {
		return strings.ReplaceAll(text, "\n", " ")
	}
	return text
}

// typeText types the text into the window (0: the one in front), then presses
// Enter if send is set. A text that is empty after cleaning does nothing, not
// even the Enter. If Windows refused the text, Enter is not pressed either: it
// would run whatever was already written in that window. Nor is it pressed
// if another window came to the front while typing.
func (s *Server) typeText(text string, send bool, window uint64) string {
	text = keyboard.Clean(text)
	if text == "" {
		return typeOK
	}
	s.typeMu.Lock()
	defer s.typeMu.Unlock()
	w, code := s.toWindow(window)
	if code != typeOK {
		return code
	}
	if !s.typer(forTarget(text, w)) {
		log.Printf("typing: Windows did not accept the text (is the window running as administrator?)")
		return typeRefused
	}
	if send {
		if s.frontWindow() != w.ID {
			return typeLost
		}
		s.pressLocked("enter")
	}
	return typeOK
}

// pressNamed presses one of keyboard.Keys in the window (0: the one in front).
// Unknown names do nothing.
func (s *Server) pressNamed(name string, window uint64) string {
	if _, ok := keyboard.Keys[name]; !ok {
		return typeOK
	}
	s.typeMu.Lock()
	defer s.typeMu.Unlock()
	if _, code := s.toWindow(window); code != typeOK {
		return code
	}
	s.pressLocked(name)
	return typeOK
}

// pressLocked presses the named key for a caller that already holds typeMu.
func (s *Server) pressLocked(name string) {
	keys, ok := keyboard.Keys[name]
	if !ok {
		return
	}
	if err := s.run(actions.Action{Type: "keys", Keys: keys}); err != nil {
		log.Printf("key %s: %v", name, err)
	}
}
