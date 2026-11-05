package config

import "testing"

func TestBrandFor(t *testing.T) {
	for target, want := range map[string]string{
		"https://www.netflix.com":           "netflix",
		"https://netflix.com/browse":        "netflix",
		"http://NETFLIX.COM":                "netflix",
		"https://www.youtube.com/watch?v=1": "youtube",
		"https://youtu.be/abc":              "youtube",
		"https://m.twitch.tv/somebody":      "twitch",
		"https://open.spotify.com":          "spotify",
		"spotify:":                          "spotify",
		"https://www.primevideo.com":        "prime",
		"https://www.disneyplus.com/home":   "disney",
		// Look-alikes and unrelated targets get no icon.
		"https://notnetflix.com":            "",
		"https://netflix.com.evil.example/": "",
		"https://example.com/netflix.com":   "",
		`C:\Program Files\App\app.exe`:      "",
		"":                                  "",
	} {
		b, ok := BrandFor(target)
		if (want != "") != ok || b.Name != want {
			t.Errorf("BrandFor(%q) = %q, %v; want %q", target, b.Name, ok, want)
		}
	}
	if b, _ := BrandFor("https://www.netflix.com"); b.Tint == "" {
		t.Error("a brand carries a default tint")
	}
}
