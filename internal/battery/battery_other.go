//go:build !windows

package battery

func list() []Device { return []Device{} }
