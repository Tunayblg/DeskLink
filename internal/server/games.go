package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Tunayblg/DeskLink/internal/art"
	"github.com/Tunayblg/DeskLink/internal/config"
)

// Game editing from the dashboard. Like the rest of /admin, these handlers
// only answer on this PC (see localOnly and adminAPI in admin.go).

func (s *Server) port() int {
	cfg, _ := s.current()
	return cfg.Port
}

// gameEntry is a game as the dashboard edits it.
type gameEntry struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	SteamAppID int    `json:"steam_appid,omitempty"`
	Target     string `json:"target,omitempty"`
	Args       string `json:"args,omitempty"`
	IconFrom   string `json:"icon_from,omitempty"`
	Cover      string `json:"cover,omitempty"`
	Icon       string `json:"icon,omitempty"` // read only: current art, for the list
}

type gamesResponse struct {
	Games []gameEntry `json:"games"`
}

func (s *Server) gamesList() gamesResponse {
	cfg, games := s.current()
	art := map[string]string{}
	for _, g := range games {
		if g.Icon != "" {
			art[g.ID] = g.Icon
		} else {
			art[g.ID] = g.Image
		}
	}
	out := gamesResponse{Games: []gameEntry{}}
	for _, g := range cfg.Games {
		out.Games = append(out.Games, gameEntry{ID: g.ID, Name: g.Name, SteamAppID: g.SteamAppID, Target: g.Target,
			Args: g.Args, IconFrom: g.IconFrom, Cover: g.Cover, Icon: art[g.ID]})
	}
	return out
}

func (s *Server) handleGamesGet(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.gamesList())
}

// handleGamesSave replaces the game list, saves config.json and sends the new
// layout to connected phones.
func (s *Server) handleGamesSave(w http.ResponseWriter, r *http.Request) {
	if s.ConfigPath == "" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Editing is not available."})
		return
	}
	var req struct {
		Games []gameEntry `json:"games"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Bad request."})
		return
	}
	games, err := normalizeGames(req.Games)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	cfg, _ := s.current()
	edit := cfg.Clone()
	edit.SetGames(games)
	saved, err := edit.Save(s.ConfigPath)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	s.apply(saved, art.Load(saved.Games))
	log.Printf("dashboard: saved %d games", len(saved.Games))
	writeJSON(w, http.StatusOK, s.gamesList())
}

func (s *Server) handleSteamGames(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"games": art.SteamGames()})
}

// handleBrowse opens the Windows file picker on the PC and returns the choice.
func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	if s.ChooseProgram == nil {
		writeJSON(w, http.StatusOK, map[string]string{})
		return
	}
	path, ok := s.ChooseProgram()
	if !ok {
		writeJSON(w, http.StatusOK, map[string]string{})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": path})
}

// handleWindow lets the dashboard enlarge its own window while the user guide
// is open, and put it back afterwards.
func (s *Server) handleWindow(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Maximize bool `json:"maximize"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Bad request."})
		return
	}
	changed := s.MaximizeWindow != nil && s.MaximizeWindow(req.Maximize)
	writeJSON(w, http.StatusOK, map[string]bool{"changed": changed})
}

var notSlug = regexp.MustCompile(`[^a-z0-9]+`)

// normalizeGames cleans up what the dashboard sent: trims names, removes the
// quotes that "Copy as path" adds, checks that program files exist, and gives
// every game a unique id.
func normalizeGames(in []gameEntry) ([]config.Game, error) {
	if len(in) > 32 {
		return nil, fmt.Errorf("At most 32 games are supported.")
	}
	used := map[string]bool{}
	var out []config.Game
	for _, e := range in {
		g := config.Game{
			Name:       strings.TrimSpace(e.Name),
			SteamAppID: e.SteamAppID,
			Target:     cleanPath(e.Target),
			Args:       strings.TrimSpace(e.Args),
			IconFrom:   cleanPath(e.IconFrom),
			Cover:      cleanPath(e.Cover),
		}
		if g.Name == "" {
			return nil, fmt.Errorf("Every game needs a name.")
		}
		if len(g.Name) > 40 {
			return nil, fmt.Errorf("%q: names can be at most 40 characters.", g.Name)
		}
		if g.SteamAppID > 0 {
			g.Target, g.Args = "", ""
		} else if g.Target == "" {
			return nil, fmt.Errorf("%q: choose a program or a Steam game.", g.Name)
		}
		for _, p := range []string{g.Target, g.IconFrom, g.Cover} {
			if err := checkFile(p); err != nil {
				return nil, fmt.Errorf("%q: %v", g.Name, err)
			}
		}

		id := strings.TrimSpace(e.ID)
		if id == "" || used[id] || !validID(id) {
			id = slug(g.Name, used)
		}
		used[id] = true
		g.ID = id
		out = append(out, g)
	}
	return out, nil
}

func cleanPath(p string) string {
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(p), `"'`))
}

// checkFile reports a missing file for absolute paths. URIs (spotify:, steam://)
// and paths with %VARIABLES% are left to the launch itself.
func checkFile(p string) error {
	if p == "" || strings.Contains(p, "%") || !(filepath.IsAbs(p) || strings.HasPrefix(p, `\\`)) {
		return nil
	}
	if _, err := os.Stat(p); err != nil {
		return fmt.Errorf("file not found: %s", p)
	}
	return nil
}

var idPattern = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

func validID(id string) bool { return idPattern.MatchString(id) }

func slug(name string, used map[string]bool) string {
	base := strings.Trim(notSlug.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if len(base) > 24 {
		base = strings.Trim(base[:24], "-")
	}
	if base == "" {
		base = "game"
	}
	id := base
	for n := 2; used[id]; n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	return id
}
