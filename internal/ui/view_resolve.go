package ui

import (
	"context"
	"errors"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/workspace"
)

// The ways a person resolves what a task left for them — an escalation, or a
// problem the gate found in it:
//
//	fix it in the task itself, carried again from its specification or from
//	its plan, with what is to change;
//	start a new intent for it;
//	or, for an escalation, settle it by a decision, when there is nothing to
//	change and something to decide.
const (
	resolveSpec   = "spec"
	resolvePlan   = "plan"
	resolveSettle = "settle"
)

// resolveActions are the ways out for one matter of the task c; ref is the
// escalation, or "" for problems of the task. Choosing one that needs words
// opens a box for them under it.
func (u *UI) resolveActions(p *project, c *workspace.Change, ref, ground string) W {
	key := c.Change.Dir + "|" + ref
	chip := func(mode, label string) W {
		btn := u.clicks.of("resolve/" + mode + "/" + key)
		return func(gtx C) D {
			if u.clicked(gtx, btn) {
				if p.resolving == key && p.resolveMode == mode {
					p.resolving, p.resolveMode = "", ""
				} else {
					p.resolving, p.resolveMode = key, mode
					p.resolveText.SetText("")
					gtx.Execute(keyFocus(&p.resolveText))
				}
			}
			on := p.resolving == key && p.resolveMode == mode
			return u.pressable(gtx, btn, on, 6, box(u.p.AccentSoft, colorNRGBA{}, 6, 0, inset(4, 10, u.text(sizeSmall, u.p.Accent, label))))
		}
	}
	newIntent := u.clicks.of("resolve/new/" + key)
	actions := []W{
		chip(resolveSpec, "Fix in this task — from its specification"),
		chip(resolvePlan, "Fix in this task — from its plan"),
		func(gtx C) D {
			if u.clicked(gtx, newIntent) {
				text := "Resolve " + c.Name()
				if ref != "" {
					text = "Close " + ref
					p.settles = []string{ref}
				}
				p.task.SetText(text + ": " + ground)
				p.selected, p.item = newKey, nil
				gtx.Execute(keyFocus(&p.task))
			}
			return u.pressable(gtx, newIntent, false, 6, box(u.p.Sunken, colorNRGBA{}, 6, 0, inset(4, 10, u.text(sizeSmall, u.p.Text, "New intent"))))
		},
	}
	if ref != "" {
		actions = append(actions, chip(resolveSettle, "Settle with a decision"))
	}
	return func(gtx C) D {
		parts := []W{u.wrap(8, actions)}
		if p.resolving == key {
			parts = append(parts, u.resolveForm(p, c, ref))
		}
		return column(8, parts...)(gtx)
	}
}

// resolveForm is the box for what is to change, or what is decided, and the
// button that does it.
func (u *UI) resolveForm(p *project, c *workspace.Change, ref string) W {
	hint, label := "what is to change — the author gets the specification as it was and these words", "Carry it again"
	switch p.resolveMode {
	case resolvePlan:
		hint = "what is to change — the planner gets these words and plans again; the specification stays"
	case resolveSettle:
		hint, label = "what you decide, and why — kept with your other decisions", "Settle"
	}
	do := u.clicks.of("resolve/do/" + c.Change.Dir + "|" + ref)
	return func(gtx C) D {
		if u.clicked(gtx, do) {
			text := strings.TrimSpace(p.resolveText.Text())
			if text != "" {
				p.resolveErr = ""
				var err error
				switch p.resolveMode {
				case resolveSettle:
					err = flow.Settle(p.path, c.Change.Dir, ref, u.flowConfig.Person, text)
					p.stale.Store(true)
				case resolveSpec:
					err = u.reopen(p, c, 1, text, ref)
				case resolvePlan:
					err = u.reopen(p, c, 2, text, ref)
				}
				if err != nil {
					p.resolveErr = err.Error()
				} else {
					p.resolving, p.resolveMode = "", ""
				}
			}
		}
		parts := []W{func(gtx C) D {
			return layout.Flex{Gap: gtx.Dp(8), Alignment: layout.End}.Layout(gtx,
				layout.Flexed(1, box(u.p.Bg, u.p.Line, 6, 8, func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return material.Editor(u.th, &p.resolveText, hint).Layout(gtx)
				})),
				layout.Rigid(func(gtx C) D {
					return u.pressable(gtx, do, false, 6, box(u.p.Accent, colorNRGBA{}, 6, 0, inset(6, 14, u.strong(sizeSmall, u.p.Surface, label))))
				}))
		}}
		if p.resolveErr != "" {
			parts = append(parts, u.text(sizeSmall, u.p.Fail, p.resolveErr))
		}
		return column(6, parts...)(gtx)
	}
}

