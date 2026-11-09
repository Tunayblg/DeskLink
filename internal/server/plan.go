package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/Tunayblg/DeskLink/internal/plan"
)

// SetPlan gives the server the day plan to show on phones. Without one the
// phones' plan page stays empty and edits are ignored.
func (s *Server) SetPlan(p *plan.Store) { s.plan = p }

// planMsg is the "plan" message: the whole plan, sent when a phone connects
// and to every phone after each change.
func (s *Server) planMsg() (serverMsg, bool) {
	if s.plan == nil {
		return serverMsg{}, false
	}
	return planMsgOf(s.plan.List()), true
}

func planMsgOf(items []plan.Item) serverMsg {
	return serverMsg{Type: "plan", Plan: &items}
}

// editPlan applies a "planadd", "plantoggle" or "plandel" message and sends the new plan to
// every phone. A refused edit goes back to the sender as an error toast.
func (s *Server) editPlan(ctx context.Context, c *websocket.Conn, msg clientMsg) {
	if s.plan == nil {
		return
	}
	var items []plan.Item
	var err error
	switch msg.Type {
	case "planadd":
		items, err = s.plan.Add(msg.Title, msg.Time, msg.Date)
	case "plantoggle":
		items, err = s.plan.Toggle(msg.ID, msg.Date)
	case "plandel":
		items, err = s.plan.Delete(msg.ID)
	}
	if err != nil {
		log.Printf("plan: %v", err)
		wsjson.Write(ctx, c, serverMsg{Type: "planerr", Error: err.Error()})
		return
	}
	s.broadcast(planMsgOf(items))
}

// broadcast sends a message to every connected phone.
func (s *Server) broadcast(msg serverMsg) {
	s.sessMu.Lock()
	defer s.sessMu.Unlock()
	for ss := range s.sessions {
		go func(ss *session) {
			ctx, cancel := context.WithTimeout(ss.ctx, 5*time.Second)
			defer cancel()
			wsjson.Write(ctx, ss.conn, msg)
		}(ss)
	}
}

// planAdmin is the body of POST /admin/api/plan: the same edits a phone can
// make, from the PC's admin page.
type planAdmin struct {
	Action string `json:"action"` // add, toggle or delete
	ID     string `json:"id"`
	Title  string `json:"title"`
	Time   string `json:"time"`
	Date   string `json:"date"`
}

func (s *Server) handlePlanGet(w http.ResponseWriter, r *http.Request) {
	items := []plan.Item{}
	if s.plan != nil {
		items = s.plan.List()
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handlePlanEdit(w http.ResponseWriter, r *http.Request) {
	if s.plan == nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "The plan is not available."})
		return
	}
	var req planAdmin
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Bad request."})
		return
	}
	var items []plan.Item
	var err error
	switch req.Action {
	case "add":
		items, err = s.plan.Add(req.Title, req.Time, req.Date)
	case "toggle":
		items, err = s.plan.Toggle(req.ID, time.Now().Format("2006-01-02"))
	case "delete":
		items, err = s.plan.Delete(req.ID)
	default:
		err = errors.New("unknown action")
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	s.broadcast(planMsgOf(items))
	writeJSON(w, http.StatusOK, items)
}
