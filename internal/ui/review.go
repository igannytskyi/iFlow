package ui

import (
	"path/filepath"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/method"
	"github.com/igannytskyi/iflow/internal/workspace"
)

// review is the one place a person is asked for anything: the intent as the
// model stated it, the criteria that will judge it, what it may touch, what
// will judge it and when it stops. Accepting fixes them and lets everything
// after run by itself; refusing returns them, with the reason, as the
// failure of the area that wrote them.
func (u *UI) review(p *project, v flow.View) W {
	return func(gtx C) D {
		yes, no := u.clicks.of("review/yes/"+p.path), u.clicks.of("review/no/"+p.path)
		if yes.Clicked(gtx) {
			p.run.Accept()
		}
		if no.Clicked(gtx) {
			ground := strings.TrimSpace(p.ground.Text())
			if ground == "" {
				ground = "refused without a reason given"
			}
			p.run.Refuse(ground)
			p.ground.SetText("")
		}
		c := p.live
		var body []W
		body = append(body, column(4,
			u.text(sizeSmall, u.p.Note, "WAITING FOR YOU"),
			u.strong(sizeHead, u.p.Text, "Is this what you want?"),
			u.text(sizeSmall, u.p.Muted, v.Question)))
		if c == nil {
			body = append(body, u.text(sizeSmall, u.p.Faint, "reading what was written…"))
		} else {
			body = append(body, u.reviewIntent(c)...)
			if v.Reviewed != "" {
				var left, questions []W
				for _, l := range strings.Split(strings.TrimSpace(v.Reviewed), "\n") {
					l = strings.TrimPrefix(l, "- ")
					if q, ok := strings.CutPrefix(l, "[question for the person] "); ok {
						questions = append(questions, u.point("?", q))
					} else {
						left = append(left, u.point("!", l))
					}
				}
				if len(questions) > 0 {
					body = append(body, box(u.p.AccentSoft, colorNRGBA{}, 8, 10, column(6,
						append([]W{u.text(sizeSmall, u.p.Accent, "QUESTIONS ONLY YOU CAN ANSWER — answer them in your notes and send it back, or accept it as it is")}, questions...)...)))
				}
				if len(left) > 0 {
					body = append(body, box(u.p.NoteSoft, colorNRGBA{}, 8, 10, column(6,
						append([]W{u.text(sizeSmall, u.p.Note, "WHAT THE REVIEWER STILL FINDS — decide whether it matters")}, left...)...)))
				}
			}
			body = append(body, u.text(sizeSmall, u.p.Faint, "THE SPECIFICATION"))
			body = append(body, u.prose(filepath.Join(c.Change.Dir, "01-specification.md"), "## The change")...)
		}
		body = append(body, column(8,
			box(u.p.Bg, u.p.Line, 6, 8, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return material.Editor(u.th, &p.ground, "your notes: what is wrong or missing, and your answers to the questions — a new author writes the specification again from them").Layout(gtx)
			}),
			row(10,
				func(gtx C) D {
					return u.pressable(gtx, yes, false, 8, box(u.p.OK, colorNRGBA{}, 8, 0, inset(8, 18, u.strong(sizeBody, u.p.Surface, "Accept — run the rest"))))
				},
				func(gtx C) D {
					return u.pressable(gtx, no, false, 8, box(u.p.FailSoft, colorNRGBA{}, 8, 0, inset(8, 16, u.strong(sizeBody, u.p.Fail, "Send back with notes"))))
				},
			),
		))
		l := u.lists.of("review/" + p.path)
		return material.List(u.th, l).Layout(gtx, len(body), func(gtx C, i int) D {
			top := unit.Dp(10)
			if i == 0 {
				top = gutter
			}
			return layout.Inset{Top: top, Bottom: 4, Left: gutter, Right: gutter}.Layout(gtx, body[i])
		})
	}
}

func (u *UI) reviewIntent(c *workspace.Change) []W {
	section := func(title string, ws ...W) W {
		return column(6, append([]W{u.text(sizeSmall, u.p.Faint, title)}, ws...)...)
	}
	var out []W
	var criteria, scope, judges, stops []W
	statement, class := "", ""
	for _, it := range c.Items {
		switch it.Kind {
		case "Intent":
			statement = it.Field("Statement")
		case "Specification":
			class = it.Field("Default class")
		case "Criterion":
			it := it
			plan := ""
			if it.Field("Plan") == "required" {
				plan = " · needs an evidence plan"
			}
			criteria = append(criteria, box(u.p.Bg, colorNRGBA{}, 6, 8, column(3,
				row(8, u.mono(sizeSmall, u.p.Accent, it.ID), u.text(sizeSmall, u.p.Faint, it.Field("Procedure")+" · "+it.Field("Required evidence")+plan)),
				u.strong(sizeSmall, u.p.Text, it.Field("Criterion")),
				u.text(sizeSmall, u.p.Muted, "when "+it.Field("When")),
				u.text(sizeSmall, u.p.Muted, "then "+it.Field("Then")),
			)))
		case "Scope":
			scope = append(scope, u.mono(sizeSmall, u.p.Text, it.ID))
		case "Arbiter":
			judges = append(judges, row(8, u.mono(sizeSmall, u.p.Text, it.ID), u.text(sizeSmall, u.p.Muted, "judges "+it.Field("What it arbitrates"))))
		case "TerminationCondition":
			stops = append(stops, u.text(sizeSmall, u.p.Muted, it.Field("Action")+" when "+method.Plain(it.Field("Condition"))))
		}
	}
	out = append(out, section("INTENT", u.text(sizeBody, u.p.Text, statement), u.text(sizeSmall, u.p.Muted, u.className(class))))
	out = append(out, section("CRITERIA — WHAT WILL DECIDE IT", criteria...))
	out = append(out, section("MAY CHANGE", scope...))
	out = append(out, section("JUDGED BY — WRITTEN BEFORE THE WORK, NEVER CHANGED BY IT", judges...))
	if len(stops) > 0 {
		out = append(out, section("STOPS", stops...))
	}
	return out
}

func (u *UI) className(code string) string {
	for _, c := range u.model.Classes {
		if c.Code == code {
			return code + " — " + c.Name + "; decided by " + strings.ToLower(c.Arbiter[:1]) + c.Arbiter[1:]
		}
	}
	return code
}
