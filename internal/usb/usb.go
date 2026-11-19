// Package usb connects an Android phone over its charging cable.
//
// When a phone with USB debugging on is plugged in, DeskLink asks Android's
// debug bridge (adb) to forward the phone's own port to a small listener on this
// PC. The phone's DeskLink app then reaches the PC at "localhost" with no Wi-Fi.
// Without adb installed nothing happens and the Wi-Fi way works as before.
package usb

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// poll is how often the cable is looked at: a phone plugged in is picked up
// within a few seconds, and adb is asked nothing while no phone is there.
const poll = 4 * time.Second

// Run watches for phones until ctx ends. devicePort is the port the phone
// connects to on itself, pcPort the listener on this PC that it is forwarded to.
func Run(ctx context.Context, devicePort, pcPort int) {
	known := map[string]bool{} // serial -> forwarded
	warned := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(poll):
		}
		adb := findADB()
		if adb == "" {
			if !warned {
				warned = true
				log.Printf("usb: adb was not found, so cable connection is off (install Android platform-tools to turn it on)")
			}
			continue
		}
		out, err := run(ctx, adb, "devices")
		if err != nil {
			continue
		}
		now := map[string]bool{}
		for _, serial := range parseDevices(out) {
			now[serial] = true
			if known[serial] {
				continue
			}
			_, err := run(ctx, adb, "-s", serial, "reverse", fmt.Sprintf("tcp:%d", devicePort), fmt.Sprintf("tcp:%d", pcPort))
			if err != nil {
				log.Printf("usb: could not forward the port for %s: %v", serial, err)
				continue
			}
			known[serial] = true
			log.Printf("usb: phone %s is connected by cable (open DeskLink on it)", serial)
		}
		for serial := range known {
			if !now[serial] {
				delete(known, serial)
				log.Printf("usb: phone %s was unplugged", serial)
			}
		}
	}
}

// parseDevices returns the serials adb lists as ready ("device", not
// "unauthorized" or "offline"). adb also prints a header and, the first time, a
// line about starting its background server; both are skipped.
func parseDevices(out string) []string {
	var serials []string
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && f[1] == "device" {
			serials = append(serials, f[0])
		}
	}
	return serials
}

// findADB looks for adb next to DeskLink, on the PATH and in the usual place
// the Android tools are installed. It returns "" when there is none.
func findADB() string {
	name := "adb"
	if runtime.GOOS == "windows" {
		name = "adb.exe"
	}
	var candidates []string
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), name))
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	for _, env := range []string{"ANDROID_HOME", "ANDROID_SDK_ROOT"} {
		if dir := os.Getenv(env); dir != "" {
			candidates = append(candidates, filepath.Join(dir, "platform-tools", name))
		}
	}
	if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
		candidates = append(candidates, filepath.Join(dir, "Android", "Sdk", "platform-tools", name))
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

func run(ctx context.Context, adb string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, adb, args...)
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
