package tui

import (
	"cmp"
	"fmt"
	"maps"
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
		return "\n" + m.st.faint.Render("  no task here yet — press n and say what you want")
	}
	out := m.head(t, width)
	question := m.question(t, width)
	lines, _, _ := m.body(t, width, m.room(t, width, height))
	out = append(out, lines...)
	for len(out) < height-len(question) {
		out = append(out, "")
	}
	return strings.Join(append(out[:max(0, height-len(question))], question...), "\n")
}

// areaHit is where an area's name is drawn in the head: its line, and the
// columns it spans.
type areaHit struct{ line, from, to, area int }

// headOf is the task's six areas, as many to a line as width holds — a
// name never cut — then a blank line; and where each name is, for the
// mouse to find it where it was drawn. What the task is, the list beside
// it says already.
func (m Model) headOf(t task, width int) ([]string, []areaHit) {
	var lines []string
	var hits []areaHit
	cur, x := "", 0
	for i, l := range m.areaLabels(t) {
		w := ansi.StringWidth(l)
		if cur != "" && x+2+w > width {
			lines, cur, x = append(lines, cur), "", 0
		}
		if cur != "" {
			cur, x = cur+"  ", x+2
		}
		hits = append(hits, areaHit{line: len(lines), from: x, to: x + w, area: i + 1})
		cur, x = cur+l, x+w
	}
	return append(lines, cur, ""), hits
}

func (m Model) head(t task, width int) []string {
	lines, _ := m.headOf(t, width)
	return lines
}

// question is what the task waits on, at the foot of the task, if anything.
func (m Model) question(t task, width int) []string {
	s, live := m.client.Run(t.run)
	if !live || s.Question == "" {
		return nil
	}
	return append(fold(m.st.waitBlock.Render("waiting for you")+" "+oneLine(s.Question), width, ""),
		m.st.faint.Render("[a] accept   [r] send back with why   [o] read the specification"))
}

// room is the lines the area open has, of height: what the head and the
// question leave — the same for drawing it and for finding what the mouse
// points at.
func (m Model) room(t task, width, height int) int {
	return max(0, height-len(m.head(t, width))-len(m.question(t, width)))
}

// body is what the area open holds, as it fits height: its lines, the row
// each line chooses (-1 for none), and how far it is scrolled.
func (m Model) body(t task, width, height int) ([]string, []int, int) {
	lines, rows, scrolls := m.content(t, width, height)
	if !scrolls {
		return lines, rows, 0
	}
	scroll := min(m.scroll, max(0, len(lines)-height))
	end := min(len(lines), scroll+max(0, height))
	if rows != nil {
		rows = rows[scroll:end]
	}
	return lines[scroll:end], rows, scroll
}

// content is all the area open holds, its rows, and whether it scrolls: the
// agent's steps keep to the room left, and do not.
func (m Model) content(t task, width, height int) ([]string, []int, bool) {
	doc := m.docs[m.dirOf(t)]
	s, live := m.client.Run(t.run)
	var lines []string
	var rows []int
	switch m.area {
	case 1:
		lines = m.spec(doc, width)
	case 2:
		lines, rows = m.plan(doc, width)
	case 3:
		lines = m.admission(doc, width)
	case 4:
		lines, rows = m.execution(t, doc, s, live, width, height)
		return lines, rows, false
	case 5:
		lines = m.assurance(doc, width)
	case 6:
		lines = m.landing(doc, t, width)
	}
	return lines, rows, true
}

// scrolled is the task's area scrolled by n lines (back where n is
// negative), held to what it holds: never past its last line, so that
// scrolling back moves at once.
func (m Model) scrolled(t task, n int) Model {
	l := m.layout()
	room := m.room(t, l.mid, l.body)
	lines, _, _ := m.content(t, l.mid, room)
	m.scroll = min(max(0, m.scroll+n), max(0, len(lines)-room))
	return m
}

