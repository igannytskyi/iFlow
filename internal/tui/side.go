package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/igannytskyi/iflow/internal/protocol"
)

// sidebar is every project, the one on view open on its tasks.
func (m Model) sidebar(width, height int) string {
	lines, _ := m.sideLines(width, height)
	return strings.Join(lines, "\n")
}

// sideLines is the list as it is drawn, every task's words whole, and what
// each line stands for; the task chosen is kept in view.
func (m Model) sideLines(width, height int) ([]string, []sideRow) {
	if len(m.projects) == 0 {
		return []string{m.st.faint.Render("no projects yet")}, []sideRow{{-1, -1}}
	}
	var before, after, tasks []string
	var rowsBefore, rowsAfter, rowsTasks []sideRow
	chosenAt := 0
	for i, p := range m.projects {
		ts := m.tasks(p)
		if i != m.proj {
			line := m.st.faint.Render(fmt.Sprintf("▸ %s  %d", p.Name, len(ts)))
			if i < m.proj {
				before, rowsBefore = append(before, line), append(rowsBefore, sideRow{i, -1})
			} else {
				after, rowsAfter = append(after, line), append(rowsAfter, sideRow{i, -1})
			}
			continue
		}
		head := "▾ " + p.Name
		if m.focus == onList {
			head = m.st.strong.Render(head)
		}
		before, rowsBefore = append(before, head), append(rowsBefore, sideRow{i, -1})
		for j, t := range ts {
			what := oneLine(t.change.Statement)
			if what == "" {
				what = t.change.Name
			}
			lead := "  "
			if j == m.sel {
				lead, chosenAt = m.st.strong.Render("› "), len(tasks)
			}
			for k, l := range fold(m.taskMark(t)+" "+what, width-2, "  ") {
				if j == m.sel && m.focus == onList {
					l = m.st.chosen.Render(ansi.Strip(l))
				}
				if k > 0 {
					lead = "  "
				}
				tasks, rowsTasks = append(tasks, lead+l), append(rowsTasks, sideRow{i, j})
			}
		}
		if len(ts) == 0 {
			tasks, rowsTasks = append(tasks, m.st.faint.Render("  no task yet — n")), append(rowsTasks, sideRow{i, -1})
		}
	}
	// The tasks scroll; the projects' lines stay where they are.
	room := max(1, height-len(before)-len(after))
	first := max(0, min(chosenAt-room/3, len(tasks)-room))
	end := min(len(tasks), first+room)
	lines := append(append(append([]string{}, before...), tasks[first:end]...), after...)
	rows := append(append(append([]sideRow{}, rowsBefore...), rowsTasks[first:end]...), rowsAfter...)
	return lines, rows
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
