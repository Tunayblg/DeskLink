// Package config loads and validates the user's button layout and game list.
//
// The config is the security boundary: the phone can only ask the server to
// run a button or game by ID. What that does is decided here, on the PC.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/Tunayblg/DeskLink/internal/actions"
)

// configVersion numbers the shape of config.json. Version 2 added a shut down
// button to the Quick group. Version 3 turned the game tiles of the home
// screen into general home screen tiles (see Shortcut).
const configVersion = 3

// Config is the content of config.json.
type Config struct {
	Version int     `json:"version,omitempty"`
	Port    int     `json:"port"`
	Groups  []Group `json:"groups"`
	Games   []Game  `json:"games"`
	// Shortcuts are the three shortcut tiles of the home screen (see Shortcut).
	Shortcuts []Shortcut `json:"shortcuts,omitempty"`

	index  map[string]actions.Action
	gameOf map[string]int // game id or game button id -> position in Games
}

// Group is a card of buttons on the home screen.
//
//	media: a now-playing card with a volume slider; the first three buttons
//	       are previous / play-pause / next, a fourth one (mute) sits next to
//	       the slider
//	list:  one button per row
//	grid:  square tiles
//	quick: game buttons as a row of tiles, other buttons as a row of icons,
//	       and live CPU/GPU mini charts below
type Group struct {
	Title   string   `json:"title"`
	Style   string   `json:"style"`
	Buttons []Button `json:"buttons"`
}

// Button is one control on the home screen.
type Button struct {
	ID      string         `json:"id"`
	Label   string         `json:"label"`
	Icon    string         `json:"icon"`
	Confirm bool           `json:"confirm,omitempty"`
	Action  actions.Action `json:"action"`
}

// Game is a tile on the games screen. Either SteamAppID or Target is set.
type Game struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	SteamAppID int    `json:"steam_appid,omitempty"`
	Target     string `json:"target,omitempty"`
	Args       string `json:"args,omitempty"`
	// Cover is an optional image file (JPEG or PNG). Without it, Steam games
	// use Steam's own cover art and other games use their executable's icon.
	Cover string `json:"cover,omitempty"`
	// IconFrom is the executable to take the icon from, if not Target
	// (e.g. a launcher that starts a different game exe).
	IconFrom string `json:"icon_from,omitempty"`
}

// Action returns what pressing the game tile does.
func (g Game) Action() actions.Action {
	if g.SteamAppID > 0 {
		return actions.Action{Type: "launch", Target: fmt.Sprintf("steam://rungameid/%d", g.SteamAppID)}
	}
	return actions.Action{Type: "launch", Target: g.Target, Args: g.Args}
}

// PublicGroup and PublicButton are what the phone is allowed to see. Action
// targets stay on the PC, except for page buttons, which the phone handles.
type PublicGroup struct {
	Title   string         `json:"title"`
	Style   string         `json:"style"`
	Buttons []PublicButton `json:"buttons"`
}

type PublicButton struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Icon    string `json:"icon"`
	Confirm bool   `json:"confirm,omitempty"`
	Page    string `json:"page,omitempty"`
	Game    string `json:"game,omitempty"` // game id; the phone shows that game's art
}

var (
	idPattern = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)
	styles    = map[string]bool{"media": true, "list": true, "grid": true, "quick": true}
)

// Load reads path, writing the default config there first if it does not exist.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		data = []byte(Default)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return nil, fmt.Errorf("writing default config: %w", err)
		}
	} else if err != nil {
		return nil, err
	}
	c, err := Parse(data)
	if err != nil {
		return nil, err
	}
	if c.Version < configVersion {
		c.migrate()
		if saved, err := c.Save(path); err == nil {
			c = saved
		} else {
			log.Printf("config: could not save the updated config.json: %v", err)
		}
	}
	return c, nil
}

// migrate brings an older config up to date. It only adds what a newer release
// ships by default, and only once (the version is saved with it), so a button
// the user removes later stays removed.
func (c *Config) migrate() {
	if c.Version < 2 {
		c.addShutdownButton()
	}
	if c.Version < 3 {
		c.movePinnedGames()
	}
	c.Version = configVersion
}

