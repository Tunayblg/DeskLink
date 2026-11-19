package stats

import (
	"syscall"
	"time"
	"unsafe"
)

var procGetIfTable = syscall.NewLazyDLL("iphlpapi.dll").NewProc("GetIfTable")

// MIB_IFROW is 860 bytes; these are the offsets of the fields used here.
const (
	ifRowSize    = 860
	ifRowType    = 516
	ifRowOper    = 544
	ifRowInOct   = 552
	ifRowOutOct  = 576
	ifTypeEther  = 6
	ifTypeWifi   = 71
	ifOperUp     = 5 // operational
	ifOperConnec = 4 // connected
)

// netSampler turns the network adapters' byte counters into a speed. It only
// reads two numbers per adapter from Windows (no traffic, no ping), and keeps
// one small buffer. Of all adapters it reports the busiest one, so a virtual
// switch that mirrors the real card is not counted twice.
type netSampler struct {
	buf  []byte
	prev map[int][2]uint32 // adapter index -> in, out counters
	at   time.Time
}

func (s *netSampler) sample() *Net {
	size := uint32(len(s.buf))
	if size == 0 {
		size = 8 << 10
		s.buf = make([]byte, size)
	}
	for tries := 0; tries < 2; tries++ {
		r, _, _ := procGetIfTable.Call(uintptr(unsafe.Pointer(&s.buf[0])), uintptr(unsafe.Pointer(&size)), 0)
		if r == 0 {
			break
		}
		if r != 122 || tries == 1 { // ERROR_INSUFFICIENT_BUFFER: grow once
			return nil
		}
		s.buf = make([]byte, size)
	}
	n := int(*(*uint32)(unsafe.Pointer(&s.buf[0])))
	now := time.Now()
	cur := map[int][2]uint32{}
	var best *Net
	var bestSum float64
	dt := now.Sub(s.at).Seconds()
	for i := 0; i < n && 4+(i+1)*ifRowSize <= len(s.buf); i++ {
		row := s.buf[4+i*ifRowSize:]
		typ := *(*uint32)(unsafe.Pointer(&row[ifRowType]))
		oper := *(*uint32)(unsafe.Pointer(&row[ifRowOper]))
		if (typ != ifTypeEther && typ != ifTypeWifi) || (oper != ifOperUp && oper != ifOperConnec) {
			continue
		}
		in := *(*uint32)(unsafe.Pointer(&row[ifRowInOct]))
		out := *(*uint32)(unsafe.Pointer(&row[ifRowOutOct]))
		cur[i] = [2]uint32{in, out}
		p, ok := s.prev[i]
		if !ok || dt <= 0 || dt > 10 {
			continue
		}
		down, up := float64(in-p[0])/dt, float64(out-p[1])/dt // uint32 wrap-around is fine
		if down+up >= bestSum {
			bestSum = down + up
			best = &Net{DownBps: int64(down), UpBps: int64(up)}
		}
	}
	s.prev, s.at = cur, now
	return best
}
