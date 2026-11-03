package media

import (
	"errors"
	"math"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

// This file talks to WinRT and Core Audio through raw COM vtables, so DeskLink
// needs no cgo and no extra runtime. Vtable indexes were checked against the
// Windows metadata; methods 0-5 of every WinRT interface are IUnknown/IInspectable.

var (
	combase  = syscall.NewLazyDLL("combase.dll")
	ole32    = syscall.NewLazyDLL("ole32.dll")
	shcore   = syscall.NewLazyDLL("shcore.dll")
	roInit   = combase.NewProc("RoInitialize")
	roFact   = combase.NewProc("RoGetActivationFactory")
	hsCreate = combase.NewProc("WindowsCreateString")
	hsDelete = combase.NewProc("WindowsDeleteString")
	hsBuffer = combase.NewProc("WindowsGetStringRawBuffer")
	coCreate = ole32.NewProc("CoCreateInstance")
	streamOv = shcore.NewProc("CreateStreamOverRandomAccessStream")
)

type guid struct {
	d1     uint32
	d2, d3 uint16
	d4     [8]byte
}

var (
	iidManagerStatics = guid{0x2050c4ee, 0x11a0, 0x57de, [8]byte{0xae, 0xd7, 0xc9, 0x7c, 0x70, 0x33, 0x82, 0x45}}
	iidAsyncInfo      = guid{0x00000036, 0, 0, [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidStream         = guid{0x0000000c, 0, 0, [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	clsidEnumerator   = guid{0xbcde0395, 0xe52f, 0x467c, [8]byte{0x8e, 0x3d, 0xc4, 0x57, 0x92, 0x91, 0x69, 0x2e}}
	iidEnumerator     = guid{0xa95664d2, 0x9614, 0x4f35, [8]byte{0xa7, 0x46, 0xde, 0x8d, 0xb6, 0x36, 0x17, 0xe6}}
	iidEndpointVolume = guid{0x5cdf2c82, 0x841e, 0x4546, [8]byte{0x97, 0x22, 0x0c, 0xf7, 0x40, 0x78, 0x22, 0x9a}}
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

// await waits for a WinRT IAsyncOperation and returns its result.
func await(op *com) *com {
	if op == nil {
		return nil
	}
	defer op.release()
	info := op.out(0, ptr(&iidAsyncInfo))
	if info == nil {
		return nil
	}
	defer info.release()
	deadline := time.Now().Add(2 * time.Second)
	for {
		var status int32
		info.call(7, ptr(&status)) // get_Status
		if status == 1 {           // Completed
			break
		}
		if status != 0 || time.Now().After(deadline) { // Canceled, Error, or stuck
			return nil
		}
		time.Sleep(3 * time.Millisecond)
	}
	return op.out(8) // GetResults
}

// source runs every COM call on one OS thread joined to the multithreaded apartment.
type source struct {
	req chan request
}

type request struct {
	prevArt   string
	setVolume int // 0-100, or -1 for a normal reading
	reply     chan reply
}

type reply struct {
	info Info
	art  []byte
}

func newSource() source {
	s := source{req: make(chan request)}
	go s.loop()
	return s
}

func (s source) read(prevArt string) (Info, []byte) {
	r := make(chan reply, 1)
	s.req <- request{prevArt: prevArt, setVolume: -1, reply: r}
	out := <-r
	return out.info, out.art
}

func (s source) setVolume(level int) {
	r := make(chan reply, 1)
	s.req <- request{setVolume: level, reply: r}
	<-r
}

type worker struct {
	manager    *com
	enumerator *com
	trackKey   string
}

func (s source) loop() {
	runtime.LockOSThread()
	roInit.Call(1) // RO_INIT_MULTITHREADED
	var w worker
	for req := range s.req {
		if req.setVolume >= 0 {
			w.endpoint(func(ep *com) {
				ep.call(7, uintptr(math.Float32bits(float32(req.setVolume)/100)), 0) // SetMasterVolumeLevelScalar
			})
			req.reply <- reply{}
			continue
		}
		info, art := w.read(req.prevArt)
		req.reply <- reply{info, art}
	}
}

func (w *worker) read(prevArt string) (Info, []byte) {
	info := Info{Volume: -1}
	info.Volume, info.Muted = w.volume()

	if w.manager == nil {
		w.manager = requestManager()
		if w.manager == nil {
			return info, nil
		}
	}
	session := w.manager.out(6) // GetCurrentSession
	if session == nil {
		w.trackKey = ""
		return info, nil
	}
	defer session.release()

	info.App = appName(session.getString(6)) // get_SourceAppUserModelId

	if pb := session.out(9); pb != nil { // GetPlaybackInfo
		var status int32
		pb.call(7, ptr(&status)) // get_PlaybackStatus
		info.Playing = status == 4
		pb.release()
	}

	if tl := session.out(8); tl != nil { // GetTimelineProperties
		var end, pos, updated int64
		tl.call(7, ptr(&end))      // get_EndTime (TimeSpan, 100 ns)
		tl.call(10, ptr(&pos))     // get_Position
		tl.call(11, ptr(&updated)) // get_LastUpdatedTime (DateTime, 100 ns since 1601)
		tl.release()
		if info.Playing && updated > 0 {
			pos += fileTimeNow() - updated
		}
		info.DurationMs = end / 10000
		info.PositionMs = min(max(pos/10000, 0), info.DurationMs)
	}

	props := await(session.out(7)) // TryGetMediaPropertiesAsync
	if props == nil {
		return info, nil
	}
	defer props.release()
	info.Title = props.getString(6)
	info.Artist = props.getString(9)
	info.Album = props.getString(10)

	key := info.App + "\x00" + info.Title + "\x00" + info.Artist + "\x00" + info.Album
	if key == w.trackKey {
		info.ArtID = prevArt
		return info, nil
	}
	art, err := thumbnail(props)
	if err != nil {
		// Players often publish the title before the cover; try again next time.
		return info, nil
	}
	w.trackKey = key
	return info, art
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

// endpoint runs f with the default playback device's volume control. The
// device is looked up each time, so switching speakers/headphones just works.
func (w *worker) endpoint(f func(ep *com)) bool {
	const clsctxAll = 0x17
	if w.enumerator == nil {
		if r, _, _ := coCreate.Call(ptr(&clsidEnumerator), 0, clsctxAll, ptr(&iidEnumerator), ptr(&w.enumerator)); int32(r) < 0 {
			w.enumerator = nil
			return false
		}
	}
	device := w.enumerator.out(4, 0, 1) // GetDefaultAudioEndpoint(eRender, eMultimedia)
	if device == nil {
		return false
	}
	defer device.release()
	ep := device.out(3, ptr(&iidEndpointVolume), clsctxAll, 0) // Activate
	if ep == nil {
		return false
	}
	defer ep.release()
	f(ep)
	return true
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

func fileTimeNow() int64 {
	var ft syscall.Filetime
	syscall.GetSystemTimeAsFileTime(&ft)
	return int64(ft.HighDateTime)<<32 | int64(ft.LowDateTime)
}
