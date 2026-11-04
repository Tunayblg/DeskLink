// Package actions defines the fixed set of things a button can do.
//
// There is deliberately no "run this shell command" action: every action type
// is validated when config.json is loaded, and the phone never sends one.
package actions

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Action describes what happens when a button is pressed.
//
//	launch: open an app, file, folder or URI (e.g. "spotify:") via the Windows shell
//	keys:   press a key combination, e.g. ["ctrl", "shift", "esc"] or ["volume_up"]
//	system: a built-in system command: "lock", "sleep" or "shutdown"
//	page:   switch the phone to another screen, e.g. "stats" (never runs on the PC)
//	game:   launch one of the configured games by its id
type Action struct {
	Type    string   `json:"type"`
	Target  string   `json:"target,omitempty"`
	Args    string   `json:"args,omitempty"`
	Keys    []string `json:"keys,omitempty"`
	Command string   `json:"command,omitempty"`
}

// Virtual-key codes, see https://learn.microsoft.com/windows/win32/inputdev/virtual-key-codes
var keyCodes = map[string]uint8{
	"ctrl": 0x11, "shift": 0x10, "alt": 0x12, "win": 0x5B,
	"enter": 0x0D, "esc": 0x1B, "tab": 0x09, "space": 0x20, "backspace": 0x08, "delete": 0x2E,
	"up": 0x26, "down": 0x28, "left": 0x25, "right": 0x27,
	"home": 0x24, "end": 0x23, "pageup": 0x21, "pagedown": 0x22, "printscreen": 0x2C,
	"volume_mute": 0xAD, "volume_down": 0xAE, "volume_up": 0xAF,
	"media_next": 0xB0, "media_prev": 0xB1, "media_stop": 0xB2, "media_play_pause": 0xB3,
}

func init() {
	for c := 'a'; c <= 'z'; c++ {
		keyCodes[string(c)] = uint8(c - 'a' + 'A')
	}
	for c := '0'; c <= '9'; c++ {
		keyCodes[string(c)] = uint8(c)
	}
	for i := 1; i <= 24; i++ {
		keyCodes[fmt.Sprintf("f%d", i)] = uint8(0x70 + i - 1)
	}
}

var systemCommands = map[string]bool{"lock": true, "sleep": true, "shutdown": true}

// Pages are screens of the phone panel that a button can open.
var pages = map[string]bool{"home": true, "games": true, "stats": true, "clock": true}

// Validate reports whether the action is well formed. Called once at config load.
func (a Action) Validate() error {
	switch a.Type {
	case "launch":
		if strings.TrimSpace(a.Target) == "" {
			return fmt.Errorf("launch action needs a target")
		}
	case "keys":
		if len(a.Keys) == 0 || len(a.Keys) > 4 {
			return fmt.Errorf("keys action needs 1-4 keys")
		}
		for _, k := range a.Keys {
			if _, ok := keyCodes[k]; !ok {
				return fmt.Errorf("unknown key %q", k)
			}
		}
	case "system":
		if !systemCommands[a.Command] {
			return fmt.Errorf("unknown system command %q", a.Command)
		}
	case "game":
		// The target must be a game ID; the config package checks that it exists.
		if strings.TrimSpace(a.Target) == "" {
			return fmt.Errorf("game action needs a target game id")
		}
	case "page":
		if !pages[a.Target] {
			return fmt.Errorf("unknown page %q (use home, games, stats or clock)", a.Target)
		}
	default:
		return fmt.Errorf("unknown action type %q (use launch, keys, system, page or game)", a.Type)
	}
	return nil
}

var envVar = regexp.MustCompile(`%([A-Za-z0-9_]+)%`)

// expandEnv replaces Windows-style %VAR% references.
func expandEnv(s string) string {
	return envVar.ReplaceAllStringFunc(s, func(m string) string {
		if v, ok := os.LookupEnv(m[1 : len(m)-1]); ok {
			return v
		}
		return m
	})
}
