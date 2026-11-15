package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"gioui.org/layout"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/gate"
	"github.com/igannytskyi/iflow/internal/harness"
	"github.com/igannytskyi/iflow/internal/method"
	"github.com/igannytskyi/iflow/internal/model"
	"github.com/igannytskyi/iflow/internal/workspace"
)

func (u *UI) itemDetail(p *project, c *workspace.Change, it *workspace.Item) []W {
	areaName := fmt.Sprint(it.Area)
	if a := u.model.Area(it.Area); a != nil {
		areaName = fmt.Sprintf("%02d %s", a.N, a.Name)
	}
	where := string(it.Stage) + ".md"
	switch it.Kind {
	case "Crossing":
		where = "record.md"
	case "CostRecord":
		where = artifact.CostFile
	}
	out := []W{column(4,
		u.text(sizeSmall, u.p.Faint, strings.ToUpper(it.Kind)),
		u.strong(sizeHead, u.p.Text, it.ID),
		u.text(sizeSmall, u.p.Muted, "produced in "+areaName+" · "+where),
	)}
	if o := u.model.Object(it.Kind); o != nil {
		out = append(out, box(u.p.Sunken, colorNRGBA{}, 6, 10, column(4,
			u.text(sizeSmall, u.p.Text, o.What),
			u.text(sizeSmall, u.p.Muted, "valid: "+o.Validity))))
	}
	if len(it.Findings) > 0 {
		var fs []W
		for _, f := range it.Findings {
			fs = append(fs, u.finding(f))
		}
		out = append(out, column(8, append([]W{u.text(sizeSmall, u.p.Faint, "THE GATE SAYS")}, fs...)...))
	} else if c.State == workspace.Checked {
		out = append(out, row(6, dot(u.p.OK, 7), u.text(sizeSmall, u.p.Muted, "the gate says nothing against this object")))
	}
	var fields []W
	for _, f := range it.Fields {
		if f.Value != "" {
			fields = append(fields, column(2, u.text(sizeSmall, u.p.Faint, f.Name), u.text(sizeBody, u.p.Text, method.Plain(f.Value))))
		}
	}
	out = append(out, column(10, append([]W{u.text(sizeSmall, u.p.Faint, "CELLS")}, fields...)...))
	if it.Kind == "Candidate" || it.Kind == "Trace" {
		unit := it.Field("Unit")
		if events, ok := u.tracesOf(c)[unit]; ok {
			out = append(out, column(6, u.text(sizeSmall, u.p.Faint, "WHAT THE EXECUTOR DID"),
				u.agentFeed(p, unit, events, forUnit(inArea(c.Costs, 4), unit))))
		}
	}
	out = append(out, u.links(p, "CITES", c, it.Refs))
	var citedBy []string
	for _, l := range c.Links() {
		if l[1] == it {
			citedBy = append(citedBy, l[0].ID)
		}
	}
	out = append(out, u.links(p, "CITED BY", c, citedBy))
	return out
}

// finding is what the gate said, with the rule it said it under — and the
// rule's own words beside it, which is where a reader needs them.
func (u *UI) finding(f gate.Finding) W {
	bg, fg := u.p.FailSoft, u.p.Fail
	if f.Severity == gate.Note {
		bg, fg = u.p.NoteSoft, u.p.Note
	}
	rule := ""
	if r := u.model.Rule(f.Rule); r != nil {
		rule = r.Text
	}
	return func(gtx C) D {
		return layout.Flex{Gap: gtx.Dp(10)}.Layout(gtx,
			layout.Rigid(u.chip(bg, fg, f.Rule)),
			layout.Flexed(1, column(2, u.text(sizeSmall, u.p.Text, f.Message), u.text(sizeSmall, u.p.Faint, rule))),
		)
	}
}

func (u *UI) links(p *project, title string, c *workspace.Change, ids []string) W {
	var ws []W
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		to, id := c.Item(id), id
		ws = append(ws, func(gtx C) D {
			if to == nil {
				return u.chip(u.p.Sunken, u.p.Faint, id+" (not defined here)")(gtx)
			}
			btn := u.clicks.of("link/" + p.path + title + id)
			if btn.Clicked(gtx) {
				p.item, p.handedView = to, ""
			}
			return u.pressable(gtx, btn, false, 4, u.chip(u.p.AccentSoft, u.p.Accent, id))
		})
	}
	if len(ws) == 0 {
		ws = append(ws, u.text(sizeSmall, u.p.Faint, "nothing"))
	}
	return column(6, u.text(sizeSmall, u.p.Faint, title), u.wrap(6, ws))
}

