package ui

import (
	"context"
	"fmt"
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
	// foundation is one of areas 7 to 13 opened from the overview, or 0.
	foundation int
	// filter is which tasks the overview lists: "", "active" or "landed".
	filter  string
	stale   atomic.Bool // the workspace is to be read again
	pending atomic.Pointer[loaded]
	// running is every change a live process is carrying now, wherever it
	// was started, as the watcher last saw it; watched says it watches.
	atWork  atomic.Pointer[map[string]bool]
	watched atomic.Bool
	// arbiters says, for the framework's own project, whether its arbiters
	// are being run against the framework as it is now: "" when not.
	arbiters atomic.Pointer[string]

	run    *flow.Run
	cancel context.CancelFunc
	live   *workspace.Change // what the run has written so far
	seen   int               // steps seen when live was last read

	area  int             // the node of the path opened under the graph
	nodeH int             // how tall the graph's nodes are drawn
	nextH int             // the tallest node drawn this frame
	item  *workspace.Item // the object open in the inspector
	// handedView is what one agent was handed, open in the panel beside the
	// task: a path inside the change's folder.
	handedView string
	open       map[string]bool // units whose every step is shown
	task       widget.Editor
	// specFolded folds the specification to its heading, to reach what is
	// under it; describing is a specification being written after the fact.
	specFolded  bool
	describing  atomic.Bool
	describeErr string
	// editing is the change whose description is open for its person to
	// change, docEdit the text, editLoaded the file it was read from.
	editing, editLoaded string
	docEdit             widget.Editor
	// closing is the change whose closing is being written, and why.
	closing  string
	closeWhy widget.Editor
	// settles are escalations the next task typed is meant to close.
	settles []string
	ground  widget.Editor

	selected string // the task on view: a change's folder, or runKey; "" is the newest
}

// closeTop closes at most one of the project's open layers, topmost first:
// the open object, then the open handover, then the open foundation page.
// It reports whether a layer was closed. The settings page is outside its
// authority: it is UI state, not the project's.
func (p *project) closeTop() bool {
	switch {
	case p.item != nil:
		p.item = nil
		return true
	case p.handedView != "":
		p.handedView = ""
		return true
	case p.foundation != 0:
		p.foundation = 0
		return true
	}
	return false
}

func newProject(path string) *project {
	p := &project{path: path, name: filepath.Base(path), area: 1, open: map[string]bool{}}
	p.task.Submit = true
	p.ground.SingleLine = false
	return p
}

// load reads the project's changes and asks the gate about them, drawing
// each answer as it arrives.
func (u *UI) load(p *project) { u.take(p, u.read(p)) }

// reload reads a project again beside the window, which takes what was read
// in the frame after it is ready: a frame never waits for a disk.
func (u *UI) reload(p *project) {
	go func() {
		p.pending.Store(u.read(p))
		u.invalidate()
	}()
}

// loaded is a project as read from disk, ready for the window to take.
type loaded struct {
	ws    *workspace.Workspace
	order []*workspace.Change
	led   *ledger.Ledger
}

func (u *UI) read(p *project) *loaded {
	ws, err := workspace.Open(u.model, filepath.Join(p.path, "changes"))
	if err != nil {
		ws, _ = workspace.Open(u.model)
	}
	led, _ := ledger.Read(p.path)
	return &loaded{ws, newestFirst(ws.Changes), led}
}

// watch looks at a project's changes once a second, beside the window: a
// run started anywhere — a terminal, another window — moves what is on
// disk, and the window reads it again; which changes are being carried now
// is kept for the frame to read without touching the disk.
func (u *UI) watch(p *project) {
	if p.watched.Swap(true) {
		return
	}
	go func() {
		var last string
		for {
			select {
			case <-u.ctx.Done():
				return
			case <-time.After(time.Second):
			}
			var sig strings.Builder
			running := map[string]bool{}
			dirs, _ := filepath.Glob(filepath.Join(p.path, "changes", "*"))
			for _, d := range dirs {
				entries, _ := os.ReadDir(d)
				for _, e := range entries {
					if info, err := e.Info(); err == nil {
						fmt.Fprintf(&sig, "%s/%s:%d:%d;", d, e.Name(), info.Size(), info.ModTime().UnixNano())
					}
				}
				if flow.Running(d) {
					running[d] = true
				}
			}
			p.atWork.Store(&running)
			if s := sig.String(); s != last {
				if last != "" {
					p.stale.Store(true)
				}
				last = s
				u.invalidate()
			}
		}
	}()
}

