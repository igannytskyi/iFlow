package ui

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"gioui.org/widget"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/gate"
	"github.com/igannytskyi/iflow/internal/ledger"
	"github.com/igannytskyi/iflow/internal/workspace"
)

// What a project can have on view besides one of its changes: its run,
// whatever folder that ends up writing to, or a new intent being written.
// Nothing selected is the project's overview.
const (
	runKey = "\x00run"
	newKey = "\x00new"
)

// project is one estate the workbench works on, and everything it remembers
// about it. Each has its own run, which goes on while another is looked at.
type project struct {
	path, name string

	ws      *workspace.Workspace
	order   []*workspace.Change // the changes, newest first
	led     *ledger.Ledger
	checked atomic.Int64
	estate  atomic.Pointer[estateFacts]
	// showDone shows the tasks that landed, which are folded away.
	showDone bool
	stale    atomic.Bool // the workspace is to be read again

	run    *flow.Run
	cancel context.CancelFunc
	live   *workspace.Change // what the run has written so far
	seen   int               // steps seen when live was last read

	area   int             // the node of the path opened under the graph
	nodeH  int             // how tall the graph's nodes are drawn
	nextH  int             // the tallest node drawn this frame
	item   *workspace.Item // the object open in the inspector
	open   map[string]bool // units whose every step is shown
	task   widget.Editor
	ground widget.Editor

	selected string // the task on view: a change's folder, or runKey; "" is the newest
}

func newProject(path string) *project {
	p := &project{path: path, name: filepath.Base(path), area: 1, open: map[string]bool{}}
	p.task.Submit = true
	p.ground.SingleLine = true
	return p
}

// load reads the project's changes and asks the gate about them, drawing
// each answer as it arrives.
func (u *UI) load(p *project) {
	ws, err := workspace.Open(u.model, filepath.Join(p.path, "changes"))
	if err != nil {
		ws, _ = workspace.Open(u.model)
	}
	p.ws = ws
	p.order = newestFirst(ws.Changes)
	p.led, _ = ledger.Read(p.path)
	if u.flowConfig != nil {
		go func() {
			p.estate.Store(readEstate(u.flowConfig.Estate, p.path))
			u.invalidate()
		}()
	} else {
		p.estate.Store(&estateFacts{err: "no estate model is configured"})
	}
	p.checked.Store(0)
	go ws.Check(u.ctx, u.gate, u.parallel, func(*workspace.Change) {
		p.checked.Add(1)
		u.invalidate()
	})
}

// view is the project's run as it stands, or nothing.
func (p *project) view() (flow.View, bool) {
	if p.run == nil {
		return flow.View{}, false
	}
	return p.run.View(), true
}

// running reports whether a task is under way and not finished.
func (p *project) running() bool {
	v, ok := p.view()
	return ok && !v.Finished
}

// state is how the project stands, in one word, for the list of projects.
func (p *project) state() string {
	v, ok := p.view()
	switch {
	case !ok:
		return "idle"
	case v.Question != "":
		return "waiting for you"
	case !v.Finished:
		return "working"
	case v.Areas[6] == flow.Done:
		return "landed"
	}
	return "stopped"
}

// start carries a task along the path in the background.
func (u *UI) start(p *project, statement string) {
	statement = strings.TrimSpace(statement)
	if statement == "" || u.flowConfig == nil || p.running() {
		return
	}
	ctx, cancel := context.WithCancel(u.ctx)
	p.cancel = cancel
	run := flow.New(*u.flowConfig, p.path, statement, u.invalidate)
	p.run, p.live, p.seen, p.item, p.area, p.selected = run, nil, 0, nil, 1, runKey
	go func() {
		_ = run.Start(ctx)
		p.stale.Store(true)
		u.invalidate()
	}()
}

// refresh rereads what the run has written when the run has moved, and
// follows the run to the area it is in.
func (p *project) refresh(v flow.View) {
	if v.Dir == "" || (len(v.Steps) == p.seen && p.live != nil) {
		return
	}
	following := p.area == current(v, p.seen)
	p.seen = len(v.Steps)
	if c, err := artifact.Load(v.Dir); err == nil {
		p.live = workspace.Build(c)
		if p.item != nil {
			if it := p.live.Item(p.item.ID); it != nil && it.Kind == p.item.Kind {
				p.item = it
			}
		}
	}
	if following {
		p.area = current(v, len(v.Steps))
	}
}

// current is the area the run is in: the last one working, waiting or
// stopped, or the last one done.
func current(v flow.View, _ int) int {
	n := 1
	for i := 1; i <= 6; i++ {
		switch v.Areas[i] {
		case flow.Working, flow.Waiting, flow.Failed:
			return i
		case flow.Done:
			n = min(i+1, 6)
		}
	}
	return n
}

// newestFirst orders changes by when their folders last changed.
func newestFirst(cs []*workspace.Change) []*workspace.Change {
	at := make(map[*workspace.Change]time.Time, len(cs))
	for _, c := range cs {
		if st, err := os.Stat(c.Change.Dir); err == nil {
			at[c] = st.ModTime()
		}
	}
	out := slices.Clone(cs)
	slices.SortStableFunc(out, func(a, b *workspace.Change) int { return at[b].Compare(at[a]) })
	return out
}

// shown is the task on view, whether a run of this session or a change
// carried before: what it wrote, how each area stands, what it cost.
type shown struct {
	c      *workspace.Change // nil while a run has written nothing yet
	v      flow.View         // the run, when live
	live   bool
	status [7]flow.Status
	costs  []artifact.Cost
}

// shownOf is the task a project has on view, if it has any.
func (p *project) shownOf() (shown, bool) {
	if p.run != nil {
		v := p.run.View()
		if p.selected == runKey || (v.Dir != "" && p.selected == v.Dir) {
			p.refresh(v)
			s := shown{c: p.live, v: v, live: true, status: v.Areas, costs: v.Costs}
			if len(s.costs) == 0 && p.live != nil {
				s.costs = p.live.Costs
			}
			return s, true
		}
	}
	for _, c := range p.order {
		if c.Change.Dir == p.selected {
			s := shown{c: c, costs: c.Costs}
			for n := 1; n <= 6; n++ {
				s.status[n] = statusOf(c, n)
			}
			return s, true
		}
	}
	return shown{}, false
}

// statusOf is how an area of a change carried before stands.
func statusOf(c *workspace.Change, n int) flow.Status {
	if n > c.Reached() {
		return flow.Idle
	}
	for _, it := range c.InArea(n) {
		for _, f := range it.Findings {
			if f.Severity == gate.Fail {
				return flow.Failed
			}
		}
	}
	return flow.Done
}

// title is what the task was for, in a line.
func (s shown) title() string {
	if s.live && s.v.Statement != "" {
		return s.v.Statement
	}
	if s.c != nil {
		if t := intentOf(s.c); t != "" {
			return t
		}
		return s.c.Name()
	}
	return ""
}

// state is how the task stands, in a word or two.
func (s shown) state(p *project) string {
	if s.live {
		return p.state()
	}
	for n := 1; n <= 6; n++ {
		if s.status[n] == flow.Failed {
			return "stopped"
		}
	}
	if s.c.Reached() == 6 {
		return "landed"
	}
	return "stopped"
}

// show puts a task on view, at the area it got to.
func (p *project) show(sel string, c *workspace.Change) {
	p.selected, p.item = sel, nil
	p.area = 1
	if c != nil {
		p.area = max(1, min(c.Reached(), 6))
		for n := 1; n <= 6; n++ {
			if statusOf(c, n) == flow.Failed {
				p.area = n
				break
			}
		}
	}
}
