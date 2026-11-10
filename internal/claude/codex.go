package claude

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Codex CLI keeps its sessions in ~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl.
// A turn opens with a "task_started" event and closes with "task_complete"
// (or "turn_aborted" when it is stopped); tool calls come in between. A
// session stays in the folder of the day it was opened, even when it is resumed
// later, so the last week of folders is looked at (by when the file was written).

func (r *Reader) codexDir() string {
	if r.CodexDir != "" {
		return r.CodexDir
	}
	if r.Dir != "" {
		return "" // a test with its own logs: leave this PC's Codex out
	}
	if d := os.Getenv("CODEX_HOME"); d != "" {
		return filepath.Join(d, "sessions")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".codex", "sessions")
}

// newestCodex returns the Codex session written most recently.
func (r *Reader) newestCodex(now time.Time) (string, time.Time) {
	root := r.codexDir()
	if root == "" {
		return "", time.Time{}
	}
	var best string
	var bestMod time.Time
	for i := 0; i < 8; i++ {
		day := now.AddDate(0, 0, -i)
		dir := filepath.Join(root, day.Format("2006"), day.Format("01"), day.Format("02"))
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
				continue
			}
			info, err := e.Info()
			if err == nil && info.ModTime().After(bestMod) {
				best, bestMod = filepath.Join(dir, e.Name()), info.ModTime()
			}
		}
	}
	return best, bestMod
}

// codexState is state for Codex: "working" or "idle", the tool being run, and
// the seconds since its newest session was written.
func (r *Reader) codexState(now time.Time) (string, string, int64) {
	path, mod := r.newestCodex(now)
	if path == "" {
		return "", "", 0
	}
	age := max(now.Sub(mod), 0)
	secs := int64(age.Seconds())
	if age > staleAfter {
		return "idle", "", secs
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", "", 0
	}
	if r.cx.path != path || !r.cx.mod.Equal(info.ModTime()) || r.cx.size != info.Size() {
		st, doing := tailState(path, scanCodex)
		r.cx = activity{path: path, mod: info.ModTime(), size: info.Size(), state: st, doing: doing}
	}
	return r.cx.state, r.cx.doing, secs
}

type codexLine struct {
	Type    string `json:"type"`
	Payload struct {
		Type string `json:"type"`
		Name string `json:"name"`
	} `json:"payload"`
}

var markPayload = []byte(`"payload"`)

// scanCodex reads a Codex session's end, newest first.
func scanCodex(f *os.File, start int64) (state, doing string, found bool) {
	lines, ok := tailLines(f, start)
	if !ok {
		return "", "", true
	}
	tool, toolDone := "", false
	for i := len(lines) - 1; i >= 0; i-- {
		b := lines[i]
		if !bytes.Contains(b, markPayload) {
			continue
		}
		var l codexLine
		if json.Unmarshal(b, &l) != nil {
			continue
		}
		switch l.Payload.Type {
		case "task_complete", "turn_aborted":
			return "idle", "", true
		case "task_started":
			return "working", tool, true
		case "function_call_output", "custom_tool_call_output":
			toolDone = true // the newest tool has answered: Codex is thinking again
		case "function_call", "custom_tool_call", "local_shell_call":
			if !toolDone && tool == "" {
				tool = shortTool(l.Payload.Name)
				if tool == "" {
					tool = "shell"
				}
			}
			toolDone = true
		}
	}
	return "", "", false
}
