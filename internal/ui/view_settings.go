package ui

import (
	"fmt"
	"slices"
	"strings"

	"gioui.org/layout"

	"github.com/igannytskyi/iflow/internal/models"
)

// settingsView is what the workbench is set to: how it looks, what carries a
// task out, and which projects it remembers.
func (u *UI) settingsView(gtx C) D {
	section := func(title, note string, ws ...W) W {
		head := []W{u.text(sizeSmall, u.p.Faint, strings.ToUpper(title))}
		if note != "" {
			head = append(head, u.text(sizeSmall, u.p.Muted, note))
		}
		return column(10, append([]W{column(2, head...)}, ws...)...)
	}
	body := []W{
		u.strong(sizeHead, u.p.Text, "Settings"),
		section("Appearance", "", row(8, u.choice("theme/light", "Light", !u.dark, func() { u.theme(false) }),
			u.choice("theme/dark", "Dark", u.dark, func() { u.theme(true) }))),
		section("Task list", "Finished tasks — landed, or closed by you — are folded under each project as Done; this shows them always.",
			row(8, u.choice("landed/fold", "Folded", !u.settings.ShowLanded, func() { u.settings.ShowLanded = false; u.settings.save() }),
				u.choice("landed/show", "Shown", u.settings.ShowLanded, func() { u.settings.ShowLanded = true; u.settings.save() }))),
		section("Costs panel", "The panel beside a task: what it cost, area by area, and what each agent was handed. Folded, it waits at the right edge; its width is dragged at its border.",
			row(8, u.choice("costs/shown", "Shown", !u.settings.HideCosts, func() { u.settings.HideCosts = false; u.settings.save() }),
				u.choice("costs/folded", "Folded", u.settings.HideCosts, func() { u.settings.HideCosts = true; u.settings.save() }),
				u.text(sizeSmall, u.p.Faint, "·"),
				u.choice("costs/right", "On the right", !u.settings.CostsLeft, func() { u.settings.CostsLeft = false; u.settings.save() }),
				u.choice("costs/left", "On the left", u.settings.CostsLeft, func() { u.settings.CostsLeft = true; u.settings.save() }))),
		section("Models", "Which model does each piece of work, and how hard it thinks. A change applies to the next task; one running keeps what it started with. Kept in "+models.File()+"; a key is read from the environment or the keychain, never kept there.",
			u.modelFacts()...),
		section("Projects", "Forgetting a project only takes it off this list; nothing in its folder is touched.", u.projectSettings()...),
	}
	if u.settings.path != "" {
		body = append(body, u.text(sizeSmall, u.p.Faint, "kept in "+u.settings.path))
	}
	return u.scroll(gtx, "settings", body)
}

func (u *UI) theme(dark bool) {
	u.setTheme(dark)
	u.settings.Dark = dark
	u.settings.save()
}

// choice is one of a few options, the chosen one marked.
func (u *UI) choice(key, label string, on bool, pick func()) W {
	return func(gtx C) D {
		btn := u.clicks.of(key)
		if u.clicked(gtx, btn) && !on {
			pick()
		}
		border := u.p.Line
		if on {
			border = u.p.Accent
		}
		bg, fg := colorNRGBA{}, u.p.Text
		if on {
			bg, fg = u.p.AccentSoft, u.p.Accent
		}
		return u.pressable(gtx, btn, false, 6, box(bg, border, 6, 0, inset(6, 16, u.text(sizeBody, fg, label))))
	}
}

func (u *UI) modelFacts() []W {
	m, err := models.Load()
	if err != nil {
		return []W{u.text(sizeSmall, u.p.Fail, err.Error())}
	}
	type role struct {
		key, label string
		spec, eff  *string
	}
	roles := []role{
		{"author", "writes the specification", &m.Roles.Author, &m.Effort.Author},
		{"reviewer", "reviews documents", &m.Roles.Reviewer, &m.Effort.Reviewer},
		{"planner", "divides the work", &m.Roles.Planner, &m.Effort.Planner},
		{"executor", "carries out each unit", &m.Roles.Executor, &m.Effort.Executor},
	}
	var out []W
	for _, r := range roles {
		var picks []W
		for _, c := range models.Catalogue {
			picks = append(picks, u.choice("model/"+r.key+"/"+c.Spec, c.Label+"  "+c.Price, *r.spec == c.Spec, func() {
				*r.spec = c.Spec
				u.applyModels(m)
			}))
		}
		var efforts []W
		for _, e := range []string{"low", "medium", "high"} {
			efforts = append(efforts, u.choice("effort/"+r.key+"/"+e, e, *r.eff == e, func() {
				*r.eff = e
				u.applyModels(m)
			}))
		}
		out = append(out, column(6,
			row(10, u.strong(sizeBody, u.p.Text, r.label), u.mono(sizeSmall, u.p.Faint, *r.spec)),
			u.wrap(6, picks),
			row(6, append([]W{u.text(sizeSmall, u.p.Muted, "thinks")}, efforts...)...),
		))
	}
	if c := u.flowConfig; c != nil {
		out = append(out, u.text(sizeSmall, u.p.Muted, fmt.Sprintf("Each unit may spend at most %s; criteria are accepted by %s.", money(c.BudgetUSD), c.Person)))
	}
	if u.modelsNote != "" {
		var lines []W
		for _, l := range strings.Split(u.modelsNote, "\n") {
			lines = append(lines, u.text(sizeSmall, u.p.Fail, l))
		}
		out = append(out, box(u.p.FailSoft, colorNRGBA{}, 6, 10, column(4, lines...)))
	}
	return out
}

// applyModels keeps a new choice of models and makes what the next task is
// carried out with from it.
func (u *UI) applyModels(m models.Config) {
	if err := m.Save(); err != nil {
		u.modelsNote = err.Error()
		return
	}
	if u.rebuild != nil {
		cfg, problems := u.rebuild()
		u.flowConfig, u.modelsNote = &cfg, problems
	}
}

func (u *UI) projectSettings() []W {
	var out []W
	for _, p := range u.projects {
		out = append(out, func(gtx C) D {
			forget := u.clicks.of("forget/" + p.path)
			if u.clicked(gtx, forget) && !p.running() {
				u.forget(p)
			}
			action := func(gtx C) D {
				return u.pressable(gtx, forget, false, 4, inset(4, 10, u.text(sizeSmall, u.p.Fail, "forget")))
			}
			if p.running() {
				action = u.text(sizeSmall, u.p.Faint, "running")
			}
			return layout.Flex{Gap: gtx.Dp(12), Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(width(160, u.strong(sizeBody, u.p.Text, p.name))),
				layout.Flexed(1, u.oneLine(sizeSmall, u.p.Muted, p.path)),
				layout.Rigid(action))
		})
	}
	if len(out) == 0 {
		out = append(out, u.text(sizeBody, u.p.Muted, "none yet"))
	}
	out = append(out, u.text(sizeSmall, u.p.Faint, fmt.Sprintf("%s remembered; add one with + on the left.", plural(len(u.projects), "project"))))
	return out
}

func (u *UI) forget(p *project) {
	u.projects = slices.DeleteFunc(u.projects, func(q *project) bool { return q == p })
	u.settings.Projects = slices.DeleteFunc(u.settings.Projects, func(s string) bool { return s == p.path })
	if u.cur == p {
		u.cur = nil
		if len(u.projects) > 0 {
			u.cur = u.projects[0]
		}
	}
	u.settings.save()
}
