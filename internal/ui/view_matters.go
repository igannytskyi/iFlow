package ui

import (
	"strings"

	"gioui.org/layout"

	"github.com/igannytskyi/iflow/internal/workspace"
)

// matter is something about a task a person may have to act on: a problem
// the gate found in it, or an escalation it left open.
type matter struct {
	kind, ref, text string
}

// matters is every problem and open escalation of a task, in plain words:
// the rule's code and the object's number are kept for the model, not shown.
func (u *UI) matters(p *project, c *workspace.Change) []matter {
	if c == nil {
		return nil
	}
	var out []matter
	if c.State == workspace.Checked {
		for _, f := range c.Report.Problems() {
			out = append(out, matter{"problem", f.Rule, plainly(f.Message)})
		}
	}
	if p.led != nil {
		for _, o := range p.led.Owed {
			if name, _, _ := strings.Cut(o.Ref, "#"); name == c.Name() {
				out = append(out, matter{"escalation", o.Ref, plainly(o.Ground)})
			}
		}
	}
	return out
}

// plainly is a finding without the codes it opens with.
func plainly(s string) string {
	s = strings.TrimSpace(s)
	for {
		head, rest, ok := strings.Cut(s, ": ")
		if !ok || len(head) > 12 || strings.Contains(head, " ") {
			break
		}
		s = rest // "R22: …", "CR-001-02: …"
	}
	if s != "" {
		s = strings.ToUpper(s[:1]) + s[1:]
	}
	return s
}

// mattersChip says how many problems and escalations a task has, and opens
// them; nothing when there are none.
func (u *UI) mattersChip(p *project, c *workspace.Change) W {
	ms := u.matters(p, c)
	if len(ms) == 0 {
		if c != nil && c.State == workspace.Checked {
			return u.chip(u.p.OKSoft, u.p.OK, "passed")
		}
		return nothing
	}
	problems, owed := 0, 0
	for _, m := range ms {
		if m.kind == "problem" {
			problems++
		} else {
			owed++
		}
	}
	var label []string
	if problems > 0 {
		label = append(label, plural(problems, "problem"))
	}
	if owed > 0 {
		label = append(label, plural(owed, "escalation"))
	}
	btn := u.clicks.of("matters/" + c.Change.Dir)
	key := "matters/" + c.Change.Dir
	return func(gtx C) D {
		if u.clicked(gtx, btn) {
			p.open[key] = !p.open[key]
		}
		arrow := " ▾"
		if p.open[key] {
			arrow = " ▴"
		}
		bg, fg := u.p.FailSoft, u.p.Fail
		if problems == 0 {
			bg, fg = u.p.NoteSoft, u.p.Note // only questions left for the person
		}
		return u.pressable(gtx, btn, false, 4, box(bg, colorNRGBA{}, 4, 0,
			inset(2, 8, u.text(sizeSmall, fg, strings.Join(label, ", ")+arrow))))
	}
}

// mattersPanel is a task's problems and escalations, each in a sentence,
// and a way to have the model explain them and offer what can be done,
// in the task's own conversation, where the person picks or says their own.
func (u *UI) mattersPanel(p *project, c *workspace.Change) W {
	if c == nil || !p.open["matters/"+c.Change.Dir] {
		return nothing
	}
	ms := u.matters(p, c)
	if len(ms) == 0 {
		return nothing
	}
	ask := u.clicks.of("matters/ask/" + c.Change.Dir)
	var rows []W
	for _, m := range ms {
		mark, col := "!", u.p.Fail
		if m.kind == "escalation" {
			mark, col = "?", u.p.Note
		}
		rows = append(rows, func(gtx C) D {
			return layout.Flex{Gap: gtx.Dp(8)}.Layout(gtx,
				layout.Rigid(width(14, u.strong(sizeBody, col, mark))),
				layout.Flexed(1, u.text(sizeBody, u.p.Text, m.text)))
		})
	}
	rows = append(rows, func(gtx C) D {
		if u.clicked(gtx, ask) {
			u.askAbout(p, c, ms)
		}
		return u.pressable(gtx, ask, false, 6, box(u.p.AccentSoft, colorNRGBA{}, 6, 0,
			inset(5, 12, u.text(sizeSmall, u.p.Accent, "What does this mean and what can I do?"))))
	})
	return box(u.p.Surface, u.p.Line, 8, 12, column(8, rows...))
}

// askAbout puts a task's matters to the model in the task's conversation:
// what they mean, in a few plain words, and the ways out, numbered, for the
// person to pick one or say their own.
func (u *UI) askAbout(p *project, c *workspace.Change, ms []matter) {
	var b strings.Builder
	b.WriteString("Explain to me, briefly and in plain words, what each of these means for this task and why it matters. ")
	b.WriteString("Then give two to four numbered ways to resolve them, each with what will happen and what it costs or risks, and say which you would choose. ")
	b.WriteString("I will answer with a number or my own way.\n\n")
	for _, m := range ms {
		b.WriteString("- " + m.kind + " (" + m.ref + "): " + m.text + "\n")
	}
	if p.talk.intent == nil {
		p.talk.intent = map[string]bool{}
	}
	p.talk.intent[c.Change.Dir] = false
	u.ask(p, c.Change.Dir, c.Change.Dir, b.String())
}

// attention marks a task in the list when it waits on the person: "?" for
// a question it asks — the criteria to accept, an escalation — and "!" for
// a problem the gate found in a task that is not finished with.
func (u *UI) attention(p *project, state string, c *workspace.Change) W {
	if state == "waiting for you" {
		return u.chip(u.p.AccentSoft, u.p.Accent, "?")
	}
	if state == "closed" || c == nil {
		return nothing
	}
	mark := ""
	for _, m := range u.matters(p, c) {
		switch {
		case m.kind == "escalation":
			mark = "?"
		case mark == "" && state != "landed":
			mark = "!"
		}
	}
	switch mark {
	case "?":
		return u.chip(u.p.NoteSoft, u.p.Note, "?")
	case "!":
		return u.chip(u.p.FailSoft, u.p.Fail, "!")
	}
	return nothing
}
