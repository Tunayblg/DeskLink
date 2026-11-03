package config

import (
	"os"
	"path/filepath"
	"testing"
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
	}, map[string]bool{"mine": true})

	path := filepath.Join(t.TempDir(), "config.json")
	saved, err := edit.Save(path)
	if err != nil {
		t.Fatal(err)
	}
	if !saved.Pinned()["mine"] || saved.Pinned()["cs2"] {
		t.Fatalf("pinned = %v", saved.Pinned())
	}
	home := saved.Groups[saved.HomeGroup()]
	if home.Buttons[0].ID != "q_mine" || home.Buttons[0].Action.Target != "mine" {
		t.Fatalf("home buttons = %+v", home.Buttons)
	}
	if len(c.Games) != 0 {
		t.Fatal("Clone shared state with the original")
	}

	// Removing a game also removes its button; the old file is kept as .bak.
	edit2 := saved.Clone()
	edit2.SetGames([]Game{{ID: "cs2", Name: "CS2", SteamAppID: 730}}, map[string]bool{"mine": true})
	saved2, err := edit2.Save(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved2.Pinned()) != 0 {
		t.Fatalf("button for a removed game survived: %v", saved2.Pinned())
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatal("no backup written")
	}

	// Invalid configs are refused and the file is left alone.
	bad := saved2.Clone()
	bad.SetGames([]Game{{ID: "x", Name: "X"}}, nil) // neither target nor steam_appid
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
		"unknown system": btn(`{"type":"system","command":"shutdown"}`),
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
