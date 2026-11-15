package ui

import (
	"fmt"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/igannytskyi/iflow/internal/gate"
	"github.com/igannytskyi/iflow/internal/method"
	"github.com/igannytskyi/iflow/internal/workspace"
)

// tally is the figures across a project's changes, as far as the gate has
// answered.
type tally struct {
	changes, landed, passing, checked           int
	criteria                                    int
	met, failed, undecided                      int
	byExecutor, byPrecommitment, notIndependent int
	reviewed, unreviewed, owed                  int
	spent                                       float64
	tokens                                      int
}

func tallyOf(ws *workspace.Workspace) tally {
	var t tally
	for _, c := range ws.Changes {
		t.changes++
		t.spent += c.Spent.CostUSD
		t.tokens += c.Spent.In() + c.Spent.Output
		if c.Reached() == 6 {
			t.landed++
		}
		if c.State == workspace.Checked {
			t.checked++
			if c.Report.Passed() {
				t.passing++
			}
		}
		for _, it := range c.Items {
			switch it.Kind {
			case "Criterion":
				t.criteria++
			case "Evidence":
				switch it.Field("Independence") {
				case "independent-by-executor":
					t.byExecutor++
				case "independent-by-precommitment":
					t.byPrecommitment++
				case "not-independent":
					t.notIndependent++
				}
			case "Verdict":
				switch it.Field("Outcome") {
				case "met":
					t.met++
				case "failed":
					t.failed++
				case "undecided":
					t.undecided++
				}
			case "Review":
				if it.Field("Reviewed before admission") == "yes" {
					t.reviewed++
				} else {
					t.unreviewed++
				}
			case "Escalation":
				if it.Field("Standing") == "open" {
					t.owed++
				}
			}
		}
	}
	return t
}

// scroll is a padded, scrolling column of widgets.
func (u *UI) scroll(gtx C, name string, body []W) D {
	l := u.lists.of(name)
	return material.List(u.th, l).Layout(gtx, len(body), func(gtx C, i int) D {
		top := unit.Dp(10)
		if i == 0 {
			top = gutter
		}
		return layout.Inset{Top: top, Bottom: 2, Left: gutter, Right: gutter}.Layout(gtx, body[i])
	})
}

func (u *UI) gateBadge(c *workspace.Change) W {
	switch {
	case c.State != workspace.Checked:
		return u.text(sizeSmall, u.p.Faint, "checking…")
	case c.Report.Passed():
		return u.chip(u.p.OKSoft, u.p.OK, "passed")
	}
	return u.chip(u.p.FailSoft, u.p.Fail, plural(len(c.Report.Problems()), "problem"))
}

func intentOf(c *workspace.Change) string {
	for _, it := range c.Items {
		if it.Kind == "Intent" {
			return it.Field("Statement")
		}
	}
	return ""
}

func (u *UI) itemRow(p *project, it *workspace.Item) W {
	return func(gtx C) D {
		btn := u.clicks.of("item/" + p.path + "/" + it.Kind + "/" + it.ID)
		if btn.Clicked(gtx) {
			p.item = it
		}
		fails, notes := 0, 0
		for _, f := range it.Findings {
			if f.Severity == gate.Fail {
				fails++
			} else {
				notes++
			}
		}
		badge := nothing
		switch {
		case fails > 0:
			badge = u.chip(u.p.FailSoft, u.p.Fail, fmt.Sprint(fails))
		case notes > 0:
			badge = u.chip(u.p.NoteSoft, u.p.Note, fmt.Sprint(notes))
		}
		return u.pressable(gtx, btn, p.item == it, 6, stretch(inset(6, 8, func(gtx C) D {
			return layout.Flex{Gap: gtx.Dp(10), Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(width(130, u.oneLine(sizeSmall, u.p.Muted, it.Kind))),
				layout.Rigid(width(150, u.oneLine(sizeSmall, u.p.Text, it.ID))),
				layout.Flexed(1, u.oneLine(sizeSmall, u.p.Muted, summary(it))),
				layout.Rigid(badge),
			)
		})))
	}
}

// summary is the one value that says most about an item.
func summary(it *workspace.Item) string {
	if it.Kind == "CostRecord" {
		parts := []string{it.Field("Money")}
		if m := it.Field("Model"); m != "" {
			parts = append(parts, m)
		}
		return strings.Join(append(parts, it.Field("Elapsed"), it.Field("What")), " · ")
	}
	for _, k := range []string{"Outcome", "Terminal state", "Independence", "Class", "Standing", "Reviewed before admission"} {
		if v := it.Field(k); v != "" {
			if t := it.Title(); t != "" {
				return v + " · " + t
			}
			return v
		}
	}
	if t := it.Title(); t != "" {
		return t
	}
	if it.Kind == "Crossing" {
		return it.Field("Stage") + " · " + it.Field("Object") + " " + method.Plain(it.Field("Note"))
	}
	return ""
}
