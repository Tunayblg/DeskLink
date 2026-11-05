// Package web embeds the phone panel so DeskLink ships as a single executable.
package web

import (
	"bytes"
	"embed"
	"io/fs"
)

//go:embed static
var static embed.FS

// Files is the panel, rooted at the static folder.
var Files, _ = fs.Sub(static, "static")

//go:embed admin/index.html admin/guide.css admin/guide.js
var adminFiles embed.FS

// AdminPage is the PC dashboard, served only on this PC at /admin. The user
// guide's styles and script live in their own files and are slotted in here,
// together with the service icons the phone panel uses too.
var AdminPage = buildAdminPage()

func buildAdminPage() []byte {
	read := func(name string) []byte {
		b, err := adminFiles.ReadFile(name)
		if err != nil {
			panic(err)
		}
		return b
	}
	page := read("admin/index.html")
	page = bytes.Replace(page, []byte("/*{{GUIDE_CSS}}*/"), read("admin/guide.css"), 1)
	page = bytes.Replace(page, []byte("/*{{GUIDE_JS}}*/"), read("admin/guide.js"), 1)
	brands, err := static.ReadFile("static/brands.js")
	if err != nil {
		panic(err)
	}
	page = bytes.Replace(page, []byte("/*{{BRANDS_JS}}*/"), brands, 1)
	return page
}

// TrayIcon is the 32×32 notification-area icon.
//
//go:embed static/icon-32.png
var TrayIcon []byte
