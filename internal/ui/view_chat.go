package ui

import (
	"fmt"
	"image"
	"path/filepath"
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/igannytskyi/iflow/internal/chat"
)

// talk is a project's conversations with a model: one about the project and
// one about each task, each its own, opened when first shown.
type talk struct {
	threads map[string]*chat.Thread // by the folder of what it is about; "" is the project
	box     widget.Editor
	// intent says the box at the foot starts a task rather than asks; it is
	// chosen per subject, and a new task's view starts in it.
	intent map[string]bool
}

// subject is what the conversation on view is about: the task on view, or
// the project; and its folder.
func (u *UI) subject(p *project) (key, dir string) {
	if p.selected == "" || p.selected == newKey {
		return "", filepath.Join(p.path, "changes")
	}
	s, ok := p.shownOf()
	switch {
	case !ok:
		return "", filepath.Join(p.path, "changes")
	case s.live && s.v.Dir != "":
		return s.v.Dir, s.v.Dir
	case s.c != nil:
		return s.c.Change.Dir, s.c.Change.Dir
	}
	return "", filepath.Join(p.path, "changes")
}

// thread is the conversation about key, read from disk the first time.
func (u *UI) thread(p *project, key, dir string) *chat.Thread {
	if p.talk.threads == nil {
		p.talk.threads = map[string]*chat.Thread{}
	}
	if t, ok := p.talk.threads[key]; ok {
		return t
	}
	t := chat.Open(filepath.Join(dir, chat.File), func() string { return chat.AboutTask(dir) })
	t.Root = p.path
	p.talk.threads[key] = t
	return t
}

// asking says whether the box at the foot asks the model rather than starts
// a task.
func (u *UI) asking(p *project, key string) bool {
	if v, ok := p.talk.intent[key]; ok {
		return !v
	}
	return p.selected != newKey
}

// conversation is the turns so far about what is on view, above the box;
// nothing when there are none.
func (u *UI) conversation(p *project) W {
	return func(gtx C) D {
		key, dir := u.subject(p)
		if !u.asking(p, key) {
			return D{}
		}
		t := u.thread(p, key, dir)
		msgs := t.Messages()
		var turns []W
		for i, m := range msgs {
			turns = append(turns, u.turn(p, key, i, m))
		}
		if t.Waiting() {
			if part := t.Partial(); part != "" {
				turns = append(turns, column(6, u.markdown(strings.Split(part, "\n"))...))
			} else {
				turns = append(turns, u.text(sizeSmall, u.p.Faint, "thinking…"))
			}
		}
		// The conversation is as tall as the person last dragged it, never
		// taller than leaves the task above it room to be read.
		g := u.splitOf("chat", 260, 80, 4000)
		height := min(gtx.Dp(g.left), gtx.Constraints.Max.Y-gtx.Dp(160))
		l := u.lists.of("chat/" + p.path + "/" + key)
		l.Axis, l.ScrollToEnd = layout.Vertical, true
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return g.grip(gtx, u) }),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, max(height, gtx.Dp(60))))
				return box(u.p.Surface, u.p.Line, 8, 0, func(gtx C) D {
					return material.List(u.th, l).Layout(gtx, len(turns), func(gtx C, i int) D {
						return layout.Inset{Top: 6, Bottom: 6}.Layout(gtx, turns[i])
					})
				})(gtx)
			}),
		)
	}
}

// turn is one message: the person's on a tinted ground, the model's plain,
// with what it cost and a way to make it a task.
func (u *UI) turn(p *project, key string, i int, m chat.Message) W {
	if m.Role == "person" {
		return box(u.p.Sunken, colorNRGBA{}, 8, 10, u.text(sizeBody, u.p.Text, m.Text))
	}
	if m.Err {
		return u.text(sizeSmall, u.p.Fail, "the model could not answer: "+m.Text)
	}
	toIntent := u.clicks.of(fmt.Sprintf("chat/intent/%s/%s/%d", p.path, key, i))
	return func(gtx C) D {
		if u.clicked(gtx, toIntent) {
			p.task.SetText(m.Text)
			p.selected, p.item = newKey, nil
			if p.talk.intent == nil {
				p.talk.intent = map[string]bool{}
			}
			p.talk.intent[""] = true
		}
		meta := m.Usage.Model
		if m.Usage.CostUSD > 0 {
			meta += " · " + money(m.Usage.CostUSD)
		}
		return column(4,
			column(6, u.markdown(strings.Split(m.Text, "\n"))...),
			row(10, u.text(sizeSmall, u.p.Faint, meta), func(gtx C) D {
				return u.pressable(gtx, toIntent, false, 5, inset(2, 6, u.text(sizeSmall, u.p.Accent, "Make it an intent")))
			}),
		)(gtx)
	}
}

// modes is the choice, beside the box, between asking and starting a task.
func (u *UI) modes(p *project, key string) W {
	ask, run := u.clicks.of("chat/mode/ask/"+p.path), u.clicks.of("chat/mode/run/"+p.path)
	return func(gtx C) D {
		set := func(intent bool) {
			if p.talk.intent == nil {
				p.talk.intent = map[string]bool{}
			}
			p.talk.intent[key] = intent
		}
		if u.clicked(gtx, ask) {
			set(false)
		}
		if u.clicked(gtx, run) {
			set(true)
		}
		on := u.asking(p, key)
		chip := func(btn *widget.Clickable, label string, sel bool) W {
			return func(gtx C) D {
				c := u.p.Muted
				if sel {
					c = u.p.Accent
				}
				return u.pressable(gtx, btn, sel, 6, inset(5, 10, u.text(sizeSmall, c, label)))
			}
		}
		return column(4, chip(ask, "Chat", on), chip(run, "Intent", !on))(gtx)
	}
}

// ask puts what was typed to the model, about what is on view.
func (u *UI) ask(p *project, key, dir, text string) {
	if u.flowConfig == nil || u.flowConfig.Model == nil {
		return
	}
	t := u.thread(p, key, dir)
	if key == "" && !t.Waiting() {
		// The project's tasks as they stand now, read here and not in the
		// goroutine that asks, which must not touch what the frame draws.
		var tasks []string
		for _, c := range p.order {
			tasks = append(tasks, fmt.Sprintf("- %s — %s", c.Name(), shown{c: c}.state(p)))
		}
		t.About = func() string { return chat.AboutProject(p.path, tasks) }
	}
	if err := t.Ask(u.ctx, u.flowConfig.Model, text, u.invalidate); err == nil {
		p.talk.box.SetText("")
	}
}
