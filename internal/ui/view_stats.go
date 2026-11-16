package ui

import (
	"fmt"
	"image"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/igannytskyi/iflow/internal/artifact"
)

// statsPanel stands beside every task: what that task has cost, in a few
// lines — what each area cost is on the graph — and below it the object or
// handover opened. It moves while a task runs.
func (u *UI) statsPanel(p *project, s shown, ok bool) W {
	return func(gtx C) D {
		fill(gtx, u.p.Surface)
		var body []W
		if ok {
			body = append(body, u.taskStats(p, s)...)
		}
		switch {
		case p.handedView != "" && s.c != nil:
			body = append(body, u.hairline)
			body = append(body, u.handedPanel(p, s)...)
		case p.item != nil && s.c != nil:
			body = append(body, u.hairline)
			body = append(body, u.objectPanel(p, s)...)
		case !ok:
			// No task is chosen: what the whole project has cost.
			body = append(body, u.projectStats(p)...)
		}
		l := u.lists.of("stats/" + p.path)
		return material.List(u.th, l).Layout(gtx, len(body), func(gtx C, i int) D {
			top := unit.Dp(12)
			if i == 0 {
				top = pad + 4
			}
			return layout.Inset{Top: top, Bottom: 2, Left: pad + 4, Right: pad + 4}.Layout(gtx, body[i])
		})
	}
}

func (u *UI) cell(value, label string, c colorNRGBA) W {
	return width(150, column(1, u.strong(sizeTitle, c, value), u.text(sizeSmall, u.p.Faint, label)))
}

func (u *UI) taskStats(p *project, s shown) []W {
	t := artifact.Total(s.costs)
	spentUSD, in, cached, out := t.CostUSD, t.Input+t.CacheRead+t.CacheWrite, t.CacheRead, t.Output
	if s.live && !s.v.Spent.Zero() {
		spentUSD, in, cached, out = s.v.Spent.CostUSD, s.v.Spent.In(), s.v.Spent.CacheRead, s.v.Spent.Output
	}
	clock := "—"
	switch {
	case s.live && len(s.v.Steps) > 0:
		end := time.Now()
		if s.v.Finished {
			end = s.v.Steps[len(s.v.Steps)-1].At
		}
		clock = elapsed(end.Sub(s.v.Steps[0].At).Milliseconds())
	case t.ElapsedMS > 0:
		clock = elapsed(t.ElapsedMS)
	}
	var models []string
	for _, c := range s.costs {
		for m := range strings.SplitSeq(c.Model, ", ") {
			if m != "" && !slices.Contains(models, m) {
				models = append(models, m)
			}
		}
	}
	who := strings.Join(models, ", ")
	if who == "" {
		who = "—"
	}
	hide := u.clicks.of("costs/hide")
	line := func(label, value string, c colorNRGBA) W {
		return func(gtx C) D {
			return layout.Flex{Alignment: layout.Baseline, Gap: gtx.Dp(8)}.Layout(gtx,
				layout.Flexed(1, u.oneLine(sizeSmall, u.p.Faint, label)),
				layout.Rigid(u.oneLine(sizeSmall, c, value)))
		}
	}
	// What the task cost, in a few lines: the money and the time first,
	// then what was read and written. What each area cost is on the graph.
	return []W{
		func(gtx C) D {
			if u.clicked(gtx, hide) {
				u.settings.HideCosts = true
				u.settings.save()
			}
			return layout.Flex{Alignment: layout.Baseline, Gap: gtx.Dp(10)}.Layout(gtx,
				layout.Rigid(u.strong(sizeTitle, u.p.Text, money(spentUSD))),
				layout.Flexed(1, u.oneLine(sizeSmall, u.p.Muted, clock+" · "+plural(len(s.costs), "call"))),
				layout.Rigid(func(gtx C) D {
					return u.pressable(gtx, hide, false, 4, inset(2, 6, u.text(sizeSmall, u.p.Muted, "fold ›")))
				}))
		},
		column(3,
			line("tokens in", tokens(in)+"  ("+tokens(cached)+" cached)", u.p.Text),
			line("tokens out", tokens(out), u.p.Text),
			line("handed by the framework", tokens(handed(s.costs)), u.p.Accent),
			line("agent steps", fmt.Sprint(t.Steps), u.p.Text),
			line("models", who, u.p.Muted),
		),
		u.byAgent(p, s),
	}
}

