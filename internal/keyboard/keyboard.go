// Package keyboard types text on the PC for the phone's Claude screen.
//
// The phone sends what was dictated or typed; this package sends it to the
// focused window as Unicode characters, the way a keyboard would. That works
// for any language (no layout mapping, no clipboard touched). A few fixed
// keys can be pressed too. Nothing here runs a command.
package keyboard

import (
	"strings"
	"unicode/utf8"
)

// MaxRunes is the longest text accepted in one go.
const MaxRunes = 2000

// Keys are the keys the phone may press on this screen, by name.
var Keys = map[string][]string{
	"enter":    {"enter"},
	"esc":      {"esc"},
	"tab":      {"tab"},
	"shifttab": {"shift", "tab"},
	"up":       {"up"},
	"down":     {"down"},
	"ctrlc":    {"ctrl", "c"},
}

// Clean makes the text safe to type: control characters go (a line break
// becomes "\n", which is typed as Shift+Enter so it does not send), the length
// is capped, and surrounding space is trimmed.
func Clean(s string) string { return strings.TrimSpace(Sanitize(s)) }

// Sanitize is Clean without the trimming. Live typing needs it: a text that is
// being dictated ends in a space between two words, and that space is real.
func Sanitize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	var b strings.Builder
	n := 0
	for _, r := range s {
		if r == utf8.RuneError {
			continue
		}
		if r == '\t' {
			r = ' '
		}
		if r < ' ' && r != '\n' || r == 0x7f {
			continue
		}
		if n >= MaxRunes {
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// TypeLive types a piece of a text that is being typed live, spaces and all.
// It reports whether Windows took all of it.
func TypeLive(text string) bool {
	text = Sanitize(text)
	if text == "" {
		return true
	}
	return typeText(text)
}

// Backspace presses Backspace n times (at most MaxRunes) and reports whether
// Windows took all of them.
func Backspace(n int) bool {
	if n <= 0 {
		return true
	}
	return backspace(min(n, MaxRunes))
}

// Type sends the text to the focused window and reports whether Windows took
// all of it (it refuses input for a window that runs as administrator). The
// caller makes sure only one text is typed at a time.
func Type(text string) bool {
	text = Clean(text)
	if text == "" {
		return false
	}
	return typeText(text)
}
