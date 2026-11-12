// Package winlist lists the windows open on the PC and brings one to the front,
// so the phone can send text to a chosen program (WhatsApp, an editor, a
// terminal) instead of whatever happens to be in front.
//
// The phone only ever names a window from the list it was given; DeskLink
// checks the choice against a fresh list before it focuses anything.
package winlist

import "errors"

// Window is one open program window.
type Window struct {
	ID    uint64 `json:"id"`             // the window handle
	App   string `json:"app"`            // the program, e.g. "WhatsApp" or "chrome"
	Title string `json:"title"`          // what the window's title bar says
	Kind  string `json:"kind,omitempty"` // set by the server: ai, term or editor
}

// ErrNotFocused means Windows would not bring the window to the front.
var ErrNotFocused = errors.New("window did not come to the front")

// MaxWindows is the most windows listed.
const MaxWindows = 60

// maxTitle is the longest title kept, in characters.
const maxTitle = 80

func shorten(s string) string {
	r := []rune(s)
	if len(r) > maxTitle {
		return string(r[:maxTitle-1]) + "…"
	}
	return s
}