// objectChip is an object, opened by a click.
func (u *UI) objectChip(p *project, it *workspace.Item) W {
	return func(gtx C) D {
		btn := u.clicks.of("obj/" + p.path + "/" + it.Kind + "/" + it.ID)
		if btn.Clicked(gtx) {
			p.item, p.handedView = it, ""
		}
		bg, fg := u.p.Sunken, u.p.Text
		if p.item != nil && p.item.ID == it.ID && p.item.Kind == it.Kind {
			bg, fg = u.p.AccentSoft, u.p.Accent
		}
		if len(it.Findings) > 0 {
			fg = u.p.Fail
		}
		return u.pressable(gtx, btn, false, 4, box(bg, colorNRGBA{}, 4, 0, inset(3, 7, row(5,
			u.text(sizeSmall, u.p.Faint, it.Kind), u.mono(sizeSmall, fg, it.ID)))))
	}
}

// agentFeed is what one unit's executor did, step by step, with what each
// model turn cost — the latest steps, or every one when opened.
func (u *UI) agentFeed(p *project, unit string, events []harness.Event, costs []artifact.Cost) W {
	return func(gtx C) D {
		toggle := u.clicks.of("feed/" + p.path + "/" + unit)
		if toggle.Clicked(gtx) {
			p.open[unit] = !p.open[unit]
		}
		open := p.open[unit]
		var steps []harness.Event
		for _, e := range events {
			if e.Kind != harness.Spent || !e.Usage.Zero() {
				steps = append(steps, e)
			}
		}
		start := 0
		if !open {
			start = max(0, len(steps)-14)
		}
		summary := costLine(costs)
		if summary == "" {
			var live model.Usage
			for _, e := range events {
				if e.Kind != harness.Ended {
					live = live.Add(e.Usage)
				}
			}
			if !live.Zero() {
				summary = usageLine(live) + " so far"
			}
		}
		label := "show every step"
		if open {
			label = "show the last steps"
		}
		rows := []W{row(8, u.mono(sizeSmall, u.p.Accent, unit), u.text(sizeSmall, u.p.Text, summary),
			func(gtx C) D {
				return u.pressable(gtx, toggle, false, 4, inset(2, 6, u.text(sizeSmall, u.p.Accent, fmt.Sprintf("%s (%d)", label, len(steps)))))
			})}
		for _, e := range steps[start:] {
			c, label := u.p.Muted, string(e.Kind)
			switch e.Kind {
			case harness.Called:
				c, label = u.p.Text, e.Tool
			case harness.Refused:
				c = u.p.Fail
			case harness.Spent:
				label = "turn"
			}
			cost := ""
			if !e.Usage.Zero() && e.Kind != harness.Ended {
				cost = "in " + tokens(e.Usage.In()) + " · out " + tokens(e.Usage.Output)
				if e.Usage.CostUSD > 0 {
					cost += " · " + money(e.Usage.CostUSD)
				}
			}
			e := e
			rows = append(rows, func(gtx C) D {
				return layout.Flex{Gap: gtx.Dp(8)}.Layout(gtx,
					layout.Rigid(u.mono(sizeSmall, u.p.Faint, e.At.Format("15:04:05"))),
					layout.Rigid(width(70, u.oneLine(sizeSmall, c, label))),
					layout.Flexed(1, u.oneLine(sizeSmall, u.p.Muted, strings.ReplaceAll(e.Detail, "\n", " "))),
					layout.Rigid(width(150, u.oneLine(sizeSmall, u.p.Faint, cost))),
				)
			})
		}
		return box(u.p.Bg, colorNRGBA{}, 6, 8, column(3, rows...))(gtx)
	}
}

// tracesOf reads the steps each unit's executor kept beside its candidate.
func (u *UI) tracesOf(c *workspace.Change) map[string][]harness.Event {
	out := map[string][]harness.Event{}
	for _, it := range c.Items {
		if it.Kind != "Candidate" {
			continue
		}
		dir := filepath.Join(c.Change.Dir, filepath.FromSlash(strings.TrimSuffix(it.Field("Stored at"), "/")))
		if events, err := harness.ReadTrace(filepath.Join(dir, harness.TraceFile)); err == nil {
			out[it.Field("Unit")] = events
		}
	}
	return out
}
