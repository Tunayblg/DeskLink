//go:build !windows

package keyboard

func typeText(text string) bool { return false }

func backspace(n int) bool { return false }
