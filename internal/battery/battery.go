// Package battery reads the charge of Bluetooth devices that are connected to
// the PC (keyboards, headsets, controllers).
//
// Windows keeps the level a device reports as a property of its device node,
// so nothing is installed and nothing talks to the device. Devices that use
// their own wireless dongle are not Bluetooth and do not show up here.
package battery

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// Device is one connected device and its charge in percent. Kind says what it
// is ("mouse", "keyboard", "headset", "gamepad" or "device") so the phone can
// draw the right icon.
type Device struct {
	Name    string `json:"name"`
	Percent int    `json:"percent"`
	Kind    string `json:"kind"`
}

// kindOf guesses what a device is from its name.
func kindOf(name string) string {
	n := strings.ToLower(name)
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(n, w) {
				return true
			}
		}
		return false
	}
	switch {
	case has("xbox", "controller", "gamepad", "dualsense", "dualshock", "kingo", "joy-con"):
		return "gamepad"
	case has("buds", "headset", "headphone", "airpods", "blackshark", "kraken", "arctis", "cloud", "wh-1000", "kulaklık"):
		return "headset"
	case has("mouse", "cobra", "viper", "deathadder", "basilisk", "naga", "mx master", "g502", "fare"):
		return "mouse"
	case has("keyboard", "klavye", " kb", "yunzii", "pusat", "k380", "keychron"):
		return "keyboard"
	}
	return "device"
}

// node is what is read from one Windows device node. Several nodes belong to
// one device (the device itself, its hands-free profile, ...) and are joined
// by the Bluetooth address in their ID.
type node struct {
	id        string
	name      string
	level     int  // percent, or -1 when the node reports no battery
	known     bool // connected is only meaningful when this is set
	connected bool
}

// addrOf pulls the 12 digit Bluetooth address out of a device ID such as
// "BTHLE\DEV_AABBCCDDEE01\..." or "BTHENUM\{...}_VID&..\A&11223344&0&AABBCCDDEE03_C00000000".
func addrOf(id string) string {
	id = strings.ToUpper(id)
	if i := strings.Index(id, `\DEV_`); i >= 0 && len(id) >= i+5+12 {
		return id[i+5 : i+5+12]
	}
	if i := strings.LastIndex(id, "&0&"); i >= 0 && len(id) >= i+3+12 {
		return id[i+3 : i+3+12]
	}
	return ""
}

// cleanName drops the profile that Windows appends to the name of a node.
func cleanName(n string) string {
	for _, suffix := range []string{" Hands-Free AG", " Hands-Free", " Avrcp Transport"} {
		n = strings.TrimSuffix(n, suffix)
	}
	return strings.TrimSpace(n)
}

// collect joins the nodes into one entry per connected device that reports a
// charge. A device that is paired but switched off keeps its last reading in
// Windows, so it is left out.
func collect(nodes []node) []Device {
	type dev struct {
		name      string
		level     int
		known     bool
		connected bool
	}
	byAddr := map[string]*dev{}
	for _, n := range nodes {
		a := addrOf(n.id)
		if a == "" {
			continue
		}
		d := byAddr[a]
		if d == nil {
			d = &dev{level: -1}
			byAddr[a] = d
		}
		// Connected if any of its nodes says so. A profile node can say "not
		// connected" for its own profile (earbuds playing music have their
		// hands-free profile closed) while the device itself is connected, and
		// the nodes come in no fixed order.
		if n.known {
			d.known = true
			d.connected = d.connected || n.connected
		}
		if n.level >= 0 && n.level <= 100 {
			d.level = n.level
		}
		// Prefer the name of the device's own node over a profile node.
		if n.name != "" && (d.name == "" || n.known) {
			d.name = cleanName(n.name)
		}
	}
	out := []Device{}
	for _, d := range byAddr {
		if d.level < 0 || !d.known || !d.connected || d.name == "" {
			continue
		}
		out = append(out, Device{Name: d.name, Percent: d.level, Kind: kindOf(d.name)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// cacheFor is short, so a device switched on or off shows within seconds.
const cacheFor = 5 * time.Second

// Reader reads the batteries, remembering the answer briefly so several
// phones share one reading. The zero value is ready to use.
type Reader struct {
	mu   sync.Mutex
	at   time.Time
	last []Device
}

// Read returns the connected devices that report a charge.
func (r *Reader) Read() []Device {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.at.IsZero() && time.Since(r.at) < cacheFor {
		return r.last
	}
	r.last, r.at = list(), time.Now()
	return r.last
}
