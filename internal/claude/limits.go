package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// LimitsFile is where Claude Code's status line command saves the plan limits
// (see docs/CLAUDE.md). It sits next to the projects folder.
const LimitsFile = "desklink-rate-limits.json"

// Limits are the plan limits Claude Code reports: how much of the five-hour
// and the weekly allowance is used. They exist only for Pro, Max and Ultra
// plans, and only while Claude Code is open and has had a reply.
type Limits struct {
	FiveHour *Window `json:"five_hour,omitempty"`
	SevenDay *Window `json:"seven_day,omitempty"`
	AgeS     int64   `json:"age_s"` // seconds since Claude Code last reported them
}

// Window is one limit.
type Window struct {
	Pct    float64 `json:"pct"`     // percent used, 0-100
	ResetS int64   `json:"reset_s"` // seconds until it resets
}

type savedWindow struct {
	Used    float64 `json:"used_percentage"`
	ResetAt int64   `json:"resets_at"`
}

// limits reads the saved file. It returns nil when there is none yet. A window
// whose reset time has passed is shown as unused: the next reply starts a new one.
func (r *Reader) limits(now time.Time) *Limits {
	path := filepath.Join(filepath.Dir(r.dir()), LimitsFile)
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var saved struct {
		FiveHour *savedWindow `json:"five_hour"`
		SevenDay *savedWindow `json:"seven_day"`
	}
	if json.Unmarshal(b, &saved) != nil || (saved.FiveHour == nil && saved.SevenDay == nil) {
		return nil
	}
	conv := func(s *savedWindow) *Window {
		if s == nil {
			return nil
		}
		left := s.ResetAt - now.Unix()
		if left <= 0 {
			return &Window{}
		}
		return &Window{Pct: min(100, max(0, s.Used)), ResetS: left}
	}
	age := int64(now.Sub(info.ModTime()).Seconds())
	return &Limits{FiveHour: conv(saved.FiveHour), SevenDay: conv(saved.SevenDay), AgeS: max(0, age)}
}
