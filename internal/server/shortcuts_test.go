package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing/fstest"

	"github.com/Tunayblg/DeskLink/internal/auth"
	"strings"
	"testing"

	"github.com/Tunayblg/DeskLink/internal/config"
)

func TestNormalizeShortcuts(t *testing.T) {
	got, err := normalizeShortcuts([]shortcutEntry{
		{Name: " YouTube ", Target: "youtube.com", Tint: "255,0,0"},
		{Name: "ignored", Target: "  "},
		{Name: "Spotify", Target: `"spotify:"`},
		{Game: "hades", Name: "ignored"},
	}, map[string]bool{"hades": true})
	if err != nil {
		t.Fatal(err)
	}
	if got[3].Game != "hades" || got[3].Name != "" {
		t.Errorf("slot 4 = %+v (a game tile carries only the game)", got[3])
	}
	if len(got) != config.ShortcutSlots {
		t.Fatalf("got %d slots", len(got))
	}
	if got[0].Name != "YouTube" || got[0].Target != "https://youtube.com" || got[0].Tint != "255,0,0" {
		t.Errorf("slot 1 = %+v", got[0])
	}
	if got[1] != (config.Shortcut{}) {
		t.Errorf("slot 2 should be empty, got %+v", got[1])
	}
	if got[2].Target != "spotify:" {
		t.Errorf("slot 3 = %+v (a URI must be kept as it is)", got[2])
	}

	for name, in := range map[string][]shortcutEntry{
		"no name":      {{Target: "netflix.com"}},
		"missing file": {{Name: "x", Target: `C:\definitely\not\here.exe`}},
		"too many":     {{}, {}, {}, {}, {}, {}, {}},
		"gone game":    {{Game: "removed"}},
	} {
		if _, err := normalizeShortcuts(in, map[string]bool{"hades": true}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestLooksLikeDomain(t *testing.T) {
	for in, want := range map[string]bool{
		"netflix.com":               true,
		"www.youtube.com/watch?v=1": true,
		"https://netflix.com":       false,
		`C:\Games\game.exe`:         false,
		"spotify:":                  false,
		"notepad":                   false,
		"my file.com":               false,
		".com":                      false,
	} {
		if got := looksLikeDomain(in); got != want {
			t.Errorf("looksLikeDomain(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestBuildShortcutsAlwaysSixTiles(t *testing.T) {
	cfg, err := config.Parse([]byte(config.Default))
	if err != nil {
		t.Fatal(err)
	}
	empty := buildShortcuts(cfg)
	if len(empty) != 6 || empty[0].ID != "shortcut-1" || empty[5].ID != "shortcut-6" || empty[0].Name != "" || empty[0].Kind != "" {
		t.Fatalf("empty slots = %+v", empty)
	}
	cfg.Games = []config.Game{{ID: "hades", Name: "Hades", Target: `C:Gameshades.exe`}}
	cfg.Shortcuts = []config.Shortcut{{Name: "Netflix", Target: "https://netflix.com", Tint: "229,9,20"}, {Game: "hades"}}
	got := buildShortcuts(cfg)
	if got[1].Kind != "game" || got[1].Game != "hades" || got[1].Name != "Hades" {
		t.Errorf("game tile = %+v", got[1])
	}
	if got[0].Kind != "web" || got[0].Name != "Netflix" || got[0].Tint != "229,9,20" || got[2].Name != "" {
		t.Errorf("tiles = %+v", got)
	}
	if strings.Contains(got[0].Image, "data:") {
		t.Error("a website has no program icon")
	}
}

// Saving from the dashboard writes config.json and reaches the phones' layout.
func TestDashboardSavesShortcuts(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(cfgPath, []byte(config.Default), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	store, _ := auth.Open(filepath.Join(dir, "devices.json"))
	s := New(cfg, store, fstest.MapFS{"index.html": {Data: []byte("ok")}}, nil)
	s.ConfigPath = cfgPath
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	post := func(body string) (int, shortcutsResponse) {
		req, _ := http.NewRequest("POST", ts.URL+"/admin/api/shortcuts", strings.NewReader(body))
		req.Header.Set("X-Admin-Token", s.adminToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var out shortcutsResponse
		json.NewDecoder(resp.Body).Decode(&out)
		resp.Body.Close()
		return resp.StatusCode, out
	}

	code, out := post(`{"slots":[{"name":"Netflix","target":"netflix.com","tint":"229,9,20"},{},{}]}`)
	if code != 200 || len(out.Slots) != 6 || out.Slots[0].Target != "https://netflix.com" {
		t.Fatalf("save: %d %+v", code, out)
	}
	if lay := s.layout(); len(lay.Shortcuts) != 6 || lay.Shortcuts[0].Name != "Netflix" || lay.Shortcuts[0].Kind != "web" || lay.Shortcuts[1].Name != "" {
		t.Fatalf("layout shortcuts: %+v", lay.Shortcuts)
	}
	again, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if a, ok := again.Find("shortcut-1"); !ok || a.Target != "https://netflix.com" {
		t.Fatalf("saved config: %+v %v", a, ok)
	}

	if code, _ := post(`{"slots":[{"name":"","target":"netflix.com"}]}`); code != 400 {
		t.Errorf("a slot without a name: status %d, want 400", code)
	}
	// A bad save changes nothing.
	if lay := s.layout(); lay.Shortcuts[0].Name != "Netflix" {
		t.Errorf("a failed save changed the layout: %+v", lay.Shortcuts)
	}

	// Clearing a slot empties it.
	if code, out := post(`{"slots":[{},{},{}]}`); code != 200 || out.Slots[0].Target != "" {
		t.Fatalf("clear: %d %+v", code, out)
	}
	if lay := s.layout(); lay.Shortcuts[0].Name != "" {
		t.Errorf("cleared slot still shows %+v", lay.Shortcuts[0])
	}
}

// A well-known service is recognized from its address, also when the user typed it in.
func TestBuildShortcutsRecognizesServices(t *testing.T) {
	cfg, err := config.Parse([]byte(config.Default))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Shortcuts = []config.Shortcut{
		{Name: "Netflix", Target: "https://www.netflix.com"},                 // no tint chosen
		{Name: "My video", Target: "https://www.youtube.com", Tint: "1,2,3"}, // tint chosen
		{Name: "Other", Target: "https://example.com"},                       // not a known service
		{Name: "Spotify", Target: "spotify:"},                                // the desktop app's link
	}
	got := buildShortcuts(cfg)
	if got[0].Brand != "netflix" || got[0].Tint != "229,9,20" {
		t.Errorf("netflix tile = %+v", got[0])
	}
	if got[1].Brand != "youtube" || got[1].Tint != "1,2,3" {
		t.Errorf("a chosen tint must be kept: %+v", got[1])
	}
	if got[2].Brand != "" || got[2].Tint != "" {
		t.Errorf("an unknown site has no brand: %+v", got[2])
	}
	if got[3].Brand != "spotify" || got[3].Kind != "app" {
		t.Errorf("spotify tile = %+v", got[3])
	}
}
