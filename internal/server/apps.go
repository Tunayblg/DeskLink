package server

import (
	"strings"

	"github.com/Tunayblg/DeskLink/internal/winlist"
)

// The window switcher on the phone's trackpad screen: every open program
// window is listed, and the chosen one is brought to the front, so the person
// on the couch can go back to a video, a game or a browser without hunting
// for it with the mouse. Unlike developer mode (see targets.go) nothing is
// typed: this is a pointer-level feature, as trusted as the trackpad itself.
// The phone names a window from the list it was given; DeskLink checks it
// against a fresh list before it focuses anything.

// helperApps are programs that keep a window open without being something to
// switch to: hardware tools, game launchers' helper windows, overlays. The
// list is only a default; the phone can ask for every window.
var helperApps = map[string]bool{
	"steamwebhelper": true, "riot client": true, "riotclientux": true, "riotclientservices": true,
	"nzxt cam": true, "razerappengine": true, "razercentral": true, "razer synapse 3": true,
	"msiafterburner": true, "rtss": true, "nvidia overlay": true, "nvcontainer": true,
	"textinputhost": true, "searchhost": true, "startmenuexperiencehost": true, "shellexperiencehost": true,
	"lockapp": true, "gamebar": true, "gamebarftserver": true, "widgets": true, "applicationframehost": true,
}

// isHelper reports whether a window is one of those helper windows or
// DeskLink's own dashboard (a browser tab titled "DeskLink").
func isHelper(w winlist.Window) bool {
	if helperApps[strings.ToLower(w.App)] {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(w.Title), "DeskLink")
}

// openWindows is the program windows the phone may switch to: without the
// helpers, unless all is set.
func (s *Server) openWindows(all bool) []winlist.Window {
	if s.windows == nil {
		return nil
	}
	list := s.windows()
	if all {
		return list
	}
	out := make([]winlist.Window, 0, len(list))
	for _, w := range list {
		if !isHelper(w) {
			out = append(out, w)
		}
	}
	return out
}

// focusApp brings a window from the open list to the front. An empty answer
// means it is in front now (see the type* codes in typing.go).
func (s *Server) focusApp(id uint64) string {
	s.typeMu.Lock() // not in the middle of someone's typing
	defer s.typeMu.Unlock()
	found := false
	for _, w := range s.openWindows(true) { // any real window, helpers included: the filter only tidies the list
		if w.ID == id && id != 0 {
			found = true
			break
		}
	}
	if !found {
		return typeGone
	}
	if s.focus == nil || s.focus(id) != nil || s.frontWindow() != id {
		return typeNoFocus
	}
	return typeOK
}
