package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tunayblg/DeskLink/internal/actions"
)

func TestDefaultIsValid(t *testing.T) {
	c, err := Parse([]byte(Default))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Groups) != 2 || len(c.Groups[0].Buttons) != 4 {
		t.Fatalf("unexpected default layout: %+v", c.Groups)
	}
	if _, ok := c.Find("mute"); !ok {
		t.Fatal("mute not found")
	}
}

func TestGameButton(t *testing.T) {
	c, err := Parse([]byte(`{"games":[{"id":"cs2","name":"CS2","steam_appid":730}],
		"groups":[{"title":"Q","style":"list","buttons":[{"id":"q_cs2","label":"CS2","icon":"gamepad","action":{"type":"game","target":"cs2"}}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := c.Find("q_cs2"); a.Target != "steam://rungameid/730" {
		t.Fatalf("game button resolves to %+v", a)
	}
	if g := c.Public()[0].Buttons[0].Game; g != "cs2" {
		t.Fatalf("public game = %q", g)
	}
	if _, err := Parse([]byte(`{"groups":[{"title":"Q","buttons":[{"id":"x","label":"X","action":{"type":"game","target":"nope"}}]}]}`)); err == nil {
		t.Fatal("button pointing at a missing game was accepted")
	}
}

func TestGames(t *testing.T) {
	c, err := Parse([]byte(`{"groups":[],"games":[
		{"id":"steamgame","name":"Steam Game","steam_appid":730},
		{"id":"exegame","name":"Exe Game","target":"C:\\Games\\game.exe","args":"-windowed"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := c.Find("steamgame"); a.Target != "steam://rungameid/730" {
		t.Errorf("steam launch target = %q", a.Target)
	}
	if a, _ := c.Find("exegame"); a.Target != `C:\Games\game.exe` || a.Args != "-windowed" {
		t.Errorf("exe launch = %+v", a)
	}
}

func TestSetGamesAndSave(t *testing.T) {
	c, err := Parse([]byte(Default))
	if err != nil {
		t.Fatal(err)
	}
	edit := c.Clone()
	edit.SetGames([]Game{
		{ID: "cs2", Name: "CS2", SteamAppID: 730},
		{ID: "mine", Name: "Mine", Target: `C:\Games\mine.exe`},
	})
	edit.Shortcuts = []Shortcut{{Game: "mine"}}
	edit.Groups[1].Buttons = append(edit.Groups[1].Buttons,
		Button{ID: "mine_btn", Label: "Mine", Icon: "gamepad", Action: actions.Action{Type: "game", Target: "mine"}})

	path := filepath.Join(t.TempDir(), "config.json")
	saved, err := edit.Save(path)
	if err != nil {
		t.Fatal(err)
	}
	if a, ok := saved.Find("shortcut-1"); !ok || a.Target != `C:\Games\mine.exe` {
		t.Fatalf("tile for the game: %+v %v", a, ok)
	}
	if len(c.Games) != 0 {
		t.Fatal("Clone shared state with the original")
	}

	// Removing a game empties its tile and drops its button; the old file is kept as .bak.
	edit2 := saved.Clone()
	edit2.SetGames([]Game{{ID: "cs2", Name: "CS2", SteamAppID: 730}})
	saved2, err := edit2.Save(path)
	if err != nil {
		t.Fatal(err)
	}
	if saved2.Slots()[0].Filled() {
		t.Fatalf("tile for a removed game survived: %+v", saved2.Slots()[0])
	}
	if _, ok := saved2.Find("mine_btn"); ok {
		t.Fatal("button for a removed game survived")
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatal("no backup written")
	}

	// Invalid configs are refused and the file is left alone.
	bad := saved2.Clone()
	bad.SetGames([]Game{{ID: "x", Name: "X"}}) // neither target nor steam_appid
	if _, err := bad.Save(path); err == nil {
		t.Fatal("invalid config saved")
	}
	if again, err := Load(path); err != nil || len(again.Games) != 1 {
		t.Fatalf("file changed after a refused save: %v %+v", err, again)
	}
}

func TestPageButton(t *testing.T) {
	c, err := Parse([]byte(`{"groups":[{"title":"X","buttons":[{"id":"s","label":"Stats","icon":"gamepad","action":{"type":"page","target":"stats"}}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if p := c.Public()[0].Buttons[0].Page; p != "stats" {
		t.Fatalf("page = %q", p)
	}
	if c.Public()[0].Style != "grid" {
		t.Fatalf("default style = %q", c.Public()[0].Style)
	}
}

func TestRejectsBadConfigs(t *testing.T) {
	btn := func(action string) string {
		return `{"groups":[{"title":"G","buttons":[{"id":"a","label":"A","action":` + action + `}]}]}`
	}
	cases := map[string]string{
		"unknown key":    btn(`{"type":"keys","keys":["hyper"]}`),
		"unknown type":   btn(`{"type":"shell","target":"rm -rf"}`),
		"empty target":   btn(`{"type":"launch","target":" "}`),
		"unknown page":   btn(`{"type":"page","target":"settings"}`),
		"unknown system": btn(`{"type":"system","command":"reboot"}`),
		"bad id":         `{"groups":[{"title":"G","buttons":[{"id":"A B","label":"A","action":{"type":"keys","keys":["a"]}}]}]}`,
		"bad style":      `{"groups":[{"title":"G","style":"fancy","buttons":[{"id":"a","label":"A","action":{"type":"keys","keys":["a"]}}]}]}`,
		"empty group":    `{"groups":[{"title":"G","buttons":[]}]}`,
		"low port":       `{"port":80}`,
		"dup across":     `{"groups":[{"title":"G","buttons":[{"id":"a","label":"A","action":{"type":"keys","keys":["a"]}}]}],"games":[{"id":"a","name":"A","steam_appid":1}]}`,
		"game no target": `{"games":[{"id":"g","name":"G"}]}`,
		"game both":      `{"games":[{"id":"g","name":"G","steam_appid":1,"target":"x.exe"}]}`,
	}
	for name, cfg := range cases {
		if _, err := Parse([]byte(cfg)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestDefaultHasShutdown(t *testing.T) {
	c, err := Parse([]byte(Default))
	if err != nil {
		t.Fatal(err)
	}
	a, ok := c.Find("shutdown")
	if !ok || a.Type != "system" || a.Command != "shutdown" {
		t.Fatalf("default config has no shutdown button: %+v", a)
	}
	if c.Version != configVersion {
		t.Errorf("default version %d, want %d", c.Version, configVersion)
	}
}

// An older config.json gets the shut down button once, after the lock button;
// removing it later sticks.
func TestShutdownMigration(t *testing.T) {
	old := strings.Replace(Default, `"version": 2,`, "", 1)
	i := strings.Index(old, `,
        { "id": "shutdown"`)
	j := strings.Index(old[i:], "}\n") // end of the shutdown line's object
	if i < 0 || j < 0 {
		t.Fatal("test setup: cannot find the shutdown button in Default")
	}
	old = old[:i] + old[i+j+1:]
	if strings.Contains(old, "shutdown") {
		t.Fatalf("test setup: still has a shutdown button:\n%s", old)
	}
	old = strings.Replace(old, `"version": 3`, `"version": 1`, 1) // a config from before the shut down button
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Find("shutdown"); !ok || c.Version != configVersion {
		t.Fatalf("not migrated: version %d", c.Version)
	}
	quick := c.Groups[1].Buttons
	if quick[len(quick)-2].ID != "lock" || quick[len(quick)-1].ID != "shutdown" {
		t.Errorf("shutdown should follow lock: %v", quick)
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Errorf("the old config should be kept as .bak: %v", err)
	}

	// The user removes it; the next start must not bring it back.
	c = c.Clone()
	c.Groups[1].Buttons = c.Groups[1].Buttons[:len(c.Groups[1].Buttons)-1]
	if _, err := c.Save(path); err != nil {
		t.Fatal(err)
	}
	again, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := again.Find("shutdown"); ok {
		t.Error("a removed shutdown button came back")
	}

	// A config without the usual lock button is left as it is.
	custom := `{"version": 1, "port": 8765, "groups": [{"title": "Q", "style": "quick", "buttons": [{"id": "calc", "label": "Calc", "icon": "terminal", "action": {"type": "launch", "target": "calc.exe"}}]}], "games": []}`
	p2 := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(p2, []byte(custom), 0o600)
	c2, err := Load(p2)
	if err != nil {
		t.Fatal(err)
	}
	if len(c2.Groups[0].Buttons) != 1 {
		t.Errorf("custom config was changed: %v", c2.Groups[0].Buttons)
	}
}

func TestGameOf(t *testing.T) {
	c, err := Parse([]byte(`{"port":8765,
		"games":[{"id":"a","name":"A","target":"C:/a.exe"},{"id":"b","name":"B","steam_appid":1}],
		"groups":[{"title":"Q","buttons":[
			{"id":"q_b","label":"B","action":{"type":"game","target":"b"}},
			{"id":"lock","label":"Lock","action":{"type":"system","command":"lock"}}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{"a": "a", "b": "b", "q_b": "b"} {
		if g, ok := c.GameOf(id); !ok || g.ID != want {
			t.Errorf("GameOf(%q) = %q, %v; want %q", id, g.ID, ok, want)
		}
	}
	for _, id := range []string{"lock", "nope", ""} {
		if _, ok := c.GameOf(id); ok {
			t.Errorf("GameOf(%q) found a game", id)
		}
	}
}

func TestParseAcceptsByteOrderMark(t *testing.T) {
	if _, err := Parse(append([]byte("\xef\xbb\xbf"), Default...)); err != nil {
		t.Fatalf("a config saved by Notepad with a BOM must load: %v", err)
	}
}