// addShutdownButton puts a shut down button after the lock button of the Quick
// group. Configs without the usual sleep/lock buttons are left alone.
func (c *Config) addShutdownButton() {
	for _, g := range c.Groups {
		for _, b := range g.Buttons {
			if b.ID == "shutdown" || (b.Action.Type == "system" && b.Action.Command == "shutdown") {
				return // already there, or the ID is taken
			}
		}
	}
	for gi := range c.Groups {
		g := &c.Groups[gi]
		if g.Style != "quick" {
			continue
		}
		lock := -1
		for i, b := range g.Buttons {
			if b.Action.Type == "system" && b.Action.Command == "lock" {
				lock = i
			}
		}
		if lock < 0 {
			continue
		}
		btn := Button{ID: "shutdown", Label: "Shut down", Icon: "power", Confirm: true,
			Action: actions.Action{Type: "system", Command: "shutdown"}}
		rest := append([]Button{btn}, g.Buttons[lock+1:]...)
		g.Buttons = append(g.Buttons[:lock+1], rest...)
		return
	}
}

// Parse decodes and validates a config.
func Parse(data []byte) (*Config, error) {
	// Notepad can save a file with a byte order mark in front; JSON does not allow one.
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("config.json is not valid JSON: %w", err)
	}
	if c.Port == 0 {
		c.Port = 8765
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) validate() error {
	if c.Port < 1024 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1024 and 65535, got %d", c.Port)
	}
	if len(c.Groups) > 8 {
		return fmt.Errorf("at most 8 groups are supported, got %d", len(c.Groups))
	}
	if len(c.Games) > 32 {
		return fmt.Errorf("at most 32 games are supported, got %d", len(c.Games))
	}

	c.index = map[string]actions.Action{}
	c.gameOf = map[string]int{}
	add := func(id string, a actions.Action) error {
		if !idPattern.MatchString(id) {
			return fmt.Errorf("id %q must be 1-32 chars of a-z, 0-9, _ or -", id)
		}
		if _, dup := c.index[id]; dup {
			return fmt.Errorf("duplicate id %q", id)
		}
		if err := a.Validate(); err != nil {
			return fmt.Errorf("%q: %w", id, err)
		}
		c.index[id] = a
		return nil
	}

	// Games first, so buttons can point at them.
	games := map[string]actions.Action{}
	gamePos := map[string]int{}
	for gi, g := range c.Games {
		if g.Name == "" || len(g.Name) > 40 {
			return fmt.Errorf("game %q: name must be 1-40 characters", g.ID)
		}
		if (g.SteamAppID > 0) == (g.Target != "") {
			return fmt.Errorf("game %q: set either steam_appid or target", g.ID)
		}
		if g.SteamAppID < 0 {
			return fmt.Errorf("game %q: invalid steam_appid", g.ID)
		}
		if err := add(g.ID, g.Action()); err != nil {
			return fmt.Errorf("game %w", err)
		}
		games[g.ID] = g.Action()
		gamePos[g.ID] = gi
		c.gameOf[g.ID] = gi
	}

	if err := c.validateShortcuts(add, gamePos); err != nil {
		return err
	}

	for gi, g := range c.Groups {
		if g.Style == "" {
			c.Groups[gi].Style = "grid"
		} else if !styles[g.Style] {
			return fmt.Errorf("group %q: style must be media, list, grid or quick", g.Title)
		}
		if len(g.Title) > 24 {
			return fmt.Errorf("group %q: title must be at most 24 characters", g.Title)
		}
		if len(g.Buttons) == 0 || len(g.Buttons) > 16 {
			return fmt.Errorf("group %q: needs 1-16 buttons", g.Title)
		}
		for _, b := range g.Buttons {
			if b.Label == "" || len(b.Label) > 40 {
				return fmt.Errorf("button %q: label must be 1-40 characters", b.ID)
			}
			a := b.Action
			pos := -1
			if a.Type == "game" {
				ga, ok := games[a.Target]
				if !ok {
					return fmt.Errorf("button %q: no game with id %q", b.ID, a.Target)
				}
				pos = gamePos[a.Target]
				a = ga
			}
			if err := add(b.ID, a); err != nil {
				return fmt.Errorf("button %w", err)
			}
			if pos >= 0 {
				c.gameOf[b.ID] = pos
			}
		}
	}
	return nil
}