// roleOf is which agent a cost record was spent by, or "" for work no
// model did — tests run, the gate asked.
func roleOf(c artifact.Cost) string {
	switch {
	case c.Turns == 0 && c.Handed == 0:
		return ""
	case strings.HasPrefix(c.What, "review"):
		return "reviewer"
	case c.Area == 1:
		return "author"
	case c.Area == 2:
		return "planner"
	case c.Area == 4:
		return "executor"
	}
	return "other"
}

// byAgent is what each kind of agent cost, and every handover, each
// opening what that agent was handed.
func (u *UI) byAgent(p *project, s shown) W {
	type sum struct {
		calls, handed, in, out int
		usd                    float64
	}
	order := []string{"author", "reviewer", "planner", "executor"}
	sums := map[string]*sum{}
	var handovers []W
	for i, c := range s.costs {
		role := roleOf(c)
		if role == "" {
			continue
		}
		if sums[role] == nil {
			sums[role] = &sum{}
		}
		t := sums[role]
		t.calls++
		t.handed += c.Handed
		t.in += c.In()
		t.out += c.Output
		t.usd += c.CostUSD
		if c.Kept == "" {
			continue
		}
		c, key := c, fmt.Sprintf("handed/%s/%d", p.path, i)
		handovers = append(handovers, func(gtx C) D {
			btn := u.clicks.of(key)
			if u.clicked(gtx, btn) {
				p.handedView, p.item = c.Kept, nil
			}
			label := c.What
			if c.Unit != "" && !strings.Contains(label, c.Unit) {
				label = c.Unit + " " + label
			}
			return u.pressable(gtx, btn, p.handedView == c.Kept, 4, stretch(inset(3, 6, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle, Gap: gtx.Dp(8)}.Layout(gtx,
					layout.Flexed(1, u.oneLine(sizeSmall, u.p.Text, label)),
					layout.Rigid(u.oneLine(sizeSmall, u.p.Accent, tokens(c.Handed))),
					layout.Rigid(u.oneLine(sizeSmall, u.p.Muted, money(c.CostUSD))))
			})))
		})
	}
	var rows []W
	for _, role := range order {
		t := sums[role]
		if t == nil {
			continue
		}
		rows = append(rows, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle, Gap: gtx.Dp(8)}.Layout(gtx,
				layout.Rigid(width(70, u.strong(sizeSmall, u.p.Text, role))),
				layout.Flexed(1, u.oneLine(sizeSmall, u.p.Muted, fmt.Sprintf("%s · handed %s · in %s · out %s", plural(t.calls, "call"), tokens(t.handed), tokens(t.in), tokens(t.out)))),
				layout.Rigid(u.oneLine(sizeSmall, u.p.Text, money(t.usd))))
		})
	}
	if len(rows) == 0 {
		return nothing
	}
	out := append([]W{u.text(sizeSmall, u.p.Faint, "BY AGENT")}, rows...)
	if len(handovers) > 0 {
		out = append(out, u.text(sizeSmall, u.p.Faint, "WHAT EACH WAS HANDED — open one to read it, word for word"))
		out = append(out, handovers...)
	}
	return column(4, out...)
}

// handedPanel is one handover read whole: what the framework gave an agent,
// and nothing it did not.
func (u *UI) handedPanel(p *project, s shown) []W {
	closer := u.clicks.of("handed/close/" + p.path)
	path := filepath.Join(s.c.Change.Dir, filepath.FromSlash(p.handedView))
	lines := u.docs.read(path)
	text := strings.Join(lines, "\n")
	head := func(gtx C) D {
		if u.clicked(gtx, closer) {
			p.handedView = ""
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, u.text(sizeSmall, u.p.Faint, "HANDED — "+p.handedView+" · "+tokens((len(text)+3)/4)+" tokens")),
			layout.Rigid(func(gtx C) D {
				return u.pressable(gtx, closer, false, 4, inset(3, 8, u.text(sizeSmall, u.p.Muted, "close ×")))
			}))
	}
	if len(lines) == 0 {
		return []W{head, u.text(sizeSmall, u.p.Faint, "not kept: work carried before handovers were kept")}
	}
	out := []W{head}
	for _, l := range lines {
		out = append(out, u.mono(sizeSmall, u.p.Text, l))
	}
	return out
}

