package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/igannytskyi/iflow/internal/flow"
)

// taskView is the task on view: what it was for, the path it took as a
// graph, and the area chosen on the graph opened under it.
func (u *UI) taskView(gtx C, p *project, s shown, ok bool) D {
	if !ok {
		return layout.Center.Layout(gtx, u.text(sizeBody, u.p.Muted, "Describe below what should change in "+p.name+"."))
	}
	head := u.intentCard(p, s)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(inset(pad+4, gutter, head)),
		layout.Rigid(inset(4, gutter, u.graph(p, s))),
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
			return u.review(p, s.v)(gtx)
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
				body = append(body, u.prose(filepath.Join(dir, "01-specification.md"), "## The change")...)
				body = append(body, u.reviews(dir, "specification")...)
			case 2:
				body = append(body, u.prose(filepath.Join(dir, "02-plan.md"), "")...)
				body = append(body, u.reviews(dir, "plan")...)
			}
		}
		var items []W
		if s.c != nil {
			for _, it := range s.c.InArea(n) {
				items = append(items, u.itemRow(p, it))
			}
		}
		if len(items) > 0 {
			body = append(body, u.text(sizeSmall, u.p.Faint, "OBJECTS — open one to see every cell"), box(u.p.Surface, u.p.Line, 8, 4, column(0, items...)))
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

// intentCard is the task's intent as the method holds it — what was asked,
// its class, what judges it — and what a person can do with it here: read
// the specification, or, once the work is past changing it, start a new
// intent from this one. A specification is never edited after admission; a
// changed intent is a new one.
func (u *UI) intentCard(p *project, s shown) W {
	return func(gtx C) D {
		state := s.state(p)
		read, again := u.clicks.of("intent/read/"+p.path), u.clicks.of("intent/again/"+p.path)
		if read.Clicked(gtx) {
			p.area, p.item = 1, nil
		}
		if again.Clicked(gtx) {
			p.task.SetText(s.title())
			p.selected, p.item = newKey, nil
			gtx.Execute(key.FocusCmd{Tag: &p.task})
		}
		class, criteria, reqs := "", 0, 0
		if s.c != nil {
			for _, it := range s.c.Items {
				switch it.Kind {
				case "Specification":
					class = it.Field("Default class")
				case "Criterion":
					criteria++
				}
			}
			for _, l := range u.docs.read(filepath.Join(s.c.Change.Dir, "01-specification.md")) {
				if strings.HasPrefix(strings.TrimSpace(l), "- FR-") {
					reqs++
				}
			}
		}
		facts := []W{row(6, dot(u.stateColor(state), 8), u.text(sizeSmall, u.stateColor(state), state))}
		if s.c != nil {
			facts = append(facts, u.mono(sizeSmall, u.p.Faint, s.c.Name()))
			if class != "" {
				facts = append(facts, u.text(sizeSmall, u.p.Muted, u.className(class)))
			}
			facts = append(facts, u.text(sizeSmall, u.p.Muted, fmt.Sprintf("%s · %s", plural(reqs, "requirement"), plural(criteria, "criterion"))))
			if !s.live || s.v.Finished {
				facts = append(facts, u.gateBadge(s.c))
			}
		}
		button := func(btn *widget.Clickable, label string, strong bool) W {
			return func(gtx C) D {
				bg, fg := u.p.Sunken, u.p.Text
				if strong {
					bg, fg = u.p.AccentSoft, u.p.Accent
				}
				return u.pressable(gtx, btn, false, 6, box(bg, colorNRGBA{}, 6, 0, inset(5, 12, u.text(sizeSmall, fg, label))))
			}
		}
		actions := []W{button(read, "Read the specification", false)}
		if !p.running() || !s.live {
			actions = append(actions, button(again, "New intent from this", true))
		}
		return box(u.p.Surface, u.p.Line, 8, 12, column(8,
			u.text(sizeSmall, u.p.Faint, "INTENT"),
			u.strong(sizeTitle, u.p.Text, s.title()),
			u.wrap(12, facts),
			row(8, actions...),
		))(gtx)
	}
}
