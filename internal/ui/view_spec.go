package ui

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/workspace"
)

// specBlock is the specification as an object of its own: the change
// described — what is wanted, how, with what, judged how — and what a person
// can do with it. While it waits for them they accept it, change it with
// their own hands, or send it back with notes; once it is accepted it is
// fixed, and a changed intent is a new one.
func (u *UI) specBlock(p *project, s shown) []W {
	c := s.c
	waiting := s.live && s.v.Question != ""
	state := s.state(p)
	fold := u.clicks.of("spec/fold/" + p.path)
	// The specification is open while its own area is chosen on the graph,
	// or while it waits for the person; choosing another area folds it to
	// its heading, so that what that area holds is what is read.
	folded := (p.area != 1 || p.specFolded) && !waiting
	var out []W
	out = append(out, func(gtx C) D {
		if u.clicked(gtx, fold) {
			if folded {
				p.area, p.specFolded = 1, false
			} else {
				p.specFolded = true
			}
		}
		label := "fold ▴"
		if folded {
			label = "open ▾"
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, u.text(sizeSmall, u.p.Faint, "SPECIFICATION — what is wanted, how, with what, and how it is judged")),
			layout.Rigid(func(gtx C) D {
				return u.pressable(gtx, fold, false, 4, inset(2, 6, u.text(sizeSmall, u.p.Muted, label)))
			}))
	})
	facts := []W{row(6, dot(u.stateColor(state), 8), u.text(sizeSmall, u.stateColor(state), state))}
	if c != nil {
		facts = append(facts, u.mono(sizeSmall, u.p.Faint, c.Name()))
		if !s.live || s.v.Finished {
			facts = append(facts, u.mattersChip(p, c))
		}
	}
	if folded {
		out = append(out, u.oneLine(sizeBody, u.p.Text, s.title()))
	} else if c == nil || !u.docs.exists(filepath.Join(c.Change.Dir, artifact.ChangeFile)) {
		// Without a document of its own, the intent is all there is to show
		// of what was wanted; with one, the document begins with it.
		out = append(out, u.strong(sizeTitle, u.p.Text, s.title()))
	}
	facts = append(facts, u.lifecycleButton(p, s))
	out = append(out, u.wrap(12, facts), u.mattersPanel(p, c), u.lifecycle(p, s))
	if waiting {
		out = append(out, u.waitingBar(p, s))
		if s.v.Reviewed != "" {
			out = append(out, u.reviewLeft(s.v.Reviewed)...)
		}
	} else if !(s.live && !s.v.Finished) && !folded {
		out = append(out, u.specActions(p, s))
	}
	if c == nil {
		return append(out, u.text(sizeSmall, u.p.Faint, "being written…"))
	}
	if folded {
		return out
	}
	path := filepath.Join(c.Change.Dir, artifact.ChangeFile)
	if p.editing == c.Change.Dir {
		return append(out, u.specEditor(p, c, path))
	}
	if !u.docs.exists(path) {
		return append(out, u.noSpec(p, s))
	}
	return append(out, box(u.p.Surface, u.p.Line, 8, 14, column(8, u.prose(path, "")...)))
}

// waitingBar is the question the task waits on, and the three answers.
func (u *UI) waitingBar(p *project, s shown) W {
	return func(gtx C) D {
		yes, no, edit := u.clicks.of("review/yes/"+p.path), u.clicks.of("review/no/"+p.path), u.clicks.of("spec/edit/"+p.path)
		if u.clicked(gtx, yes) {
			p.run.Accept()
		}
		if u.clicked(gtx, no) {
			ground := strings.TrimSpace(p.ground.Text())
			if ground == "" {
				ground = "sent back without notes"
			}
			p.run.Refuse(ground)
			p.ground.SetText("")
		}
		if u.clicked(gtx, edit) && s.c != nil {
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
		if u.clicked(gtx, again) {
			p.task.SetText(s.title())
			p.selected, p.item = newKey, nil
			gtx.Execute(key.FocusCmd{Tag: &p.task})
		}
		ws := []W{u.small(again, "New intent from this", u.p.AccentSoft, u.p.Accent)}
		state := s.state(p)
		if state == "stopped" && s.c != nil {
			closer := u.clicks.of("intent/close/" + s.c.Change.Dir)
			if u.clicked(gtx, closer) {
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
		if u.clicked(gtx, cancel) {
			p.editing, p.editLoaded = "", ""
		}
		if u.clicked(gtx, save) {
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

// noSpec is a task carried before its specification was written whole:
// what it has — the intent and, where there is one, the short description
// its record held — and the offer to write the whole of it, after the fact.
func (u *UI) noSpec(p *project, s shown) W {
	return func(gtx C) D {
		write := u.clicks.of("spec/describe/" + s.c.Change.Dir)
		if u.clicked(gtx, write) && u.flowConfig != nil && !p.describing.Load() {
			p.describing.Store(true)
			cfg, repo, dir := *u.flowConfig, p.path, s.c.Change.Dir
			go func() {
				if err := flow.Describe(u.ctx, cfg, repo, dir); err != nil {
					p.describeErr = err.Error()
				}
				p.describing.Store(false)
				p.stale.Store(true)
				u.invalidate()
			}()
		}
		parts := []W{
			u.text(sizeBody, u.p.Muted, "This task was carried before a specification was written whole, so it has only what is above and its record below."),
		}
		if old := u.prose(filepath.Join(s.c.Change.Dir, "01-specification.md"), "## The change"); len(old) > 0 {
			parts = append(parts, column(8, old...))
		}
		switch {
		case p.describing.Load():
			parts = append(parts, u.text(sizeSmall, u.p.Accent, "writing the specification from what was asked, its record and the code as it stands…"))
		case u.flowConfig == nil:
		default:
			parts = append(parts, u.button(write, "Write the full specification", u.p.AccentSoft, u.p.Accent),
				u.text(sizeSmall, u.p.Faint, "An author is handed what was asked, its record and the code its scope holds now, and describes the change as it was meant and built; it is recorded as written after the fact."))
		}
		if p.describeErr != "" {
			parts = append(parts, u.text(sizeSmall, u.p.Fail, p.describeErr))
		}
		return box(u.p.Surface, u.p.Line, 8, 14, column(10, parts...))(gtx)
	}
}
