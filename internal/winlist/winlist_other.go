//go:build !windows

package winlist

// List returns nothing: there are no windows to list on this system.
func List() []Window { return nil }

// Focus always fails here.
func Focus(id uint64) error { return ErrNotFocused }

// Foreground returns the window in front, or 0 if there is none.
func Foreground() uint64 { return 0 }
