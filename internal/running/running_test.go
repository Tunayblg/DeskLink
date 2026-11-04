package running

import (
	"runtime"
	"testing"
)

func TestName(t *testing.T) {
	for in, want := range map[string]string{"TestGame.exe": "testgame", "Game": "game", "A.EXE": "a"} {
		if got := Name(in); got != want {
			t.Errorf("Name(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNamesSeesThisTest(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("process list is Windows only")
	}
	n := Names()
	if len(n) == 0 {
		t.Fatal("no processes listed")
	}
	if !n["running.test"] {
		t.Errorf("the test binary itself is missing from %d processes", len(n))
	}
}
