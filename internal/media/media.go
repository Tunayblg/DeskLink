// Package media reports what is playing on the PC (title, artist, cover art,
// progress) and the system volume, for the now-playing card on the phone.
//
// On Windows it reads the same System Media Transport Controls that power the
// volume flyout, so it works with Spotify, browsers and most players.
package media

import (
	"hash/fnv"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Info is one reading. Empty Title means nothing is playing.
type Info struct {
	App        string `json:"app,omitempty"`
	Title      string `json:"title,omitempty"`
	Artist     string `json:"artist,omitempty"`
	Album      string `json:"album,omitempty"`
	Playing    bool   `json:"playing"`
	PositionMs int64  `json:"position_ms"`
	DurationMs int64  `json:"duration_ms"`
	ArtID      string `json:"art_id,omitempty"`
	Volume     int    `json:"volume"` // 0-100, -1 if unknown
	Muted      bool   `json:"muted"`
}

// Same reports whether two readings differ only in playback position.
func (i Info) Same(o Info) bool {
	i.PositionMs, o.PositionMs = 0, 0
	return i == o
}

// Watcher takes readings. The zero value is ready to use; nothing is loaded
// until the first call.
type Watcher struct {
	mu    sync.Mutex
	at    time.Time
	last  Info
	art   map[string][]byte // art ID -> image bytes (only the current track is kept)
	src   source
	ready bool
}

const cacheFor = 700 * time.Millisecond

// Sample returns the current reading, cached briefly so several phones share one.
func (w *Watcher) Sample() Info {
	w.mu.Lock()
	defer w.mu.Unlock()
	if time.Since(w.at) < cacheFor {
		return w.last
	}
	if !w.ready {
		w.src = newSource()
		w.ready = true
	}
	info, art := w.src.read(w.last.ArtID)
	if art != nil {
		info.ArtID = artID(art)
		w.art = map[string][]byte{info.ArtID: art}
	}
	w.last, w.at = info, time.Now()
	return info
}

// SetVolume sets the master volume (0-100) of the default playback device.
func (w *Watcher) SetVolume(level int) {
	level = min(max(level, 0), 100)
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.ready {
		w.src = newSource()
		w.ready = true
	}
	w.src.setVolume(level)
	w.at = time.Time{} // next Sample reads the new level
}

// Art returns the image for an art ID from the latest reading.
func (w *Watcher) Art(id string) []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.art[id]
}

// appName turns an app user model ID ("Spotify.exe",
// "Microsoft.ZuneMusic_8wekyb…!Microsoft.ZuneMusic") into a label.
func appName(id string) string {
	id = strings.TrimSuffix(id, ".exe")
	if i := strings.IndexAny(id, "_!"); i > 0 {
		id = id[:i]
	}
	if i := strings.LastIndex(id, "."); i >= 0 && i < len(id)-1 {
		id = id[i+1:]
	}
	known := map[string]string{"spotify": "Spotify", "spotifymusic": "Spotify", "chrome": "Chrome", "msedge": "Edge", "opera": "Opera",
		"firefox": "Firefox", "zunemusic": "Media Player", "vlc": "VLC", "brave": "Brave"}
	if n, ok := known[strings.ToLower(id)]; ok {
		return n
	}
	return id
}

func artID(b []byte) string {
	h := fnv.New64a()
	h.Write(b)
	return strconv.FormatUint(h.Sum64(), 36)
}