// meter is a thin bar filled to a share of its width.
func (u *UI) meter(share float64) W {
	return func(gtx C) D {
		w, h := gtx.Constraints.Max.X, gtx.Dp(3)
		box(u.p.Sunken, colorNRGBA{}, 2, 0, func(gtx C) D { return D{Size: image.Pt(w, h)} })(gtx)
		if share > 0 {
			box(u.p.Accent, colorNRGBA{}, 2, 0, func(gtx C) D { return D{Size: image.Pt(max(int(float64(w)*share), h), h)} })(gtx)
		}
		return D{Size: image.Pt(w, h)}
	}
}

func (u *UI) objectPanel(p *project, s shown) []W {
	closer := u.clicks.of("object/close/" + p.path)
	return append([]W{func(gtx C) D {
		if u.clicked(gtx, closer) {
			p.item = nil
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, u.text(sizeSmall, u.p.Faint, "OBJECT")),
			layout.Rigid(func(gtx C) D {
				return u.pressable(gtx, closer, false, 4, inset(3, 8, u.text(sizeSmall, u.p.Muted, "close ×")))
			}))
	}}, u.itemDetail(p, s.c, p.item)...)
}

func (u *UI) projectStats(p *project) []W {
	t := tallyOf(p.ws)
	out := []W{
		u.text(sizeSmall, u.p.Faint, "PROJECT"),
		u.wrap(8, []W{
			u.cell(money(t.spent), "spent on it", u.p.Text),
			u.cell(tokens(t.tokens), "tokens", u.p.Text),
			u.cell(fmt.Sprint(t.changes), "tasks", u.p.Text),
			u.cell(fmt.Sprint(t.landed), "landed", u.p.Text),
			u.cell(fmt.Sprintf("%d / %d", t.passing, t.checked), "pass the gate", u.p.Text),
			u.cell(fmt.Sprintf("%d / %d", t.met, t.met+t.failed+t.undecided), "criteria met", u.p.Text),
		}),
	}
	return out
}

// escalations is what the project's tasks left for a person, each opening
// the object that raised it.
func (u *UI) escalations(p *project) W {
	if p.led == nil || len(p.led.Owed) == 0 {
		return u.text(sizeSmall, u.p.Muted, "Nothing is owed: no escalation is open.")
	}
	rows := []W{u.text(sizeSmall, u.p.Faint, "OPEN ESCALATIONS — what a task left for a person")}
	for _, o := range p.led.Owed {
		name, _, _ := strings.Cut(o.Ref, "#")
		btn, settle := u.clicks.of("owed/"+p.path+o.Ref), u.clicks.of("owed/settle/"+p.path+o.Ref)
		rows = append(rows, func(gtx C) D {
			if u.clicked(gtx, settle) {
				p.task.SetText("Close " + o.Ref + ": " + o.Ground)
				p.settles = []string{o.Ref}
				p.selected, p.item = newKey, nil
				gtx.Execute(key.FocusCmd{Tag: &p.task})
			}
			if u.clicked(gtx, btn) {
				for _, c := range p.order {
					if c.Name() == name {
						p.show(c.Change.Dir, c)
						p.item = c.Item(o.Ref)
					}
				}
			}
			return column(4,
				func(gtx C) D {
					return u.pressable(gtx, btn, false, 4, stretch(inset(5, 6, column(2,
						u.mono(sizeSmall, u.p.Note, o.Ref),
						u.text(sizeSmall, u.p.Text, o.Ground)))))
				},
				func(gtx C) D {
					return u.pressable(gtx, settle, false, 6, box(u.p.AccentSoft, colorNRGBA{}, 6, 0,
						inset(4, 10, u.text(sizeSmall, u.p.Accent, "Close it with a new intent"))))
				})(gtx)
		})
	}
	return column(4, rows...)
}

// handed is what the framework gave the models to read, before anything
// they or their provider added.
func handed(cs []artifact.Cost) int {
	n := 0
	for _, c := range cs {
		n += c.Handed
	}
	return n
}
