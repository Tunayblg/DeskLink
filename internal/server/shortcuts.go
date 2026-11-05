package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/Tunayblg/DeskLink/internal/art"
	"github.com/Tunayblg/DeskLink/internal/config"
)

// publicShortcut is a home screen tile as the phone sees it. A slot without a
// name is empty, and the phone draws a "+" there. Pressing a tile sends its ID.
type publicShortcut struct {
	ID    string `json:"id"`
	Name  string `json:"name,omitempty"`
	Kind  string `json:"kind,omitempty"`  // "game", "web" or "app"; empty for an empty slot
	Game  string `json:"game,omitempty"`  // the game's ID, for a game tile (its picture is in the layout's games)
	Tint  string `json:"tint,omitempty"`  // "r,g,b" tile color
	Image string `json:"image,omitempty"` // data: URI of a program's icon
	Brand string `json:"brand,omitempty"` // a known service (Netflix, YouTube …): the panel draws its icon
}

// buildShortcuts makes the home screen tiles for the phone. Games use their own
// picture, programs carry the program's icon, and websites are drawn from their
// name and color.
func buildShortcuts(cfg *config.Config) []publicShortcut {
	names := map[string]string{}
	for _, g := range cfg.Games {
		names[g.ID] = g.Name
	}
	out := make([]publicShortcut, 0, config.ShortcutSlots)
	for i, s := range cfg.Slots() {
		p := publicShortcut{ID: config.ShortcutID(i)}
		switch {
		case !s.Filled():
		case s.Game != "":
			p.Name, p.Kind, p.Game = names[s.Game], "game", s.Game
		case config.IsWebAddress(s.Target):
			p.Name, p.Kind, p.Tint = s.Name, "web", s.Tint
		default:
			p.Name, p.Kind, p.Tint = s.Name, "app", s.Tint
			p.Image = art.AppIcon(s.Target, 96)
		}
		// A known service gets its own icon and, unless one was chosen, its own color.
		if b, ok := config.BrandFor(s.Target); ok && s.Game == "" {
			p.Brand = b.Name
			if p.Tint == "" {
				p.Tint = b.Tint
			}
		}
		out = append(out, p)
	}
	return out
}

// Shortcut editing from the dashboard (local to this PC, like the games).

type shortcutEntry struct {
	Game   string `json:"game,omitempty"`
	Name   string `json:"name"`
	Target string `json:"target"`
	Tint   string `json:"tint,omitempty"`
	Icon   string `json:"icon,omitempty"`  // read only: a program's icon, for the dashboard
	Brand  string `json:"brand,omitempty"` // read only: a known service, for the dashboard
}

type shortcutsResponse struct {
	Slots []shortcutEntry `json:"slots"`
}

func (s *Server) shortcutsList() shortcutsResponse {
	cfg, games := s.current()
	s.cfgMu.RLock()
	pub := s.shortcuts
	s.cfgMu.RUnlock()
	gameArt := map[string]string{}
	for _, g := range games {
		gameArt[g.ID] = g.Icon
		if g.Icon == "" {
			gameArt[g.ID] = g.Image
		}
	}
	gameName := map[string]string{}
	for _, g := range cfg.Games {
		gameName[g.ID] = g.Name
	}
	out := shortcutsResponse{Slots: []shortcutEntry{}}
	for i, sc := range cfg.Slots() {
		e := shortcutEntry{Game: sc.Game, Name: sc.Name, Target: sc.Target, Tint: sc.Tint}
		if sc.Game != "" {
			e.Name, e.Icon = gameName[sc.Game], gameArt[sc.Game]
		} else if i < len(pub) {
			e.Icon, e.Brand = pub[i].Image, pub[i].Brand
			if e.Tint == "" {
				e.Tint = pub[i].Tint
			}
		}
		out.Slots = append(out.Slots, e)
	}
	return out
}

func (s *Server) handleShortcutsGet(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.shortcutsList())
}

// handleShortcutsSave replaces the three shortcut slots, saves config.json and
// sends the new layout to connected phones.
func (s *Server) handleShortcutsSave(w http.ResponseWriter, r *http.Request) {
	if s.ConfigPath == "" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Editing is not available."})
		return
	}
	var req shortcutsResponse
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Bad request."})
		return
	}
	cfg, _ := s.current()
	known := map[string]bool{}
	for _, g := range cfg.Games {
		known[g.ID] = true
	}
	slots, err := normalizeShortcuts(req.Slots, known)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	edit := cfg.Clone()
	edit.Shortcuts = slots
	saved, err := edit.Save(s.ConfigPath)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	s.apply(saved, art.Load(saved.Games))
	log.Printf("dashboard: saved shortcuts")
	writeJSON(w, http.StatusOK, s.shortcutsList())
}

// normalizeShortcuts cleans up what the dashboard sent: trims text, removes the
// quotes "Copy as path" adds, checks that program files exist, and turns a
// bare address like "netflix.com" into a web address. games holds the IDs of
// the games a tile may point at.
func normalizeShortcuts(in []shortcutEntry, games map[string]bool) ([]config.Shortcut, error) {
	if len(in) > config.ShortcutSlots {
		return nil, fmt.Errorf("There are only %d shortcut slots.", config.ShortcutSlots)
	}
	out := make([]config.Shortcut, config.ShortcutSlots)
	for i, e := range in {
		target := cleanPath(e.Target)
		name := strings.TrimSpace(e.Name)
		if e.Game != "" {
			if !games[e.Game] {
				return nil, fmt.Errorf("Slot %d: that game no longer exists.", i+1)
			}
			out[i] = config.Shortcut{Game: e.Game}
			continue
		}
		if target == "" {
			continue
		}
		if name == "" {
			return nil, fmt.Errorf("Slot %d needs a name.", i+1)
		}
		if len(name) > 40 {
			return nil, fmt.Errorf("%q: names can be at most 40 characters.", name)
		}
		if looksLikeDomain(target) {
			target = "https://" + target
		}
		if !config.IsWebAddress(target) {
			if err := checkFile(target); err != nil {
				return nil, fmt.Errorf("%q: %v", name, err)
			}
		}
		out[i] = config.Shortcut{Name: name, Target: target, Tint: strings.TrimSpace(e.Tint)}
	}
	return out, nil
}

// looksLikeDomain reports whether text is a bare web address such as
// "netflix.com" or "www.youtube.com/watch": no scheme, no path separators or
// drive letters before the first slash, and a dot in the host part.
func looksLikeDomain(s string) bool {
	if strings.ContainsAny(s, " \t:%") || strings.Contains(s, `\`) {
		return false
	}
	host, _, _ := strings.Cut(s, "/")
	return strings.Contains(host, ".") && !strings.HasPrefix(host, ".") && !strings.HasSuffix(host, ".")
}
