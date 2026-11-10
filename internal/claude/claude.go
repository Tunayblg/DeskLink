// Package claude summarises how much Claude Code has been used on this PC.
//
// Claude Code writes every reply, with its token counts, to JSON-lines files
// under ~/.claude/projects. This package reads those files (only the part
// added since the last look) and adds the numbers up: the current five-hour
// window, today and the last seven days. Only numbers leave this package, never
// what was said.
//
// The plan limit shown by Claude's own /usage command is not stored on the PC,
// so it cannot be read here. The five-hour window is worked out the way the
// usage tools for Claude Code do it: it starts at the hour of the first reply
// and lasts five hours.
package claude

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	window    = 5 * time.Hour
	keepFor   = 8 * 24 * time.Hour // older replies are neither read nor kept
	refreshes = 3 * time.Second    // files are looked at most this often
)

// Usage is what the phone shows.
type Usage struct {
	Found  bool    `json:"found"`            // false: no Claude Code logs on this PC
	Active bool    `json:"active"`           // a five-hour window is running
	Block  Block   `json:"block"`            // the running (or last) window
	Today  int64   `json:"today"`            // tokens since midnight
	Week   int64   `json:"week"`             // tokens in the last seven days
	Models []Model `json:"models,omitempty"` // the window's tokens by model family, biggest first
	IdleS  int64   `json:"idle_s"`           // seconds since the last reply, -1 if none
	Limits *Limits `json:"limits,omitempty"` // the plan limits in percent; absent until Claude Code has reported them
	State  string  `json:"state,omitempty"`  // "working": Claude is on a prompt; "idle": it has answered; absent: no recent session
	StateS int64   `json:"state_s"`          // seconds since the newest session wrote anything
	Agent  string  `json:"agent,omitempty"`  // whose state that is: "claude" or "codex" (the one used last)
	Doing  string  `json:"doing,omitempty"`  // while working: the tool being run; empty while it thinks
}

// Block is one five-hour window.
type Block struct {
	Tokens  int64 `json:"tokens"`   // input + output + cache writes
	Output  int64 `json:"output"`   // output tokens alone
	Cached  int64 `json:"cached"`   // tokens read back from the cache (not counted in Tokens)
	Replies int   `json:"replies"`  // number of replies
	ResetS  int64 `json:"reset_s"`  // seconds until the window ends, 0 if none is running
	ElapseS int64 `json:"elapse_s"` // seconds since it started
}

// Model is a share of the window's tokens.
type Model struct {
	Name   string `json:"name"`
	Tokens int64  `json:"tokens"`
}

type reply struct {
	at                          time.Time
	model                       string
	in, out, cacheWrite, cacheR int64
}

type fileState struct {
	offset int64
	mod    time.Time
}

// Reader reads the logs. The zero value is ready to use.
type Reader struct {
	// Dir is the folder with the project logs; empty means ~/.claude/projects
	// (or $CLAUDE_CONFIG_DIR/projects).
	Dir string
	// CodexDir is Codex CLI's sessions folder; empty means ~/.codex/sessions
	// (or $CODEX_HOME/sessions), and none at all when Dir is set.
	CodexDir string
	// Now is the clock; nil means time.Now. Tests set it.
	Now func() time.Time

	mu      sync.Mutex
	checked time.Time
	files   map[string]*fileState
	replies map[string]reply // message ID -> reply
	seen    int              // files found on the last look

	newest    string    // the session log written most recently
	newestMod time.Time // when it was written
	act       activity  // what the end of that log says, kept until the log changes
	cx        activity  // the same for Codex's newest session
}

func (r *Reader) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *Reader) dir() string {
	if r.Dir != "" {
		return r.Dir
	}
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return filepath.Join(d, "projects")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "projects")
}

// Sample returns the current numbers. It reads new log lines when the last
// look was a few seconds ago, so several phones share one pass.
func (r *Reader) Sample() Usage {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	if !r.checked.IsZero() && now.Sub(r.checked) < refreshes && r.Now == nil {
		// The countdown still has to move, so only the reading is reused.
		return r.sum(now)
	}
	r.checked = now
	r.scan(now)
	return r.sum(now)
}

// scan reads what was added to the log files since the last time.
func (r *Reader) scan(now time.Time) {
	if r.files == nil {
		r.files = map[string]*fileState{}
		r.replies = map[string]reply{}
	}
	root := r.dir()
	r.seen = 0
	r.newest, r.newestMod = "", time.Time{}
	present := map[string]bool{}
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		info, err := d.Info()
		if err != nil || now.Sub(info.ModTime()) > keepFor {
			return nil
		}
		r.seen++
		present[path] = true
		if !isSubagentLog(path) && info.ModTime().After(r.newestMod) {
			r.newest, r.newestMod = path, info.ModTime()
		}
		st := r.files[path]
		if st == nil {
			st = &fileState{}
			r.files[path] = st
		}
		if info.Size() < st.offset { // rewritten: start again
			st.offset = 0
		}
		if info.Size() == st.offset {
			return nil
		}
		st.offset = r.read(path, st.offset, now)
		st.mod = info.ModTime()
		return nil
	})
	for p := range r.files {
		if !present[p] {
			delete(r.files, p)
		}
	}
	for id, rp := range r.replies {
		if now.Sub(rp.at) > keepFor {
			delete(r.replies, id)
		}
	}
}

