package claude

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// How the phone learns whether Claude is still busy with a prompt.
//
// Claude Code appends to the session log as it works: the prompt, each reply
// (with a stop reason), each tool result. The end of the newest log therefore
// tells it: a reply that stopped on "end_turn" means Claude has answered; a
// prompt, a tool call or a tool result as the last entry means it is still
// going. A log that has not been written to for a long time is not busy
// whatever its last line says (the session was closed or interrupted).
const (
	tailBytes  = 256 << 10 // how much of the end of a log is looked at
	staleAfter = 10 * time.Minute
)

type activity struct {
	path  string
	mod   time.Time
	size  int64
	state string // "working", "idle" or "" if the tail says nothing
	doing string // while working: the tool being run ("Bash", "Edit"...), "" while it thinks
}

// isSubagentLog reports whether a log belongs to a helper agent. Those write
// while the main session waits, so they must not decide its state.
func isSubagentLog(path string) bool {
	return strings.HasPrefix(filepath.Base(path), "agent-") ||
		strings.Contains(filepath.ToSlash(path), "/subagents/")
}

// state returns "working" or "idle", the tool being run, and the seconds since
// the newest log was written.
func (r *Reader) state(now time.Time) (string, string, int64) {
	if r.newest == "" {
		return "", "", 0
	}
	age := now.Sub(r.newestMod)
	if age < 0 {
		age = 0
	}
	secs := int64(age.Seconds())
	if age > staleAfter {
		return "idle", "", secs
	}
	info, err := os.Stat(r.newest)
	if err != nil {
		return "", "", 0
	}
	if r.act.path != r.newest || !r.act.mod.Equal(info.ModTime()) || r.act.size != info.Size() {
		st, doing := tailState(r.newest, scanTail)
		r.act = activity{path: r.newest, mod: info.ModTime(), size: info.Size(), state: st, doing: doing}
	}
	return r.act.state, r.act.doing, secs
}

type tailLine struct {
	Type        string `json:"type"`
	IsSidechain bool   `json:"isSidechain"`
	Message     struct {
		StopReason string          `json:"stop_reason"`
		Content    json.RawMessage `json:"content"`
	} `json:"message"`
}

// toolName returns the tool a reply calls, "" if it calls none.
func toolName(content json.RawMessage) string {
	var blocks []struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if json.Unmarshal(content, &blocks) != nil {
		return ""
	}
	name := ""
	for _, b := range blocks {
		if b.Type == "tool_use" && b.Name != "" {
			name = b.Name
		}
	}
	return shortTool(name)
}

// shortTool keeps a tool name short enough for the phone: MCP tools are
// "mcp__server__tool", only the tool part is kept.
func shortTool(name string) string {
	if i := strings.LastIndex(name, "__"); i >= 0 {
		name = name[i+2:]
	}
	if len(name) > 24 {
		name = name[:24]
	}
	return name
}

var markUser = []byte(`"type":"user"`)

// interruptText starts the entry Claude Code writes when the person stops it.
const interruptText = "[Request interrupted by user"

// interrupted reports whether a user entry is that notice. It must open the
// content: a prompt that merely quotes the phrase is a prompt.
func interrupted(content json.RawMessage) bool {
	var s string
	if json.Unmarshal(content, &s) == nil {
		return strings.HasPrefix(s, interruptText)
	}
	var blocks []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(content, &blocks) == nil {
		for _, b := range blocks {
			if strings.HasPrefix(b.Text, interruptText) {
				return true
			}
		}
	}
	return false
}

// maxTail is how far back the end of a log is searched when its last entry is
// very large (a tool result that returned a big file can be several hundred KB).
const maxTail = 8 << 20

// tailState reads the end of a log and works out the state with scan, looking
// further back while scan finds nothing.
func tailState(path string, scan func(*os.File, int64) (string, string, bool)) (string, string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", ""
	}
	for size := int64(tailBytes); ; size *= 4 {
		start := max(info.Size()-size, 0)
		if state, doing, found := scan(f, start); found || start == 0 || size >= maxTail {
			return state, doing
		}
	}
}

// tailLines reads a file from start on and splits it into lines, dropping a
// first line that was cut in the middle.
func tailLines(f *os.File, start int64) ([][]byte, bool) {
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, false
	}
	buf, err := io.ReadAll(f)
	if err != nil {
		return nil, false
	}
	lines := bytes.Split(buf, []byte("\n"))
	if start > 0 && len(lines) > 0 {
		lines = lines[1:]
	}
	return lines, true
}

// scanTail looks at the entries from offset start on, newest first. found is
// false if none of them says anything, so a wider look may help.
func scanTail(f *os.File, start int64) (state, doing string, found bool) {
	lines, ok := tailLines(f, start)
	if !ok {
		return "", "", true
	}
	for i := len(lines) - 1; i >= 0; i-- {
		b := lines[i]
		if !bytes.Contains(b, markAssistant) && !bytes.Contains(b, markUser) {
			continue
		}
		var l tailLine
		if json.Unmarshal(b, &l) != nil || l.IsSidechain {
			continue // unfinished last line, or a helper agent's
		}
		switch l.Type {
		case "assistant":
			switch l.Message.StopReason {
			case "end_turn", "stop_sequence", "max_tokens", "refusal":
				return "idle", "", true
			}
			return "working", toolName(l.Message.Content), true // a tool call, or still streaming
		case "user":
			if interrupted(l.Message.Content) {
				return "idle", "", true
			}
			return "working", "", true // a prompt waiting for an answer, or a tool result
		}
	}
	return "", "", false
}
