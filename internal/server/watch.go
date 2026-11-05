package server

import (
	"context"
	"log"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/Tunayblg/DeskLink/internal/actions"
)

// Watch mode (the phone's "series" screen): play/pause and jump back and forward.
// Players listen to keys, not to an API, so the PC is sent the
// same keys a keyboard would. The phone can only pick from this list.
var watchKeys = map[string][]string{
	"playpause": {"media_play_pause"}, // the media key: any player, any window
	"back":      {"left"},             // YouTube, Netflix, VLC, Prime and most players jump a few seconds
	"forward":   {"right"},
}

// watch presses the keys for one of the watch buttons. Unknown names do nothing.
func (s *Server) watch(action string) {
	keys, ok := watchKeys[action]
	if !ok {
		return
	}
	if err := s.run(actions.Action{Type: "keys", Keys: keys}); err != nil {
		log.Printf("watch %s: %v", action, err)
	}
}

// answerBrightness sends the screen's brightness to the phone. Without a
// "brightness" field the phone knows this screen cannot be controlled.
func (s *Server) answerBrightness(ctx context.Context, c *websocket.Conn) error {
	reply := serverMsg{Type: "brightness"}
	if l, ok := s.getBrightness(); ok {
		reply.Brightness = &l
	}
	return wsjson.Write(ctx, c, reply)
}
