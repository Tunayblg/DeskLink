package main

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"
)

// The version appears in three places. A release with one of them forgotten
// would show one number in the app, another in the file properties and a third
// on the phone.
func TestVersionIsTheSameEverywhere(t *testing.T) {
	var info struct {
		FixedFileInfo struct {
			FileVersion struct{ Major, Minor, Patch int }
		}
		StringFileInfo struct{ FileVersion, ProductVersion string }
	}
	raw, err := os.ReadFile("versioninfo.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &info); err != nil {
		t.Fatal(err)
	}
	fixed := info.FixedFileInfo.FileVersion
	numbers := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`).FindStringSubmatch(version)
	if numbers == nil {
		t.Fatalf("version %q is not major.minor.patch", version)
	}
	want := [3]int{fixed.Major, fixed.Minor, fixed.Patch}
	var got [3]int
	for i := range got {
		for _, c := range numbers[i+1] {
			got[i] = got[i]*10 + int(c-'0')
		}
	}
	if got != want {
		t.Errorf("main.go says %s but versioninfo.json says %d.%d.%d (run go generate ./cmd/desklink after changing it)", version, fixed.Major, fixed.Minor, fixed.Patch)
	}
	if info.StringFileInfo.FileVersion != version || info.StringFileInfo.ProductVersion != version {
		t.Errorf("versioninfo.json text versions %q and %q differ from %s", info.StringFileInfo.FileVersion, info.StringFileInfo.ProductVersion, version)
	}

	gradle, err := os.ReadFile("../../android/app/build.gradle.kts")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`versionName = "([^"]+)"`).FindSubmatch(gradle)
	if m == nil {
		t.Fatal("no versionName in android/app/build.gradle.kts")
	}
	if string(m[1]) != version {
		t.Errorf("the Android app says %s but main.go says %s", m[1], version)
	}
}
