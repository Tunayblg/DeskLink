package usb

import (
	"reflect"
	"testing"
)

func TestParseDevices(t *testing.T) {
	out := "* daemon not running; starting now at tcp:5037\r\n* daemon started successfully\r\n" +
		"List of devices attached\r\n" +
		"R58M12345AB\tdevice\r\n" +
		"emulator-5554\toffline\r\n" +
		"ZX1G22\tunauthorized\r\n" +
		"192.168.1.5:5555\tdevice\r\n\r\n"
	want := []string{"R58M12345AB", "192.168.1.5:5555"}
	if got := parseDevices(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("parseDevices = %v, want %v", got, want)
	}
	if got := parseDevices("List of devices attached\n\n"); len(got) != 0 {
		t.Fatalf("no phones should give none, got %v", got)
	}
}