// areaLabels is the six areas as they stand, each a block of the colour of
// how it stands, the one open marked.
func (m Model) areaLabels(t task) []string {
	s, live := m.client.Run(t.run)
	var out []string
	for n := 1; n <= 6; n++ {
		var st protocol.Status
		switch {
		case live:
			st = s.Areas[n]
		case n <= t.change.Reached:
			st = protocol.Done
		case n == t.change.Reached+1 && t.change.State == protocol.Stopped:
			st = protocol.Failed
		}
		name := fmt.Sprintf("%d %s", n, areaNames[n])
		style := m.st.idleBlock
		switch st {
		case protocol.Done:
			style, name = m.st.goodBlock, name+" ✓"
		case protocol.Working:
			style, name = m.st.activeBlock, name+" ●"
		case protocol.Waiting:
			style, name = m.st.waitBlock, name+" ?"
		case protocol.Failed:
			style, name = m.st.badBlock, name+" ✗"
		}
		if n == m.area {
			style = m.st.chosen.Bold(true).Padding(0, 1) // the area open, lit as all that is chosen is
		}
		out = append(out, style.Render(name))
	}
	return out
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
		return []string{m.st.faint.Render("nothing is specified yet")}
	}
	key := fmt.Sprintf("%s@%d", doc.Change.Dir, width)
	out, ok := m.md[key]
	if !ok {
		style := glamour.WithAutoStyle()
		if m.theme == Light {
			style = glamour.WithStandardStyle("light") // the terminal is set to white, whatever it says
		}
		r, err := glamour.NewTermRenderer(style, glamour.WithWordWrap(width-2))
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
// brief, and the unit each line chooses.
func (m Model) plan(doc protocol.ChangeDoc, width int) ([]string, []int) {
	if doc.Plan == nil {
		return []string{m.st.faint.Render("not planned yet")}, []int{-1}
	}
	var out []string
	var rows []int
	add := func(row int, lines ...string) {
		for _, l := range lines {
			out, rows = append(out, l), append(rows, row)
		}
	}
	for i, u := range doc.Plan.Units {
		line := fmt.Sprintf("%s  %-8s %s", u.ID, u.Kind, u.Title)
		if i != m.row {
			add(i, fold(line, width, "  ")...)
			continue
		}
		for _, l := range fold(line, width, "  ") {
			add(i, m.lit(l, width))
		}
		for _, l := range fold("scope: "+strings.Join(u.Scope, ", ")+"   criteria: "+strings.Join(u.Criteria, ", "), width-2, "  ") {
			add(i, m.st.faint.Render("  "+l))
		}
		for _, p := range u.Provides {
			for _, l := range fold("provides: "+p.Signature+" — "+p.File, width-2, "  ") {
				add(i, m.st.faint.Render("  "+l))
			}
		}
		for _, l := range strings.Split(wrap(u.Brief, width-4), "\n") {
			add(i, "  "+l)
		}
		add(i, "")
	}
	return out, rows
}

// admission is what was decided of each unit before any was carried.
func (m Model) admission(doc protocol.ChangeDoc, width int) []string {
	if doc.Admission == nil {
		return []string{m.st.faint.Render("not admitted yet")}
	}
	var out []string
	for _, d := range doc.Admission.Decisions {
		out = append(out, fmt.Sprintf("%s  %s", d.Unit, d.Outcome))
		for _, l := range fold(oneLine(d.Condition), width-2, "") {
			out = append(out, m.st.faint.Render("  "+l))
		}
	}
	for _, c := range doc.Admission.Conflicts {
		out = append(out, "", m.st.waiting.Render("meet: "+c.Units))
		for _, l := range fold(oneLine(c.Region)+" — "+oneLine(c.Resolution), width-2, "") {
			out = append(out, m.st.faint.Render("  "+l))
		}
	}
	return out
}

// execution is each unit as it was carried, and what the agent of the one
// chosen did: live as its run tells it, or as its candidate keeps it; and
// the unit each line chooses.
func (m Model) execution(t task, doc protocol.ChangeDoc, s service.State, live bool, width, height int) ([]string, []int) {
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
		return []string{m.st.faint.Render("nothing carried yet")}, []int{-1}
	}
	var out []string
	var rows []int
	for i, u := range units {
		sign := m.st.faint.Render("○")
		switch {
		case u.terminal == string(meta.CandidateEnd) || u.candidate != "":
			sign = m.st.good.Render("✓")
		case u.terminal != "":
			sign = m.st.bad.Render("✗")
		case len(s.Agent[u.id]) > 0:
			sign = m.st.active.Render("●")
		}
		for _, l := range fold(fmt.Sprintf("%s %s  %-8s %s", sign, u.id, u.kind, u.title), width, "  ") {
			if i == m.row {
				l = m.lit(l, width)
			}
			out, rows = append(out, l), append(rows, i)
		}
	}
	out = append(out, "")
	chosenUnit := units[min(m.row, len(units)-1)]
	events := s.Agent[chosenUnit.id]
	where := "live"
	if !live || len(events) == 0 {
		events = m.traces[m.dirOf(t)+"/"+chosenUnit.candidate]
		where = chosenUnit.candidate
	}
	out = append(out, m.st.faint.Render(chosenUnit.id+" · its agent · "+cmp.Or(where, "no candidate")))
	// The agent's steps, whole, newest at the foot: as many of the last as
	// the room holds.
	var steps []string
	for _, e := range events {
		steps = append(steps, fold(m.event(e), width, "    ")...)
	}
	room := max(1, height-len(out))
	out = append(out, steps[max(0, len(steps)-room):]...)
	if len(events) == 0 {
		out = append(out, m.st.faint.Render("  no steps kept"))
	}
	for len(rows) < len(out) {
		rows = append(rows, -1)
	}
	return out, rows
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
func (m Model) event(e protocol.Event) string {
	detail := oneLine(e.Detail)
	switch e.Kind {
	case "called":
		return "› " + strings.ToLower(e.Tool) + " " + detail
	case "got":
		return m.st.faint.Render("  " + detail)
	case "refused":
		return m.st.bad.Render("  refused: " + detail)
	case "ended":
		return m.st.strong.Render("■ " + detail)
	}
	return m.st.faint.Render(e.Kind + " " + detail)
}

// assurance is the verdict on every criterion, with the criterion's words.
func (m Model) assurance(doc protocol.ChangeDoc, width int) []string {
	if doc.Assurance == nil {
		return []string{m.st.faint.Render("not judged yet")}
	}
	words := map[string]string{}
	if doc.Specification != nil {
		for _, c := range doc.Specification.Criteria {
			words[c.ID] = c.Criterion
		}
	}
	var out []string
	for _, v := range doc.Assurance.Verdicts {
		sign := m.st.good.Render("✓")
		if v.Outcome != meta.Met {
			sign = m.st.bad.Render("✗")
		}
		out = append(out, fmt.Sprintf("%s %s  %s  %s", sign, v.Criterion, v.Outcome, m.st.faint.Render(v.Unit+" · "+string(v.State))))
		if w := words[v.Criterion]; w != "" {
			for _, l := range fold(oneLine(w), width-2, "") {
				out = append(out, m.st.faint.Render("  "+l))
			}
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
		out = append(out, m.st.faint.Render("not landed"), "")
	}
	out = append(out, m.st.faint.Render("record"))
	for _, x := range doc.Record {
		lead := fmt.Sprintf("%2d %s %-13s ", x.N, x.At, x.Stage)
		out = append(out, fold(lead+oneLine(x.Note), width, strings.Repeat(" ", ansi.StringWidth(lead)))...)
	}
	if t.change.State == protocol.Stopped {
		out = append(out, "", m.st.faint.Render("[R] resume from where it stopped"))
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
	lines := []string{m.st.strong.Render("cost"), fmt.Sprintf("$%.3f", total[0]),
		m.st.faint.Render(fmt.Sprintf("%dk in · %dk out", in/1000, outTok/1000)), ""}
	if handedN > 0 {
		lines = append(lines, m.st.faint.Render("handed an agent"), fmt.Sprintf("%dk tokens on average", handed/handedN/1000), "")
	}
	lines = append(lines, m.st.faint.Render("by area"))
	for n := 1; n <= 6; n++ {
		if byArea[n] > 0 {
			lines = append(lines, fmt.Sprintf("%d %-8s $%.3f", n, areaNames[n], byArea[n]))
		}
	}
	lines = append(lines, "", m.st.faint.Render("by model"))
	for _, k := range sortedByCost(byModel) {
		lines = append(lines, fold(fmt.Sprintf("$%.3f %s", byModel[k], k), width, "       ")...)
	}
	if len(byUnit) > 0 {
		lines = append(lines, "", m.st.faint.Render("dearest units"))
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
