package server

import (
	"testing"

	"github.com/Tunayblg/DeskLink/internal/winlist"
)

// Only developer windows are offered, named for people, AI first.
func TestDevTargetsKeepsOnlyDeveloperWindows(t *testing.T) {
	all := []winlist.Window{
		{ID: 1, App: "Spotify", Title: "Some song"},
		{ID: 2, App: "Code", Title: "app.js - streamdeck - Visual Studio Code"},
		{ID: 3, App: "chrome", Title: "Inbox - Gmail - Google Chrome"},
		{ID: 4, App: "WindowsTerminal", Title: "claude"},
		{ID: 5, App: "chrome", Title: "ChatGPT - Google Chrome"},
		{ID: 6, App: "WhatsApp", Title: "WhatsApp"},
		{ID: 7, App: "Claude", Title: "Claude"},
		{ID: 8, App: "pycharm64", Title: "project - PyCharm"},
	}
	got := devTargets(all)
	var ids []uint64
	var names []string
	for _, w := range got {
		ids = append(ids, w.ID)
		names = append(names, w.App+"/"+w.Kind)
	}
	// AI (5 ChatGPT in Chrome, 7 Claude) in front-to-back order, then the terminal, then editors.
	wantIDs := []uint64{5, 7, 4, 2, 8}
	if len(ids) != len(wantIDs) {
		t.Fatalf("got %v %v, want ids %v", ids, names, wantIDs)
	}
	for i := range wantIDs {
		if ids[i] != wantIDs[i] {
			t.Fatalf("got %v %v, want ids %v", ids, names, wantIDs)
		}
	}
	if got[0].App != "ChatGPT" || got[1].App != "Claude" || got[2].App != "Terminal" || got[3].App != "VS Code" || got[4].App != "JetBrains" {
		t.Errorf("names %v", names)
	}
}
