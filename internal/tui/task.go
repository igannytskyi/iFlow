package tui

import (
	"cmp"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/x/ansi"

	"github.com/igannytskyi/iflow/internal/meta"
	"github.com/igannytskyi/iflow/internal/protocol"
	"github.com/igannytskyi/iflow/internal/service"
)

// task is the task on view: its title and its six areas, the one open
// below them, and the question it waits on.
func (m Model) task(width, height int) string {
	_, t, ok := m.current()
	if !ok {
		return "\n" + faint.Render("  no task here yet — press n and say what you want")
	}
	s, live := m.client.Run(t.run)
	doc := m.docs[m.dirOf(t)]
	title := oneLine(t.change.Statement)
	if live {
		title = oneLine(s.Statement)
	}
	if dir := m.dirOf(t); dir != "" {
		title = m.term.link(filepath.Join(dir, "change.md"), ansi.Truncate(title, width-2, "…"))
	}
	out := append(strings.Split(rule.Width(width).Render(title), "\n"), m.areas(t, s, live))
	h := height - len(out)
	var question []string
	if live && s.Question != "" {
		question = []string{waiting.Render("waiting for you · ") + oneLine(s.Question),
			faint.Render("[a] accept   [r] send back with why   [o] read the specification")}
		h -= len(question) + 1
	}
	var body []string
	switch m.area {
	case 1:
		body = m.spec(doc, width)
	case 2:
		body = m.plan(doc, width)
	case 3:
		body = m.admission(doc, width)
	case 4:
		body = m.execution(t, doc, s, live, width, h)
	case 5:
		body = m.assurance(doc, width)
	case 6:
		body = m.landing(doc, t, width)
	}
	if m.area != 4 {
		body = window(body, m.scroll, h)
	}
	out = append(out, body...)
	for len(out) < height-len(question) {
		out = append(out, "")
	}
	if len(question) > 0 {
		out = append(out[:height-len(question)], question...)
	}
	return strings.Join(out, "\n")
}

// window is the lines from scroll on that fit height.
func window(lines []string, scroll, height int) []string {
	scroll = min(scroll, max(0, len(lines)-height))
	return lines[scroll:min(len(lines), scroll+max(0, height))]
}

// areas is the six areas as they stand, the one open marked.
func (m Model) areas(t task, s service.State, live bool) string {
	var parts []string
	for n := 1; n <= 6; n++ {
		name := fmt.Sprintf("%d %s", n, areaNames[n])
		var st protocol.Status
		switch {
		case live:
			st = s.Areas[n]
		case n <= t.change.Reached:
			st = protocol.Done
		case n == t.change.Reached+1 && t.change.State == protocol.Stopped:
			st = protocol.Failed
		}
		switch st {
		case protocol.Done:
			name = good.Render(name + " ✓")
		case protocol.Working:
			name = active.Render(name + " ●")
		case protocol.Waiting:
			name = waiting.Render(name + " ?")
		case protocol.Failed:
			name = bad.Render(name + " ✗")
		default:
			name = faint.Render(name)
		}
		if n == m.area && m.focus == onTask {
			name = chosen.Render(ansi.Strip(name))
		}
		parts = append(parts, name)
	}
	return strings.Join(parts, "  ")
}

// rows is how many lines can be chosen in the area open: units, verdicts.
func (m Model) rows(t task) int {
	doc := m.docs[m.dirOf(t)]
	switch m.area {
	case 2, 4:
		if doc.Plan != nil {
			return len(doc.Plan.Units)
		}
		s, _ := m.client.Run(t.run)
		return len(s.Agent)
	}
	return 0
}

// spec is the change described, drawn as Markdown for the width.
func (m Model) spec(doc protocol.ChangeDoc, width int) []string {
	if doc.Document == "" {
		return []string{faint.Render("nothing is specified yet")}
	}
	key := fmt.Sprintf("%s@%d", doc.Change.Dir, width)
	out, ok := m.md[key]
	if !ok {
		r, err := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(width-2))
		if err == nil {
			out, err = r.Render(doc.Document)
		}
		if err != nil {
			out = doc.Document
		}
		m.md[key] = out
	}
	return strings.Split(strings.Trim(out, "\n"), "\n")
}

// plan is the units the change is divided into, the one chosen open on its
// brief.
func (m Model) plan(doc protocol.ChangeDoc, width int) []string {
	if doc.Plan == nil {
		return []string{faint.Render("not planned yet")}
	}
	var out []string
	for i, u := range doc.Plan.Units {
		line := fmt.Sprintf("%s  %-8s %s", u.ID, u.Kind, u.Title)
		if i == m.row {
			out = append(out, chosen.Render(ansi.Truncate(line, width, "…")))
			out = append(out, faint.Render("  scope: "+strings.Join(u.Scope, ", ")+"   criteria: "+strings.Join(u.Criteria, ", ")))
			for _, p := range u.Provides {
				out = append(out, faint.Render("  provides: "+p.Signature+" — "+p.File))
			}
			for _, l := range strings.Split(wrap(u.Brief, width-4), "\n") {
				out = append(out, "  "+l)
			}
			out = append(out, "")
			continue
		}
		out = append(out, line)
	}
	return out
}

