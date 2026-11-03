// Package config loads and validates the user's button layout and game list.
//
// The config is the security boundary: the phone can only ask the server to
// run a button or game by ID. What that does is decided here, on the PC.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"

	"github.com/Tunayblg/DeskLink/internal/actions"
)

// Config is the content of config.json.
type Config struct {
	Port   int     `json:"port"`
	Groups []Group `json:"groups"`
	Games  []Game  `json:"games"`

	index map[string]actions.Action
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
	return Parse(data)
}

// Parse decodes and validates a config.
func Parse(data []byte) (*Config, error) {
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
	for _, g := range c.Games {
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
			if a.Type == "game" {
				ga, ok := games[a.Target]
				if !ok {
					return fmt.Errorf("button %q: no game with id %q", b.ID, a.Target)
				}
				a = ga
			}
			if err := add(b.ID, a); err != nil {
				return fmt.Errorf("button %w", err)
			}
		}
	}
	return nil
}

// Find returns the action for a button or game ID.
func (c *Config) Find(id string) (actions.Action, bool) {
	a, ok := c.index[id]
	return a, ok
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
        { "id": "lock",    "label": "Lock PC",    "icon": "lock",       "confirm": true, "action": { "type": "system", "command": "lock" } }
      ]
    }
  ],
  "games": []
}
`
