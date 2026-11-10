package media

import (
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// This file talks to WinRT and Core Audio through raw COM vtables, so DeskLink
// needs no cgo and no extra runtime. Vtable indexes were checked against the
// Windows metadata; methods 0-5 of every WinRT interface are IUnknown/IInspectable.

var (
	combase   = syscall.NewLazyDLL("combase.dll")
	ole32     = syscall.NewLazyDLL("ole32.dll")
	shcore    = syscall.NewLazyDLL("shcore.dll")
	kernel32  = syscall.NewLazyDLL("kernel32.dll")
	roInit    = combase.NewProc("RoInitialize")
	roFact    = combase.NewProc("RoGetActivationFactory")
	hsCreate  = combase.NewProc("WindowsCreateString")
	hsDelete  = combase.NewProc("WindowsDeleteString")
	hsBuffer  = combase.NewProc("WindowsGetStringRawBuffer")
	coCreate  = ole32.NewProc("CoCreateInstance")
	coFree    = ole32.NewProc("CoTaskMemFree")
	streamOv  = shcore.NewProc("CreateStreamOverRandomAccessStream")
	imageName = kernel32.NewProc("QueryFullProcessImageNameW")
	version   = syscall.NewLazyDLL("version.dll")
	verSize   = version.NewProc("GetFileVersionInfoSizeW")
	verInfo   = version.NewProc("GetFileVersionInfoW")
	verQuery  = version.NewProc("VerQueryValueW")
)

type guid struct {
	d1     uint32
	d2, d3 uint16
	d4     [8]byte
}

var (
	iidManagerStatics  = guid{0x2050c4ee, 0x11a0, 0x57de, [8]byte{0xae, 0xd7, 0xc9, 0x7c, 0x70, 0x33, 0x82, 0x45}}
	iidAsyncInfo       = guid{0x00000036, 0, 0, [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidStream          = guid{0x0000000c, 0, 0, [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	clsidEnumerator    = guid{0xbcde0395, 0xe52f, 0x467c, [8]byte{0x8e, 0x3d, 0xc4, 0x57, 0x92, 0x91, 0x69, 0x2e}}
	iidEnumerator      = guid{0xa95664d2, 0x9614, 0x4f35, [8]byte{0xa7, 0x46, 0xde, 0x8d, 0xb6, 0x36, 0x17, 0xe6}}
	iidEndpointVolume  = guid{0x5cdf2c82, 0x841e, 0x4546, [8]byte{0x97, 0x22, 0x0c, 0xf7, 0x40, 0x78, 0x22, 0x9a}}
	iidSessionManager2 = guid{0x77aa99a0, 0x1bd6, 0x484f, [8]byte{0x8b, 0xc7, 0x2c, 0x65, 0x4c, 0x9a, 0x9b, 0x6f}}
	iidSessionControl2 = guid{0xbfb7ff88, 0x7239, 0x4fc9, [8]byte{0x8f, 0xa2, 0x07, 0xc9, 0x50, 0xbe, 0x9c, 0x6d}}
	iidSimpleVolume    = guid{0x87ce5498, 0x68d6, 0x44e5, [8]byte{0x92, 0x15, 0x6f, 0x9d, 0x9e, 0xb4, 0xa2, 0xf5}}
	iidChannelVolume   = guid{0x1c158861, 0xb533, 0x4b30, [8]byte{0xb1, 0xcf, 0xe8, 0x53, 0xe5, 0x1c, 0x59, 0xb8}}
)

// com is a COM interface pointer: its first word points to the vtable.
type com struct{ vtbl *[32]uintptr }

func (o *com) call(i int, args ...uintptr) int32 {
	r, _, _ := syscall.SyscallN(o.vtbl[i], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	return int32(r)
}

func (o *com) release() {
	if o != nil {
		o.call(2)
	}
}

func ptr[T any](v *T) uintptr { return uintptr(unsafe.Pointer(v)) }

// out calls a method whose last argument receives an interface pointer.
func (o *com) out(i int, args ...uintptr) *com {
	var r *com
	if o.call(i, append(args, ptr(&r))...) < 0 {
		return nil
	}
	return r
}

func hstring(s string) uintptr {
	u, _ := syscall.UTF16FromString(s)
	var h uintptr
	hsCreate.Call(uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), ptr(&h))
	return h
}

// getString calls a getter that returns an HSTRING and converts it.
func (o *com) getString(i int) string {
	var h uintptr
	if o.call(i, ptr(&h)) < 0 || h == 0 {
		return ""
	}
	defer hsDelete.Call(h)
	var n uint32
	p, _, _ := hsBuffer.Call(h, ptr(&n))
	if p == 0 || n == 0 {
		return ""
	}
	// p points into the HSTRING, which stays alive until the deferred delete.
	return syscall.UTF16ToString(unsafe.Slice(*(**uint16)(unsafe.Pointer(&p)), n))
}

// wait waits for a WinRT async operation to finish and reports whether it
// completed. It does not release op.
func wait(op *com) bool {
	info := op.out(0, ptr(&iidAsyncInfo))
	if info == nil {
		return false
	}
	defer info.release()
	deadline := time.Now().Add(2 * time.Second)
	for {
		var status int32
		info.call(7, ptr(&status)) // get_Status
		if status == 1 {           // Completed
			return true
		}
		if status != 0 || time.Now().After(deadline) { // Canceled, Error, or stuck
			return false
		}
		time.Sleep(3 * time.Millisecond)
	}
}

// await waits for a WinRT IAsyncOperation that returns an object.
func await(op *com) *com {
	if op == nil {
		return nil
	}
	defer op.release()
	if !wait(op) {
		return nil
	}
	return op.out(8) // GetResults
}

// source runs every COM call on one OS thread joined to the multithreaded apartment.
type source struct {
	req chan request
}

type request struct {
	prevArt map[string]string // source ID -> art ID of the last reading
	cmd     *command          // nil for a reading
	mixer   bool              // read the volume mixer instead
	reply   chan reply
}

type reply struct {
	state State
	art   map[string][]byte // source ID -> new cover image
	mixer Mixer
}

func newSource() source {
	s := source{req: make(chan request)}
	go s.loop()
	return s
}

func (s source) read(prevArt map[string]string) (State, map[string][]byte) {
	r := make(chan reply, 1)
	s.req <- request{prevArt: prevArt, reply: r}
	out := <-r
	return out.state, out.art
}

func (s source) readMixer() Mixer {
	r := make(chan reply, 1)
	s.req <- request{mixer: true, reply: r}
	return (<-r).mixer
}

func (s source) do(c command) {
	r := make(chan reply, 1)
	s.req <- request{cmd: &c, reply: r}
	<-r
}

type worker struct {
	manager    *com
	enumerator *com
	trackKeys  map[string]string    // source ID -> the track its cover belongs to
	procs      map[uint32]proc      // process ID -> its executable
	names      map[string]string    // executable path -> display name
	unmuted    map[string]float32   // mixer IDs -> volume before a mute, where mute is volume zero
	lastInfo   map[string]Source    // source ID -> the track text last read, kept if a read comes back empty
	stale      map[string]time.Time // source ID -> since when a new track has shown the old track's cover
}

type proc struct{ id, path string }

func (s source) loop() {
	runtime.LockOSThread()
	roInit.Call(1) // RO_INIT_MULTITHREADED
	w := worker{trackKeys: map[string]string{}, procs: map[uint32]proc{}, names: map[string]string{}, unmuted: map[string]float32{}, lastInfo: map[string]Source{}, stale: map[string]time.Time{}}
	for req := range s.req {
		w.handle(req)
	}
}

// handle answers one request. The Windows audio and media services are COM
// objects that can misbehave (no sound device, a service that is still
// starting, a player that goes away mid-read). That must never take DeskLink
// down with it: a failed request is logged, answered with nothing, and the COM
// objects are looked up afresh next time.
func (w *worker) handle(req request) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("media: a Windows audio or media call failed (%v); carrying on without it", r)
			w.manager, w.enumerator = nil, nil
			req.reply <- reply{}
		}
	}()
	switch {
	case req.mixer:
		req.reply <- reply{mixer: w.readMixer()}
	case req.cmd != nil:
		w.do(*req.cmd)
		req.reply <- reply{}
	default:
		st, art := w.read(req.prevArt)
		req.reply <- reply{state: st, art: art}
	}
}

func (w *worker) do(c command) {
	level := uintptr(math.Float32bits(float32(c.level) / 100))
	switch c.kind {
	case "volume":
		w.endpoint(func(ep *com) {
			ep.call(7, level, 0) // SetMasterVolumeLevelScalar
		})
	case "appvolume":
		w.appSessions(c.exes, func(_ bool, vol appVolume) {
			vol.setLevel(float32(c.level) / 100)
		})
	case "output":
		w.setOutput(c.source)
	case "micset":
		w.mics(func(ep *com) {
			ep.call(14, uintptr(c.level), 0) // SetMute
		})
	case "mastermute":
		w.endpoint(func(ep *com) {
			var m int32
			ep.call(15, ptr(&m)) // GetMute
			set := uintptr(1)
			if m != 0 {
				set = 0
			}
			ep.call(14, set, 0) // SetMute
		})
	case "mute":
		key := strings.Join(c.exes, ",")
		muted, found := false, false
		w.appSessions(c.exes, func(_ bool, vol appVolume) {
			if !found {
				muted, found = vol.muted(), true
				if !muted && vol.simple == nil {
					w.unmuted[key] = vol.level()
				}
			}
		})
		if !found {
			return
		}
		restore, ok := w.unmuted[key]
		if !ok || restore == 0 {
			restore = 1
		}
		w.appSessions(c.exes, func(_ bool, vol appVolume) {
			switch {
			case vol.simple != nil:
				set := uintptr(1)
				if muted {
					set = 0
				}
				vol.simple.call(5, set, 0) // SetMute
			case muted:
				vol.setLevel(restore)
			default:
				vol.setLevel(0)
			}
		})
	case "toggle", "next", "prev":
		method := map[string]int{"toggle": 20, "next": 16, "prev": 17}[c.kind] // TryTogglePlayPause/TrySkipNext/TrySkipPrevious
		w.sessions(func(id string, s *com) {
			if id == c.source {
				if op := s.out(method); op != nil {
					wait(op)
					op.release()
				}
			}
		})
	}
}

// sessions calls f for every app that publishes media controls. Two sessions
// of the same app (two browser tabs) get "#2", "#3" appended to their ID.
func (w *worker) sessions(f func(id string, s *com)) {
	if w.manager == nil {
		if w.manager = requestManager(); w.manager == nil {
			return
		}
	}
	list := w.manager.out(7) // GetSessions
	if list == nil {
		return
	}
	defer list.release()
	var n uint32
	list.call(7, ptr(&n)) // IVectorView::get_Size
	seen := map[string]int{}
	for i := uint32(0); i < n && i < 12; i++ {
		s := list.out(6, uintptr(i)) // IVectorView::GetAt
		if s == nil {
			continue
		}
		id := s.getString(6) // get_SourceAppUserModelId
		if seen[id]++; seen[id] > 1 {
			id += "#" + strconv.Itoa(seen[id])
		}
		f(id, s)
		s.release()
	}
}

func (w *worker) read(prevArt map[string]string) (State, map[string][]byte) {
	st := State{}
	st.Volume, st.Muted = w.volume()
	art := map[string][]byte{}
	keys := map[string]string{}
	w.sessions(func(id string, s *com) {
		src, key, img := w.readSession(id, s)
		if key != "" && key == w.trackKeys[id] {
			src.ArtID = prevArt[id]
			keys[id] = key
		} else if img != nil {
			// A player often announces a new track before it swaps the cover, and
			// a fast reading then finds the old cover under the new title. Keep
			// the old cover and look again; a new track that really shares its
			// cover (the next song of an album) is accepted after a moment.
			old := w.trackKeys[id]
			if old != "" && old != key && prevArt[id] != "" && artID(img) == prevArt[id] {
				since, waiting := w.stale[id]
				if !waiting {
					since = time.Now()
					w.stale[id] = since
				}
				if time.Since(since) < 2500*time.Millisecond {
					src.ArtID = prevArt[id]
					keys[id] = old
					img = nil
				}
			}
			if img != nil {
				delete(w.stale, id)
				art[id] = img
				keys[id] = key
			}
		}
		// Without a cover yet, players often publish it a moment later; the
		// key stays unset so the next reading tries again.
		src.Volume = -1
		w.appSessions(sourceExes(id), func(active bool, vol appVolume) {
			if src.Volume >= 0 && !active {
				return
			}
			src.Volume, src.Muted = int(vol.level()*100+0.5), vol.muted()
		})
		st.Sources = append(st.Sources, src)
	})
	w.trackKeys = keys
	if w.manager != nil {
		if cur := w.manager.out(6); cur != nil { // GetCurrentSession
			st.Current = cur.getString(6)
			cur.release()
		}
	}
	return st, art
}

// readSession reads one app's track. key identifies the track; img is its
// cover, read only when the track differs from the one seen last time.
func (w *worker) readSession(id string, session *com) (src Source, key string, img []byte) {
	src = Source{ID: id, App: appName(id)}

	if pb := session.out(9); pb != nil { // GetPlaybackInfo
		var status int32
		pb.call(7, ptr(&status)) // get_PlaybackStatus
		src.Playing = status == 4
		pb.release()
	}

	if tl := session.out(8); tl != nil { // GetTimelineProperties
		var end, pos, updated int64
		tl.call(7, ptr(&end))      // get_EndTime (TimeSpan, 100 ns)
		tl.call(10, ptr(&pos))     // get_Position
		tl.call(11, ptr(&updated)) // get_LastUpdatedTime (DateTime, 100 ns since 1601)
		tl.release()
		if src.Playing && updated > 0 {
			pos += fileTimeNow() - updated
		}
		src.DurationMs = end / 10000
		src.PositionMs = min(max(pos/10000, 0), src.DurationMs)
	}

	props := await(session.out(7)) // TryGetMediaPropertiesAsync
	if props == nil {
		// Between two tracks a player can answer with nothing. Keep showing the
		// last track instead of an empty card.
		if last, ok := w.lastInfo[id]; ok {
			src.Title, src.Artist, src.Album = last.Title, last.Artist, last.Album
			return src, w.trackKeys[id], nil
		}
		return src, "", nil
	}
	defer props.release()
	src.Title = props.getString(6)
	src.Artist = props.getString(9)
	src.Album = props.getString(10)
	if len(w.lastInfo) > 32 {
		clear(w.lastInfo)
	}
	w.lastInfo[id] = Source{Title: src.Title, Artist: src.Artist, Album: src.Album}

	key = src.Title + "\x00" + src.Artist + "\x00" + src.Album
	if key == w.trackKeys[id] {
		return src, key, nil
	}
	img, err := thumbnail(props)
	if err != nil {
		return src, "", nil
	}
	return src, key, img
}

func requestManager() *com {
	name := hstring("Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager")
	defer hsDelete.Call(name)
	var statics *com
	if r, _, _ := roFact.Call(name, ptr(&iidManagerStatics), ptr(&statics)); int32(r) < 0 || statics == nil {
		return nil
	}
	defer statics.release()
	return await(statics.out(6)) // RequestAsync
}

const maxArt = 1 << 20

func thumbnail(props *com) ([]byte, error) {
	ref := props.out(15) // get_Thumbnail
	if ref == nil {
		return nil, errors.New("no thumbnail")
	}
	defer ref.release()
	ras := await(ref.out(6)) // OpenReadAsync
	if ras == nil {
		return nil, errors.New("cannot open thumbnail")
	}
	defer ras.release()
	var stream *com
	if r, _, _ := streamOv.Call(uintptr(unsafe.Pointer(ras)), ptr(&iidStream), ptr(&stream)); int32(r) < 0 || stream == nil {
		return nil, errors.New("cannot read thumbnail")
	}
	defer stream.release()

	var out []byte
	buf := make([]byte, 32<<10)
	for len(out) < maxArt {
		var n uint32
		hr := stream.call(3, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), ptr(&n)) // IStream::Read
		out = append(out, buf[:n]...)
		if hr != 0 || n == 0 {
			break
		}
	}
	if len(out) == 0 {
		return nil, errors.New("empty thumbnail")
	}
	return out, nil
}

const clsctxAll = 0x17

// device runs f with the default playback device. It is looked up each time,
// so switching speakers/headphones just works.
func (w *worker) device(f func(dev *com)) bool {
	return w.defaultDevice(0, 1, f) // eRender, eMultimedia
}

// defaultDevice runs f with the default device for a flow (0 playback,
// 1 recording) and role (0 console, 1 multimedia, 2 communications).
func (w *worker) defaultDevice(flow, role uintptr, f func(dev *com)) bool {
	if w.enumerator == nil {
		if r, _, _ := coCreate.Call(ptr(&clsidEnumerator), 0, clsctxAll, ptr(&iidEnumerator), ptr(&w.enumerator)); int32(r) < 0 {
			w.enumerator = nil
			return false
		}
	}
	dev := w.enumerator.out(4, flow, role) // GetDefaultAudioEndpoint
	if dev == nil {
		return false
	}
	defer dev.release()
	f(dev)
	return true
}

// endpoint runs f with the default playback device's volume control.
func (w *worker) endpoint(f func(ep *com)) bool {
	ok := false
	w.device(func(dev *com) {
		ep := dev.out(3, ptr(&iidEndpointVolume), clsctxAll, 0) // Activate
		if ep == nil {
			return
		}
		defer ep.release()
		f(ep)
		ok = true
	})
	return ok
}

// mics runs f with the volume control of the default microphone and, if it
// is a different one, the default microphone for calls (Discord, Teams).
func (w *worker) mics(f func(ep *com)) {
	seen := map[string]bool{}
	for _, role := range []uintptr{0, 2} {
		w.defaultDevice(1, role, func(dev *com) {
			var p *uint16
			if dev.call(5, ptr(&p)) < 0 || p == nil { // GetId
				return
			}
			id := syscall.UTF16ToString(unsafe.Slice(p, wcslen(p)))
			coFree.Call(uintptr(unsafe.Pointer(p)))
			if seen[id] {
				return
			}
			seen[id] = true
			if ep := dev.out(3, ptr(&iidEndpointVolume), clsctxAll, 0); ep != nil { // Activate
				f(ep)
				ep.release()
			}
		})
	}
}

// wcslen is the length of a NUL-terminated UTF-16 string (at most 512).
func wcslen(p *uint16) int {
	n := 0
	for n < 512 && *(*uint16)(unsafe.Add(unsafe.Pointer(p), n*2)) != 0 {
		n++
	}
	return n
}

// mic reports whether there is a microphone and whether it is muted.
func (w *worker) mic() *Mic {
	var m *Mic
	w.mics(func(ep *com) {
		if m == nil {
			var muted int32
			ep.call(15, ptr(&muted)) // GetMute
			m = &Mic{Muted: muted != 0}
		}
	})
	return m
}

// volume reads the master volume and mute state.
func (w *worker) volume() (int, bool) {
	var level float32
	var muted int32
	ok := w.endpoint(func(ep *com) {
		ep.call(9, ptr(&level))  // GetMasterVolumeLevelScalar
		ep.call(15, ptr(&muted)) // GetMute
	})
	if !ok {
		return -1, false
	}
	return int(level*100 + 0.5), muted != 0
}

// appVolume is one mixer entry's volume. Windows normally offers
// ISimpleAudioVolume; some builds only hand out IChannelAudioVolume, which has
// no mute, so there a mute is a volume of zero.
type appVolume struct {
	simple, channel *com
}

func (v appVolume) level() float32 {
	var level float32
	if v.simple != nil {
		v.simple.call(4, ptr(&level)) // GetMasterVolume
		return level
	}
	var n uint32
	v.channel.call(3, ptr(&n)) // GetChannelCount
	for i := uint32(0); i < n && i < 16; i++ {
		var l float32
		v.channel.call(5, uintptr(i), ptr(&l)) // GetChannelVolume
		level = max(level, l)
	}
	return level
}

func (v appVolume) setLevel(level float32) {
	bits := uintptr(math.Float32bits(level))
	if v.simple != nil {
		v.simple.call(3, bits, 0) // SetMasterVolume
		return
	}
	var n uint32
	v.channel.call(3, ptr(&n))
	for i := uint32(0); i < n && i < 16; i++ {
		v.channel.call(4, uintptr(i), bits, 0) // SetChannelVolume
	}
}

func (v appVolume) muted() bool {
	if v.simple != nil {
		var m int32
		v.simple.call(6, ptr(&m)) // GetMute
		return m != 0
	}
	return v.level() == 0
}

// mixerSessions calls f with every live entry of the volume mixer.
func (w *worker) mixerSessions(f func(p proc, active bool, vol appVolume)) {
	w.device(func(dev *com) {
		mgr := dev.out(3, ptr(&iidSessionManager2), clsctxAll, 0) // Activate
		if mgr == nil {
			return
		}
		defer mgr.release()
		list := mgr.out(5) // GetSessionEnumerator
		if list == nil {
			return
		}
		defer list.release()
		var n int32
		list.call(3, ptr(&n)) // GetCount
		for i := int32(0); i < n && i < 64; i++ {
			ctl := list.out(4, uintptr(i)) // GetSession
			if ctl == nil {
				continue
			}
			var state int32
			ctl.call(3, ptr(&state)) // GetState: 0 inactive, 1 active, 2 expired
			c2 := ctl.out(0, ptr(&iidSessionControl2))
			var vol appVolume
			if vol.simple = ctl.out(0, ptr(&iidSimpleVolume)); vol.simple == nil {
				vol.channel = ctl.out(0, ptr(&iidChannelVolume))
			}
			if c2 != nil && (vol.simple != nil || vol.channel != nil) && state != 2 {
				var pid uint32
				c2.call(14, ptr(&pid)) // GetProcessId
				if p := w.process(pid); p.id != "" {
					f(p, state == 1, vol)
				}
			}
			vol.simple.release()
			vol.channel.release()
			c2.release()
			ctl.release()
		}
	})
}

// appSessions calls f with the volume control of every mixer entry that
// belongs to one of the given mixer IDs (a browser can have several).
func (w *worker) appSessions(exes []string, f func(active bool, vol appVolume)) {
	w.mixerSessions(func(p proc, active bool, vol appVolume) {
		for _, e := range exes {
			if p.id == e {
				f(active, vol)
				return
			}
		}
	})
}

// self is DeskLink's own executable, left out of the mixer.
var self = func() string {
	exe, _ := os.Executable()
	return strings.ToLower(strings.TrimSuffix(filepath.Base(exe), filepath.Ext(exe)))
}()

// readMixer lists the programs in the mixer, one entry per program: those
// playing sound first, then the rest, each group by name.
func (w *worker) readMixer() Mixer {
	m := Mixer{}
	m.Volume, m.Muted = w.volume()
	m.Mic = w.mic()
	m.Outputs = w.outputs()
	at := map[string]int{}
	w.mixerSessions(func(p proc, active bool, vol appVolume) {
		if p.id == self {
			return
		}
		i, ok := at[p.id]
		if !ok {
			i = len(m.Apps)
			at[p.id] = i
			m.Apps = append(m.Apps, MixerApp{ID: p.id, Name: w.displayName(p), Path: p.path, Volume: -1})
		}
		a := &m.Apps[i]
		if a.Volume < 0 || (active && !a.Active) {
			a.Volume, a.Muted = int(vol.level()*100+0.5), vol.muted()
		}
		a.Active = a.Active || active
	})
	sort.SliceStable(m.Apps, func(i, j int) bool {
		a, b := m.Apps[i], m.Apps[j]
		if a.Active != b.Active {
			return a.Active
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	return m
}

// process returns a process's executable: its mixer ID (name, lowercase,
// without ".exe") and full path.
func (w *worker) process(pid uint32) proc {
	if pid == 0 {
		return proc{}
	}
	if p, ok := w.procs[pid]; ok {
		return p
	}
	const queryLimited = 0x1000 // PROCESS_QUERY_LIMITED_INFORMATION
	h, err := syscall.OpenProcess(queryLimited, false, pid)
	if err != nil {
		return proc{}
	}
	defer syscall.CloseHandle(h)
	buf := make([]uint16, 512)
	size := uint32(len(buf))
	if r, _, _ := imageName.Call(uintptr(h), 0, ptr(&buf[0]), ptr(&size)); r == 0 {
		return proc{}
	}
	path := syscall.UTF16ToString(buf[:size])
	p := proc{id: strings.ToLower(strings.TrimSuffix(strings.TrimSuffix(filepath.Base(path), ".exe"), ".EXE")), path: path}
	if len(w.procs) > 256 {
		clear(w.procs)
	}
	w.procs[pid] = p
	return p
}

// displayName is the program's own description of itself ("Spotify",
// "Google Chrome"), or its file name.
func (w *worker) displayName(p proc) string {
	if n, ok := w.names[p.path]; ok {
		return n
	}
	name := fileDescription(p.path)
	if name == "" || len(name) > 40 {
		name = strings.TrimSuffix(filepath.Base(p.path), filepath.Ext(p.path))
	}
	if len(w.names) > 256 {
		clear(w.names)
	}
	w.names[p.path] = name
	return name
}

// fileDescription reads the FileDescription from an executable's version info.
func fileDescription(path string) string {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	size, _, _ := verSize.Call(uintptr(unsafe.Pointer(p)), 0)
	if size == 0 || size > 1<<20 {
		return ""
	}
	data := make([]byte, size)
	if r, _, _ := verInfo.Call(uintptr(unsafe.Pointer(p)), 0, size, uintptr(unsafe.Pointer(&data[0]))); r == 0 {
		return ""
	}
	query := func(q string) (uintptr, uint32) {
		qp, _ := syscall.UTF16PtrFromString(q)
		var out uintptr
		var n uint32
		if r, _, _ := verQuery.Call(uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(qp)), ptr(&out), ptr(&n)); r == 0 || out == 0 {
			return 0, 0
		}
		return out, n
	}
	langs := []string{"040904b0", "040904e4", "000004b0"}
	if t, n := query(`\VarFileInfo\Translation`); n >= 4 {
		pair := *(**[2]uint16)(unsafe.Pointer(&t))
		langs = append([]string{fmt.Sprintf("%04x%04x", pair[0], pair[1])}, langs...)
	}
	for _, l := range langs {
		if s, n := query(`\StringFileInfo\` + l + `\FileDescription`); n > 0 {
			return strings.TrimSpace(syscall.UTF16ToString(unsafe.Slice(*(**uint16)(unsafe.Pointer(&s)), n)))
		}
	}
	return ""
}

func fileTimeNow() int64 {
	var ft syscall.Filetime
	syscall.GetSystemTimeAsFileTime(&ft)
	return int64(ft.HighDateTime)<<32 | int64(ft.LowDateTime)
}