// admission is what was decided of each unit before any was carried.
func (m Model) admission(doc protocol.ChangeDoc, width int) []string {
	if doc.Admission == nil {
		return []string{faint.Render("not admitted yet")}
	}
	var out []string
	for _, d := range doc.Admission.Decisions {
		out = append(out, fmt.Sprintf("%s  %s", d.Unit, d.Outcome), faint.Render("  "+oneLine(d.Condition)))
	}
	for _, c := range doc.Admission.Conflicts {
		out = append(out, "", waiting.Render("meet: "+c.Units), faint.Render("  "+oneLine(c.Region)+" — "+oneLine(c.Resolution)))
	}
	return out
}

// execution is each unit as it was carried, and what the agent of the one
// chosen did: live as its run tells it, or as its candidate keeps it.
func (m Model) execution(t task, doc protocol.ChangeDoc, s service.State, live bool, width, height int) []string {
	type unit struct{ id, kind, title, terminal, candidate string }
	var units []unit
	if doc.Plan != nil {
		for _, u := range doc.Plan.Units {
			units = append(units, unit{id: u.ID, kind: string(u.Kind), title: u.Title})
		}
	} else {
		for _, id := range slices.Sorted(maps.Keys(s.Agent)) {
			units = append(units, unit{id: id})
		}
	}
	if doc.Execution != nil {
		for _, r := range doc.Execution.Runs {
			if i := slices.IndexFunc(units, func(u unit) bool { return u.id == r.Unit }); i >= 0 {
				units[i].terminal, units[i].candidate = string(r.Terminal), r.Candidate
			}
		}
	}
	if len(units) == 0 {
		return []string{faint.Render("nothing carried yet")}
	}
	var out []string
	for i, u := range units {
		sign := faint.Render("○")
		switch {
		case u.terminal == string(meta.CandidateEnd) || u.candidate != "":
			sign = good.Render("✓")
		case u.terminal != "":
			sign = bad.Render("✗")
		case len(s.Agent[u.id]) > 0:
			sign = active.Render("●")
		}
		line := fmt.Sprintf("%s %s  %-8s %s", sign, u.id, u.kind, u.title)
		if i == m.row {
			line = chosen.Render(ansi.Strip(line))
		}
		out = append(out, ansi.Truncate(line, width, "…"))
	}
	out = append(out, "")
	chosenUnit := units[min(m.row, len(units)-1)]
	events := s.Agent[chosenUnit.id]
	where := "live"
	if !live || len(events) == 0 {
		events = m.traces[m.dirOf(t)+"/"+chosenUnit.candidate]
		where = chosenUnit.candidate
	}
	out = append(out, faint.Render(chosenUnit.id+" · its agent · "+cmp.Or(where, "no candidate")))
	room := max(1, height-len(out))
	for _, e := range events[max(0, len(events)-room):] {
		out = append(out, ansi.Truncate(event(e), width, "…"))
	}
	if len(events) == 0 {
		out = append(out, faint.Render("  no steps kept"))
	}
	return out
}

// traceOnView asks for the steps of the chosen unit's candidate, in the
// execution area, where no run tells them and they are not read yet.
func (m Model) traceOnView() tea.Cmd {
	if m.area != 4 || m.focus != onTask {
		return nil
	}
	_, t, ok := m.current()
	if !ok {
		return nil
	}
	dir := m.dirOf(t)
	doc := m.docs[dir]
	if doc.Plan == nil || doc.Execution == nil || m.row >= len(doc.Plan.Units) {
		return nil
	}
	id := doc.Plan.Units[m.row].ID
	if s, live := m.client.Run(t.run); live && len(s.Agent[id]) > 0 {
		return nil
	}
	for _, r := range doc.Execution.Runs {
		if r.Unit == id && r.Candidate != "" {
			if _, have := m.traces[dir+"/"+r.Candidate]; !have {
				return m.readTrace(dir, r.Candidate)
			}
		}
	}
	return nil
}

// event is one thing an agent did, in a line.
func event(e protocol.Event) string {
	detail := oneLine(e.Detail)
	switch e.Kind {
	case "called":
		return "› " + strings.ToLower(e.Tool) + " " + detail
	case "got":
		return faint.Render("  " + detail)
	case "refused":
		return bad.Render("  refused: " + detail)
	case "ended":
		return strong.Render("■ " + detail)
	}
	return faint.Render(e.Kind + " " + detail)
}

