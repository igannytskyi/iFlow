package tui

import (
	"fmt"
	"strings"

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

// chat is the conversation on view: its turns, newest at the foot, and the
// answer being written.
func (m Model) chat(width, height int) string {
	ref := m.talking
	about := "the project"
	if ref.Dir != "" {
		about = "this task"
	}
	head := m.st.strong.Render("talk about "+about) + m.st.faint.Render(" — the model reads what it has written, and changes nothing")
	s, _ := m.client.Chat(ref)
	var lines []string
	for _, msg := range s.Messages {
		who := m.st.you.Render("you")
		if msg.Role != "person" {
			who = m.st.model.Render("model")
		}
		if msg.Err {
			who = m.st.bad.Render("failed")
		}
		lines = append(lines, who)
		for _, l := range strings.Split(wrap(msg.Text, width-2), "\n") {
			lines = append(lines, "  "+l)
		}
		lines = append(lines, "")
	}
	if s.Waiting {
		lines = append(lines, m.st.model.Render("model"))
		for _, l := range strings.Split(wrap(s.Partial+" …", width-2), "\n") {
			lines = append(lines, "  "+l)
		}
	}
	if len(lines) == 0 {
		lines = []string{m.st.faint.Render("ask anything: why it stopped, what a unit was handed, what to do next")}
	}
	room := max(1, height-2)
	lines = lines[max(0, len(lines)-room):]
	if s.CostUSD > 0 {
		head += "\n" + m.st.faint.Render(fmt.Sprintf("this conversation has cost $%.3f", s.CostUSD))
	} else {
		head += "\n"
	}
	return head + "\n" + strings.Join(lines, "\n")
}
