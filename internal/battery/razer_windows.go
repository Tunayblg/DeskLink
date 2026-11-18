package battery

import (
	"syscall"
	"unsafe"
)

// Razer devices that use their own 2.4 GHz dongle do not show their charge to
// Windows, but they answer the same question Razer's own software asks: a
// "get battery" request sent as a HID feature report. Only that read-only
// request is sent, and only to Razer (vendor 1532) interfaces.

var (
	hid                  = syscall.NewLazyDLL("hid.dll")
	procHidAttributes    = hid.NewProc("HidD_GetAttributes")
	procHidSetFeature    = hid.NewProc("HidD_SetFeature")
	procHidGetFeature    = hid.NewProc("HidD_GetFeature")
	procHidProductString = hid.NewProc("HidD_GetProductString")
	procHidPreparsed     = hid.NewProc("HidD_GetPreparsedData")
	procHidFreePreparsed = hid.NewProc("HidD_FreePreparsedData")
	procHidCaps          = hid.NewProc("HidP_GetCaps")
	procIfaceListSize    = cfgmgr.NewProc("CM_Get_Device_Interface_List_SizeW")
	procIfaceList        = cfgmgr.NewProc("CM_Get_Device_Interface_ListW")
)

const razerVendor = 0x1532

// GUID_DEVINTERFACE_HID.
var guidHID = syscall.GUID{Data1: 0x4D1E55B2, Data2: 0xF16F, Data3: 0x11CF,
	Data4: [8]byte{0x88, 0xCB, 0x00, 0x11, 0x11, 0x00, 0x00, 0x30}}

type hidAttributes struct {
	size    uint32
	vendor  uint16
	product uint16
	version uint16
}

func readRazer() []Device {
	var out []Device
	seen := map[uint16]bool{}
	for _, path := range hidPaths() {
		// Opened without any access: the mouse's own interface refuses read and
		// write access (Windows keeps it for the mouse driver), but feature
		// reports still go through.
		h, err := syscall.CreateFile(path, 0,
			syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, 0, 0)
		if err != nil {
			continue
		}
		a := hidAttributes{size: uint32(unsafe.Sizeof(hidAttributes{}))}
		ok, _, _ := procHidAttributes.Call(uintptr(h), uintptr(unsafe.Pointer(&a)))
		if ok != 0 && a.vendor == razerVendor && !seen[a.product] && featureLen(h) == razerReportLen {
			if pct, ok := razerBattery(h); ok {
				seen[a.product] = true
				out = append(out, Device{Name: productName(h, a.product), Percent: pct, Kind: "mouse"})
			}
		}
		syscall.CloseHandle(h)
	}
	return out
}

// featureLen is the size of the interface's feature report (0 when it has none),
// so a request is only sent where a Razer report fits.
func featureLen(h syscall.Handle) int {
	var pre uintptr
	if r, _, _ := procHidPreparsed.Call(uintptr(h), uintptr(unsafe.Pointer(&pre))); r == 0 {
		return 0
	}
	defer procHidFreePreparsed.Call(pre)
	var caps [64]byte // HIDP_CAPS; the feature report length is the fifth 16 bit field
	if r, _, _ := procHidCaps.Call(pre, uintptr(unsafe.Pointer(&caps[0]))); r == 0 {
		return 0
	}
	return int(caps[8]) | int(caps[9])<<8
}

// razerBattery asks one interface for the battery, trying each transaction ID.
func razerBattery(h syscall.Handle) (int, bool) {
	for _, txn := range razerTransactions {
		req := razerRequest(txn, 0x07, 0x80)
		if r, _, _ := procHidSetFeature.Call(uintptr(h), uintptr(unsafe.Pointer(&req[0])), uintptr(len(req))); r == 0 {
			return 0, false // not the interface that takes Razer reports
		}
		// The device needs a moment to answer.
		sleepMs(30)
		resp := make([]byte, razerReportLen)
		if r, _, _ := procHidGetFeature.Call(uintptr(h), uintptr(unsafe.Pointer(&resp[0])), uintptr(len(resp))); r == 0 {
			continue
		}
		if pct, ok := razerLevel(resp); ok {
			return pct, true
		}
	}
	return 0, false
}

// productName is the name the device gives itself, or one made from its ID.
func productName(h syscall.Handle, pid uint16) string {
	buf := make([]uint16, 64)
	if r, _, _ := procHidProductString.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)*2)); r != 0 {
		if s := syscall.UTF16ToString(buf); s != "" {
			return s
		}
	}
	return "Razer"
}

// hidPaths lists the paths of the present HID interfaces that belong to Razer.
func hidPaths() []*uint16 {
	var size uint32
	if r, _, _ := procIfaceListSize.Call(uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&guidHID)), 0, 0); r != 0 || size < 2 {
		return nil
	}
	buf := make([]uint16, size)
	if r, _, _ := procIfaceList.Call(uintptr(unsafe.Pointer(&guidHID)), 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(size), 0); r != 0 {
		return nil
	}
	var paths []*uint16
	start := 0
	for i, c := range buf {
		if c != 0 {
			continue
		}
		if i > start {
			p := syscall.UTF16ToString(buf[start:i])
			if isRazerPath(p) {
				if u, err := syscall.UTF16PtrFromString(p); err == nil {
					paths = append(paths, u)
				}
			}
		}
		start = i + 1
	}
	return paths
}
