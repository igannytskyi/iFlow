package ui

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/igannytskyi/iflow/internal/workspace"
)

// projectView is a project as a whole: what it has cost, how its tasks
// stand, and the foundations — areas 7 to 13 — that hold for every task
// rather than for one.
func (u *UI) projectView(gtx C, p *project) D {
	t := tallyOf(p.ws)
	active := 0
	for _, c := range p.order {
		if c.Reached() < 6 {
			active++
		}
	}
	if p.running() {
		active++
	}
	owed := 0
	if p.led != nil {
		owed = len(p.led.Owed)
	}
	body := []W{
		column(4, u.strong(sizeHead, u.p.Text, p.name), u.mono(sizeSmall, u.p.Faint, p.path)),
		u.wrap(8, []W{
			u.cell(money(t.spent), "spent", u.p.Text),
			u.cell(tokens(t.tokens), "tokens", u.p.Text),
			u.cell(fmt.Sprint(t.changes), "tasks", u.p.Text),
			u.cell(fmt.Sprint(active), "active", u.p.Accent),
			u.cell(fmt.Sprint(t.landed), "landed", u.p.OK),
			u.cell(fmt.Sprintf("%d / %d", t.passing, t.checked), "pass the gate", u.p.Text),
			u.cell(fmt.Sprintf("%d / %d", t.met, t.met+t.failed+t.undecided), "criteria met", u.p.Text),
			u.cell(fmt.Sprint(owed), "escalations open", u.p.Note),
		}),
		u.hairline,
		u.text(sizeSmall, u.p.Faint, "TASKS — newest first"),
	}
	if len(p.order) == 0 && p.run == nil {
		body = append(body, u.text(sizeBody, u.p.Muted, "No task yet: start one with + New intent on the left."))
	}
	for _, c := range p.order {
		body = append(body, u.taskLine(p, c))
	}
	body = append(body, u.hairline, u.text(sizeSmall, u.p.Faint, "FOUNDATIONS — what holds for every task, not for one"))
	var cards []W
	for _, f := range foundationsOf(p, p.estate.Load()) {
		cards = append(cards, u.foundationCard(f))
	}
	body = append(body, u.wrap(10, cards))
	body = append(body, u.hairline, u.escalations(p))
	return u.scroll(gtx, "project/"+p.path, body)
}

// taskLine is one task in the project's list: how far it got, what the gate
// says, what it cost, what it was for.
func (u *UI) taskLine(p *project, c *workspace.Change) W {
	return func(gtx C) D {
		btn := u.clicks.of("project/task/" + c.Change.Dir)
		if btn.Clicked(gtx) {
			p.show(c.Change.Dir, c)
		}
		s := shown{c: c}
		for n := 1; n <= 6; n++ {
			s.status[n] = statusOf(c, n)
		}
		state := s.state(p)
		spent := "—"
		if len(c.Costs) > 0 {
			spent = money(c.Spent.CostUSD)
		}
		return u.pressable(gtx, btn, false, 6, stretch(inset(7, 8, func(gtx C) D {
			return layout.Flex{Gap: gtx.Dp(12), Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(width(90, row(6, dot(u.stateColor(state), 8), u.oneLine(sizeSmall, u.stateColor(state), state)))),
				layout.Rigid(width(220, u.oneLine(sizeBody, u.p.Text, c.Name()))),
				layout.Rigid(width(96, u.gateBadge(c))),
				layout.Rigid(width(70, u.oneLine(sizeSmall, u.p.Text, spent))),
				layout.Flexed(1, u.oneLine(sizeSmall, u.p.Muted, intentOf(c))),
			)
		})))
	}
}

func (u *UI) foundationCard(f foundation) W {
	a := u.model.Area(f.n)
	return func(gtx C) D {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(300))
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		var rows []W
		rows = append(rows, row(8, u.mono(sizeSmall, u.p.Accent, fmt.Sprintf("%02d", f.n)), u.strong(sizeBody, u.p.Text, a.Name)))
		if len(f.facts) == 0 {
			rows = append(rows, u.text(sizeSmall, u.p.Faint, f.empty))
		}
		for _, fact := range f.facts {
			if len(fact[0]) > 14 {
				rows = append(rows, column(1, u.text(sizeBody, u.p.Text, fact[0]), u.text(sizeSmall, u.p.Muted, fact[1])))
				continue
			}
			rows = append(rows, func(gtx C) D {
				return layout.Flex{Gap: gtx.Dp(8), Alignment: layout.Baseline}.Layout(gtx,
					layout.Rigid(u.strong(sizeBody, u.p.Text, fact[0])),
					layout.Flexed(1, u.text(sizeSmall, u.p.Muted, fact[1])))
			})
		}
		return box(u.p.Surface, u.p.Line, 8, 12, column(5, rows...))(gtx)
	}
}

// newIntentView is what a new intent goes through, said once, above the box
// it is typed in.
func (u *UI) newIntentView(p *project) W {
	return func(gtx C) D {
		who := "the configured model"
		if c := u.flowConfig; c != nil {
			who = c.Model.Name()
		}
		steps := []string{
			"An author (" + who + ") writes the specification: the change described, numbered requirements, design, edge cases, open questions, and the criteria that will judge it.",
			"A reviewer reads it as the builders will, and sends it back with what is missing — twice at most.",
			"You read it and accept it, or send it back with notes. This is the one place you are asked.",
			"Then a planner divides the work into units, a reviewer checks the plan, and one agent per unit carries it out; the checks and the landing run by themselves.",
		}
		var ws []W
		ws = append(ws, u.strong(sizeHead, u.p.Text, "New intent in "+p.name),
			u.text(sizeBody, u.p.Muted, "Say below what should change, in your own words."))
		for i, s := range steps {
			ws = append(ws, u.point(fmt.Sprint(i+1)+".", s))
		}
		return layout.Inset{Top: unit.Dp(48), Left: gutter, Right: gutter}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(720))
			return column(10, ws...)(gtx)
		})
	}
}
