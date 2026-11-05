package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Tunayblg/DeskLink/internal/auth"
	"github.com/Tunayblg/DeskLink/internal/config"
)

// The dashboard asks the PC to enlarge its own window while the guide is open.
func TestDashboardWindowEndpoint(t *testing.T) {
	cfg, _ := config.Parse([]byte(config.Default))
	store, _ := auth.Open(filepath.Join(t.TempDir(), "devices.json"))
	s := New(cfg, store, fstest.MapFS{"index.html": {Data: []byte("ok")}}, nil)
	var asked []bool
	s.MaximizeWindow = func(on bool) bool { asked = append(asked, on); return on } // only "maximize" changes anything
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	post := func(token, body string) (int, string) {
		req, _ := http.NewRequest("POST", ts.URL+"/admin/api/window", strings.NewReader(body))
		if token != "" {
			req.Header.Set("X-Admin-Token", token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var sb strings.Builder
		buf := make([]byte, 256)
		n, _ := resp.Body.Read(buf)
		sb.Write(buf[:n])
		return resp.StatusCode, strings.TrimSpace(sb.String())
	}

	if code, _ := post("", `{"maximize":true}`); code != http.StatusForbidden && code != http.StatusUnauthorized {
		t.Errorf("without the dashboard's token: status %d, want it refused", code)
	}
	if len(asked) != 0 {
		t.Fatal("the window was touched without a token")
	}
	if code, body := post(s.adminToken, `{"maximize":true}`); code != 200 || body != `{"changed":true}` {
		t.Errorf("maximize: %d %s", code, body)
	}
	if code, body := post(s.adminToken, `{"maximize":false}`); code != 200 || body != `{"changed":false}` {
		t.Errorf("restore: %d %s", code, body)
	}
	if code, _ := post(s.adminToken, `not json`); code != http.StatusBadRequest {
		t.Errorf("bad body: status %d", code)
	}
	if len(asked) != 2 || !asked[0] || asked[1] {
		t.Errorf("window calls = %v, want [true false]", asked)
	}

	// With no window support (another system), nothing changes and nothing breaks.
	s.MaximizeWindow = nil
	if code, body := post(s.adminToken, `{"maximize":true}`); code != 200 || body != `{"changed":false}` {
		t.Errorf("no window support: %d %s", code, body)
	}
}