// line is the part of a log line that matters.
type line struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	RequestID string    `json:"requestId"`
	Message   struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage struct {
			Input      int64 `json:"input_tokens"`
			Output     int64 `json:"output_tokens"`
			CacheWrite int64 `json:"cache_creation_input_tokens"`
			CacheRead  int64 `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

var (
	markAssistant = []byte(`"type":"assistant"`)
	markUsage     = []byte(`"usage"`)
)

// read parses the complete lines of a file from offset on and returns the
// offset after the last one (a line still being written is left for next time).
func (r *Reader) read(path string, offset int64, now time.Time) int64 {
	f, err := os.Open(path)
	if err != nil {
		return offset
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset
	}
	br := bufio.NewReaderSize(f, 64<<10)
	for {
		b, err := br.ReadBytes('\n')
		if err != nil { // end of file, possibly in the middle of a line
			return offset
		}
		offset += int64(len(b))
		if !bytes.Contains(b, markAssistant) || !bytes.Contains(b, markUsage) {
			continue
		}
		var l line
		if json.Unmarshal(b, &l) != nil || l.Type != "assistant" || l.Message.Model == "" || strings.HasPrefix(l.Message.Model, "<") {
			continue
		}
		if now.Sub(l.Timestamp) > keepFor {
			continue
		}
		u := l.Message.Usage
		key := l.Message.ID + "|" + l.RequestID
		if l.Message.ID == "" {
			key = path + "|" + l.Timestamp.String()
		}
		// A reply is logged once per content block, each with the same
		// counts (the output count grows while it streams): keep the largest.
		if old, ok := r.replies[key]; ok && old.out >= u.Output {
			continue
		}
		r.replies[key] = reply{at: l.Timestamp, model: l.Message.Model, in: u.Input, out: u.Output, cacheWrite: u.CacheWrite, cacheR: u.CacheRead}
	}
}

func (rp reply) tokens() int64 { return rp.in + rp.out + rp.cacheWrite }

// family shortens a model ID to the name people use: "claude-opus-5-5" -> "Opus".
func family(model string) string {
	m := strings.ToLower(model)
	for _, f := range []string{"fable", "opus", "sonnet", "haiku"} {
		if strings.Contains(m, f) {
			return strings.ToUpper(f[:1]) + f[1:]
		}
	}
	return strings.TrimPrefix(m, "claude-")
}

// sum adds up the replies kept so far.
func (r *Reader) sum(now time.Time) Usage {
	u := Usage{Found: r.seen > 0, IdleS: -1, Limits: r.limits(now)}
	u.State, u.Doing, u.StateS = r.state(now)
	if u.State != "" {
		u.Agent = "claude"
	}
	// Codex too: the one that wrote last is the one being used.
	if st, doing, secs := r.codexState(now); st != "" && (u.State == "" || secs < u.StateS) {
		u.State, u.Doing, u.StateS, u.Agent = st, doing, secs, "codex"
	}
	if len(r.replies) == 0 {
		return u
	}
	all := make([]reply, 0, len(r.replies))
	for _, rp := range r.replies {
		all = append(all, rp)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].at.Before(all[j].at) })

	y, m, d := now.Date()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	weekAgo := now.Add(-7 * 24 * time.Hour)

	// Windows: one starts at the hour of a reply that comes after the previous
	// window has ended, and lasts five hours.
	var start, end time.Time
	var block []reply
	for _, rp := range all {
		if end.IsZero() || !rp.at.Before(end) {
			start = rp.at.Truncate(time.Hour)
			end = start.Add(window)
			block = block[:0]
		}
		block = append(block, rp)
		if !rp.at.Before(midnight) {
			u.Today += rp.tokens()
		}
		if rp.at.After(weekAgo) {
			u.Week += rp.tokens()
		}
	}
	last := all[len(all)-1].at
	u.IdleS = int64(now.Sub(last).Seconds())
	if u.IdleS < 0 {
		u.IdleS = 0
	}
	// The last window counts as running only if it has not ended yet.
	if now.Before(end) {
		u.Active = true
		u.Block.ResetS = int64(end.Sub(now).Seconds())
		u.Block.ElapseS = int64(now.Sub(start).Seconds())
		by := map[string]int64{}
		for _, rp := range block {
			u.Block.Tokens += rp.tokens()
			u.Block.Output += rp.out
			u.Block.Cached += rp.cacheR
			u.Block.Replies++
			by[family(rp.model)] += rp.tokens()
		}
		for n, t := range by {
			u.Models = append(u.Models, Model{Name: n, Tokens: t})
		}
		sort.Slice(u.Models, func(i, j int) bool {
			if u.Models[i].Tokens != u.Models[j].Tokens {
				return u.Models[i].Tokens > u.Models[j].Tokens
			}
			return u.Models[i].Name < u.Models[j].Name
		})
	}
	return u
}
