package ui

import (
	"fmt"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/workspace"
)

// taskView is the task on view: what it was for, the path it took as a
// graph, and the area chosen on the graph opened under it.
func (u *UI) taskView(gtx C, p *project, s shown, ok bool) D {
	if !ok {
		return layout.Center.Layout(gtx, u.text(sizeBody, u.p.Muted, "Describe below what should change in "+p.name+"."))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(inset(pad+4, gutter, u.graph(p, s))),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		layout.Rigid(u.hairline),
		layout.Flexed(1, u.areaDetail(p, s)),
	)
}

// areaDetail is the area chosen on the graph: what it is waiting for, what
// it did, the objects it produced, and in execution what each agent did.
func (u *UI) areaDetail(p *project, s shown) W {
	return func(gtx C) D {
		n := p.area
		if s.live && s.v.Question != "" {
			n = 1
		}
		// First the specification, whole, for the task as a whole; then the
		// metadata of the area chosen on the graph: what it did, what it
		// produced, and the method's record of it.
		body := u.specBlock(p, s)
		body = append(body, u.hairline)
		a := u.model.Area(n)
		body = append(body, column(4,
			u.text(sizeSmall, u.p.Faint, "METADATA — of the area chosen on the graph"),
			row(10, u.mono(sizeSmall, u.p.Accent, fmt.Sprintf("%02d", n)), u.strong(sizeTitle, u.p.Text, a.Name), u.statusChip(s.status[n]),
				u.text(sizeSmall, u.p.Muted, costLine(inArea(s.costs, n))))))
		if s.live {
			for _, st := range s.v.Steps {
				if st.Area != n {
					continue
				}
				c := u.p.Muted
				if st.Bad {
					c = u.p.Fail
				}
				body = append(body, func(gtx C) D {
					return layout.Flex{Gap: gtx.Dp(10)}.Layout(gtx,
						layout.Rigid(u.mono(sizeSmall, u.p.Faint, st.At.Format("15:04:05"))),
						layout.Flexed(1, u.text(sizeSmall, c, st.Text)),
					)
				})
			}
		}
		if s.c != nil {
			dir := s.c.Change.Dir
			switch n {
			case 1:
				body = append(body, u.reviews(dir, "specification")...)
			case 2:
				body = append(body, u.content(p, s, 2)...)
				body = append(body, u.reviews(dir, "plan")...)
			default:
				body = append(body, u.content(p, s, n)...)
			}
		}
		var items []W
		if s.c != nil {
			for _, it := range s.c.InArea(n) {
				items = append(items, u.itemRow(p, it))
			}
		}
		if n == 1 && s.c != nil {
			for _, it := range s.c.Items {
				if it.Kind == "Specification" {
					body = append(body, u.text(sizeSmall, u.p.Muted, u.className(it.Field("Default class"))))
				}
			}
		}
		if len(items) > 0 {
			body = append(body, u.text(sizeSmall, u.p.Faint, "THE RECORD — open an object to see every cell"), box(u.p.Surface, u.p.Line, 8, 4, column(0, items...)))
		} else if s.status[n] == flow.Idle {
			body = append(body, u.text(sizeSmall, u.p.Faint, "not reached"))
		}
		if n == 4 {
			agent := s.v.Agent
			if len(agent) == 0 && s.c != nil {
				agent = u.tracesOf(s.c)
			}
			for _, unit := range sortedStrings(agent) {
				body = append(body, u.agentFeed(p, unit, agent[unit], forUnit(inArea(s.costs, 4), unit)))
			}
		}
		l := u.lists.of("task/detail/" + p.path)
		return material.List(u.th, l).Layout(gtx, len(body), func(gtx C, i int) D {
			top := unit.Dp(8)
			if i == 0 {
				top = pad
			}
			return layout.Inset{Top: top, Bottom: 4, Left: gutter, Right: gutter}.Layout(gtx, body[i])
		})
	}
}

// areaTimes is when an area of a live run began and, if it has, ended.
func areaTimes(v flow.View, n int) (time.Time, time.Time) {
	var first, last time.Time
	for _, s := range v.Steps {
		if s.Area == n {
			if first.IsZero() {
				first = s.At
			}
			last = s.At
		}
	}
	if v.Areas[n] == flow.Working || v.Areas[n] == flow.Waiting {
		last = time.Now()
	}
	return first, last
}

func (u *UI) statusChip(s flow.Status) W {
	switch s {
	case flow.Working:
		return u.chip(u.p.AccentSoft, u.p.Accent, string(s))
	case flow.Waiting:
		return u.chip(u.p.NoteSoft, u.p.Note, string(s))
	case flow.Done:
		return u.chip(u.p.OKSoft, u.p.OK, string(s))
	case flow.Failed:
		return u.chip(u.p.FailSoft, u.p.Fail, string(s))
	}
	return u.chip(u.p.Sunken, u.p.Faint, "not reached")
}

