package ui

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/io/key"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/workspace"
)

// specPanel is the specification as an object of its own: the change
// described — what is wanted, how, with what, judged how — and what a person
// can do with it. While it waits for them they accept it, change it with
// their own hands, or send it back with notes; once it is accepted it is
// fixed, and a changed intent is a new one.
func (u *UI) specPanel(p *project, s shown) []W {
	c := s.c
	waiting := s.live && s.v.Question != ""
	var out []W
	out = append(out, u.text(sizeSmall, u.p.Faint, "SPECIFICATION — the change described"))
	if c == nil {
		return append(out, u.text(sizeSmall, u.p.Faint, "being written…"))
	}
	path := filepath.Join(c.Change.Dir, artifact.ChangeFile)
	from := ""
	if _, err := os.Stat(path); err != nil {
		// Written before the change had a document of its own: its
		// description lives inside the specification's record.
		path, from = filepath.Join(c.Change.Dir, "01-specification.md"), "## The change"
	}
	if waiting {
		out = append(out, u.waitingBar(p, s))
		if s.v.Reviewed != "" {
			out = append(out, u.reviewLeft(s.v.Reviewed)...)
		}
	} else {
		out = append(out, u.text(sizeSmall, u.p.Faint, "Accepted, a specification is fixed: a changed intent is a new one — start it from the task's title above."))
	}
	if p.editing == c.Change.Dir {
		out = append(out, u.specEditor(p, c, path))
		return out
	}
	doc := u.prose(path, from)
	if len(doc) == 0 {
		doc = []W{u.text(sizeSmall, u.p.Faint, "no description was written for this task")}
	}
	out = append(out, box(u.p.Surface, u.p.Line, 8, 14, column(8, doc...)))
	return out
}

// waitingBar is the question the task waits on, and the three answers.
func (u *UI) waitingBar(p *project, s shown) W {
	return func(gtx C) D {
		yes, no, edit := u.clicks.of("review/yes/"+p.path), u.clicks.of("review/no/"+p.path), u.clicks.of("spec/edit/"+p.path)
		if yes.Clicked(gtx) {
			p.run.Accept()
		}
		if no.Clicked(gtx) {
			ground := strings.TrimSpace(p.ground.Text())
			if ground == "" {
				ground = "sent back without notes"
			}
			p.run.Refuse(ground)
			p.ground.SetText("")
		}
		if edit.Clicked(gtx) && s.c != nil {
			p.editing = s.c.Change.Dir
			gtx.Execute(key.FocusCmd{Tag: &p.docEdit})
		}
		return box(u.p.NoteSoft, colorNRGBA{}, 8, 12, column(8,
			u.strong(sizeBody, u.p.Note, "Waiting for you — is this the change you want?"),
			u.text(sizeSmall, u.p.Text, s.v.Question),
			box(u.p.Bg, u.p.Line, 6, 8, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return material.Editor(u.th, &p.ground, "notes: what is wrong or missing, your answers to the questions — a new author rewrites it from them").Layout(gtx)
			}),
			row(10,
				u.button(yes, "Accept — run the rest", u.p.OK, u.p.Surface),
				u.button(no, "Send back with notes", u.p.FailSoft, u.p.Fail),
				u.button(edit, "Change it myself", u.p.AccentSoft, u.p.Accent),
			),
		))(gtx)
	}
}

// specActions is what can be done with a specification no longer waiting.
func (u *UI) specActions(p *project, s shown) W {
	return func(gtx C) D {
		again := u.clicks.of("intent/again/" + p.path)
		if again.Clicked(gtx) {
			p.task.SetText(s.title())
			p.selected, p.item = newKey, nil
			gtx.Execute(key.FocusCmd{Tag: &p.task})
		}
		ws := []W{u.small(again, "New intent from this", u.p.AccentSoft, u.p.Accent)}
		state := s.state(p)
		if state == "stopped" && s.c != nil {
			closer := u.clicks.of("intent/close/" + s.c.Change.Dir)
			if closer.Clicked(gtx) {
				if p.closing == s.c.Change.Dir {
					p.closing = ""
				} else {
					p.closing = s.c.Change.Dir
				}
			}
			ws = append(ws, u.small(closer, "Close this task…", u.p.Sunken, u.p.Text))
		}
		parts := []W{row(8, ws...)}
		if s.c != nil {
			if closed, note := s.c.Closed(); closed {
				parts = append(parts, u.text(sizeSmall, u.p.Muted, note))
			}
			if p.closing == s.c.Change.Dir {
				parts = append(parts, u.closeForm(p, s.c))
			}
		}
		return column(8, parts...)(gtx)
	}
}