// assurance is the verdict on every criterion, with the criterion's words.
func (m Model) assurance(doc protocol.ChangeDoc, width int) []string {
	if doc.Assurance == nil {
		return []string{faint.Render("not judged yet")}
	}
	words := map[string]string{}
	if doc.Specification != nil {
		for _, c := range doc.Specification.Criteria {
			words[c.ID] = c.Criterion
		}
	}
	var out []string
	for _, v := range doc.Assurance.Verdicts {
		sign := good.Render("✓")
		if v.Outcome != meta.Met {
			sign = bad.Render("✗")
		}
		out = append(out, fmt.Sprintf("%s %s  %s  %s", sign, v.Criterion, v.Outcome, faint.Render(v.Unit+" · "+string(v.State))))
		if w := words[v.Criterion]; w != "" {
			out = append(out, faint.Render("  "+ansi.Truncate(oneLine(w), width-4, "…")))
		}
	}
	return out
}

// landing is each unit as it entered the estate, and the change's record.
func (m Model) landing(doc protocol.ChangeDoc, t task, width int) []string {
	var out []string
	if doc.Landing != nil {
		for _, e := range doc.Landing.Entries {
			out = append(out, fmt.Sprintf("%d %s  entered %s", e.Order, e.Unit, e.EnteredAt))
		}
		out = append(out, "")
	} else {
		out = append(out, faint.Render("not landed"), "")
	}
	out = append(out, faint.Render("record"))
	for _, x := range doc.Record {
		out = append(out, ansi.Truncate(fmt.Sprintf("%2d %s %-13s %s", x.N, x.At, x.Stage, oneLine(x.Note)), width, "…"))
	}
	if t.change.State == protocol.Stopped {
		out = append(out, "", faint.Render("[R] resume from where it stopped"))
	}
	return out
}

// costs is what the task on view spent: in all, by area, by model, and its
// dearest units; and how much the agents were handed.
func (m Model) costs(width int) string {
	_, t, ok := m.current()
	if !ok {
		return ""
	}
	doc := m.docs[m.dirOf(t)]
	var total, byArea [7]float64
	byModel := map[string]float64{}
	byUnit := map[string]float64{}
	in, outTok, handed, handedN := 0, 0, 0, 0
	for _, c := range doc.Costs {
		total[0] += c.CostUSD
		if c.Area >= 1 && c.Area <= 6 {
			byArea[c.Area] += c.CostUSD
		}
		byModel[cmp.Or(c.Model, c.Actor)] += c.CostUSD
		if c.Unit != "" {
			byUnit[c.Unit] += c.CostUSD
		}
		in += c.Input + c.CacheRead + c.CacheWrite
		outTok += c.Output
		if c.Handed > 0 {
			handed += c.Handed
			handedN++
		}
	}
	if s, live := m.client.Run(t.run); live && s.Spent.CostUSD > total[0] {
		total[0] = s.Spent.CostUSD
	}
	lines := []string{strong.Render("cost"), fmt.Sprintf("$%.3f", total[0]),
		faint.Render(fmt.Sprintf("%dk in · %dk out", in/1000, outTok/1000)), ""}
	if handedN > 0 {
		lines = append(lines, faint.Render("handed an agent"), fmt.Sprintf("%dk tokens on average", handed/handedN/1000), "")
	}
	lines = append(lines, faint.Render("by area"))
	for n := 1; n <= 6; n++ {
		if byArea[n] > 0 {
			lines = append(lines, fmt.Sprintf("%d %-8s $%.3f", n, areaNames[n], byArea[n]))
		}
	}
	lines = append(lines, "", faint.Render("by model"))
	for _, k := range sortedByCost(byModel) {
		lines = append(lines, ansi.Truncate(fmt.Sprintf("$%.3f %s", byModel[k], k), width, "…"))
	}
	if len(byUnit) > 0 {
		lines = append(lines, "", faint.Render("dearest units"))
		for _, k := range sortedByCost(byUnit)[:min(5, len(byUnit))] {
			lines = append(lines, fmt.Sprintf("$%.3f %s", byUnit[k], k))
		}
	}
	return strings.Join(lines, "\n")
}

func sortedByCost(m map[string]float64) []string {
	keys := slices.Collect(maps.Keys(m))
	slices.SortFunc(keys, func(a, b string) int { return cmp.Or(cmp.Compare(m[b], m[a]), cmp.Compare(a, b)) })
	return keys
}

// wrap is s in lines of at most width runes, by words.
func wrap(s string, width int) string {
	var out, line strings.Builder
	for _, para := range strings.Split(strings.TrimSpace(s), "\n") {
		for _, w := range strings.Fields(para) {
			if line.Len() > 0 && ansi.StringWidth(line.String())+1+ansi.StringWidth(w) > width {
				out.WriteString(line.String() + "\n")
				line.Reset()
			}
			if line.Len() > 0 {
				line.WriteByte(' ')
			}
			line.WriteString(w)
		}
		out.WriteString(line.String() + "\n")
		line.Reset()
	}
	return strings.TrimRight(out.String(), "\n")
}
