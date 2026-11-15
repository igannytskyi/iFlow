package ui

import (
	"context"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/io/key"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/gate"
	"github.com/igannytskyi/iflow/internal/method"
	"github.com/igannytskyi/iflow/internal/workspace"
)

// UI is the whole interface and everything it remembers between frames.
type UI struct {
	win      *app.Window
	ctx      context.Context
	parallel int
	th       *material.Theme
	p        palette
	dark     bool
	model    *method.Model
	gate     *gate.Gate

	flowConfig *flow.Config
	rebuild    func() (flow.Config, string)
	modelsNote string
	settings   *settings
	projects   []*project
	cur        *project
	preferring bool // the settings are open in place of a project

	adding widget.Editor // a project's path, typed when no folder chooser can be called
	typing bool          // the path field is shown
	chosen chan chosen   // what the system's folder chooser answered

	clicks clicks
	lists  lists
	docs   docs
	splits map[string]*split
}

// Options says what to show and what to work with.
type Options struct {
	Model *method.Model
	Gate  *gate.Gate
	// Parallel is how many changes the gate is asked about at once.
	Parallel int
	// Projects are opened beside those the workbench remembers; the first
	// named is the one shown.
	Projects []string
	// Examples are opened when no project is named and none is remembered,
	// so that a first start has something to show.
	Examples []string
	// Flow is what a task is carried out with; nil leaves the workbench
	// able to show but not to run.
	Flow *flow.Config
	// ModelsProblem is what is wrong with the choice of models, if
	// anything; Rebuild makes Flow again after the choice changes.
	ModelsProblem string
	Rebuild       func() (flow.Config, string)
}

func newUI(ctx context.Context, o Options, s *settings) *UI {
	u := &UI{model: o.Model, gate: o.Gate, flowConfig: o.Flow, rebuild: o.Rebuild, modelsNote: o.ModelsProblem, settings: s, ctx: ctx, parallel: max(o.Parallel, 1),
		clicks: clicks{}, lists: lists{}, docs: docs{}, splits: map[string]*split{}}
	u.adding.SingleLine, u.adding.Submit = true, true
	u.chosen = make(chan chosen, 1)
	u.setTheme(s.Dark)
	named := o.Projects
	if len(named) == 0 && len(s.Projects) == 0 {
		named = o.Examples
	}
	for _, p := range named {
		if abs, err := filepath.Abs(p); err == nil {
			s.add(abs)
		}
	}
	open := s.Open
	if len(o.Projects) > 0 {
		open, _ = filepath.Abs(o.Projects[0])
	}
	for _, path := range s.Projects {
		p := newProject(path)
		u.projects = append(u.projects, p)
		u.load(p)
		if path == open || u.cur == nil {
			u.cur = p
		}
	}
	s.save()
	return u
}

// Run opens the window and returns when it is closed.
func Run(ctx context.Context, o Options) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	u := newUI(ctx, o, loadSettings())
	u.win = new(app.Window)
	u.win.Option(app.Title("iFlow"), app.Size(unit.Dp(1440), unit.Dp(900)), app.MinSize(unit.Dp(1000), unit.Dp(620)))
	// A signal to stop closes the window, which ends the loop below.
	go func() {
		<-ctx.Done()
		u.win.Perform(system.ActionClose)
	}()
	var ops op.Ops
	for {
		switch e := u.win.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			u.frame(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

// frame draws one frame. While any task runs, the next frame is asked for a
// second later, so that clocks move even when nothing else does.
func (u *UI) frame(gtx C) {
	u.keys(gtx)
	for _, p := range u.projects {
		if p.stale.Swap(false) {
			u.load(p)
		}
		if p.running() {
			gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(time.Second)})
		}
	}
	if u.cur != nil && u.cur.ws != nil {
		u.cur.ws.RLock()
		defer u.cur.ws.RUnlock()
	}
	u.layout(gtx)
}

func (u *UI) setTheme(dark bool) {
	u.dark = dark
	u.p = lightPalette
	if dark {
		u.p = darkPalette
	}
	u.th = newTheme(u.p)
}