// Clone returns a deep copy, for editing.
func (c *Config) Clone() *Config {
	b, _ := json.Marshal(c)
	var out Config
	json.Unmarshal(b, &out)
	return &out
}

// Save validates c and writes it to path, keeping the previous file as
// path + ".bak". The file is replaced atomically.
func (c *Config) Save(path string) (*Config, error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, err
	}
	checked, err := Parse(data)
	if err != nil {
		return nil, err
	}
	if old, err := os.ReadFile(path); err == nil {
		os.WriteFile(path+".bak", old, 0o600)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, path); err != nil {
		return nil, err
	}
	return checked, nil
}

// HomeGroup is the group that held the home screen's game tiles before they
// became general tiles (see movePinnedGames): the first quick group, or else
// the first list group. -1 if there is none.
func (c *Config) HomeGroup() int {
	for _, style := range []string{"quick", "list"} {
		for i, g := range c.Groups {
			if g.Style == style {
				return i
			}
		}
	}
	return -1
}

// SetGames replaces the game list. Home screen tiles and buttons that pointed
// at a game that no longer exists are emptied or dropped, so the config stays valid.
func (c *Config) SetGames(games []Game) {
	c.Games = games
	exists := map[string]bool{}
	for _, g := range games {
		exists[g.ID] = true
	}
	for i, sc := range c.Shortcuts {
		if sc.Game != "" && !exists[sc.Game] {
			c.Shortcuts[i] = Shortcut{} // its game is gone
		}
	}
	for gi := range c.Groups {
		var keep []Button
		for _, b := range c.Groups[gi].Buttons {
			if b.Action.Type == "game" && !exists[b.Action.Target] {
				continue
			}
			keep = append(keep, b)
		}
		c.Groups[gi].Buttons = keep
	}
}

// Find returns the action for a button or game ID.
func (c *Config) Find(id string) (actions.Action, bool) {
	a, ok := c.index[id]
	return a, ok
}

// GameOf returns the game that a game id or a game button launches.
func (c *Config) GameOf(id string) (Game, bool) {
	i, ok := c.gameOf[id]
	if !ok || i < 0 || i >= len(c.Games) {
		return Game{}, false
	}
	return c.Games[i], true
}

// Public returns the home screen layout without action details.
func (c *Config) Public() []PublicGroup {
	out := make([]PublicGroup, len(c.Groups))
	for i, g := range c.Groups {
		pg := PublicGroup{Title: g.Title, Style: g.Style, Buttons: make([]PublicButton, len(g.Buttons))}
		for j, b := range g.Buttons {
			pb := PublicButton{ID: b.ID, Label: b.Label, Icon: b.Icon, Confirm: b.Confirm}
			switch b.Action.Type {
			case "page":
				pb.Page = b.Action.Target
			case "game":
				pb.Game = b.Action.Target
			}
			pg.Buttons[j] = pb
		}
		out[i] = pg
	}
	return out
}

// Default is written to config.json on first run.
const Default = `{
  "version": 3,
  "port": 8765,
  "groups": [
    {
      "title": "Media",
      "style": "media",
      "buttons": [
        { "id": "prev",     "label": "Previous",   "icon": "prev",        "action": { "type": "keys", "keys": ["media_prev"] } },
        { "id": "play",     "label": "Play/Pause", "icon": "play-pause",  "action": { "type": "keys", "keys": ["media_play_pause"] } },
        { "id": "next",     "label": "Next",       "icon": "next",        "action": { "type": "keys", "keys": ["media_next"] } },
        { "id": "mute",     "label": "Mute",       "icon": "mute",        "action": { "type": "keys", "keys": ["volume_mute"] } }
      ]
    },
    {
      "title": "Quick",
      "style": "quick",
      "buttons": [
        { "id": "spotify", "label": "Spotify",    "icon": "music",      "action": { "type": "launch", "target": "spotify:" } },
        { "id": "sleep",   "label": "Sleep",      "icon": "moon",       "confirm": true, "action": { "type": "system", "command": "sleep" } },
        { "id": "lock",    "label": "Lock PC",    "icon": "lock",       "confirm": true, "action": { "type": "system", "command": "lock" } },
        { "id": "shutdown", "label": "Shut down", "icon": "power",      "confirm": true, "action": { "type": "system", "command": "shutdown" } }
      ]
    }
  ],
  "games": []
}
`
