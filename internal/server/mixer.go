package server

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/Tunayblg/DeskLink/internal/art"
	"github.com/Tunayblg/DeskLink/internal/config"
	"github.com/Tunayblg/DeskLink/internal/media"
	"github.com/Tunayblg/DeskLink/internal/running"
)

// The volume mixer page in game mode: the PC's volume and the programs worth
// a slider (the game, music, browsers, voice chat), each with its own slider
// and mute. Launchers and helpers that merely opened audio (Steam, RGB tools)
// are left out.

const (
	mixerApps = 7  // most programs shown; the ones playing sound come first
	iconSize  = 64 // mixer icons, in pixels
)

type mixerApp struct {
	media.MixerApp
	Game bool `json:"game,omitempty"` // one of the games set up in DeskLink
}

type mixerView struct {
	Volume int        `json:"volume"`
	Muted  bool       `json:"muted"`
	Apps   []mixerApp `json:"apps"`
	Mic    *media.Mic `json:"mic,omitempty"`
}

// gameRef is how the mixer recognizes a configured game: by its executable
// (mixer ID) or, since a launcher stub often starts the real game
// (Game.exe -> Game-Win64-Shipping.exe), by any program in its folder.
type gameRef struct {
	id   string // the game's id in config.json
	exe  string // lowercase, without ".exe"
	dir  string // lowercase, with a trailing separator; "" if unknown
	name string
}

func gameExes(games []config.Game) []gameRef {
	var out []gameRef
	for _, g := range games {
		exe := art.GameExe(g)
		if !strings.EqualFold(filepath.Ext(exe), ".exe") {
			continue
		}
		r := gameRef{id: g.ID, exe: running.Name(filepath.Base(exe)), name: g.Name}
		if filepath.IsAbs(exe) {
			r.dir = strings.ToLower(filepath.Dir(exe)) + string(filepath.Separator)
		}
		out = append(out, r)
	}
	return out
}

// gameName returns the configured game a mixer entry belongs to.
func gameName(games []gameRef, a media.MixerApp) (string, bool) {
	path := strings.ToLower(a.Path)
	for _, g := range games {
		if a.ID == g.exe || (g.dir != "" && strings.HasPrefix(path, g.dir)) {
			return g.name, true
		}
	}
	return "", false
}

// soundApps are programs shown in the mixer while they play sound, even
// without media controls (voice chat, players that do not publish them).
var soundApps = map[string]bool{"spotify": true, "discord": true, "chrome": true, "msedge": true, "opera": true,
	"firefox": true, "brave": true, "vlc": true, "teamspeak3": true, "ts3client_win64": true}

func (s *Server) mixerView() mixerView {
	m := s.readMixer()
	playing := s.nowPlaying().Exes()
	s.cfgMu.RLock()
	games := s.gameExes
	s.cfgMu.RUnlock()
	v := mixerView{Volume: m.Volume, Muted: m.Muted, Apps: []mixerApp{}, Mic: m.Mic}
	for _, a := range m.Apps {
		if len(v.Apps) == mixerApps {
			break
		}
		app := mixerApp{MixerApp: a}
		if name, ok := gameName(games, a); ok {
			app.Name, app.Game = name, true
		} else if !playing[a.ID] && !(soundApps[a.ID] && a.Active) {
			continue
		}
		v.Apps = append(v.Apps, app)
	}
	return v
}

// icon returns a program's icon as a data URI, cached by path.
func (s *Server) icon(path string) string {
	s.iconMu.Lock()
	defer s.iconMu.Unlock()
	if uri, ok := s.icons[path]; ok {
		return uri
	}
	uri := art.AppIcon(path, iconSize)
	if len(s.icons) > 128 {
		clear(s.icons)
	}
	s.icons[path] = uri
	return uri
}

// knownApp reports whether id is "" (the PC) or a program in the mixer.
func (s *Server) knownApp(id string) bool {
	if id == "" {
		return true
	}
	_, ok := s.readMixer().Find(id)
	return ok
}

// streamMixer sends the mixer when it changes (and every few seconds).
// Each program's icon is sent once per connection.
func (s *Server) streamMixer(ctx context.Context, c *websocket.Conn) {
	t := time.NewTicker(statsInterval)
	defer t.Stop()
	var prev string
	var sentAt time.Time
	sentIcon := map[string]bool{}
	for {
		v := s.mixerView()
		key := fmt.Sprint(v.Volume, v.Muted, v.Apps, v.Mic != nil && v.Mic.Muted, v.Mic != nil) // not v itself: its Mic is a pointer
		if key != prev || time.Since(sentAt) >= mediaResync {
			msg := serverMsg{Type: "mixer", Mixer: &v}
			for _, a := range v.Apps {
				if sentIcon[a.ID] {
					continue
				}
				sentIcon[a.ID] = true
				if uri := s.icon(a.Path); uri != "" {
					if msg.Icons == nil {
						msg.Icons = map[string]string{}
					}
					msg.Icons[a.ID] = uri
				}
			}
			if err := wsjson.Write(ctx, c, msg); err != nil {
				return
			}
			prev, sentAt = key, time.Now()
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
