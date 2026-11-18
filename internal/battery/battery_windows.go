package battery

import (
	"syscall"
	"unsafe"
)

var (
	cfgmgr                 = syscall.NewLazyDLL("cfgmgr32.dll")
	procIDListSize         = cfgmgr.NewProc("CM_Get_Device_ID_List_SizeW")
	procIDList             = cfgmgr.NewProc("CM_Get_Device_ID_ListW")
	procLocateDevNode      = cfgmgr.NewProc("CM_Locate_DevNodeW")
	procGetDevNodeProperty = cfgmgr.NewProc("CM_Get_DevNode_PropertyW")
)

const (
	filterEnumerator = 0x1   // CM_GETIDLIST_FILTER_ENUMERATOR
	filterPresent    = 0x100 // CM_GETIDLIST_FILTER_PRESENT

	typeByte    = 0x03 // DEVPROP_TYPE_BYTE
	typeBoolean = 0x11 // DEVPROP_TYPE_BOOLEAN
	typeString  = 0x12 // DEVPROP_TYPE_STRING
)

type devPropKey struct {
	guid syscall.GUID
	pid  uint32
}

var (
	// The level a device reports, in percent.
	keyBattery = devPropKey{syscall.GUID{Data1: 0x104EA319, Data2: 0x6EE2, Data3: 0x4701,
		Data4: [8]byte{0xBD, 0x47, 0x8D, 0xDB, 0xF4, 0x25, 0xBB, 0xE5}}, 2}
	// Whether the device is connected right now (DEVPKEY_Device_IsConnected).
	keyConnected = devPropKey{syscall.GUID{Data1: 0x83DA6326, Data2: 0x97A6, Data3: 0x4088,
		Data4: [8]byte{0x94, 0x53, 0xA1, 0x92, 0x3F, 0x57, 0x3B, 0x29}}, 15}
	// DEVPKEY_Device_FriendlyName.
	keyFriendly = devPropKey{syscall.GUID{Data1: 0xA45C254E, Data2: 0xDF1C, Data3: 0x4EFD,
		Data4: [8]byte{0x80, 0x20, 0x67, 0xD1, 0x46, 0xA8, 0x50, 0xE0}}, 14}
)

// list reads every present Bluetooth device node and joins them (see collect).
func list() []Device {
	var nodes []node
	for _, enum := range []string{"BTHENUM", "BTHLE"} {
		for _, id := range deviceIDs(enum) {
			if n, ok := readNode(id); ok {
				nodes = append(nodes, n)
			}
		}
	}
	return append(collect(nodes), readRazer()...)
}

// deviceIDs lists the IDs of the present devices under one enumerator.
func deviceIDs(enum string) []string {
	filter, err := syscall.UTF16PtrFromString(enum)
	if err != nil {
		return nil
	}
	var size uint32
	if r, _, _ := procIDListSize.Call(uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(filter)),
		filterEnumerator|filterPresent); r != 0 || size < 2 {
		return nil
	}
	buf := make([]uint16, size)
	if r, _, _ := procIDList.Call(uintptr(unsafe.Pointer(filter)), uintptr(unsafe.Pointer(&buf[0])),
		uintptr(size), filterEnumerator|filterPresent); r != 0 {
		return nil
	}
	// The buffer holds the IDs one after the other, each ending in a zero.
	var ids []string
	start := 0
	for i, c := range buf {
		if c != 0 {
			continue
		}
		if i > start {
			ids = append(ids, syscall.UTF16ToString(buf[start:i]))
		}
		start = i + 1
	}
	return ids
}

func readNode(id string) (node, bool) {
	p, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		return node{}, false
	}
	var inst uint32
	if r, _, _ := procLocateDevNode.Call(uintptr(unsafe.Pointer(&inst)), uintptr(unsafe.Pointer(p)), 0); r != 0 {
		return node{}, false
	}
	n := node{id: id, level: -1}
	if b, t := property(inst, &keyBattery); t == typeByte && len(b) >= 1 {
		n.level = int(b[0])
	}
	if b, t := property(inst, &keyConnected); t == typeBoolean && len(b) >= 1 {
		n.known, n.connected = true, b[0] != 0
	}
	if b, t := property(inst, &keyFriendly); t == typeString && len(b) >= 2 {
		u := make([]uint16, len(b)/2)
		for i := range u {
			u[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
		}
		n.name = syscall.UTF16ToString(u)
	}
	return n, true
}

// property returns the raw bytes and type of one device node property, or a
// zero type when the node does not have it.
func property(inst uint32, key *devPropKey) ([]byte, uint32) {
	var typ uint32
	buf := make([]byte, 512)
	size := uint32(len(buf))
	if r, _, _ := procGetDevNodeProperty.Call(uintptr(inst), uintptr(unsafe.Pointer(key)),
		uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0); r != 0 {
		return nil, 0
	}
	return buf[:size], typ
}