// specEditor is the change described, in a person's hands before they
// accept it. What they write becomes the description the agents build from;
// the criteria that judge it are the specification's record, and stay as
// they are — to change those, send it back.
func (u *UI) specEditor(p *project, c *workspace.Change, path string) W {
	return func(gtx C) D {
		if p.editLoaded != path {
			b, _ := os.ReadFile(path)
			p.docEdit.SetText(string(b))
			p.editLoaded = path
		}
		save, cancel := u.clicks.of("spec/save/"+p.path), u.clicks.of("spec/cancel/"+p.path)
		if cancel.Clicked(gtx) {
			p.editing, p.editLoaded = "", ""
		}
		if save.Clicked(gtx) {
			if err := os.WriteFile(path, []byte(p.docEdit.Text()), 0o644); err == nil {
				_ = artifact.Record{Name: c.Name(), Dir: c.Change.Dir}.Append(artifact.Crossing{At: time.Now().Format("2006-01-02"),
					Stage: "specification", Object: artifact.ChangeFile, Note: "the change described, edited by its person before acceptance"})
				p.editing, p.editLoaded = "", ""
				if p.run != nil {
					p.run.Accept()
				}
			}
		}
		return column(8,
			u.text(sizeSmall, u.p.Muted, "Your edits become what the agents build from. The criteria that judge it stay as they are; to change those, send it back with notes instead."),
			box(u.p.Surface, u.p.Accent, 8, 12, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				ed := material.Editor(u.th, &p.docEdit, "")
				ed.Font.Typeface = "monospace"
				ed.TextSize = sizeSmall
				return ed.Layout(gtx)
			}),
			row(10, u.button(save, "Accept with my edits", u.p.OK, u.p.Surface), u.button(cancel, "Cancel", u.p.Sunken, u.p.Text)),
		)(gtx)
	}
}

// reviewLeft is what the reviewing agent still found, its questions for
// the person apart from its findings.
func (u *UI) reviewLeft(reviewed string) []W {
	var left, questions []W
	for _, l := range strings.Split(strings.TrimSpace(reviewed), "\n") {
		l = strings.TrimPrefix(l, "- ")
		if q, ok := strings.CutPrefix(l, "[question for the person] "); ok {
			questions = append(questions, u.point("?", q))
		} else {
			left = append(left, u.point("!", l))
		}
	}
	var out []W
	if len(questions) > 0 {
		out = append(out, box(u.p.AccentSoft, colorNRGBA{}, 8, 10, column(6,
			append([]W{u.text(sizeSmall, u.p.Accent, "QUESTIONS ONLY YOU CAN ANSWER — answer them in your notes, or accept it as it is")}, questions...)...)))
	}
	if len(left) > 0 {
		out = append(out, box(u.p.NoteSoft, colorNRGBA{}, 8, 10, column(6,
			append([]W{u.text(sizeSmall, u.p.Note, "WHAT THE REVIEWER STILL FINDS — decide whether it matters")}, left...)...)))
	}
	return out
}

// metaPanel is the method's record of the specification: the objects it
// holds, each opening to every cell.
func (u *UI) metaPanel(p *project, s shown) []W {
	out := []W{u.text(sizeSmall, u.p.Faint, "METADATA — the specification's record: intent, class, criteria, scope, judges, when to stop")}
	if s.c == nil {
		return out
	}
	class := ""
	var items []W
	for _, it := range s.c.InArea(1) {
		if it.Kind == "Specification" {
			class = it.Field("Default class")
		}
		items = append(items, u.itemRow(p, it))
	}
	if class != "" {
		out = append(out, u.text(sizeSmall, u.p.Muted, u.className(class)))
	}
	if len(items) > 0 {
		out = append(out, box(u.p.Surface, u.p.Line, 8, 4, column(0, items...)))
	}
	return out
}

// button is a pressable label on a colour.
func (u *UI) button(btn *widget.Clickable, label string, bg, fg colorNRGBA) W {
	return func(gtx C) D {
		return u.pressable(gtx, btn, false, 8, box(bg, colorNRGBA{}, 8, 0, inset(8, 16, u.strong(sizeBody, fg, label))))
	}
}

// small is a pressable label on a colour, sized for a line of actions.
func (u *UI) small(btn *widget.Clickable, label string, bg, fg colorNRGBA) W {
	return func(gtx C) D {
		return u.pressable(gtx, btn, false, 6, box(bg, colorNRGBA{}, 6, 0, inset(5, 12, u.text(sizeSmall, fg, label))))
	}
}