// verify runs the framework's own arbiters beside the window when the
// project is the framework and its code has moved since they last passed:
// R22 holds every landing until what judges the framework passes against it
// as it is now, and the window makes that run rather than only asking for
// it. One run at a time; the project is read again when it ends.
func (u *UI) verify(p *project) {
	home := gate.Home()
	if home != p.path || !fileExists(filepath.Join(p.path, "internal", "gate", "gate.go")) {
		return
	}
	if _, ok := gate.Verified(home)[gate.ArbitersKey(home)]; ok {
		return
	}
	running := "the framework's own tests are running against it as it is now (R22)"
	if !p.arbiters.CompareAndSwap(nil, &running) {
		return
	}
	u.invalidate()
	go func() {
		run, err := gate.Arbiters(u.ctx, home)
		switch {
		case err != nil:
			msg := "the framework's tests could not be run: " + err.Error()
			p.arbiters.Store(&msg)
		case !run.Passed:
			msg := "the framework's own tests fail against it as it is now, so R22 holds its landings"
			p.arbiters.Store(&msg)
		default:
			p.arbiters.Store(nil)
		}
		p.stale.Store(true)
		u.invalidate()
	}()
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

// at says whether a live process is carrying the change in dir now.
func (p *project) at(dir string) bool {
	m := p.atWork.Load()
	return m != nil && (*m)[dir]
}

// take puts what was read in place, on the window's goroutine, and asks the
// gate and the estate about it beside the window.
func (u *UI) take(p *project, l *loaded) {
	u.watch(p)
	u.verify(p)
	p.ws, p.order, p.led = l.ws, l.order, l.led
	u.docs.bump()
	if u.flowConfig != nil {
		go func() {
			p.estate.Store(readEstate(u.flowConfig.Estate, p.path))
			u.invalidate()
		}()
	} else {
		p.estate.Store(&estateFacts{err: "no estate model is configured"})
	}
	p.checked.Store(0)
	ws := l.ws
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
	run.Settles(p.settles...)
	p.settles = nil
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
	// running is a change being carried now by a run this window did not
	// start: from a terminal, or another window.
	running bool
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
			s := shown{c: c, costs: c.Costs, running: p.at(c.Change.Dir)}
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
	if s.c != nil && !(s.live && !s.v.Finished) {
		if closed, _ := s.c.Closed(); closed {
			return "closed"
		}
	}
	if s.live {
		return p.state()
	}
	if s.c != nil && s.running {
		return "working"
	}
	// How a task stands is how far it went: landed, or not. What the gate
	// says of it now — by rules that may be newer than the task — is shown
	// beside it, and does not unmake a landing.
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

// close records that a person closed a change that will not be finished,
// and why — appended to its record, never deleting anything — and reads
// the project again.
func (u *UI) close(p *project, c *workspace.Change, why string) error {
	why = strings.TrimSpace(why)
	if why == "" {
		why = "no longer needed"
	}
	intent := c.Name()
	for _, it := range c.Items {
		if it.Kind == "Intent" {
			intent = it.ID
		}
	}
	by := "a person"
	if u.flowConfig != nil {
		by = u.flowConfig.Person
	}
	err := artifact.Record{Name: c.Name(), Dir: c.Change.Dir}.Append(artifact.Crossing{
		At: time.Now().Format("2006-01-02"), Stage: workspace.ClosedStage, Object: intent, Note: "closed by " + by + ": " + why})
	if err != nil {
		return err
	}
	if p.run != nil {
		if v := p.run.View(); v.Finished && v.Dir == c.Change.Dir {
			p.run, p.live = nil, nil
		}
	}
	p.closing, p.selected, p.item = "", "", nil
	p.closeWhy.SetText("")
	p.stale.Store(true)
	u.invalidate()
	return nil
}
