package media

import (
	"log"
	"runtime"
	"sort"
	"syscall"
	"unsafe"
)

// Playback devices: the list the phone picks from, and switching the
// default one (what Windows' sound settings call the output device).
//
// Windows has no documented call for setting the default device. Every tool
// that does it (the sound control panel included) uses IPolicyConfig, which
// has stayed the same from Windows 7 to 11.

var (
	clsidPolicyConfig = guid{0x870af99c, 0x171d, 0x4f9e, [8]byte{0xaf, 0x0d, 0xe6, 0x3d, 0xf4, 0x0c, 0x2b, 0xc9}}
	iidPolicyConfig   = guid{0xf8679f50, 0x850a, 0x41cf, [8]byte{0x9c, 0x72, 0x43, 0x0f, 0x29, 0x02, 0x90, 0xc8}}
	pkeyFriendlyName  = propKey{guid{0xa45c254e, 0xdf1c, 0x4efd, [8]byte{0x80, 0x20, 0x67, 0xd1, 0x46, 0xa8, 0x50, 0xe0}}, 14}
	propClear         = ole32.NewProc("PropVariantClear")
)

// maxOutputs caps the list; nobody has more playback devices than this.
const maxOutputs = 16

type propKey struct {
	fmtid guid
	pid   uint32
}

// propVariant is a PROPVARIANT: a type tag, padding, and a value of up to
// two pointers.
type propVariant struct {
	vt         uint16
	r1, r2, r3 uint16
	val        [2]uintptr
}

// deviceID returns an IMMDevice's ID.
func deviceID(dev *com) string {
	var p *uint16
	if dev.call(5, ptr(&p)) < 0 || p == nil { // GetId
		return ""
	}
	defer coFree.Call(uintptr(unsafe.Pointer(p)))
	return syscall.UTF16ToString(unsafe.Slice(p, wcslen(p)))
}

// friendlyName returns the name Windows shows for a device, such as
// "Headphones (WH-1000XM4)".
func friendlyName(dev *com) string {
	store := dev.out(4, 0) // OpenPropertyStore(STGM_READ)
	if store == nil {
		return ""
	}
	defer store.release()
	var pv propVariant
	if store.call(5, ptr(&pkeyFriendlyName), ptr(&pv)) < 0 { // GetValue
		return ""
	}
	defer propClear.Call(ptr(&pv))
	if pv.vt != 31 || pv.val[0] == 0 { // VT_LPWSTR
		return ""
	}
	p := *(**uint16)(unsafe.Pointer(&pv.val[0]))
	return syscall.UTF16ToString(unsafe.Slice(p, wcslen(p)))
}

// outputs lists the active playback devices, the default one marked.
func (w *worker) outputs() []Output {
	def := ""
	if !w.device(func(dev *com) { def = deviceID(dev) }) {
		return nil
	}
	coll := w.enumerator.out(3, 0, 1) // EnumAudioEndpoints(eRender, DEVICE_STATE_ACTIVE)
	if coll == nil {
		return nil
	}
	defer coll.release()
	var n uint32
	if coll.call(3, ptr(&n)) < 0 { // GetCount
		return nil
	}
	var out []Output
	for i := uint32(0); i < n && len(out) < maxOutputs; i++ {
		dev := coll.out(4, uintptr(i)) // Item
		if dev == nil {
			continue
		}
		id, name := deviceID(dev), friendlyName(dev)
		dev.release()
		if id == "" {
			continue
		}
		if name == "" {
			name = "Audio device"
		}
		out = append(out, Output{ID: id, Name: name, Default: id == def})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// setOutput makes a playback device the default for every role (games and
// music, and calls). Only a device from the current list is accepted.
func (w *worker) setOutput(id string) {
	found := false
	for _, o := range w.outputs() {
		if o.ID == id {
			found = !o.Default
			break
		}
	}
	if !found {
		return
	}
	var pc *com
	if r, _, _ := coCreate.Call(ptr(&clsidPolicyConfig), 0, clsctxAll, ptr(&iidPolicyConfig), ptr(&pc)); int32(r) < 0 || pc == nil {
		log.Printf("media: Windows would not switch the playback device (%#x)", uint32(r))
		return
	}
	defer pc.release()
	u, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		return
	}
	for role := uintptr(0); role < 3; role++ { // eConsole, eMultimedia, eCommunications
		if hr := pc.call(13, uintptr(unsafe.Pointer(u)), role); hr < 0 { // SetDefaultEndpoint
			log.Printf("media: switching the playback device failed (%#x)", uint32(hr))
		}
	}
	runtime.KeepAlive(u)
}
