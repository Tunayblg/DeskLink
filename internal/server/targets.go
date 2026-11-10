package server

import (
	"sort"
	"strings"

	"github.com/Tunayblg/DeskLink/internal/winlist"
)

// Developer mode only offers the windows a developer sends prompts to: AI
// assistants, terminals and code editors. The list the phone sees is this one,
// and a window outside it is refused even if its number is known.

// Kinds of target, which is also the order they are listed in.
const (
	kindAI     = "ai"
	kindTerm   = "term"
	kindEditor = "editor"
)

type devApp struct {
	name string
	kind string
}

// Programs by the name of their executable (lowercase, without ".exe").
var devApps = map[string]devApp{
	"claude":          {"Claude", kindAI},
	"codex":           {"Codex", kindAI},
	"chatgpt":         {"ChatGPT", kindAI},
	"windowsterminal": {"Terminal", kindTerm},
	"wt":              {"Terminal", kindTerm},
	"powershell":      {"PowerShell", kindTerm},
	"pwsh":            {"PowerShell", kindTerm},
	"cmd":             {"Command Prompt", kindTerm},
	"conhost":         {"Console", kindTerm},
	"alacritty":       {"Alacritty", kindTerm},
	"wezterm-gui":     {"WezTerm", kindTerm},
	"kitty":           {"kitty", kindTerm},
	"mintty":          {"Git Bash", kindTerm},
	"hyper":           {"Hyper", kindTerm},
	"tabby":           {"Tabby", kindTerm},
	"cursor":          {"Cursor", kindEditor},
	"code":            {"VS Code", kindEditor},
	"code - insiders": {"VS Code", kindEditor},
	"windsurf":        {"Windsurf", kindEditor},
	"zed":             {"Zed", kindEditor},
	"devenv":          {"Visual Studio", kindEditor},
	"sublime_text":    {"Sublime Text", kindEditor},
}

// JetBrains editors have a version in the name (idea64, pycharm64...).
var jetBrains = []string{"idea", "pycharm", "webstorm", "goland", "rider", "clion", "phpstorm", "datagrip", "rustrover"}

// Browsers show their AI tabs by the page title.
var browsers = map[string]bool{"chrome": true, "msedge": true, "firefox": true, "brave": true, "opera": true, "vivaldi": true, "arc": true}

// AI names found in a browser tab's title.
var webAI = []struct{ word, name string }{
	{"claude", "Claude"},
	{"chatgpt", "ChatGPT"},
	{"codex", "Codex"},
	{"gemini", "Gemini"},
	{"copilot", "Copilot"},
	{"perplexity", "Perplexity"},
}

func classify(w winlist.Window) (devApp, bool) {
	exe := strings.ToLower(w.App)
	if a, ok := devApps[exe]; ok {
		return a, true
	}
	for _, p := range jetBrains {
		if strings.HasPrefix(exe, p) {
			return devApp{"JetBrains", kindEditor}, true
		}
	}
	if browsers[exe] {
		if name, ok := webAIName(w.Title); ok {
			return devApp{name, kindAI}, true
		}
	}
	return devApp{}, false
}

// webAIName finds an AI site in a browser title. The site's name must be a
// whole part of the title ("Chat - Claude - Google Chrome", "Google Gemini"),
// not just a word in it: "About Claude - Gmail" is mail, not Claude.
func webAIName(title string) (string, bool) {
	t := strings.ToLower(title)
	for _, sep := range []string{" | ", " – ", " — ", ": "} {
		t = strings.ReplaceAll(t, sep, " - ")
	}
	for _, part := range strings.Split(t, " - ") {
		part = strings.TrimSpace(part)
		for _, ai := range webAI {
			if part == ai.word || part == "google "+ai.word || part == "microsoft "+ai.word {
				return ai.name, true
			}
		}
	}
	return "", false
}

// devTargets keeps the windows worth sending a prompt to, named for people and
// ordered AI first, then terminals, then editors (the front-to-back order is
// kept inside each group).
func devTargets(all []winlist.Window) []winlist.Window {
	var out []winlist.Window
	for _, w := range all {
		if a, ok := classify(w); ok {
			out = append(out, winlist.Window{ID: w.ID, App: a.name, Title: w.Title, Kind: a.kind})
		}
	}
	rank := map[string]int{kindAI: 0, kindTerm: 1, kindEditor: 2}
	sort.SliceStable(out, func(i, j int) bool { return rank[out[i].Kind] < rank[out[j].Kind] })
	return out
}
