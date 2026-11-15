package ui

import (
	"fmt"
	"image"
	"slices"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/igannytskyi/iflow/internal/artifact"
)

// statsPanel stands beside every task: what the task has cost, area by area,
// and below it the object opened, or what the whole project has cost and
// still owes. It moves while a task runs.
func (u *UI) statsPanel(p *project, s shown, ok bool) W {
	return func(gtx C) D {
		fill(gtx, u.p.Surface)
		var body []W
		if ok {
			body = append(body, u.taskStats(p, s)...)
		}
		if p.item != nil && s.c != nil {
			body = append(body, u.hairline)
			body = append(body, u.objectPanel(p, s)...)
		} else {
			if ok {
				body = append(body, u.hairline)
			}
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
	out1 := []W{
		func(gtx C) D {
			if hide.Clicked(gtx) {
				u.settings.HideCosts = true
				u.settings.save()
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, u.text(sizeSmall, u.p.Faint, "THIS TASK")),
				layout.Rigid(func(gtx C) D {
					return u.pressable(gtx, hide, false, 4, inset(2, 6, u.text(sizeSmall, u.p.Muted, "fold ›")))
				}))
		},
		column(1, u.strong(unit.Sp(30), u.p.Text, money(spentUSD)), u.text(sizeSmall, u.p.Faint, "spent")),
		u.wrap(8, []W{
			u.cell(tokens(in), "tokens in", u.p.Text),
			u.cell(tokens(cached), "of them cached", u.p.Muted),
			u.cell(tokens(out), "tokens out", u.p.Text),
			u.cell(fmt.Sprint(t.Steps), "agent steps", u.p.Text),
			u.cell(clock, "elapsed", u.p.Text),
			u.cell(fmt.Sprint(len(s.costs)), "model calls", u.p.Text),
			u.cell(tokens(handed(s.costs)), "handed by the framework", u.p.Accent),
		}),
		column(1, u.text(sizeBody, u.p.Text, who), u.text(sizeSmall, u.p.Faint, "models")),
	}
	// What each area cost, as a share of the most any area cost.
	most := 0.0
	for n := 1; n <= 6; n++ {
		most = max(most, artifact.Total(inArea(s.costs, n)).CostUSD)
	}
	var rows []W
	for n := 1; n <= 6; n++ {
		a := artifact.Total(inArea(s.costs, n))
		name := u.model.Area(n).Name
		share := 0.0
		if most > 0 {
			share = a.CostUSD / most
		}
		btn := u.clicks.of(fmt.Sprintf("stats/area/%s/%d", p.path, n))
		rows = append(rows, func(gtx C) D {
			if btn.Clicked(gtx) {
				p.area = n
			}
			value := "—"
			if a.CostUSD > 0 || a.ElapsedMS > 0 {
				value = money(a.CostUSD)
				if a.ElapsedMS > 0 {
					value += " · " + elapsed(a.ElapsedMS)
				}
			}
			return u.pressable(gtx, btn, p.area == n, 4, inset(4, 6, column(3,
				func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, u.oneLine(sizeSmall, u.p.Text, fmt.Sprintf("%02d  %s", n, name))),
						layout.Rigid(u.oneLine(sizeSmall, u.p.Muted, value)))
				},
				u.meter(share),
			)))
		})
	}
	return append(out1, column(2, append([]W{u.text(sizeSmall, u.p.Faint, "BY AREA")}, rows...)...))
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
		if closer.Clicked(gtx) {
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
	return append(out, u.escalations(p))
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
		btn := u.clicks.of("owed/" + p.path + o.Ref)
		rows = append(rows, func(gtx C) D {
			if btn.Clicked(gtx) {
				for _, c := range p.order {
					if c.Name() == name {
						p.show(c.Change.Dir, c)
						p.item = c.Item(o.Ref)
					}
				}
			}
			return u.pressable(gtx, btn, false, 4, stretch(inset(5, 6, column(2,
				u.mono(sizeSmall, u.p.Note, o.Ref),
				u.text(sizeSmall, u.p.Text, o.Ground)))))
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
