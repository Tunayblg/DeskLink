//go:build !windows

package desktop

import (
	"errors"
	"fmt"
	"os"
)

func Text() Strings { return english }

func Alert(msg string) { fmt.Fprintln(os.Stderr, msg) }

func OpenApp(url string) { fmt.Println("Dashboard:", url) }

func Autostart() bool { return false }

func SetAutostart(bool, string) error {
	return errors.New("start with Windows is only available on Windows")
}
