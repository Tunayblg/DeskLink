package config

import (
	"encoding/json"
	"github.com/Tunayblg/DeskLink/internal/actions"
	"strings"
	"testing"
)

func withShortcuts(t *testing.T, s []Shortcut) (*Config, error) {
	t.Helper()
	c, err := Parse([]byte(Default))
	if err != nil {
		t.Fatal(err)
	}
	c.Shortcuts = s
	b, _ := json.Marshal(c)
	return Parse(b)
}

func TestShortcuts(t *testing.T) {
	c, err := withShortcuts(t, []Shortcut{
		{Name: "Netflix", Target: "https://www.netflix.com", Tint: "229,9,20"},
		{},
		{Name: "Notepad", Target: `C:\Windows\notepad.exe`},
	})
	if err != nil {
		t.Fatal(err)
	}
	a, ok := c.Find("shortcut-1")
	if !ok || a.Type != "launch" || a.Target != "https://www.netflix.com" {
		t.Fatalf("shortcut-1 = %+v %v", a, ok)
	}
	if _, ok := c.Find("shortcut-2"); ok {
		t.Error("an empty slot must not be pressable")
	}
	if _, ok := c.Find("shortcut-3"); !ok {
		t.Error("shortcut-3 missing")
	}
	if got := c.Slots(); len(got) != ShortcutSlots || got[1].Target != "" {
		t.Errorf("Slots() = %+v", got)
	}
}

func TestShortcutsRejectBadInput(t *testing.T) {
	for name, s := range map[string][]Shortcut{
		"too many":        {{Name: "a", Target: "https://a.com"}, {}, {}, {}, {}, {}, {Name: "g", Target: "https://g.com"}},
		"no such game":    {{Game: "nope"}},
		"game and target": {{Game: "g1", Target: "https://a.com"}},
		"no name":         {{Target: "https://a.com"}},
		"long name":       {{Name: strings.Repeat("x", 41), Target: "https://a.com"}},
		"no host":         {{Name: "a", Target: "https://"}},
		"space in url":    {{Name: "a", Target: "https://a.com/some thing"}},
		"bad tint":        {{Name: "a", Target: "https://a.com", Tint: "300,1,1"}},
		"tint text":       {{Name: "a", Target: "https://a.com", Tint: "red"}},
	} {
		if _, err := withShortcuts(t, s); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func configWithGames(t *testing.T) *Config {
	t.Helper()
	c, err := Parse([]byte(Default))
	if err != nil {
		t.Fatal(err)
	}
	c.Games = []Game{{ID: "g1", Name: "Game One", Target: `C:\Games\one.exe`}, {ID: "g2", Name: "Game Two", SteamAppID: 570}}
	return c
}

func TestShortcutCanBeAGame(t *testing.T) {
	c := configWithGames(t)
	c.Shortcuts = []Shortcut{{Game: "g2"}, {Name: "Netflix", Target: "https://netflix.com"}, {Game: "g1"}}
	b, _ := json.Marshal(c)
	c, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	a, ok := c.Find("shortcut-1")
	if !ok || a.Target != "steam://rungameid/570" {
		t.Fatalf("shortcut-1 = %+v %v", a, ok)
	}
	if g, ok := c.GameOf("shortcut-3"); !ok || g.ID != "g1" {
		t.Errorf("a game tile must be known as a game (so a running game is not started twice): %+v %v", g, ok)
	}
	if _, ok := c.GameOf("shortcut-2"); ok {
		t.Error("a website is not a game")
	}
}

// Removing a game empties the tile that pointed at it instead of breaking the config.
func TestRemovingAGameClearsItsTile(t *testing.T) {
	c := configWithGames(t)
	c.Shortcuts = []Shortcut{{Game: "g1"}, {Game: "g2"}}
	c.SetGames(c.Games[1:])
	if c.Shortcuts[0].Filled() || c.Shortcuts[1].Game != "g2" {
		t.Fatalf("tiles = %+v", c.Shortcuts)
	}
	b, _ := json.Marshal(c)
	if _, err := Parse(b); err != nil {
		t.Fatalf("config no longer valid: %v", err)
	}
}

// Game tiles that sat in the home group become the first home screen tiles.
func TestPinnedGamesMoveToTiles(t *testing.T) {
	c := configWithGames(t)
	quick := &c.Groups[c.HomeGroup()]
	quick.Buttons = append([]Button{
		{ID: "q_g1", Label: "Game One", Icon: "gamepad", Action: actions.Action{Type: "game", Target: "g1"}},
		{ID: "q_g2", Label: "Game Two", Icon: "gamepad", Action: actions.Action{Type: "game", Target: "g2"}},
	}, quick.Buttons...)
	c.Shortcuts = []Shortcut{{Name: "Netflix", Target: "https://netflix.com"}}
	c.Version = 2
	b, _ := json.Marshal(c)
	c, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	c.migrate()
	if c.Version != configVersion {
		t.Fatalf("version %d", c.Version)
	}
	if len(c.Shortcuts) != 3 || c.Shortcuts[0].Game != "g1" || c.Shortcuts[1].Game != "g2" || c.Shortcuts[2].Name != "Netflix" {
		t.Fatalf("tiles = %+v", c.Shortcuts)
	}
	for _, b := range c.Groups[c.HomeGroup()].Buttons {
		if b.Action.Type == "game" {
			t.Errorf("game button %q should be gone from the group", b.ID)
		}
	}
	data, _ := json.Marshal(c)
	if _, err := Parse(data); err != nil {
		t.Fatalf("migrated config is not valid: %v", err)
	}
}
