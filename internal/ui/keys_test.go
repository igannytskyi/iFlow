package ui

import (
	"path/filepath"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/igannytskyi/iflow/internal/gate"
)

// A shortcut pressed on the keyboard reaches the dispatcher through the
// window's router, modifiers and all — not only when called by hand.
func TestShortcutsArriveThroughTheRouter(t *testing.T) {
	greeting := filepath.Join(gate.Home(), "examples", "greeting")
	u := testUI(t, nil, greeting, t.TempDir())
	var r input.Router
	frame := func() {
		var ops op.Ops
		gtx := layout.Context{Ops: &ops, Source: r.Source()}
		u.keys(gtx)
		r.Frame(&ops)
	}
	frame()
	r.Queue(key.Event{Name: "N", Modifiers: key.ModShortcut, State: key.Press})
	frame()
	if u.cur.selected != newKey {
		t.Errorf("Cmd/Ctrl+N did not open a new intent: selected %q", u.cur.selected)
	}
	r.Queue(key.Event{Name: "2", Modifiers: key.ModShortcut, State: key.Press})
	frame()
	if u.cur != u.projects[1] {
		t.Error("Cmd/Ctrl+2 did not choose the second project")
	}
	u.preferring = true
	r.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	frame()
	if u.preferring {
		t.Error("Escape did not close the settings")
	}
}
