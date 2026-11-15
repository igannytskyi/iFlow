package ui

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/flow"
)

// foundationView is one of areas 7 to 13 opened from the overview: not its
// totals but everything they are totals of.
func (u *UI) foundationView(gtx C, p *project) D {
	n := p.foundation
	a := u.model.Area(n)
	back := u.clicks.of("foundation/back/" + p.path)
	body := []W{
		func(gtx C) D {
			if u.clicked(gtx, back) {
				p.foundation = 0
			}
			return u.pressable(gtx, back, false, 4, inset(3, 6, u.text(sizeSmall, u.p.Muted, "← "+p.name)))
		},
		row(10, u.mono(sizeSmall, u.p.Accent, fmt.Sprintf("%02d", n)), u.strong(sizeHead, u.p.Text, a.Name)),
		u.text(sizeSmall, u.p.Muted, a.Aim),
		u.hairline,
	}
	body = append(body, memo(u.docs, fmt.Sprintf("foundation|%s|%d", p.path, n), func() []W { return u.foundationDetail(p, n) })...)
	return u.scroll(gtx, fmt.Sprintf("foundation/%s/%d", p.path, n), body)
}

func (u *UI) foundationDetail(p *project, n int) []W {
	e := p.estate.Load()
	table := func(headers []string, rows [][]string, widths []int) W {
		return func(gtx C) D {
			line := func(cells []string, c colorNRGBA, strong bool) W {
				return func(gtx C) D {
					var kids []layout.FlexChild
					for i, cell := range cells {
						w := u.text(sizeSmall, c, cell)
						if strong {
							w = u.text(sizeSmall, u.p.Faint, strings.ToUpper(cell))
						}
						if i < len(widths) && widths[i] > 0 {
							kids = append(kids, layout.Rigid(width(unit.Dp(widths[i]), w)))
						} else {
							kids = append(kids, layout.Flexed(1, w))
						}
					}
					return layout.Flex{Gap: gtx.Dp(12)}.Layout(gtx, kids...)
				}
			}
			ws := []W{line(headers, u.p.Faint, true)}
			for _, r := range rows {
				ws = append(ws, line(r, u.p.Text, false))
			}
			if len(rows) == 0 {
				ws = append(ws, u.text(sizeSmall, u.p.Faint, "nothing yet"))
			}
			return box(u.p.Surface, u.p.Line, 8, 12, column(6, ws...))(gtx)
		}
	}
	var out []W
	switch n {
	case 7:
		if e == nil || e.err != "" {
			return []W{u.text(sizeBody, u.p.Muted, "The estate has not been read.")}
		}
		var regions [][]string
		for _, g := range e.regionRows {
			verdict := g.Verdict
			if g.TestGround {
				verdict = "where tests are kept"
			}
			regions = append(regions, []string{g.Region, fmt.Sprint(g.Symbols), fmt.Sprintf("%d (%.0f%%)", g.Named, g.Share*100), verdict})
		}
		var files [][]string
		for _, f := range e.fileNames {
			files = append(files, []string{f[0], f[1]})
		}
		out = append(out, u.text(sizeSmall, u.p.Faint, "REGIONS — directories holding code, and whether any test names what they define"),
			table([]string{"Region", "Names", "Named by a test", "Verdict"}, regions, []int{220, 70, 120, 0}),
			u.text(sizeSmall, u.p.Faint, "FILES — every source file and the names it defines; what an author is shown instead of the code"),
			table([]string{"File", "Defines"}, files, []int{260, 0}))
	case 8:
		var rows [][]string
		for _, c := range p.order {
			for _, it := range c.Items {
				if it.Kind == "Crossing" {
					rows = append(rows, []string{it.Field("At"), c.Name(), it.Field("Stage"), it.Field("Object"), it.Field("Note")})
				}
			}
		}
		out = append(out, u.text(sizeSmall, u.p.Faint, "EVERY CROSSING — newest task first, each in the order it was recorded"),
			table([]string{"At", "Task", "Stage", "Object", "Note"}, rows, []int{90, 170, 100, 110, 0}))
	case 9:
		var tasks [][]string
		byArea := map[int]artifact.Cost{}
		byModel := map[string]artifact.Cost{}
		for _, c := range p.order {
			t := artifact.Total(c.Costs)
			h := handed(c.Costs)
			tasks = append(tasks, []string{c.Name(), money(t.CostUSD), tokens(t.Input + t.CacheWrite), tokens(t.CacheRead), tokens(t.Output), tokens(h), fmt.Sprint(len(c.Costs))})
			for _, k := range c.Costs {
				a := byArea[k.Area]
				byArea[k.Area] = addCost(a, k)
				m := k.Model
				if m == "" {
					m = k.Actor
				}
				byModel[m] = addCost(byModel[m], k)
			}
		}
		var areas, ms [][]string
		for a := 0; a <= 13; a++ {
			if k, ok := byArea[a]; ok {
				name := "the whole run"
				if ar := u.model.Area(a); ar != nil {
					name = fmt.Sprintf("%02d %s", a, ar.Name)
				}
				areas = append(areas, []string{name, money(k.CostUSD), tokens(k.Input + k.CacheWrite + k.CacheRead), tokens(k.Output), elapsed(k.ElapsedMS)})
			}
		}
		for _, m := range sortedStrings(byModel) {
			k := byModel[m]
			ms = append(ms, []string{m, money(k.CostUSD), tokens(k.Input + k.CacheWrite + k.CacheRead), tokens(k.Output)})
		}
		out = append(out, u.text(sizeSmall, u.p.Faint, "BY TASK"),
			table([]string{"Task", "Spent", "Read fresh", "From cache", "Written", "Handed", "Records"}, tasks, []int{200, 70, 90, 90, 80, 80, 0}),
			u.text(sizeSmall, u.p.Faint, "BY AREA, ACROSS TASKS"),
			table([]string{"Area", "Spent", "Read", "Written", "Time"}, areas, []int{220, 80, 90, 90, 0}),
			u.text(sizeSmall, u.p.Faint, "BY MODEL OR WHAT DID THE WORK"),
			table([]string{"Model", "Spent", "Read", "Written"}, ms, []int{260, 80, 90, 0}))
	case 10:
		var decisions [][]string
		for _, l := range u.lines(filepath.Join(p.path, "changes", flow.DecisionsFile)) {
			if strings.HasPrefix(l, "- ") {
				decisions = append(decisions, []string{strings.TrimPrefix(l, "- ")})
			}
		}
		var learned [][]string
		if e != nil {
			for _, c := range e.learned {
				learned = append(learned, []string{c.At, strings.Join(c.From, ", "), strings.Join(c.To, ", "), c.Why})
			}
		}
		out = append(out, u.text(sizeSmall, u.p.Faint, "YOUR DECISIONS — every later author is handed them"),
			table([]string{"Decision"}, decisions, []int{0}),
			u.text(sizeSmall, u.p.Faint, "EDGES LEARNED FROM LANDINGS — ground a change reached that reading did not find"),
			table([]string{"At", "From", "Reaches", "Why"}, learned, []int{90, 200, 200, 0}))
	case 11:
		var owed, settled, sentBack, reviews [][]string
		if p.led != nil {
			for _, o := range p.led.Owed {
				owed = append(owed, []string{o.Ref, o.Ground})
			}
			for _, ref := range sortedStrings(p.led.Settled) {
				settled = append(settled, []string{ref, p.led.Settled[ref]})
			}
		}
		for _, c := range p.order {
			for _, it := range c.Items {
				note := it.Field("Note")
				if it.Kind == "Crossing" && (strings.Contains(note, "sent back") || strings.Contains(note, "refused")) {
					sentBack = append(sentBack, []string{it.Field("At"), c.Name(), note})
				}
			}
			files := u.docs.glob(filepath.Join(c.Change.Dir, "reviews", "*.md"))
			for _, f := range files {
				lines := u.docs.read(f)
				if len(lines) > 2 {
					reviews = append(reviews, []string{c.Name(), strings.TrimPrefix(firstLine(lines), "# "), strings.TrimSpace(lines[2])})
				}
			}
		}
		out = append(out, u.text(sizeSmall, u.p.Faint, "ESCALATIONS OPEN — the system asking for a person"),
			table([]string{"Escalation", "Ground"}, owed, []int{220, 0}),
			u.text(sizeSmall, u.p.Faint, "SETTLED"), table([]string{"Escalation", "Settled by"}, settled, []int{220, 0}),
			u.text(sizeSmall, u.p.Faint, "SENT BACK OR REFUSED — a person acting"), table([]string{"At", "Task", "Note"}, sentBack, []int{90, 170, 0}),
			u.text(sizeSmall, u.p.Faint, "AGENT REVIEWS — consumers refusing before the person"), table([]string{"Task", "Review", "Verdict"}, reviews, []int{170, 220, 0}))
	case 12:
		var accepted, decisions [][]string
		for _, c := range p.order {
			for _, it := range c.Items {
				switch it.Kind {
				case "Review":
					accepted = append(accepted, []string{c.Name(), it.Field("By"), it.Field("At"), it.Field("Reviewed before admission"), it.Field("Note")})
				case "AdmissionDecision":
					decisions = append(decisions, []string{c.Name(), it.ID, it.Field("Outcome"), it.Field("Deciding condition")})
				}
			}
		}
		out = append(out, u.text(sizeSmall, u.p.Faint, "WHO ACCEPTED WHAT"),
			table([]string{"Task", "By", "At", "Read first", "Note"}, accepted, []int{170, 220, 90, 80, 0}),
			u.text(sizeSmall, u.p.Faint, "WHAT WAS ADMITTED, AND ON WHAT"),
			table([]string{"Task", "Unit", "Outcome", "Deciding condition"}, decisions, []int{170, 110, 90, 0}))
	case 13:
		var rows [][]string
		for _, c := range p.order {
			gate := "checking"
			if c.Report.Passed() {
				gate = "passes"
			} else if c.Report.Problems() != nil {
				gate = plural(len(c.Report.Problems()), "problem")
			}
			met, failed, other, rep, ev := 0, 0, 0, 0, 0
			for _, it := range c.Items {
				switch it.Kind {
				case "Verdict":
					switch it.Field("Outcome") {
					case "met":
						met++
					case "failed":
						failed++
					default:
						other++
					}
				case "Evidence":
					ev++
					if it.Field("Repeatable") == "yes" {
						rep++
					}
				}
			}
			rows = append(rows, []string{c.Name(), gate, fmt.Sprint(met), fmt.Sprint(failed), fmt.Sprint(other), fmt.Sprintf("%d / %d", rep, ev)})
		}
		slices.SortStableFunc(rows, func(a, b []string) int { return strings.Compare(a[1], b[1]) })
		out = append(out, u.text(sizeSmall, u.p.Faint, "EVERY TASK — what the gate says, and what its verdicts rest on"),
			table([]string{"Task", "Gate", "Met", "Failed", "Undecided", "Repeatable"}, rows, []int{200, 110, 50, 60, 80, 0}))
	}
	return out
}

func addCost(a, k artifact.Cost) artifact.Cost {
	a.Input += k.Input
	a.CacheRead += k.CacheRead
	a.CacheWrite += k.CacheWrite
	a.Output += k.Output
	a.CostUSD += k.CostUSD
	a.ElapsedMS += k.ElapsedMS
	return a
}
