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
		layout.Rigid(inset(8, gutter, u.taskTitle(p, s))),
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
		a := u.model.Area(n)
		var body []W
		body = append(body, row(10,
			u.mono(sizeSmall, u.p.Accent, fmt.Sprintf("%02d", n)), u.strong(sizeTitle, u.p.Text, a.Name), u.statusChip(s.status[n]),
			u.text(sizeSmall, u.p.Muted, costLine(inArea(s.costs, n)))))
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
		// The documents an area writes are read as documents: the change as
		// specified, the plan with every unit described, and what the
		// reviewers said of each.
		if s.c != nil {
			dir := s.c.Change.Dir
			switch n {
			case 1:
				body = append(body, u.specPanel(p, s)...)
				body = append(body, u.reviews(dir, "specification")...)
				body = append(body, u.metaPanel(p, s)...)
			case 2:
				body = append(body, u.content(p, s, 2)...)
				body = append(body, u.reviews(dir, "plan")...)
			default:
				body = append(body, u.content(p, s, n)...)
			}
		}
		var items []W
		if s.c != nil && n != 1 { // area 1 draws its record in its own panel
			for _, it := range s.c.InArea(n) {
				items = append(items, u.itemRow(p, it))
			}
		}
		if len(items) > 0 {
			body = append(body, u.text(sizeSmall, u.p.Faint, "METADATA — the method's record of it; open an object to see every cell"), box(u.p.Surface, u.p.Line, 8, 4, column(0, items...)))
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

// composer is where a task is typed, at the foot of every view of a project.
// While one runs it waits, and offers to stop it.
func (u *UI) composer(p *project) W {
	return func(gtx C) D {
		busy := p.running()
		for {
			e, ok := p.task.Update(gtx)
			if !ok {
				break
			}
			if s, ok := e.(widget.SubmitEvent); ok && !busy {
				u.send(p, s.Text)
			}
		}
		send, stop := u.clicks.of("run/send/"+p.path), u.clicks.of("run/stop/"+p.path)
		if send.Clicked(gtx) && !busy {
			u.send(p, p.task.Text())
		}
		if stop.Clicked(gtx) && busy && p.cancel != nil {
			p.cancel()
		}
		hint := "What should change in " + p.name + "? Enter runs it."
		switch {
		case u.flowConfig == nil:
			hint = "No model is configured, so nothing can be run from here."
		case busy:
			hint = "A task is running in " + p.name + " — wait for it to end, or stop it, before the next."
		}
		button := func(gtx C) D {
			return u.pressable(gtx, send, false, 8, box(u.p.Accent, colorNRGBA{}, 8, 0, inset(9, 18, u.strong(sizeBody, u.p.Surface, "Run"))))
		}
		if busy {
			button = func(gtx C) D {
				return u.pressable(gtx, stop, false, 8, box(u.p.FailSoft, colorNRGBA{}, 8, 0, inset(9, 18, u.strong(sizeBody, u.p.Fail, "Stop"))))
			}
		}
		return layout.Flex{Alignment: layout.End, Gap: gtx.Dp(10)}.Layout(gtx,
			layout.Flexed(1, box(u.p.Surface, u.p.Line, 8, 10, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				ed := material.Editor(u.th, &p.task, hint)
				ed.TextSize = sizeBody
				return ed.Layout(gtx)
			})),
			layout.Rigid(button),
		)
	}
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
		if confirm.Clicked(gtx) {
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
				if btn.Clicked(gtx) {
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

// taskTitle is what the task was for and how it stands, in a line or two.
func (u *UI) taskTitle(p *project, s shown) W {
	return func(gtx C) D {
		state := s.state(p)
		facts := []W{row(6, dot(u.stateColor(state), 8), u.text(sizeSmall, u.stateColor(state), state))}
		if s.c != nil {
			facts = append(facts, u.mono(sizeSmall, u.p.Faint, s.c.Name()))
			if !s.live || s.v.Finished {
				facts = append(facts, u.gateBadge(s.c))
			}
		}
		return column(4, u.strong(sizeTitle, u.p.Text, s.title()), u.wrap(12, facts))(gtx)
	}
}
