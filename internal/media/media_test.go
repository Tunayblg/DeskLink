package media

import "testing"

// TestSample only checks that reading does not crash; what it returns depends
// on what is playing on the machine running the test.
func TestSample(t *testing.T) {
	var w Watcher
	st := w.Sample()
	for _, s := range st.Sources {
		if s.ArtID != "" && w.Art(s.ArtID) == nil {
			t.Fatalf("art %q missing", s.ArtID)
		}
		t.Logf("%+v art=%d bytes", s, len(w.Art(s.ArtID)))
	}
	t.Logf("current=%q volume=%d muted=%v", st.Current, st.Volume, st.Muted)
}

func TestAppName(t *testing.T) {
	for in, want := range map[string]string{
		"Spotify.exe": "Spotify",
		"SpotifyAB.SpotifyMusic_zpdnekdrzrea0!Spotify": "Spotify",
		"chrome": "Chrome",
		"Microsoft.ZuneMusic_8wekyb3d8bbwe!Microsoft.ZuneMusic": "Media Player",
		"Opera.exe": "Opera",
		"OperaSoftware.OperaGXWebBrowser.1783246124": "Opera GX",
	} {
		if got := appName(in); got != want {
			t.Errorf("appName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestProcessNames(t *testing.T) {
	for in, want := range map[string]string{
		"Spotify.exe": "spotify",
		"SpotifyAB.SpotifyMusic_zpdnekdrzrea0!Spotify": "spotify",
		"MSEdge":    "msedge",
		"Opera.exe": "opera",
		"OperaSoftware.OperaGXWebBrowser.1783246124": "opera",
	} {
		if got := processNames(in); got[0] != want {
			t.Errorf("processNames(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSame(t *testing.T) {
	a := State{Sources: []Source{{ID: "x", Title: "A", PositionMs: 1}}}
	b := State{Sources: []Source{{ID: "x", Title: "A", PositionMs: 9}}}
	if !a.Same(b) {
		t.Error("position change should not count")
	}
	b.Sources[0].Volume = 3
	if a.Same(b) {
		t.Error("volume change should count")
	}
}
