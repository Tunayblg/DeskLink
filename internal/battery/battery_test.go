package battery

import "testing"

func TestAddrOf(t *testing.T) {
	cases := map[string]string{
		`BTHLE\DEV_AABBCCDDEE01\A&55667788&0&AABBCCDDEE01`:                                                         "AABBCCDDEE01",
		`BTHENUM\DEV_AABBCCDDEE02\A&11223344&0&BLUETOOTHDEVICE_AABBCCDDEE02`:                                       "AABBCCDDEE02",
		`BTHENUM\{0000111E-0000-1000-8000-00805F9B34FB}_VID&00010094_PID&0004\A&11223344&0&AABBCCDDEE03_C00000000`: "AABBCCDDEE03",
		`USB\VID_1532&PID_00B0\5&1&0&2`:                                                                            "",
	}
	for id, want := range cases {
		if got := addrOf(id); got != want {
			t.Errorf("addrOf(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestKindOf(t *testing.T) {
	cases := map[string]string{
		"Xbox Wireless Controller": "gamepad",
		"Test Gamepad":             "gamepad",
		"Test Buds":                "headset",
		"Test Headset":             "headset",
		"Test Mouse":               "mouse",
		"Test Keyboard":            "keyboard",
		"Test Mini KB":             "keyboard",
		"Something Else":           "device",
	}
	for name, want := range cases {
		if got := kindOf(name); got != want {
			t.Errorf("kindOf(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestRazerBattery(t *testing.T) {
	req := razerRequest(0x1F, 0x07, 0x80)
	if len(req) != 91 || req[2] != 0x1F || req[6] != 0x02 || req[7] != 0x07 || req[8] != 0x80 || req[89] != (0x1F^0x02^0x07^0x80) {
		t.Fatalf("request = % x", req[:12])
	}
	// An answer as a Razer mouse gives it: status 2, level 0x67 of 255.
	resp := make([]byte, 91)
	resp[1], resp[10] = 0x02, 0x67
	if pct, ok := razerLevel(resp); !ok || pct != 40 {
		t.Errorf("level = %d %v, want 40", pct, ok)
	}
	// Status 4 means the mouse did not answer (switched off): no reading.
	resp[1] = 0x04
	if _, ok := razerLevel(resp); ok {
		t.Error("an unanswered request must not give a level")
	}
	if _, ok := razerLevel(resp[:20]); ok {
		t.Error("a short answer must not give a level")
	}
}

func TestCollect(t *testing.T) {
	nodes := []node{
		// A keyboard that is connected: the battery sits on its own node.
		{id: `BTHLE\DEV_AABBCCDDEE01\x`, name: "Test Keyboard", level: 71, known: true, connected: true},
		// A headset that is paired but off: the profile node still holds an old level.
		{id: `BTHENUM\DEV_AABBCCDDEE03\x`, name: "Test Headset 2", level: -1, known: true, connected: false},
		{id: `BTHENUM\{0000111E}_VID&1_PID&4\A&1&0&AABBCCDDEE03_C0`, name: "Test Headset 2 Hands-Free AG", level: 37},
		// A headset that is on: the level comes from the profile node, the name from the device node.
		{id: `BTHENUM\DEV_AABBCCDDEE02\x`, name: "Test Buds", level: -1, known: true, connected: true},
		{id: `BTHENUM\{0000111E}_L\A&1&0&AABBCCDDEE02_C0`, name: "Test Buds Hands-Free AG", level: 100},
		// Nonsense levels are ignored.
		{id: `BTHLE\DEV_AAAAAAAAAAAA\x`, name: "Odd", level: 250, known: true, connected: true},
	}
	got := collect(nodes)
	if len(got) != 2 || got[0] != (Device{"Test Buds", 100, "headset"}) || got[1] != (Device{"Test Keyboard", 71, "keyboard"}) {
		t.Fatalf("collect = %+v", got)
	}
}
