//go:build !windows

package actions

import "fmt"

// Run is only implemented on Windows for now. This stub lets the rest of the
// project build and test on other platforms (e.g. Linux CI).
func Run(a Action) error {
	return fmt.Errorf("action %q is not supported on this platform yet", a.Type)
}