// keys: Escape closes what was opened last.
func (u *UI) keys(gtx C) {
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameEscape})
		if !ok {
			return
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press && u.cur != nil {
			u.cur.item = nil
		}
	}
}

func (u *UI) layout(gtx C) D {
	fill(gtx, u.p.Bg)
	return u.splitOf("projects", 250, 190, 420).layout(gtx, u, u.sidebar, u.main, false)
}

// sidebar is every project and how each stands, a way to add one, and the
// way to the settings.
func (u *UI) sidebar(gtx C) D {
	fill(gtx, u.p.Surface)
	select {
	case c := <-u.chosen:
		if c.unavailable {
			u.typing = true
		} else {
			u.addProject(c.path)
		}
	default:
	}
	for {
		e, ok := u.adding.Update(gtx)
		if !ok {
			break
		}
		if s, ok := e.(widget.SubmitEvent); ok {
			u.addProject(s.Text)
		}
	}
	plus, prefs := u.clicks.of("project/add"), u.clicks.of("settings")
	if plus.Clicked(gtx) {
		u.choose()
	}
	if prefs.Clicked(gtx) {
		u.preferring = !u.preferring
	}
	var rows []W
	for _, p := range u.projects {
		rows = append(rows, u.projectRow(p))
		if p == u.cur && !u.preferring {
			rows = append(rows, u.taskRows(p)...)
		}
	}
	l := u.lists.of("projects")
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(inset(gutter-4, pad, column(2,
			u.strong(sizeHead, u.p.Text, "iFlow"),
			u.text(sizeSmall, u.p.Muted, "acceptance of change on evidence")))),
		layout.Rigid(inset(4, pad, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, u.text(sizeSmall, u.p.Faint, "PROJECTS")),
				layout.Rigid(func(gtx C) D {
					return u.pressable(gtx, plus, u.typing, 6, inset(0, 9, u.strong(sizeHead, u.p.Accent, "+")))
				}),
			)
		})),
		layout.Rigid(func(gtx C) D {
			if !u.typing {
				return D{}
			}
			return inset(4, pad, box(u.p.Bg, u.p.Accent, 6, 8, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				ed := material.Editor(u.th, &u.adding, "path to a project, then Enter")
				ed.TextSize = sizeSmall
				return ed.Layout(gtx)
			}))(gtx)
		}),
		layout.Flexed(1, func(gtx C) D {
			return material.List(u.th, l).Layout(gtx, len(rows), func(gtx C, i int) D {
				return layout.Inset{Left: 8, Right: 8, Bottom: 2}.Layout(gtx, rows[i])
			})
		}),
		layout.Rigid(inset(pad, 8, func(gtx C) D {
			return u.pressable(gtx, prefs, u.preferring, 6, stretch(inset(7, 10, u.text(sizeBody, u.p.Muted, "Settings"))))
		})),
	)
}

// choose asks the system for a folder to add; where it cannot, a path field
// opens instead.
func (u *UI) choose() {
	if u.typing {
		u.typing = false
		return
	}
	go func() {
		path, err := pickFolder(u.ctx, "Choose the folder of a project")
		if err != nil || path != "" {
			u.chosen <- chosen{path: path, unavailable: err != nil}
			u.invalidate()
		}
	}()
}

// chosen is what the folder chooser answered: a folder, or that there is no
// chooser to ask.
type chosen struct {
	path        string
	unavailable bool
}

func (u *UI) addProject(path string) {
	path = strings.TrimSpace(path)
	if strings.HasPrefix(path, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(h, path[2:])
		}
	}
	abs, err := filepath.Abs(path)
	if path == "" || err != nil {
		return
	}
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		return
	}
	u.adding.SetText("")
	u.typing = false
	if i := slices.IndexFunc(u.projects, func(p *project) bool { return p.path == abs }); i >= 0 {
		u.select_(u.projects[i])
		return
	}
	p := newProject(abs)
	u.projects = append(u.projects, p)
	u.load(p)
	u.settings.add(abs)
	u.select_(p)
}

