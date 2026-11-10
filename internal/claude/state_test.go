package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sessionLog(t *testing.T, lines ...string) *Reader {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "s.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return &Reader{Dir: dir, Now: time.Now}
}

const (
	promptLine   = `{"type":"user","message":{"role":"user","content":"hi"}}`
	toolCallLine = `{"type":"assistant","message":{"id":"a","model":"claude-opus-5-5","stop_reason":"tool_use","usage":{"output_tokens":1}}}`
	toolDoneLine = `{"type":"user","message":{"role":"user","content":[{"type":"tool_result"}]}}`
	answerLine   = `{"type":"assistant","message":{"id":"b","model":"claude-opus-5-5","stop_reason":"end_turn","usage":{"output_tokens":2}}}`
	helperLine   = `{"type":"user","isSidechain":true,"message":{"role":"user","content":"sub"}}`
)

func TestStateFollowsTheLastEntry(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"prompt sent", []string{promptLine}, "working"},
		{"tool call running", []string{promptLine, toolCallLine}, "working"},
		{"tool result in", []string{promptLine, toolCallLine, toolDoneLine}, "working"},
		{"answered", []string{promptLine, toolCallLine, toolDoneLine, answerLine}, "idle"},
		{"helper agent writing after the answer", []string{promptLine, answerLine, helperLine}, "idle"},
		{"interrupted", []string{promptLine, `{"type":"user","message":{"content":[{"type":"text","text":"[Request interrupted by user]"}]}}`}, "idle"},
		{"interrupted, plain text", []string{promptLine, `{"type":"user","message":{"content":"[Request interrupted by user for tool use]"}}`}, "idle"},
		{"prompt that quotes the notice", []string{answerLine, `{"type":"user","message":{"content":"why does it say [Request interrupted by user] here?"}}`}, "working"},
		{"huge last entry", []string{promptLine, toolCallLine, `{"type":"user","message":{"content":[{"type":"tool_result","content":"` + strings.Repeat("x", 600<<10) + `"}]}}`}, "working"},
	}
	for _, c := range cases {
		u := sessionLog(t, c.lines...).Sample()
		if u.State != c.want {
			t.Errorf("%s: state %q, want %q", c.name, u.State, c.want)
		}
	}
}

func TestStateIsIdleWhenTheLogIsOld(t *testing.T) {
	r := sessionLog(t, promptLine)
	r.Now = func() time.Time { return time.Now().Add(staleAfter + time.Minute) }
	if u := r.Sample(); u.State != "idle" {
		t.Fatalf("state %q for a log nobody wrote to for a while, want idle", u.State)
	}
}

func TestNoLogsNoState(t *testing.T) {
	r := &Reader{Dir: t.TempDir(), Now: time.Now}
	if u := r.Sample(); u.State != "" {
		t.Fatalf("state %q without any log", u.State)
	}
}