// composer is the box at the foot of every view of a project: it asks a
// model about what is on view, with the conversation so far above it, or
// starts a task. While a task runs, starting another waits, and the box
// offers to stop it; asking never waits for a task.
func (u *UI) composer(p *project) W {
	return func(gtx C) D {
		key, dir := u.subject(p)
		chatting := u.asking(p, key)
		busy := p.running() && !chatting
		ed := &p.task
		if chatting {
			ed = &p.talk.box
			ed.Submit = true
		}
		for {
			e, ok := ed.Update(gtx)
			if !ok {
				break
			}
			if s, ok := e.(widget.SubmitEvent); ok && !busy {
				u.acted = true
				u.enter(p, chatting, key, dir, s.Text)
			}
		}
		send, stop := u.clicks.of("run/send/"+p.path), u.clicks.of("run/stop/"+p.path)
		if u.clicked(gtx, send) && !busy {
			u.enter(p, chatting, key, dir, ed.Text())
		}
		if u.clicked(gtx, stop) && busy && p.cancel != nil {
			p.cancel()
		}
		hint := "What should change in " + p.name + "? Enter runs it."
		label := "Run"
		switch {
		case u.flowConfig == nil:
			hint = "No model is configured, so nothing can be asked or run from here."
		case chatting && key == "":
			hint, label = "Ask about "+p.name+" — Enter sends. Each task has its own conversation.", "Send"
		case chatting:
			hint, label = "Ask about this task — what it was handed, why it stopped, what to do. Enter sends.", "Send"
		case busy:
			hint = "A task is running in " + p.name + " — wait for it to end, or stop it, before the next."
		}
		button := func(gtx C) D {
			return u.pressable(gtx, send, false, 8, box(u.p.Accent, colorNRGBA{}, 8, 0, inset(9, 18, u.strong(sizeBody, u.p.Surface, label))))
		}
		if busy {
			button = func(gtx C) D {
				return u.pressable(gtx, stop, false, 8, box(u.p.FailSoft, colorNRGBA{}, 8, 0, inset(9, 18, u.strong(sizeBody, u.p.Fail, "Stop"))))
			}
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(u.conversation(p)),
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Alignment: layout.End, Gap: gtx.Dp(10)}.Layout(gtx,
					layout.Rigid(u.modes(p, key)),
					layout.Flexed(1, box(u.p.Surface, u.p.Line, 8, 10, func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						e := material.Editor(u.th, ed, hint)
						e.TextSize = sizeBody
						return e.Layout(gtx)
					})),
					layout.Rigid(button),
				)
			}),
		)
	}
}

// enter is what Enter does in the box: asks, or starts a task.
func (u *UI) enter(p *project, chatting bool, key, dir, text string) {
	if chatting {
		u.ask(p, key, dir, text)
		return
	}
	u.send(p, text)
}

// send starts what was typed and shows its run.
func (u *UI) send(p *project, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	u.start(p, text)
	p.task.SetText("")
}

// closeForm is closing a task that will not be finished: why, or which
// later task does what it was for. Nothing is deleted; the record says it
// was closed, and by whom.
func (u *UI) closeForm(p *project, c *workspace.Change) W {
	return func(gtx C) D {
		confirm := u.clicks.of("close/confirm/" + c.Change.Dir)
		if u.clicked(gtx, confirm) {
			_ = u.close(p, c, p.closeWhy.Text())
		}
		var by []W
		for _, later := range p.order {
			if later == c {
				break // only tasks after it can stand in for it
			}
			if later.Reached() == 6 {
				later := later
				btn := u.clicks.of("close/by/" + c.Change.Dir + "/" + later.Change.Dir)
				if u.clicked(gtx, btn) {
					_ = u.close(p, c, "superseded by "+later.Name()+", which landed what it was for")
				}
				by = append(by, func(gtx C) D {
					return u.pressable(gtx, btn, false, 6, box(u.p.OKSoft, colorNRGBA{}, 6, 0, inset(5, 12, u.text(sizeSmall, u.p.OK, "Superseded by "+later.Name()))))
				})
			}
		}
		parts := []W{
			u.text(sizeSmall, u.p.Muted, "Closing keeps everything it wrote and says it will not be finished; nothing it made entered the project."),
		}
		if len(by) > 0 {
			parts = append(parts, u.wrap(8, by))
		}
		parts = append(parts, func(gtx C) D {
			return layout.Flex{Gap: gtx.Dp(8), Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, box(u.p.Bg, u.p.Line, 6, 8, func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return material.Editor(u.th, &p.closeWhy, "why it is closed").Layout(gtx)
				})),
				layout.Rigid(func(gtx C) D {
					return u.pressable(gtx, confirm, false, 6, box(u.p.Sunken, colorNRGBA{}, 6, 0, inset(6, 14, u.text(sizeSmall, u.p.Text, "Close"))))
				}))
		})
		return box(u.p.Bg, colorNRGBA{}, 6, 10, column(8, parts...))(gtx)
	}
}
