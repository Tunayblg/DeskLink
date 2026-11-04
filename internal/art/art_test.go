package art

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNotTheGame(t *testing.T) {
	for name, skip := range map[string]bool{
		"TslGame.exe":             false,
		"Game.exe":                false,
		"UnityCrashHandler64.exe": true,
		"unins000.exe":            true,
		"BEService_x64.exe":       true,
		"TslGame_BE.exe":          true,
		"UnrealCEFSubProcess.exe": true,
		"EasyAntiCheat_Setup.exe": true,
	} {
		if got := notTheGame.MatchString(name); got != skip {
			t.Errorf("%s: skip=%v, want %v", name, got, skip)
		}
	}
}

func TestCoverIn(t *testing.T) {
	write := func(dir string, parts ...string) {
		p := filepath.Join(append([]string{dir}, parts...)...)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cache := t.TempDir()
	write(cache, "10", "library_600x900.jpg")                   // older layout
	write(cache, "20", "abc123", "library_600x900.jpg")         // newer layout, in a hash folder
	write(cache, "30", "abc123", "library_600x900_turkish.jpg") // only a localized copy
	write(cache, "40", "header.jpg")                            // neither is a poster: the exe icon is used instead
	write(cache, "40", "def456", "library_capsule.jpg")         // (a capsule or a header is not a poster)
	write(cache, "50_library_600x900.jpg")                      // oldest layout
	write(cache, "60", "library_600x900.jpg")                   // plain beats localized
	write(cache, "60", "x", "library_600x900_german.jpg")

	for _, tc := range []struct {
		appID int
		want  string // path below the cache, "" for none
	}{
		{10, "10/library_600x900.jpg"},
		{20, "20/abc123/library_600x900.jpg"},
		{30, "30/abc123/library_600x900_turkish.jpg"},
		{40, ""},
		{50, "50_library_600x900.jpg"},
		{60, "60/library_600x900.jpg"},
		{70, ""},
	} {
		got := coverIn(cache, tc.appID)
		if tc.want == "" {
			if got != "" {
				t.Errorf("app %d: found %q, want none", tc.appID, got)
			}
			continue
		}
		if want := filepath.Join(cache, filepath.FromSlash(tc.want)); got != want {
			t.Errorf("app %d: got %q, want %q", tc.appID, got, want)
		}
	}
}
