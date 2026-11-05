//go:build !windows

package brightness

func get() (int, bool) { return 0, false }
func set(level int)    {}
