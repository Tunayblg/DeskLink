// Package running tells which programs are running on this PC, so a game that
// is already open is not started a second time.
package running

import "strings"

// Names returns the names of the running programs, lowercase and without
// ".exe" (so "TestGame.exe" is "testgame"). It returns nil if the list cannot be read,
// which callers treat as "nothing is running": a failed lookup must never keep
// a game from starting.
func Names() map[string]bool { return names() }

// Name is the form of an executable's file name that Names uses.
func Name(file string) string {
	file = strings.ToLower(file)
	return strings.TrimSuffix(file, ".exe")
}
