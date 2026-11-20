package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/igannytskyi/iflow/internal/protocol"
)

// sidebar is every project, the one on view open on its tasks.
func (m Model) sidebar(width, height int) string {
	if len(m.projects) == 0 {
		return m.st.faint.Render("no projects yet")
	}
	var lines []string
	for i, p := range m.projects {
		ts := m.tasks(p)
		if i != m.proj {
			lines = append(lines, m.st.faint.Render(fmt.Sprintf("▸ %s  %d", p.Name, len(ts))))
			continue
		}
		head := "▾ " + p.Name
		if m.focus == onList {
			head = m.st.strong.Render(head)
		}
		lines = append(lines, head)
		room := max(1, height-len(m.projects))
		first := max(0, min(m.sel-room/2, len(ts)-room))
		for j, t := range ts[first:min(len(ts), first+room)] {
			what := oneLine(t.change.Statement)
			if what == "" {
				what = t.change.Name
			}
			line := ansi.Truncate(m.taskMark(t)+" "+what, width-2, "…")
			switch {
			case first+j == m.sel && m.focus == onList:
				line = m.st.strong.Render("› ") + m.st.chosen.Render(ansi.Strip(line))
			case first+j == m.sel:
				line = m.st.strong.Render("› ") + line
			default:
				line = "  " + line
			}
			lines = append(lines, line)
		}
		if len(ts) == 0 {
			lines = append(lines, m.st.faint.Render("  no task yet — n"))
		}
	}
	return strings.Join(lines, "\n")
}

// taskMark is how a task stands, in one sign: by its run where one carries
// it, by its folder where not.
func (m Model) taskMark(t task) string {
	if s, ok := m.client.Run(t.run); ok && !s.Finished {
		return m.mark(s)
	}
	switch t.change.State {
	case protocol.Landed:
		return m.st.good.Render("✓")
	case protocol.Carried:
		return m.st.active.Render("●")
	case protocol.Closed:
		return m.st.faint.Render("–")
	}
	return m.st.bad.Render("✗")
}
