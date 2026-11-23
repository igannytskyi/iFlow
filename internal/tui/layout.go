package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// layout is where each part of the screen is: the same for drawing it and
// for finding what the mouse points at.
type layout struct {
	top       int // the lines above the columns
	body      int // the lines between the top and the foot
	side, mid int // the widths of the list and of the task
	costs     int // the width of the costs, 0 where they do not fit
	chat      int // the width of the conversation, 0 while it is closed
	chatX     int // the column the conversation begins at
}

// The conversation is a column of its own, on the right, the whole height:
// its turns above, the line typed on at its foot. Its width is the
// person's — dragged at its edge, or < and > — within these bounds.
const (
	minChat = 32
	minMid  = 30
)

func (m Model) layout() layout {
	l := layout{top: len(m.top())}
	l.body = max(4, m.height-l.top-len(m.foot()))
	l.side = min(36, max(22, m.width/4))
	rest := m.width - l.side - 3
	if m.chatOpen {
		want := m.chatW
		if want == 0 {
			want = rest * 9 / 20
		}
		l.chat = min(max(want, minChat), max(minChat, rest-minMid-3))
		rest -= l.chat + 3
		l.chatX = m.width - l.chat
	}
	l.mid = rest
	if l.mid >= 110 {
		l.costs = 30
		l.mid -= l.costs + 3
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
	// The conversation's edge is dragged to make it wider or narrower.
	edge := l.chat > 0 && x >= l.chatX-3 && x < l.chatX
	switch {
	case msg.Action == tea.MouseActionRelease:
		m.dragging = false
		return m, nil
	case msg.Action == tea.MouseActionMotion && m.dragging:
		m.chatW = max(minChat, m.width-x-1)
		return m, nil
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && edge && row >= 0:
		m.dragging = true
		return m, nil
	}
	if row < 0 || row >= l.body {
		return m, nil
	}
	inList := x < l.side
	inMid := x >= l.side+3 && x < l.side+3+l.mid
	inChat := l.chat > 0 && x >= l.chatX
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
		case inChat:
			if step == "up" {
				return m.chatScrolled(3), nil
			}
			return m.chatScrolled(-3), nil
		case inList:
			p, _, _ := m.current()
			return m.listKey(k, p)
		case inMid && m.focus == onTask:
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
			return m.begin(asking, "")
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
			_, rows, _ := m.body(t, l.mid, l.body-start)
			if i := row - start; i < len(rows) && rows[i] >= 0 {
				m.focus, m.row = onTask, rows[i]
				return m, m.traceOnView()
			}
			m.focus = onTask
		}
	}
	return m, nil
}

// talk opens the conversation beside the task — about the task where one
// is open, about the project where not — and puts the cursor in it.
func (m Model) talk() (tea.Model, tea.Cmd) {
	if len(m.projects) == 0 {
		return m, nil
	}
	ref := m.chatRef()
	opened := !m.chatOpen || ref != m.talking
	m.chatOpen, m.talking = true, ref
	if opened {
		m.chatScroll = 0
	}
	next, cmd := m.begin(asking, "")
	if !opened {
		return next, cmd
	}
	return next, tea.Batch(cmd, func() tea.Msg { return result(m.client.Talk(m.ctx, ref), "") })
}
