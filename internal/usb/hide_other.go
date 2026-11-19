//go:build !windows

package usb

import "os/exec"

func hideWindow(*exec.Cmd) {}
