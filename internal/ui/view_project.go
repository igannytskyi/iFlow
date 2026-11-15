package ui

import (
	"fmt"
	"image"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/workspace"
)

// projectView is a project as a whole: what it has cost, how its tasks
// stand, and the foundations — areas 7 to 13 — that hold for every task
// rather than for one.
func (u *UI) projectView(gtx C, p *project) D {
	if p.foundation != 0 {
		return u.foundationView(gtx, p)
	}
	t := tallyOf(p.ws)
	active := 0
	for _, c := range p.order {
		if closed, _ := c.Closed(); c.Reached() < 6 && !closed {
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
		row(8, u.text(sizeSmall, u.p.Faint, "TASKS — newest first"),
			u.choice("filter/all/"+p.path, "All", p.filter == "", func() { p.filter = "" }),
			u.choice("filter/active/"+p.path, "Active", p.filter == "active", func() { p.filter = "active" }),
			u.choice("filter/landed/"+p.path, "Done", p.filter == "landed", func() { p.filter = "landed" })),
	}
	if len(p.order) == 0 && p.run == nil {
		body = append(body, u.text(sizeBody, u.p.Muted, "No task yet: start one with + New intent on the left."))
	}
	for _, c := range p.order {
		closed, _ := c.Closed()
		done := closed || (c.Reached() == 6 && statusOf(c, 6) == flow.Done)
		if (p.filter == "active" && done) || (p.filter == "landed" && !done) {
			continue
		}
		body = append(body, u.taskLine(p, c))
	}
	var next []W
	for _, c := range p.order {
		for i, later := range u.laterOf(c.Change.Dir) {
			next = append(next, u.laterRow(p, c, i, later))
		}
	}
	if len(next) > 0 {
		body = append(body, u.hairline, u.text(sizeSmall, u.p.Faint, "NEXT — parts of earlier tasks too large for one change, each a task of its own"))
		body = append(body, next...)
	}
	body = append(body, u.hairline, u.text(sizeSmall, u.p.Faint, "FOUNDATIONS — what holds for every task, not for one"))
	var cards []func(int) W
	e := p.estate.Load()
	for _, f := range memo(u.docs, "foundations|"+p.path, func() []foundation { return foundationsOf(p, e) }) {
		cards = append(cards, u.openable(p, f.n, u.foundationCard(f)))
	}
	body = append(body, u.grid(280, 10, cards))
	body = append(body, u.hairline, u.escalations(p))
	return u.scroll(gtx, "project/"+p.path, body)
}

// taskLine is one task in the project's list: how far it got, what the gate
// says, what it cost, what it was for.
func (u *UI) taskLine(p *project, c *workspace.Change) W {
	return func(gtx C) D {
		btn := u.clicks.of("project/task/" + c.Change.Dir)
		if u.clicked(gtx, btn) {
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

func (u *UI) foundationCard(f foundation) func(minH int) W {
	a := u.model.Area(f.n)
	return func(minH int) W {
		return u.card(minH, u.foundationBody(f, a.Name))
	}
}

// card is a box at least minH tall, whatever it holds.
func (u *UI) card(minH int, body W) W {
	return func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return box(u.p.Surface, u.p.Line, 8, 12, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			if minH > 0 {
				gtx.Constraints.Min.Y = max(0, minH-gtx.Dp(24)-gtx.Dp(2))
			}
			d := body(gtx)
			d.Size.Y = max(d.Size.Y, gtx.Constraints.Min.Y)
			return d
		})(gtx)
	}
}

func (u *UI) foundationBody(f foundation, name string) W {
	return func(gtx C) D {
		var rows []W
		rows = append(rows, row(8, u.mono(sizeSmall, u.p.Accent, fmt.Sprintf("%02d", f.n)), u.strong(sizeBody, u.p.Text, name)))
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
		return column(5, rows...)(gtx)
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

// laterRow is one part an earlier task left for later, started with a click
// as a new intent already written.
func (u *UI) laterRow(p *project, c *workspace.Change, i int, text string) W {
	return func(gtx C) D {
		btn := u.clicks.of(fmt.Sprintf("later/%s/%d", c.Change.Dir, i))
		if u.clicked(gtx, btn) {
			p.task.SetText(text)
			p.selected, p.item = newKey, nil
			gtx.Execute(key.FocusCmd{Tag: &p.task})
		}
		return layout.Flex{Gap: gtx.Dp(12), Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(width(200, u.oneLine(sizeSmall, u.p.Faint, "after "+c.Name()))),
			layout.Flexed(1, u.text(sizeBody, u.p.Text, text)),
			layout.Rigid(func(gtx C) D {
				return u.pressable(gtx, btn, false, 6, box(u.p.AccentSoft, colorNRGBA{}, 6, 0, inset(5, 12, u.text(sizeSmall, u.p.Accent, "Start"))))
			}),
		)
	}
}

// grid lays cards out in equal columns, as many as fit at least minW wide,
// and every card as tall as the tallest, so that a card with less to say is
// no smaller than its neighbours. A card is asked for at a height: 0 to
// measure it, then the height of the tallest.
func (u *UI) grid(minW, gap unit.Dp, cards []func(minH int) W) W {
	return func(gtx C) D {
		g := gtx.Dp(gap)
		total := gtx.Constraints.Max.X
		cols := max(1, (total+g)/(gtx.Dp(minW)+g))
		w := (total - g*(cols-1)) / cols
		at := func(h int) C {
			cg := gtx
			cg.Constraints = layout.Constraints{Min: image.Pt(w, 0), Max: image.Pt(w, gtx.Constraints.Max.Y)}
			return cg
		}
		tallest := 0
		for _, c := range cards {
			tallest = max(tallest, recordOp(at(0), c(0)).dims.Size.Y) // measured, never drawn
		}
		y := 0
		for i := 0; i < len(cards); i += cols {
			for j := i; j < min(i+cols, len(cards)); j++ {
				recordOp(at(tallest), cards[j](tallest)).at(gtx, (j-i)*(w+g), y)
			}
			y += tallest + g
		}
		return D{Size: image.Pt(total, max(0, y-g))}
	}
}

// openable is a card that opens the foundation it sums up.
func (u *UI) openable(p *project, n int, card func(minH int) W) func(minH int) W {
	return func(minH int) W {
		return func(gtx C) D {
			btn := u.clicks.of(fmt.Sprintf("foundation/open/%s/%d", p.path, n))
			if u.clicked(gtx, btn) {
				p.foundation = n
			}
			return btn.Layout(gtx, card(minH))
		}
	}
}
