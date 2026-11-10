package keyboard

import (
	"strings"
	"testing"
)

func TestClean(t *testing.T) {
	cases := []struct{ in, want string }{
		{"  merhaba dünya  ", "merhaba dünya"},
		{"a\r\nb\rc", "a\nb\nc"},
		{"tab\there", "tab here"},
		{"bell\x07 and \x1b[31m", "bell and [31m"},
		{"çğıöşü İĞÜ 😀", "çğıöşü İĞÜ 😀"},
		{"\n\n", ""},
	}
	for _, c := range cases {
		if got := Clean(c.in); got != c.want {
			t.Errorf("Clean(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCleanCapsLength(t *testing.T) {
	got := Clean(strings.Repeat("ü", MaxRunes*2))
	if n := len([]rune(got)); n != MaxRunes {
		t.Fatalf("kept %d characters, want %d", n, MaxRunes)
	}
}

func TestKeysAreKnownToTheKeyPresser(t *testing.T) {
	if len(Keys["shifttab"]) != 2 || Keys["enter"][0] != "enter" {
		t.Fatal("unexpected key table")
	}
}
