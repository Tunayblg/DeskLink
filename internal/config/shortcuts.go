package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/Tunayblg/DeskLink/internal/actions"
)

// ShortcutSlots is how many tiles the phone's home screen has (two rows of three).
const ShortcutSlots = 6

// Shortcut is one tile of the home screen: one of your games, a web address
// (Netflix, YouTube, anything) or a program, opened on the PC with one tap. A
// slot with neither a game nor a target is empty and shows a "+" on the phone.
type Shortcut struct {
	Game   string `json:"game,omitempty"` // the ID of one of the games
	Name   string `json:"name,omitempty"`
	Target string `json:"target,omitempty"` // https://… address, or a program, file or URI
	Tint   string `json:"tint,omitempty"`   // tile color as "r,g,b" (websites)
}

// Filled reports whether the slot has something to open.
func (s Shortcut) Filled() bool { return s.Game != "" || strings.TrimSpace(s.Target) != "" }

// ShortcutID is the button ID a home screen tile is pressed by.
func ShortcutID(slot int) string { return fmt.Sprintf("shortcut-%d", slot+1) }

// IsWebAddress reports whether a shortcut target is a web address.
func IsWebAddress(target string) bool {
	l := strings.ToLower(target)
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://")
}

// Slots returns the shortcuts padded with empty ones up to ShortcutSlots.
func (c *Config) Slots() []Shortcut {
	out := make([]Shortcut, ShortcutSlots)
	copy(out, c.Shortcuts)
	return out
}

var tintPattern = regexp.MustCompile(`^(\d{1,3}),(\d{1,3}),(\d{1,3})$`)

// validateShortcuts checks the home screen tiles and registers the filled
// ones. gamePos maps game IDs to their place in Games.
func (c *Config) validateShortcuts(add func(string, actions.Action) error, gamePos map[string]int) error {
	if len(c.Shortcuts) > ShortcutSlots {
		return fmt.Errorf("at most %d shortcuts are supported, got %d", ShortcutSlots, len(c.Shortcuts))
	}
	for i, s := range c.Shortcuts {
		if !s.Filled() {
			continue
		}
		id := ShortcutID(i)
		if s.Game != "" {
			pos, ok := gamePos[s.Game]
			if !ok {
				return fmt.Errorf("shortcut %d: no game with id %q", i+1, s.Game)
			}
			if s.Target != "" {
				return fmt.Errorf("shortcut %d: set either game or target", i+1)
			}
			if err := add(id, c.Games[pos].Action()); err != nil {
				return fmt.Errorf("shortcut %w", err)
			}
			c.gameOf[id] = pos
			continue
		}
		if s.Name == "" || len(s.Name) > 40 {
			return fmt.Errorf("shortcut %d: name must be 1-40 characters", i+1)
		}
		if IsWebAddress(s.Target) {
			u, err := url.Parse(s.Target)
			if err != nil || u.Host == "" || strings.ContainsAny(s.Target, " \t\r\n\"") {
				return fmt.Errorf("shortcut %d: %q is not a valid web address", i+1, s.Target)
			}
		}
		if m := tintPattern.FindStringSubmatch(s.Tint); s.Tint != "" && (m == nil || atoi(m[1]) > 255 || atoi(m[2]) > 255 || atoi(m[3]) > 255) {
			return fmt.Errorf("shortcut %d: tint must look like 229,9,20", i+1)
		}
		if err := add(id, actions.Action{Type: "launch", Target: s.Target}); err != nil {
			return fmt.Errorf("shortcut %w", err)
		}
	}
	return nil
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}

// movePinnedGames turns the game tiles that used to sit in the home group into
// the first home screen tiles, in the same order. Tiles that would not fit
// stay in the games list. A group left without any button is not emptied.
func (c *Config) movePinnedGames() {
	home := c.HomeGroup()
	if home < 0 {
		return
	}
	g := &c.Groups[home]
	var tiles []Shortcut
	var keep []Button
	for _, b := range g.Buttons {
		if b.Action.Type == "game" {
			tiles = append(tiles, Shortcut{Game: b.Action.Target})
		} else {
			keep = append(keep, b)
		}
	}
	if len(tiles) == 0 || len(keep) == 0 {
		return
	}
	for _, s := range c.Shortcuts {
		if s.Filled() {
			tiles = append(tiles, s)
		}
	}
	if len(tiles) > ShortcutSlots {
		tiles = tiles[:ShortcutSlots]
	}
	g.Buttons, c.Shortcuts = keep, tiles
}
