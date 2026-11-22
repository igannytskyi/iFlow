package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// layout is where each part of the screen is: the same for drawing it and
// for finding what the mouse points at.
type layout struct {
	top          int // the lines above the columns
	body         int // the lines between the top and the foot
	side, mid    int // the widths of the list and of the task
	costs        int // the width of the costs, 0 where they do not fit
	taskH, chatH int // the task's lines, and the conversation's below it
}

func (m Model) layout() layout {
	l := layout{top: len(m.top())}
	l.body = max(4, m.height-l.top-len(m.foot()))
	l.side = min(36, max(22, m.width/4))
	l.mid = m.width - l.side - 3
	if l.mid >= 110 {
		l.costs = 30
		l.mid -= l.costs + 3
	}
	l.taskH = l.body
	if m.chatOpen {
		l.chatH = max(6, l.body*2/5)
		l.taskH = l.body - l.chatH - 1
	}
	return l
}

// sideRow is what a line of the list stands for: a project, or one of its
// tasks.
type sideRow struct {
	proj, task int // task is -1 for the project's own line
}

// areaAt is the area whose name is drawn at column x of the areas' line,
// or 0.
func (m Model) areaAt(x int, labels []string) int {
	at := 0
	for i, l := range labels {
		w := ansi.StringWidth(l)
		if x >= at && x < at+w {
			return i + 1
		}
		at += w + 2
	}
	return 0
}

// mouse acts on what the mouse did: a click opens the task or the area it
// is on, chooses a unit, or puts the cursor in the conversation; the wheel
// moves along the list or the task.
func (m Model) mouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	l := m.layout()
	row, x := msg.Y-l.top, msg.X
	if row < 0 || row >= l.body {
		return m, nil
	}
	inList := x < l.side
	inMid := x >= l.side+3 && x < l.side+3+l.mid
	inChat := inMid && m.chatOpen && row > l.taskH
	switch msg.Button {
	case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		step := "down"
		if msg.Button == tea.MouseButtonWheelUp {
			step = "up"
		}
		k := tea.KeyMsg{Type: tea.KeyDown}
		if step == "up" {
			k = tea.KeyMsg{Type: tea.KeyUp}
		}
		switch {
		case inList:
			p, _, _ := m.current()
			return m.listKey(k, p)
		case inMid && !inChat && m.focus == onTask:
			_, t, _ := m.current()
			return m.taskKey(k, t)
		}
		return m, nil
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
	default:
		return m, nil
	}
	switch {
	case inList:
		_, rows := m.sideLines(l.side, l.body)
		if row >= len(rows) || rows[row].proj < 0 {
			return m, nil
		}
		r := rows[row]
		if r.task < 0 {
			if r.proj != m.proj {
				m.proj, m.sel, m.focus = r.proj, 0, onList
			}
			return m, m.onView()
		}
		m.proj, m.sel = r.proj, r.task
		_, t, _ := m.current()
		m.focus, m.row, m.scroll = onTask, 0, 0
		m.area = m.areaOf(t)
		return m, m.onView()
	case inChat:
		if m.mode != asking {
			return m.begin(asking, "ask ▸ ")
		}
	case inMid:
		_, t, ok := m.current()
		if !ok {
			return m, nil
		}
		mx := x - (l.side + 3)
		switch {
		case row == 0: // the areas' line, the head's first
			if n := m.areaAt(mx, m.areaLabels(t)); n > 0 {
				m.focus, m.area, m.row, m.scroll = onTask, n, 0, 0
				return m, tea.Batch(m.onView(), m.traceOnView())
			}
		case row >= len(m.head(t, l.mid)):
			start := len(m.head(t, l.mid))
			_, rows, _ := m.body(t, l.mid, l.taskH-start)
			if i := row - start; i < len(rows) && rows[i] >= 0 {
				m.focus, m.row = onTask, rows[i]
				return m, m.traceOnView()
			}
			m.focus = onTask
		}
	}
	return m, nil
}

// talk opens the conversation below the task — about the task where one is
// open, about the project where not — and puts the cursor in it.
func (m Model) talk() (tea.Model, tea.Cmd) {
	if len(m.projects) == 0 {
		return m, nil
	}
	ref := m.chatRef()
	opened := !m.chatOpen || ref != m.talking
	m.chatOpen, m.talking = true, ref
	next, cmd := m.begin(asking, "ask ▸ ")
	if !opened {
		return next, cmd
	}
	return next, tea.Batch(cmd, func() tea.Msg { return result(m.client.Talk(m.ctx, ref), "") })
}
