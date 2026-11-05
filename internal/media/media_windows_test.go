package media

import "testing"

// A broken Windows audio or media object must not crash DeskLink: the request
// is answered with nothing and the objects are looked up afresh.
func TestWorkerSurvivesBrokenObjects(t *testing.T) {
	// A COM pointer without a method table: calling anything on it faults, which
	// is what a half-initialized audio service can look like.
	broken := &com{}
	for name, req := range map[string]request{
		"reading": {prevArt: map[string]string{}},
		"mixer":   {mixer: true},
		"command": {cmd: &command{kind: "volume", level: 50}},
	} {
		w := &worker{enumerator: broken, manager: broken, trackKeys: map[string]string{}, procs: map[uint32]proc{}, names: map[string]string{}, unmuted: map[string]float32{}}
		req.reply = make(chan reply, 1)
		w.handle(req) // must not panic
		select {
		case <-req.reply:
		default:
			t.Errorf("%s: the request was never answered", name)
		}
		if w.enumerator != nil || w.manager != nil {
			t.Errorf("%s: broken objects must be dropped", name)
		}
	}
}
