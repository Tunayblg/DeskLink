package web

import (
	"bytes"
	"testing"
)

// The dashboard is put together from three files; a placeholder left behind
// would ship a dashboard without its user guide.
func TestAdminPageIsAssembled(t *testing.T) {
	if bytes.Contains(AdminPage, []byte("{{GUIDE_")) || bytes.Contains(AdminPage, []byte("{{BRANDS_")) {
		t.Fatal("a guide placeholder was not replaced")
	}
	for _, want := range []string{"{{NONCE}}", "{{ADMIN_TOKEN}}", "window.DeskGuide", ".g-leaf", "window.DeskBrands"} {
		if !bytes.Contains(AdminPage, []byte(want)) {
			t.Errorf("dashboard is missing %q", want)
		}
	}
}
