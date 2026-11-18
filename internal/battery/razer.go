package battery

import (
	"strings"
	"time"
)

// isRazerPath tells a Razer HID interface path ("\\?\hid#vid_1532&pid_00b0&mi_00#...") from others,
// so nothing is ever sent to a device of another maker.
func isRazerPath(p string) bool {
	return strings.Contains(strings.ToLower(p), "vid_1532&")
}

// razerTransactions are the transaction IDs different Razer generations use;
// the one a device does not know is answered with status 4 (no answer).
var razerTransactions = []byte{0x1F, 0x3F, 0x9F, 0xFF}

// razerReportLen is the 90 byte Razer report plus the leading report ID.
const razerReportLen = 91

// razerRequest builds a "get" request for one command (class, id).
func razerRequest(txn, class, id byte) []byte {
	b := make([]byte, razerReportLen)
	b[2] = txn   // transaction ID
	b[6] = 0x02  // data size
	b[7] = class // command class
	b[8] = id    // command id
	var crc byte
	for _, v := range b[3:89] {
		crc ^= v
	}
	b[89] = crc
	return b
}

// razerLevel turns an answer to the battery request into percent. The status
// byte is 2 when the device understood and answered; the level is 0-255.
func razerLevel(resp []byte) (int, bool) {
	if len(resp) < razerReportLen || resp[1] != 0x02 {
		return 0, false
	}
	return (int(resp[10])*100 + 127) / 255, true
}

func sleepMs(n int) { time.Sleep(time.Duration(n) * time.Millisecond) }
