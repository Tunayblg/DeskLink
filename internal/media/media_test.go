package media

import "testing"

// TestSample only checks that reading does not crash; what it returns depends
// on what is playing on the machine running the test.
func TestSample(t *testing.T) {
	var w Watcher
	info := w.Sample()
	if info.ArtID != "" && w.Art(info.ArtID) == nil {
		t.Fatalf("art %q missing", info.ArtID)
	}
	t.Logf("%+v art=%d bytes", info, len(w.Art(info.ArtID)))
}

func TestAppName(t *testing.T) {
	for in, want := range map[string]string{
		"Spotify.exe": "Spotify",
		"SpotifyAB.SpotifyMusic_zpdnekdrzrea0!Spotify": "Spotify",
		"chrome": "Chrome",
		"Microsoft.ZuneMusic_8wekyb3d8bbwe!Microsoft.ZuneMusic": "Media Player",
		"Opera.exe": "Opera",
	} {
		if got := appName(in); got != want {
			t.Errorf("appName(%q) = %q, want %q", in, got, want)
		}
	}
}
