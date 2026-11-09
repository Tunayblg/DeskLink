// Package plan stores reminders with optional times and daily completion.
package plan

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	maxItems = 60
	maxTitle = 60
	dateForm = "2006-01-02"
)

// Item is one task. Time is wall clock "HH:MM" on the phone, or empty.
type Item struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Time   string `json:"time,omitempty"`    // when it is due, "HH:MM"; empty: any time that day
	Date   string `json:"date,omitempty"`    // "YYYY-MM-DD"; empty means every day
	DoneOn string `json:"done_on,omitempty"` // the last day it was ticked off, "YYYY-MM-DD"
}

// Store holds the tasks and saves them after every change.
type Store struct {
	mu    sync.Mutex
	path  string
	items []Item
}

// Open reads the plan at path. A missing file is an empty plan; a broken one
// is an error so it is not silently overwritten. Fields from older versions
// (an end time) are ignored.
func Open(path string) (*Store, error) {
	s := &Store{path: path}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &s.items); err != nil {
		return nil, err
	}
	return s, nil
}

// List returns a copy of the plan.
func (s *Store) List() []Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.copyLocked()
}

func (s *Store) copyLocked() []Item {
	out := make([]Item, len(s.items))
	copy(out, s.items)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].Time != "" && (out[j].Time == "" || out[i].Time < out[j].Time)
	})
	return out
}

// Add validates and stores a new task and returns the whole plan.
func (s *Store) Add(title, at, date string) ([]Item, error) {
	title = strings.TrimSpace(title)
	// A reminder needs either a name or a time; with a time alone, the clock is its name.
	if (title == "" && at == "") || utf8.RuneCountInString(title) > maxTitle {
		return nil, errors.New("a task needs a name or a time (name up to 60 characters)")
	}
	if at != "" && !validClock(at) {
		return nil, errors.New("time must be HH:MM")
	}
	if date != "" && !validDate(date) {
		return nil, errors.New("date must be YYYY-MM-DD")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.items) >= maxItems {
		return nil, errors.New("the plan is full")
	}
	s.items = append(s.items, Item{ID: newID(), Title: title, Time: at, Date: date})
	if err := s.saveLocked(); err != nil {
		s.items = s.items[:len(s.items)-1]
		return nil, err
	}
	return s.copyLocked(), nil
}

// Delete removes a task (an unknown ID is not an error) and returns the plan.
func (s *Store) Delete(id string) ([]Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.items
	kept := make([]Item, 0, len(old))
	for _, it := range old {
		if it.ID != id {
			kept = append(kept, it)
		}
	}
	if len(kept) == len(old) {
		return s.copyLocked(), nil
	}
	s.items = kept
	if err := s.saveLocked(); err != nil {
		s.items = old
		return nil, err
	}
	return s.copyLocked(), nil
}

// Toggle records completion using the phone's validated local day.
func (s *Store) Toggle(id, date string) ([]Item, error) {
	if !validDate(date) {
		return nil, errors.New("date must be YYYY-MM-DD")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.items {
		it := &s.items[i]
		if it.ID != id {
			continue
		}
		old := it.DoneOn
		if old == date || (it.Date != "" && old != "") {
			it.DoneOn = ""
		} else {
			it.DoneOn = date
		}
		if err := s.saveLocked(); err != nil {
			it.DoneOn = old
			return nil, err
		}
		return s.copyLocked(), nil
	}
	return nil, errors.New("unknown task ID")
}

func validDate(date string) bool {
	t, err := time.Parse("2006-01-02", date)
	return err == nil && t.Format("2006-01-02") == date
}

func (s *Store) saveLocked() error {
	b, err := json.MarshalIndent(s.items, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// validClock reports whether s is "HH:MM" on a 24-hour clock.
func validClock(s string) bool {
	if len(s) != 5 || s[2] != ':' {
		return false
	}
	t, err := time.Parse("15:04", s)
	return err == nil && t.Format("15:04") == s
}

func newID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}
