// Package web embeds the phone panel so DeskLink ships as a single executable.
package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var static embed.FS

// Files is the panel, rooted at the static folder.
var Files, _ = fs.Sub(static, "static")

// AdminPage is the PC dashboard, served only on this PC at /admin.
//
//go:embed admin.html
var AdminPage []byte

// TrayIcon is the 32×32 notification-area icon.
//
//go:embed static/icon-32.png
var TrayIcon []byte
