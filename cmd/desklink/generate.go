package main

// The icon and the file properties Windows shows for desklink.exe (right click
// -> Properties -> Details) are compiled into rsrc_windows_amd64.syso, which the
// Go linker picks up by its name. Regenerate it after changing icon.ico or
// versioninfo.json, and keep the version there in step with `version` in
// main.go (a test checks that they match):
//
//	go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@v1.4.1
//	go generate ./cmd/desklink
//
//go:generate goversioninfo -64 -o rsrc_windows_amd64.syso versioninfo.json