func (u *UI) select_(p *project) {
	u.cur, u.preferring = p, false
	u.settings.Open = p.path
	u.settings.save()
}

// stateColor is the colour a project's state is shown in.
func (u *UI) stateColor(s string) colorNRGBA {
	switch s {
	case "working":
		return u.p.Accent
	case "waiting for you":
		return u.p.Note
	case "stopped":
		return u.p.Fail
	case "landed":
		return u.p.OK
	case "closed":
		return u.p.Muted
	}
	return u.p.Faint
}

func (u *UI) projectRow(p *project) W {
	return func(gtx C) D {
		btn := u.clicks.of("project/" + p.path)
		if btn.Clicked(gtx) {
			u.select_(p)
			p.selected, p.item = "", nil
		}
		state := p.state()
		line := state
		if v, ok := p.view(); ok {
			if !v.Spent.Zero() {
				line += " · " + money(v.Spent.CostUSD)
			}
			if !v.Finished && len(v.Steps) > 0 {
				line += " · " + elapsed(time.Since(v.Steps[0].At).Milliseconds())
			}
		} else if p.ws != nil {
			line = plural(len(p.ws.Changes), "task")
		}
		return u.pressable(gtx, btn, u.cur == p && !u.preferring, 6, stretch(inset(7, 10, column(3,
			row(8, dot(u.stateColor(state), 8), u.strong(sizeBody, u.p.Text, p.name)),
			u.oneLine(sizeSmall, u.stateColor(state), line),
		))))
	}
}

// taskRows are what a project offers under its name: a new intent, the
// tasks still active — running, waiting, stopped — and, folded away until
// asked for, those that landed.
func (u *UI) taskRows(p *project) []W {
	out := []W{u.newIntentRow(p)}
	runDir := ""
	if p.run != nil {
		v := p.run.View()
		runDir = v.Dir
		out = append(out, u.taskRow(p, runKey, v.Statement, p.state(), money(v.Spent.CostUSD), nil))
	}
	var done []W
	for _, c := range p.order {
		if c.Change.Dir == runDir {
			continue
		}
		s := shown{c: c}
		for n := 1; n <= 6; n++ {
			s.status[n] = statusOf(c, n)
		}
		spent := ""
		if len(c.Costs) > 0 {
			spent = money(c.Spent.CostUSD)
		}
		row := u.taskRow(p, c.Change.Dir, c.Name(), s.state(p), spent, c)
		if st := s.state(p); st == "landed" || st == "closed" {
			done = append(done, row)
		} else {
			out = append(out, row)
		}
	}
	if len(done) > 0 {
		out = append(out, u.foldRow(p, len(done)))
		if p.showDone || u.settings.ShowLanded {
			out = append(out, done...)
		}
	}
	return out
}

func (u *UI) newIntentRow(p *project) W {
	return func(gtx C) D {
		btn := u.clicks.of("task/new/" + p.path)
		if btn.Clicked(gtx) {
			p.selected, p.item = newKey, nil
			gtx.Execute(key.FocusCmd{Tag: &p.task})
		}
		return layout.Inset{Left: 18}.Layout(gtx, func(gtx C) D {
			return u.pressable(gtx, btn, p.selected == newKey, 5, stretch(inset(5, 8, u.text(sizeSmall, u.p.Accent, "+ New intent"))))
		})
	}
}

func (u *UI) foldRow(p *project, n int) W {
	return func(gtx C) D {
		btn := u.clicks.of("task/fold/" + p.path)
		if btn.Clicked(gtx) {
			p.showDone = !p.showDone
		}
		label := "▸ Done (" + strconv.Itoa(n) + ")"
		if p.showDone || u.settings.ShowLanded {
			label = "▾ Done (" + strconv.Itoa(n) + ")"
		}
		return layout.Inset{Left: 18}.Layout(gtx, func(gtx C) D {
			return u.pressable(gtx, btn, false, 5, stretch(inset(5, 8, u.text(sizeSmall, u.p.Muted, label))))
		})
	}
}