// reopen carries the task c again from an area, with what is to change, and
// shows its run; an escalation it was opened for closes when every
// criterion is met again.
func (u *UI) reopen(p *project, c *workspace.Change, from int, notes, ref string) error {
	if u.flowConfig == nil {
		return errNoModel
	}
	if p.running() {
		return errBusy
	}
	ctx, cancel := context.WithCancel(u.ctx)
	run, err := flow.Reopen(*u.flowConfig, p.path, c.Change.Dir, from, notes, u.invalidate)
	if err != nil {
		cancel()
		return err
	}
	if ref != "" {
		run.Settles(ref)
	}
	p.cancel = cancel
	p.run, p.live, p.seen, p.item, p.area, p.selected = run, nil, 0, nil, from, runKey
	go func() {
		_ = run.Start(ctx)
		p.stale.Store(true)
		u.invalidate()
	}()
	return nil
}

func keyFocus(tag any) key.FocusCmd { return key.FocusCmd{Tag: tag} }

var (
	errNoModel = errors.New("no model is configured, so nothing can be carried from here")
	errBusy    = errors.New("a task is running in this project; wait for it to end, or stop it, first")
)

// lifecycleButton is the one thing to do with a task as it stands, beside
// its state where it is seen first: Stop a task at work, wherever it runs;
// Resume one that stopped, from where it stopped.
func (u *UI) lifecycleButton(p *project, s shown) W {
	c := s.c
	if c == nil {
		return nothing
	}
	state := s.state(p)
	stop, resume := u.clicks.of("life/stop/"+c.Change.Dir), u.clicks.of("life/resume/"+c.Change.Dir)
	button := func(gtx C, btn *widget.Clickable, label string, bg, fg colorNRGBA) D {
		return u.pressable(gtx, btn, false, 6, box(bg, colorNRGBA{}, 6, 0, inset(5, 16, u.strong(sizeBody, fg, label))))
	}
	return func(gtx C) D {
		switch state {
		case "working", "waiting for you":
			if u.clicked(gtx, stop) {
				p.resolveErr = ""
				if s.live && p.cancel != nil {
					p.cancel() // this window's own run
				} else if err := flow.Stop(c.Change.Dir, u.person()); err != nil {
					p.resolveErr = err.Error()
				}
			}
			return button(gtx, stop, "■ Stop", u.p.Fail, u.p.Surface)
		case "stopped":
			if u.clicked(gtx, resume) {
				p.resolveErr = ""
				if err := u.resume(p, c); err != nil {
					p.resolveErr = err.Error()
				}
			}
			return button(gtx, resume, "▶ Resume from where it stopped", u.p.Accent, u.p.Surface)
		}
		return D{}
	}
}

// lifecycle is, under a task that stopped, the other ways on: carried again
// with something changed, from its specification or its plan, or a new
// intent; and what stopped a stop or a resume, if anything did.
func (u *UI) lifecycle(p *project, s shown) W {
	c := s.c
	if c == nil {
		return nothing
	}
	state := s.state(p)
	return func(gtx C) D {
		var parts []W
		if state == "stopped" {
			parts = append(parts, u.resolveActions(p, c, "", "it stopped"))
		}
		if p.resolveErr != "" && p.resolving == "" {
			parts = append(parts, u.text(sizeSmall, u.p.Fail, p.resolveErr))
		}
		if len(parts) == 0 {
			return D{}
		}
		return column(8, parts...)(gtx)
	}
}

// resume carries a stopped task on from where it stopped, and shows its run.
func (u *UI) resume(p *project, c *workspace.Change) error {
	if u.flowConfig == nil {
		return errNoModel
	}
	if p.running() {
		return errBusy
	}
	ctx, cancel := context.WithCancel(u.ctx)
	run, err := flow.Resume(*u.flowConfig, p.path, c.Change.Dir, u.invalidate)
	if err != nil {
		cancel()
		return err
	}
	p.cancel = cancel
	p.run, p.live, p.seen, p.item, p.selected = run, nil, 0, nil, runKey
	go func() {
		_ = run.Start(ctx)
		p.stale.Store(true)
		u.invalidate()
	}()
	return nil
}

func (u *UI) person() string {
	if u.flowConfig != nil && u.flowConfig.Person != "" {
		return u.flowConfig.Person
	}
	return "the person at the workbench"
}
