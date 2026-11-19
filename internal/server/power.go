package server

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/Tunayblg/DeskLink/internal/actions"
)

// maxPowerDelay is the longest a sleep or shut down can be put off from the phone.
const maxPowerDelay = 2 * time.Hour

// powerPlan is a sleep or shut down that is waiting for its time. The phone
// shows it as a small countdown (it can cancel it), and every phone sees it.
type powerPlan struct {
	Command string `json:"command"` // "sleep" or "shutdown"
	At      int64  `json:"at"`      // when it happens, Unix milliseconds
	In      int64  `json:"in"`      // milliseconds left when this was sent (the phone's clock may differ from the PC's)
}

// powerState holds at most one plan: a new one replaces the old one.
type powerState struct {
	mu    sync.Mutex
	plan  *powerPlan
	timer *time.Timer
}

// schedulePower puts off the sleep or shut down button id by delay. Only those
// two system commands can wait: a delayed press of anything else is refused.
func (s *Server) schedulePower(id string, delay time.Duration) error {
	cfg, _ := s.current()
	a, ok := cfg.Find(id)
	if !ok || a.Type != "system" || (a.Command != "sleep" && a.Command != "shutdown") {
		return errors.New("only sleep and shut down can wait")
	}
	if delay <= 0 || delay > maxPowerDelay {
		return errors.New("that delay is not allowed")
	}
	s.power.mu.Lock()
	if s.power.timer != nil {
		s.power.timer.Stop()
	}
	plan := &powerPlan{Command: a.Command, At: time.Now().Add(delay).UnixMilli()}
	s.power.plan = plan
	s.power.timer = time.AfterFunc(delay, func() { s.firePower(plan, a) })
	s.power.mu.Unlock()
	log.Printf("power: %s in %s", a.Command, delay.Round(time.Second))
	s.broadcast(s.powerMsg())
	return nil
}

// firePower runs the waiting action, unless it was cancelled or replaced.
func (s *Server) firePower(plan *powerPlan, a actions.Action) {
	s.power.mu.Lock()
	if s.power.plan != plan {
		s.power.mu.Unlock()
		return
	}
	s.power.plan, s.power.timer = nil, nil
	s.power.mu.Unlock()
	s.broadcast(s.powerMsg())
	log.Printf("power: running %s", a.Command)
	if err := s.run(a); err != nil {
		log.Printf("power: %s: %v", a.Command, err)
	}
}

// cancelPower drops the waiting action, if there is one.
func (s *Server) cancelPower() {
	s.power.mu.Lock()
	had := s.power.plan != nil
	if s.power.timer != nil {
		s.power.timer.Stop()
	}
	s.power.plan, s.power.timer = nil, nil
	s.power.mu.Unlock()
	if had {
		log.Printf("power: cancelled")
		s.broadcast(s.powerMsg())
	}
}

// powerMsg is what a phone is told: the waiting action, or nothing.
func (s *Server) powerMsg() serverMsg {
	s.power.mu.Lock()
	defer s.power.mu.Unlock()
	m := serverMsg{Type: "power"}
	if s.power.plan != nil {
		p := *s.power.plan
		p.In = max(0, p.At-time.Now().UnixMilli())
		m.Power = &p
	}
	return m
}
