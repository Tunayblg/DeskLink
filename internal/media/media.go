// Package media reports what is playing on the PC, per app (title, artist,
// cover art, progress, the app's own volume), plus the system volume, for the
// now-playing card on the phone. It can also control one app directly.
//
// On Windows it reads the same System Media Transport Controls that power the
// volume flyout (so it works with Spotify, browsers and most players), and
// the per-app volumes from the Windows volume mixer.
package media

import (
	"hash/fnv"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Source is one app that is playing (or paused).
type Source struct {
	ID         string `json:"id"` // the app's media identity, e.g. "Spotify.exe"
	App        string `json:"app"`
	Title      string `json:"title,omitempty"`
	Artist     string `json:"artist,omitempty"`
	Album      string `json:"album,omitempty"`
	Playing    bool   `json:"playing"`
	PositionMs int64  `json:"position_ms"`
	DurationMs int64  `json:"duration_ms"`
	ArtID      string `json:"art_id,omitempty"`
	Volume     int    `json:"volume"` // the app's volume in the mixer, 0-100; -1 if unknown
	Muted      bool   `json:"muted"`
}

// State is one reading.
type State struct {
	Sources []Source `json:"sources"`
	Current string   `json:"current,omitempty"` // the source Windows itself shows first
	Volume  int      `json:"volume"`            // system volume 0-100, -1 if unknown
	Muted   bool     `json:"muted"`
}

// Same reports whether two readings differ only in playback positions.
func (s State) Same(o State) bool {
	if s.Current != o.Current || s.Volume != o.Volume || s.Muted != o.Muted || len(s.Sources) != len(o.Sources) {
		return false
	}
	for i := range s.Sources {
		a, b := s.Sources[i], o.Sources[i]
		a.PositionMs, b.PositionMs = 0, 0
		if a != b {
			return false
		}
	}
	return true
}

// Exes are the mixer IDs of every app with a media source.
func (s State) Exes() map[string]bool {
	out := map[string]bool{}
	for _, src := range s.Sources {
		for _, e := range sourceExes(src.ID) {
			out[e] = true
		}
	}
	return out
}

// Find returns the source with the given ID.
func (s State) Find(id string) (Source, bool) {
	for _, src := range s.Sources {
		if src.ID == id {
			return src, true
		}
	}
	return Source{}, false
}

// Actions a phone may ask for on one source.
var actions = map[string]bool{"toggle": true, "next": true, "prev": true, "mute": true}

// ValidAction reports whether a is a supported control action.
func ValidAction(a string) bool { return actions[a] }

// Watcher takes readings. The zero value is ready to use; nothing is loaded
// until the first call.
type Watcher struct {
	mu    sync.Mutex
	at    time.Time
	last  State
	art   map[string][]byte // art ID -> image bytes, for the current tracks only
	src   source
	ready bool

	mixerAt time.Time
	mixer   Mixer
}

const cacheFor = 700 * time.Millisecond

func (w *Watcher) init() {
	if !w.ready {
		w.src = newSource()
		w.ready = true
	}
}

// Sample returns the current reading, cached briefly so several phones share one.
func (w *Watcher) Sample() State {
	w.mu.Lock()
	defer w.mu.Unlock()
	if time.Since(w.at) < cacheFor {
		return w.last
	}
	w.init()
	prev := map[string]string{}
	for _, s := range w.last.Sources {
		prev[s.ID] = s.ArtID
	}
	st, newArt := w.src.read(prev)
	keep := map[string][]byte{}
	for i, s := range st.Sources {
		if b, ok := newArt[s.ID]; ok {
			st.Sources[i].ArtID = artID(b)
			keep[st.Sources[i].ArtID] = b
		} else if b, ok := w.art[s.ArtID]; ok {
			keep[s.ArtID] = b
		}
	}
	w.art = keep
	w.last, w.at = st, time.Now()
	return st
}

// SetVolume sets the system volume (0-100) of the default playback device.
func (w *Watcher) SetVolume(level int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.init()
	w.src.do(command{kind: "volume", level: min(max(level, 0), 100)})
	w.at, w.mixerAt = time.Time{}, time.Time{} // the next reading has the new level
}

// SetAppVolume sets one source's volume in the Windows mixer (0-100).
func (w *Watcher) SetAppVolume(id string, level int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.init()
	w.src.do(command{kind: "appvolume", exes: sourceExes(id), level: min(max(level, 0), 100)})
	w.at, w.mixerAt = time.Time{}, time.Time{}
}

// Control sends a playback action ("toggle", "next", "prev") to one source,
// or toggles its mute in the mixer ("mute").
func (w *Watcher) Control(id, action string) {
	if !ValidAction(action) {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.init()
	w.src.do(command{kind: action, source: id, exes: sourceExes(id)})
	w.at, w.mixerAt = time.Time{}, time.Time{}
}

// MixerApp is one program in the Windows volume mixer.
type MixerApp struct {
	ID     string `json:"id"` // executable name, lowercase, without ".exe"
	Name   string `json:"name"`
	Path   string `json:"-"`
	Volume int    `json:"volume"` // 0-100
	Muted  bool   `json:"muted"`
	Active bool   `json:"active"` // playing sound right now
}

// Mixer is the system volume and every program that has opened audio.
type Mixer struct {
	Volume int        `json:"volume"` // -1 if unknown
	Muted  bool       `json:"muted"`
	Apps   []MixerApp `json:"apps"`
	Mic    *Mic       `json:"mic,omitempty"` // nil without a microphone
}

// Mic is the default microphone.
type Mic struct {
	Muted bool `json:"muted"`
}

// Find returns the app with the given ID.
func (m Mixer) Find(id string) (MixerApp, bool) {
	for _, a := range m.Apps {
		if a.ID == id {
			return a, true
		}
	}
	return MixerApp{}, false
}

// Mixer reads the volume mixer, cached briefly so several phones share one reading.
func (w *Watcher) Mixer() Mixer {
	w.mu.Lock()
	defer w.mu.Unlock()
	if time.Since(w.mixerAt) < cacheFor {
		return w.mixer
	}
	w.init()
	w.mixer, w.mixerAt = w.src.readMixer(), time.Now()
	return w.mixer
}

// SetMixerVolume sets one program's volume (0-100) by its mixer ID.
func (w *Watcher) SetMixerVolume(app string, level int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.init()
	w.src.do(command{kind: "appvolume", exes: []string{app}, level: min(max(level, 0), 100)})
	w.at, w.mixerAt = time.Time{}, time.Time{}
}

// SetMic mutes or unmutes the microphone. It sets a state rather than
// flipping the current one, so a repeated or delayed request cannot undo itself.
func (w *Watcher) SetMic(muted bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.init()
	level := 0
	if muted {
		level = 1
	}
	w.src.do(command{kind: "micset", level: level})
	w.mixerAt = time.Time{}
}

// ToggleMute mutes or unmutes one program by its mixer ID, or the whole PC for "".
func (w *Watcher) ToggleMute(app string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.init()
	if app == "" {
		w.src.do(command{kind: "mastermute"})
	} else {
		w.src.do(command{kind: "mute", exes: []string{app}})
	}
	w.at, w.mixerAt = time.Time{}, time.Time{}
}

// Art returns the image for an art ID from the latest reading.
func (w *Watcher) Art(id string) []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.art[id]
}

type command struct {
	kind   string   // volume, appvolume, toggle, next, prev, mute, mastermute, micset
	source string   // media source, for toggle/next/prev
	exes   []string // mixer IDs, for appvolume/mute
	level  int
}

// sourceExes are the mixer IDs of a media source's app.
func sourceExes(id string) []string {
	return processNames(strings.SplitN(id, "#", 2)[0])
}

// appName turns an app user model ID ("Spotify.exe",
// "Microsoft.ZuneMusic_8wekyb…!Microsoft.ZuneMusic") into a label.
func appName(id string) string {
	token := appToken(id)
	known := map[string]string{"spotify": "Spotify", "spotifymusic": "Spotify", "chrome": "Chrome", "msedge": "Edge", "opera": "Opera",
		"firefox": "Firefox", "zunemusic": "Media Player", "vlc": "VLC", "brave": "Brave",
		"operagxwebbrowser": "Opera GX", "operawebbrowser": "Opera"}
	if n, ok := known[strings.ToLower(token)]; ok {
		return n
	}
	return token
}

// appToken is the short app name inside an app user model ID.
func appToken(id string) string {
	id = strings.TrimSuffix(id, ".exe")
	if i := strings.IndexAny(id, "_!"); i > 0 {
		id = id[:i]
	}
	// The last dotted part, skipping numbers ("OperaSoftware.OperaGXWebBrowser.1783246124").
	parts := strings.Split(id, ".")
	for i := len(parts) - 1; i >= 0; i-- {
		if p := parts[i]; p != "" && strings.Trim(p, "0123456789") != "" {
			return p
		}
	}
	return id
}

// processNames are the executables (lowercase, without .exe) whose audio
// belongs to a media source, for matching it to the volume mixer.
func processNames(sourceID string) []string {
	t := strings.ToLower(appToken(sourceID))
	switch t {
	case "spotifymusic":
		return []string{"spotify"}
	case "operagxwebbrowser", "operawebbrowser":
		return []string{"opera"}
	case "zunemusic":
		return []string{"microsoft.media.player", "music.ui"}
	}
	return []string{t}
}

func artID(b []byte) string {
	h := fnv.New64a()
	h.Write(b)
	return strconv.FormatUint(h.Sum64(), 36)
}
