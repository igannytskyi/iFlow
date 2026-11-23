package tui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/igannytskyi/iflow/internal/protocol"
)

// chatRef is the conversation beside what is on view: about the task, where
// one is open and has a folder, about the project where not.
func (m Model) chatRef() protocol.ChatRef {
	p, t, ok := m.current()
	ref := protocol.ChatRef{Project: p.Path}
	if ok && m.focus == onTask {
		ref.Dir = m.dirOf(t)
	}
	return ref
}

// chat is the conversation in its column, width by height: what it is
// about, its turns — the newest at the foot, scrolled back by chatScroll —
// and the line typed on, at the column's own foot.
func (m Model) chat(width, height int) string {
	head, lines := m.chatHead(width), m.chatLines(width)
	foot := []string{m.st.faint.Render(strings.Repeat("─", width))}
	line := m.ask
	line.Width = max(4, width-ansi.StringWidth(line.Prompt)-1)
	if m.mode == asking {
		foot = append(foot, line.View())
	} else {
		foot = append(foot, m.st.faint.Render("› click here or press c to type"))
	}

	room := max(1, height-len(head)-len(foot))
	end := len(lines) - min(m.chatScroll, max(0, len(lines)-room))
	start := max(0, end-room)
	shown := slices.Clone(lines[start:end])
	if start > 0 {
		shown[0] = m.st.faint.Render(fmt.Sprintf("↑ %d lines above — PgUp or the wheel", start))
	}
	for len(shown) < room {
		shown = append(shown, "")
	}
	out := append(append(head, shown...), foot...)
	return lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height).Render(strings.Join(out, "\n"))
}

// chatScrolled is the conversation scrolled back by n lines more (fewer,
// where n is negative), never past its first line nor its newest.
func (m Model) chatScrolled(n int) Model {
	l := m.layout()
	if l.chat == 0 {
		return m
	}
	room := max(1, l.body-len(m.chatHead(l.chat))-2)
	m.chatScroll = min(max(0, m.chatScroll+n), max(0, len(m.chatLines(l.chat))-room))
	return m
}

// chatHead is what the conversation is about, and what it has cost.
func (m Model) chatHead(width int) []string {
	about := "the project"
	if m.talking.Dir != "" {
		about = "this task"
	}
	s, _ := m.client.Chat(m.talking)
	head := []string{m.st.strong.Render("talk about " + about)}
	sub := "reads what it has written, changes nothing"
	if s.CostUSD > 0 {
		sub = fmt.Sprintf("$%.3f so far · ", s.CostUSD) + sub
	}
	head = append(head, fold(m.st.faint.Render(sub), width, "")...)
	return append(head, m.st.faint.Render(strings.Repeat("─", width)))
}

// chatLines is the conversation's turns, wrapped to width, oldest first.
func (m Model) chatLines(width int) []string {
	s, _ := m.client.Chat(m.talking)
	var lines []string
	add := func(text, indent string) {
		for _, l := range strings.Split(text, "\n") {
			lines = append(lines, strings.Split(ansi.Wrap(indent+l, width, ""), "\n")...)
		}
	}
	for _, msg := range s.Messages {
		who := m.st.you.Render("you")
		if msg.Role != "person" {
			who = m.st.model.Render("model")
		}
		if msg.Err {
			who = m.st.bad.Render("failed")
		}
		lines = append(lines, who)
		add(printable(msg.Text), "")
		// The pages it cites, each a link where the terminal opens links.
		for i, src := range msg.Sources {
			what := printable(cmp.Or(src.Title, src.URL))
			for k, l := range fold(fmt.Sprintf("[%d] %s — %s", i+1, what, printable(src.URL)), width, "    ") {
				if k == 0 {
					l = m.term.webLink(src.URL, l)
				}
				lines = append(lines, m.st.faint.Render(l))
			}
		}
		lines = append(lines, "")
	}
	if s.Waiting {
		lines = append(lines, m.st.model.Render("model"))
		add(printable(s.Partial)+" …", "")
	}
	if len(lines) == 0 {
		add(m.st.faint.Render("ask anything: why it stopped, what a unit was handed, what to do next"), "")
	}
	return lines
}

// printable is text a model or a page wrote, without what a terminal would
// read as an instruction to it rather than as text: control characters,
// the escape sequences among them, leaving the lines as they are.
func printable(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\t':
			return r
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0):
			return -1
		}
		return r
	}, s)
}