func (u *UI) taskRow(p *project, key, title, state, spent string, c *workspace.Change) W {
	return func(gtx C) D {
		btn := u.clicks.of("task/" + p.path + "/" + key)
		if btn.Clicked(gtx) {
			p.show(key, c)
		}
		on := p.selected == key
		line := func(gtx C) D {
			return u.pressable(gtx, btn, on, 5, stretch(inset(5, 8, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle, Gap: gtx.Dp(8)}.Layout(gtx,
					layout.Rigid(dot(u.stateColor(state), 7)),
					layout.Flexed(1, u.oneLine(sizeSmall, u.p.Text, title)),
					layout.Rigid(u.oneLine(sizeSmall, u.p.Faint, spent)),
				)
			})))
		}
		// A task that stopped and will not be finished is closed from where
		// it is seen: the cross opens it with the form to close it.
		if state != "stopped" || c == nil {
			return layout.Inset{Left: 18}.Layout(gtx, line)
		}
		closer := u.clicks.of("task/close/" + c.Change.Dir)
		if closer.Clicked(gtx) {
			p.show(key, c)
			p.closing = c.Change.Dir
		}
		return layout.Inset{Left: 18}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle, Gap: gtx.Dp(2)}.Layout(gtx,
				layout.Flexed(1, line),
				layout.Rigid(func(gtx C) D {
					return u.pressable(gtx, closer, p.closing == c.Change.Dir, 5, inset(4, 7, u.text(sizeSmall, u.p.Muted, "close")))
				}))
		})
	}
}

// main is the open project: the task on view, with the box for the next
// task under it, and beside it what the task and the project have cost.
func (u *UI) main(gtx C) D {
	if u.preferring {
		return u.settingsView(gtx)
	}
	p := u.cur
	if p == nil {
		return inset(gutter, gutter, u.text(sizeBody, u.p.Muted, "Add a project with + on the left: the folder of a repository you want to work on."))(gtx)
	}
	switch p.selected {
	case "":
		return u.projectView(gtx, p)
	case newKey:
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Flexed(1, u.newIntentView(p)),
			layout.Rigid(u.hairline),
			layout.Rigid(inset(pad, gutter, u.composer(p))),
		)
	}
	s, ok := p.shownOf()
	if !ok {
		p.selected = ""
		return u.projectView(gtx, p)
	}
	center := func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D { return u.taskView(gtx, p, s, ok) }),
			layout.Rigid(u.hairline),
			layout.Rigid(inset(pad, gutter, u.composer(p))),
		)
	}
	if u.settings.HideCosts {
		return layout.Flex{}.Layout(gtx, layout.Flexed(1, center), layout.Rigid(u.costsStrip))
	}
	if u.settings.CostsLeft {
		return u.splitOf("stats/left", 380, 300, 640).layout(gtx, u, u.statsPanel(p, s, ok), center, false)
	}
	return u.splitOf("stats", 380, 300, 640).layout(gtx, u, center, u.statsPanel(p, s, ok), true)
}

// invalidate asks for a new frame, when there is a window to draw it in.
func (u *UI) invalidate() {
	if u.win != nil {
		u.win.Invalidate()
	}
}

// Main hands the main thread to the windowing system, which some platforms
// require, and does not return. Run is called on another goroutine.
func Main() { app.Main() }

// costsStrip is the panel of costs folded to its edge; a click unfolds it.
func (u *UI) costsStrip(gtx C) D {
	btn := u.clicks.of("costs/show")
	if btn.Clicked(gtx) {
		u.settings.HideCosts = false
		u.settings.save()
	}
	return btn.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min = image.Pt(gtx.Dp(30), gtx.Constraints.Max.Y)
		gtx.Constraints.Max.X = gtx.Constraints.Min.X
		fill(gtx, u.p.Surface)
		return layout.N.Layout(gtx, inset(14, 0, u.text(sizeSmall, u.p.Accent, "$ ‹")))
	})
}
