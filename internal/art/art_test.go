package art

import "testing"

func TestNotTheGame(t *testing.T) {
	for name, skip := range map[string]bool{
		"TslGame.exe":             false,
		"Game.exe":                false,
		"UnityCrashHandler64.exe": true,
		"unins000.exe":            true,
		"BEService_x64.exe":       true,
		"TslGame_BE.exe":          true,
		"UnrealCEFSubProcess.exe": true,
		"EasyAntiCheat_Setup.exe": true,
	} {
		if got := notTheGame.MatchString(name); got != skip {
			t.Errorf("%s: skip=%v, want %v", name, got, skip)
		}
	}
}
