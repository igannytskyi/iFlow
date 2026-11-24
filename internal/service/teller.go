package service

import (
	"sync"

	"github.com/igannytskyi/iflow/internal/protocol"
)

// Teller is what a screen tells the person of their runs, wherever it
// tells it — a terminal's notification, the system's: that a run waits for
// them, landed or stopped, once each. It is safe for use from the client's
// goroutine and the screen's at once.
type Teller struct {
	mu   sync.Mutex
	told map[string]string
}

// News is what the person is to be told of run as it now stands, or ""
// where it is nothing, or what they were told of it last.
func (t *Teller) News(run string, s State) string {
	var what string
	switch {
	case s.Question != "":
		what = "waiting for you: " + s.Question
	case s.Finished && s.Areas[6] == protocol.Done:
		what = "landed"
	case s.Finished:
		what = "stopped"
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if what == "" || t.told[run] == what {
		return ""
	}
	if t.told == nil {
		t.told = map[string]string{}
	}
	t.told[run] = what
	return what
}
