package claude

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func entry(id, model, ts string, in, out, cw, cr int) string {
	return fmt.Sprintf(`{"type":"assistant","timestamp":%q,"requestId":"r-%s","message":{"id":%q,"model":%q,"usage":{"input_tokens":%d,"output_tokens":%d,"cache_creation_input_tokens":%d,"cache_read_input_tokens":%d}}}`+"\n",
		ts, id, id, model, in, out, cw, cr)
}

func TestSample(t *testing.T) {
	dir := t.TempDir()
	proj := filepath.Join(dir, "proj")
	os.MkdirAll(proj, 0o755)
	now := time.Date(2026, 11, 8, 15, 30, 0, 0, time.UTC)
	log := filepath.Join(proj, "a.jsonl")
	body := `{"type":"user","message":{"content":"secret prompt"}}` + "\n" +
		entry("m0", "claude-opus-5-5", "2026-11-07T01:10:00Z", 10, 10, 0, 0) + // yesterday, an older window
		entry("m1", "claude-opus-5-5", "2026-11-08T12:20:00Z", 100, 50, 1000, 5000) +
		entry("m1", "claude-opus-5-5", "2026-11-08T12:20:01Z", 100, 70, 1000, 5000) + // same reply, more output: counted once
		entry("m2", "claude-sonnet-5-5", "2026-11-08T14:00:00Z", 20, 30, 0, 100) +
		`{"type":"assistant","timestamp":"2026-11-08T14:01:00Z","message":{"id":"m3","model":"<synthetic>","usage":{"input_tokens":9}}}` + "\n" +
		`{"type":"assistant","partial` // a line still being written
	os.WriteFile(log, []byte(body), 0o644)
	os.Chtimes(log, now, now)

	r := &Reader{Dir: dir, Now: func() time.Time { return now }}
	u := r.Sample()
	if !u.Found || !u.Active {
		t.Fatalf("found/active = %v/%v", u.Found, u.Active)
	}
	// Window starts 12:00, ends 17:00; now is 15:30.
	if want := int64(100 + 70 + 1000 + 20 + 30); u.Block.Tokens != want {
		t.Errorf("block tokens = %d, want %d", u.Block.Tokens, want)
	}
	if u.Block.ResetS != 90*60 || u.Block.Replies != 2 || u.Block.Cached != 5100 {
		t.Errorf("block = %+v", u.Block)
	}
	if u.Today != 1220 || u.Week != 1220+20 {
		t.Errorf("today/week = %d/%d", u.Today, u.Week)
	}
	if len(u.Models) != 2 || u.Models[0].Name != "Opus" || u.Models[1].Name != "Sonnet" {
		t.Errorf("models = %+v", u.Models)
	}
	if u.IdleS != int64((90 * time.Minute).Seconds()) {
		t.Errorf("idle = %d", u.IdleS)
	}

	// The unfinished line is read once it is complete, and nothing is counted twice.
	f, _ := os.OpenFile(log, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString(`":1}` + "\n" + entry("m4", "claude-haiku-5-5", "2026-11-08T15:00:00Z", 1, 2, 3, 0))
	f.Close()
	u = r.Sample()
	if u.Block.Tokens != 1220+6 || u.Block.Replies != 3 {
		t.Errorf("after append: %+v", u.Block)
	}
}

func TestWindowEnded(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 8, 20, 0, 0, 0, time.UTC)
	os.WriteFile(filepath.Join(dir, "a.jsonl"), []byte(entry("m1", "claude-opus-5-5", "2026-11-08T12:20:00Z", 1, 1, 0, 0)), 0o644)
	os.Chtimes(filepath.Join(dir, "a.jsonl"), now, now)
	u := (&Reader{Dir: dir, Now: func() time.Time { return now }}).Sample()
	if u.Active || u.Block.Tokens != 0 || u.Today != 2 {
		t.Errorf("usage = %+v", u)
	}
}

func TestNoLogs(t *testing.T) {
	u := (&Reader{Dir: filepath.Join(t.TempDir(), "missing")}).Sample()
	if u.Found || u.IdleS != -1 {
		t.Errorf("usage = %+v", u)
	}
}

func TestLimits(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "projects")
	os.MkdirAll(dir, 0o755)
	now := time.Date(2026, 11, 8, 15, 30, 0, 0, time.UTC)
	r := &Reader{Dir: dir, Now: func() time.Time { return now }}
	if u := r.Sample(); u.Limits != nil {
		t.Fatalf("limits before the file exists: %+v", u.Limits)
	}
	body := fmt.Sprintf(`{"five_hour":{"used_percentage":23.5,"resets_at":%d},"seven_day":{"used_percentage":141,"resets_at":%d}}`,
		now.Add(90*time.Minute).Unix(), now.Add(-time.Minute).Unix())
	os.WriteFile(filepath.Join(root, LimitsFile), []byte(body), 0o644)
	l := r.Sample().Limits
	if l == nil || l.FiveHour == nil || l.FiveHour.Pct != 23.5 || l.FiveHour.ResetS != 5400 {
		t.Fatalf("five hour = %+v", l)
	}
	// Past its reset time a window reads as unused.
	if l.SevenDay == nil || l.SevenDay.Pct != 0 || l.SevenDay.ResetS != 0 {
		t.Errorf("seven day = %+v", l.SevenDay)
	}
	os.WriteFile(filepath.Join(root, LimitsFile), []byte("garbage"), 0o644)
	if r.Sample().Limits != nil {
		t.Error("garbage file should give no limits")
	}
}
