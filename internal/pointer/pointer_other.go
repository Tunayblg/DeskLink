//go:build !windows

package pointer

func move(dx, dy int)                  {}
func button(name string, down bool)    {}
func wheel(horizontal bool, delta int) {}
