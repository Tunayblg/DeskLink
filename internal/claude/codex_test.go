package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDoingNamesTheRunningTool(t *testing.T) {
	call := `{"type":"assistant","message":{"id":"c","model":"claude-opus-5-5","stop_reason":"tool_use","content":[{"type":"text","text":"x"},{"type":"tool_use","name":"Bash"}],"usage":{"output_tokens":1}}}`
	mcp := `{"type":"assistant","message":{"id":"d","model":"claude-opus-5-5","content":[{"type":"tool_use","name":"mcp__github__create_issue"}],"usage":{"output_tokens":1}}}`
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"tool call", []string{promptLine, call}, "Bash"},
		{"mcp tool", []string{promptLine, mcp}, "create_issue"},
		{"tool answered: thinking", []string{promptLine, call, toolDoneLine}, ""},
		{"done", []string{promptLine, call, toolDoneLine, answerLine}, ""},
	}
	for _, c := range cases {
		u := sessionLog(t, c.lines...).Sample()
		if u.Doing != c.want || u.Agent != "claude" {
			t.Errorf("%s: agent %q doing %q, want claude %q", c.name, u.Agent, u.Doing, c.want)
		}
	}
}

// codexLog writes a Codex session under today's folder and a Claude log that
// was written earlier, so Codex is the one in use.
func codexLog(t *testing.T, lines ...string) *Reader {
	t.Helper()
	r := sessionLog(t, promptLine, answerLine)
	old := time.Now().Add(-time.Minute)
	os.Chtimes(filepath.Join(r.Dir, "s.jsonl"), old, old)
	root := t.TempDir()
	now := time.Now()
	day := filepath.Join(root, now.Format("2006"), now.Format("01"), now.Format("02"))
	if err := os.MkdirAll(day, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(day, "rollout-1.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.CodexDir = root
	return r
}

func TestCodexState(t *testing.T) {
	started := `{"type":"event_msg","payload":{"type":"task_started"}}`
	call := `{"type":"response_item","payload":{"type":"custom_tool_call","name":"apply_patch"}}`
	out := `{"type":"response_item","payload":{"type":"custom_tool_call_output"}}`
	done := `{"type":"event_msg","payload":{"type":"task_complete"}}`
	cases := []struct {
		name         string
		lines        []string
		state, doing string
	}{
		{"started", []string{started}, "working", ""},
		{"running a tool", []string{started, call}, "working", "apply_patch"},
		{"tool answered", []string{started, call, out}, "working", ""},
		{"complete", []string{started, call, out, done}, "idle", ""},
		{"aborted", []string{started, `{"type":"event_msg","payload":{"type":"turn_aborted"}}`}, "idle", ""},
	}
	for _, c := range cases {
		u := codexLog(t, c.lines...).Sample()
		if u.Agent != "codex" || u.State != c.state || u.Doing != c.doing {
			t.Errorf("%s: %s %q %q, want codex %q %q", c.name, u.Agent, u.State, u.Doing, c.state, c.doing)
		}
	}
}

func TestNewerClaudeWinsOverCodex(t *testing.T) {
	r := codexLog(t, `{"type":"event_msg","payload":{"type":"task_started"}}`)
	day := time.Now()
	p := filepath.Join(r.CodexDir, day.Format("2006"), day.Format("01"), day.Format("02"), "rollout-1.jsonl")
	old := time.Now().Add(-2 * time.Minute)
	os.Chtimes(p, old, old)
	now := time.Now()
	os.Chtimes(filepath.Join(r.Dir, "s.jsonl"), now, now)
	if u := r.Sample(); u.Agent != "claude" || u.State != "idle" {
		t.Fatalf("got %s %q, want claude idle", u.Agent, u.State)
	}
}
