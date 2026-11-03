//go:build !windows

package art

import (
	"errors"
	"image"
)

func exeIcon(string) (image.Image, error) {
	return nil, errors.New("icons are only extracted on Windows")
}

func steamPath() string { return "" }
