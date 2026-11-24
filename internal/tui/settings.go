package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/igannytskyi/iflow/internal/models"
)

// Settings are the window's, in the terminal: the model of each role and
// how hard it thinks, the cheapest provider, what a change may spend, how
// often its specification and plan are reviewed — kept where the window
// keeps them (models.json), read by the service at every change it starts
// — and the screen's own colours. Each is switched where it stands: ←→ or
// a click changes it, and it is saved at once.

// setting is one line of the settings: what it is, what it is now, and how
// it moves to its next or its last value.
type setting struct {
	name, about string
	value       func(models.Config, Model) string
	step        func(*models.Config, *Model, int)
}

var efforts = []string{"low", "medium", "high"}
var budgets = []float64{0.5, 1, 2, 3, 5, 10}

// cycle is the value of list n steps from now, round.
func cycle[T comparable](list []T, now T, n int) T {
	i := slices.Index(list, now)
	if i < 0 {
		return list[0]
	}
	return list[((i+n)%len(list)+len(list))%len(list)]
}

func modelLabel(spec string) string {
	for _, c := range models.Catalogue {
		if c.Spec == spec {
			return c.Label + "  " + c.Price
		}
	}
	if spec == "" {
		return "none"
	}
	return spec
}

// roleSetting is a role's model, among those offered.
func roleSetting(name, about string, of func(*models.Roles) *string, optional bool) setting {
	return setting{name: name, about: about,
		value: func(c models.Config, _ Model) string { return modelLabel(*of(&c.Roles)) },
		step: func(c *models.Config, _ *Model, n int) {
			specs := make([]string, 0, len(models.Catalogue)+1)
			if optional {
				specs = append(specs, "")
			}
			for _, ch := range models.Catalogue {
				specs = append(specs, ch.Spec)
			}
			if now := *of(&c.Roles); !slices.Contains(specs, now) {
				specs = append(specs, now) // a model set by hand stays among the choices
			}
			*of(&c.Roles) = cycle(specs, *of(&c.Roles), n)
		}}
}

// effortSetting is how hard a role's model thinks.
func effortSetting(name string, of func(*models.Roles) *string) setting {
	return setting{name: name + " thinks", about: "low, medium or high",
		value: func(c models.Config, _ Model) string { return *of(&c.Effort) },
		step:  func(c *models.Config, _ *Model, n int) { *of(&c.Effort) = cycle(efforts, *of(&c.Effort), n) }}
}

var settings = []setting{
	roleSetting("author", "writes the specification", func(r *models.Roles) *string { return &r.Author }, false),
	roleSetting("reviewer", "reviews the specification and the plan", func(r *models.Roles) *string { return &r.Reviewer }, false),
	roleSetting("planner", "divides the work", func(r *models.Roles) *string { return &r.Planner }, false),
	roleSetting("executor", "carries out each unit", func(r *models.Roles) *string { return &r.Executor }, false),
	roleSetting("stronger executor", "takes a unit the executor could not", func(r *models.Roles) *string { return &r.Stronger }, true),
	effortSetting("author", func(r *models.Roles) *string { return &r.Author }),
	effortSetting("reviewer", func(r *models.Roles) *string { return &r.Reviewer }),
	effortSetting("planner", func(r *models.Roles) *string { return &r.Planner }),
	effortSetting("executor", func(r *models.Roles) *string { return &r.Executor }),
	{name: "provider", about: "for a model served through OpenRouter",
		value: func(c models.Config, _ Model) string {
			if c.FloorPricing {
				return "the cheapest for the same model"
			}
			return "OpenRouter decides"
		},
		step: func(c *models.Config, _ *Model, _ int) { c.FloorPricing = !c.FloorPricing }},
	{name: "budget", about: "what one change may spend, all its models together",
		value: func(c models.Config, _ Model) string { return fmt.Sprintf("$%.2f a change", budgetOf(c)) },
		step:  func(c *models.Config, _ *Model, n int) { c.BudgetUSD = cycle(budgets, budgetOf(*c), n) }},
	{name: "review", about: "how often a specification and a plan are reviewed",
		value: func(c models.Config, _ Model) string {
			if roundsOf(c) == 1 {
				return "once"
			}
			return "twice"
		},
		step: func(c *models.Config, _ *Model, _ int) { c.ReviewRounds = 3 - roundsOf(*c) }},
	{name: "colours", about: "this screen's",
		value: func(_ models.Config, m Model) string {
			if m.theme == Light {
				return "light"
			}
			return "the terminal's"
		},
		step: func(_ *models.Config, m *Model, _ int) {
			m.theme = cycle([]Theme{Light, Terminal}, m.theme, 1)
			m.st, m.md = stylesOf(m.theme), map[string]string{}
		}},
}

func budgetOf(c models.Config) float64 {
	if c.BudgetUSD > 0 {
		return c.BudgetUSD
	}
	return 2
}

func roundsOf(c models.Config) int {
	if c.ReviewRounds == 2 {
		return 2
	}
	return 1
}

// settingsView is the settings in the middle, width by height: a line each,
// the chosen one lit.
func (m Model) settingsView(width, height int) string {
	c, err := models.Load()
	lines := []string{m.st.strong.Render("settings") + m.st.faint.Render(" — kept where the window keeps them; a change started after this one uses them"), ""}
	if err != nil {
		lines = append(lines, m.st.bad.Render("the settings could not be read: "+err.Error()))
	}
	for i, s := range settings {
		line := fmt.Sprintf("%-20s %s", s.name, s.value(c, m))
		if i == m.setRow {
			line = m.lit(line+"   ‹ ›", width)
		}
		lines = append(lines, line)
		if i == m.setRow {
			lines = append(lines, m.st.faint.Render("  "+s.about))
		}
	}
	if m.setNote != "" {
		lines = append(lines, "", m.st.bad.Render(m.setNote))
	}
	return strings.Join(lines, "\n")
}

// settingsRow is the setting drawn on line row of the view, or -1.
func (m Model) settingsRow(row int) int {
	r := row - 2 // after the title and the blank line
	switch {
	case r < 0 || r == m.setRow+1: // the line that says what the chosen one is
		return -1
	case r > m.setRow+1:
		r--
	}
	if r >= len(settings) {
		return -1
	}
	return r
}

// switchSetting moves the chosen setting n values on, and saves it.
func (m Model) switchSetting(n int) Model {
	c, err := models.Load()
	if err != nil {
		m.setNote = err.Error()
		return m
	}
	settings[m.setRow].step(&c, &m, n)
	if err := c.Save(); err != nil {
		m.setNote = "not saved: " + err.Error()
		return m
	}
	m.setNote = ""
	return m
}

// settingsKey is a key while the settings are open.
func (m Model) settingsKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc", ",", "q":
		m.settingsOpen = false
	case "up", "k":
		m.setRow = max(0, m.setRow-1)
	case "down", "j":
		m.setRow = min(len(settings)-1, m.setRow+1)
	case "right", "l", "enter", " ":
		m = m.switchSetting(1)
	case "left", "h":
		m = m.switchSetting(-1)
	}
	return m, nil
}
