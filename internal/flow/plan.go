package flow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/meta"
	"github.com/igannytskyi/iflow/internal/model"
)

// unit is one work unit as the plan derives it.
type unit struct {
	id, class, aoe  string
	title, brief    string
	provides        []string // the names and signatures it gives the others
	tests           bool     // it writes the tests that judge the change
	scope, criteria []string
	preparatoryFor  string
	dependsOn       string
}

const plannerSystem = `You divide a specified change into units of work. Each unit is carried out by an agent that sees only its own unit: the brief you write, the files it may change, and the code around them. It never sees the specification, the person, or the other units' briefs. You answer with one JSON object and nothing else.

Rules you must follow:
- Tests come first: one or more units of kind "tests", whose scope is only among the new test files you are given. They judge the change from outside, calling the code by the names the code units will give it; they never define what the change itself provides — only fakes of what it depends on.
- Then units of kind "code", in the order they are carried out, whose scope is only among the files the change may touch. Each later unit can read what the earlier ones wrote, and nothing a later one will write — so order them by what they provide: the types and interfaces first, then what uses them, then what wires them into the entry point. A unit never defines what another unit provides. Keep a unit to one coherent piece; wiring something into what exists is a unit of its own when it touches another file.
- "provides" of a code unit is the contract: every type, function and method it defines that another unit uses, each with its full signature in the estate's language (for Go: "func NewServer(c *Catalog, clock Clock) *Server", "type Clock interface { Now() time.Time }"). Every agent is handed every unit's provides, and the tests call exactly these.
- Every criterion is assigned to at least one code unit; the last code unit carries the criterion that nothing else moves. A tests unit carries the criteria its tests judge.
- Each unit is written as a ticket complete for someone who has read nothing else: its objective; the requirements (FR-n) and criteria it serves; what it relies on that exists or that earlier units provide; its tasks in order; the names and signatures it provides, exactly — the same in the tests units and the code units; its constraints (what not to touch or change); its edge cases; and when it is done.

The JSON object has exactly these keys:
- "approach": {"summary": one paragraph, "steps": points in order, "risks": points}
- "units": a list of {"title", "kind": "tests" or "code", "scope": [paths], "criteria": [criterion ids], "requirements": [FR ids], "objective": one paragraph, "relies_on": [points], "tasks": [points, in order], "provides": [signatures or names], "constraints": [points], "edge_cases": [points], "done_when": [points]}`

// planDraft is what the planner answers.
type planDraft struct {
	Approach struct {
		Summary string   `json:"summary"`
		Steps   []string `json:"steps"`
		Risks   []string `json:"risks"`
	} `json:"approach"`
	Units []ticket `json:"units"`
}

// ticket is one unit as the planner writes it: everything its agent needs,
// and nothing about the other units but what it relies on from them.
type ticket struct {
	Title        string   `json:"title"`
	Kind         string   `json:"kind"`
	Scope        []string `json:"scope"`
	Criteria     []string `json:"criteria"`
	Requirements []string `json:"requirements"`
	Objective    string   `json:"objective"`
	ReliesOn     []string `json:"relies_on"`
	Tasks        []string `json:"tasks"`
	Provides     []string `json:"provides"`
	Constraints  []string `json:"constraints"`
	EdgeCases    []string `json:"edge_cases"`
	DoneWhen     []string `json:"done_when"`
}

// brief is the ticket as its agent reads it.
func (t ticket) brief() string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(t.Objective) + "\n")
	if len(t.Requirements) > 0 || len(t.Criteria) > 0 {
		b.WriteString("\nServes: " + strings.Join(append(slices.Clone(t.Requirements), t.Criteria...), ", ") + "\n")
	}
	for _, sec := range []struct {
		title  string
		points []string
	}{{"Relies on", t.ReliesOn}, {"Tasks", t.Tasks}, {"Provides", t.Provides}, {"Constraints", t.Constraints},
		{"Edge cases", t.EdgeCases}, {"Done when", t.DoneWhen}} {
		if len(sec.points) == 0 {
			continue
		}
		b.WriteString("\n" + sec.title + ":\n")
		for i, p := range sec.points {
			if sec.title == "Tasks" {
				fmt.Fprintf(&b, "%d. %s\n", i+1, strings.TrimSpace(p))
			} else {
				b.WriteString("- " + strings.TrimSpace(p) + "\n")
			}
		}
	}
	return b.String()
}

// plan is area 2: the specification divided into units by an agent that
// reads it and the outline of what it touches, and each unit described for
// the agent that will carry it out. What the planner proposes is held to the
// method by code before it is written: tests first, scopes within what the
// specification allows, every criterion covered.
func (r *Run) plan(ctx context.Context) error {
	r.set(2, Working)
	d := r.spec
	var all, byTests []string
	for i, c := range d.Criteria {
		id := r.id("CR", i+1)
		all = append(all, id)
		if c.New && len(d.NewTests) > 0 {
			byTests = append(byTests, id)
		}
	}
	var units []unit
	var approach []artifact.Part
	var planner string
	feedback := ""
	if r.from == 2 {
		feedback = r.reopened
	}
	for round := 1; ; round++ {
		units, approach, planner = r.planned(ctx, all, byTests, feedback)
		if units == nil || round > reviewRounds {
			break
		}
		spec := r.specText()
		text := planText(units, approach)
		v, err := r.reviewed(ctx, 2, "plan", planReviewSystem, "# The specification\n\n"+spec+"\n# The plan\n\n"+text, round)
		if err != nil {
			r.say(2, "", "the review could not be had: %v", err)
			break
		}
		if v.sufficient() {
			break
		}
		feedback = "\n\n# Your previous plan\n\n" + text + "\n# What a reviewer found in it\n\n" + v.forAuthor() +
			"\nPlan again, whole, resolving every blocking and major finding."
	}
	if units == nil {
		units, planner = r.derivedUnits(all, byTests), "the framework, deriving the units from the specification alone: "+planner
	}
	repo := r.cfg.Estate.Repo(r.repo)
	totalFirm, totalReach := 0, 0
	for i := range units {
		if units[i].tests {
			units[i].aoe = "the arbiter only, derived, high"
			continue
		}
		reach, err := repo.Affects(units[i].scope)
		if err != nil {
			return r.fail(2, "the estate could not say what the scope reaches: %v", err)
		}
		firm := 0
		for _, x := range reach {
			if x.Confidence != "low" {
				firm++
			}
		}
		totalFirm, totalReach = totalFirm+firm, totalReach+len(reach)
		units[i].aoe = fmt.Sprintf("%d file(s) reached firmly, %d on a name alone, derived, medium", firm, len(reach)-firm)
	}
	r.units = units
	// doc is the plan described, for plan.md; record, below, is the plan's
	// record in the data model.
	doc := artifact.PlanDoc{Name: r.slug, Spec: r.id("SPEC"), Phases: "1",
		Intermediate: "yes — the tests alone fail and nothing ships on them; each code unit leaves the estate building",
		Residue:      "none — remove the new tests and what the code units added",
		Coverage: fmt.Sprintf("The units cover all %d criteria. The estate reached %d file(s) from the scope, %d of them firmly.",
			len(all), totalReach, totalFirm),
		Approach: approach, Planner: planner}
	for _, u := range units {
		doc.Briefs = append(doc.Briefs, artifact.Part{Title: u.id + " — " + u.title, Text: u.brief})
		r.say(2, u.id, "%s: %s — %s", u.id, u.title, strings.Join(u.scope, ", "))
	}
	record := meta.Plan{Schema: meta.SchemaVersion, Name: r.slug, Specification: r.id("SPEC"), Phases: 1,
		IntermediateValid: doc.Intermediate, Residue: doc.Residue, CoverageText: doc.Coverage, Planner: planner}
	for _, u := range units {
		record.Units = append(record.Units, meta.WorkUnit{ID: u.id, Phase: 1, Class: meta.Class(u.class), Scope: u.scope,
			AreaOfEffect: u.aoe, Criteria: u.criteria, DependsOn: splitIDs(u.dependsOn), PreparatoryFor: splitIDs(u.preparatoryFor),
			Title: u.title, Brief: u.brief, Provides: u.provides, Tests: u.tests})
	}
	if err := artifact.Save(r.dir, artifact.Plan, record); err != nil {
		return r.fail(2, "the plan could not be written: %v", err)
	}
	_ = os.WriteFile(filepath.Join(r.dir, artifact.PlanFile), []byte(doc.Document()), 0o644)
	if err := os.MkdirAll(filepath.Join(r.dir, artifact.UnitsDir), 0o755); err == nil {
		for _, u := range units {
			ticket := "# " + u.id + " — " + u.title + "\n\nScope: " + strings.Join(u.scope, ", ") + "\n\n" + r.goal(u) + "\n"
			_ = os.WriteFile(filepath.Join(r.dir, artifact.UnitsDir, u.id+".md"), []byte(ticket), 0o644)
		}
	}
	if err := r.record("plan", r.id("SPEC"), fmt.Sprintf("%d unit(s), planned by %s", len(units), planner)); err != nil {
		return err
	}
	if rep, err := r.check(ctx, 2); err != nil || !rep.Passed() {
		return r.fail(2, "the plan does not pass the gate")
	}
	r.set(2, Done)
	return nil
}

// planned asks the planner, up to three times, for units the method can
// accept; it returns nil units, and why, when none came.
func (r *Run) planned(ctx context.Context, all, byTests []string, feedback string) ([]unit, []artifact.Part, string) {
	spec := r.specText()
	if spec == "" {
		return nil, nil, "the specification could not be read"
	}
	repo := r.cfg.Estate.Repo(r.repo)
	var b strings.Builder
	b.WriteString("# The specification\n\n" + spec + "\n# What the units may touch\n\n")
	b.WriteString("New test files, for tests units: " + strings.Join(r.spec.NewTests, ", ") + "\n")
	b.WriteString("Files the change may touch, for code units: " + strings.Join(r.spec.Scope, ", ") + "\n")
	b.WriteString("Criteria judged by the new tests: " + strings.Join(byTests, ", ") + "\n")
	b.WriteString("\n# The estate, as names — no code\n\n" + repo.Map(r.cfg.packBudget()/4))
	prompt := b.String() + feedback
	var why string
	m := r.cfg.planner()
	for attempt := 1; attempt <= 3; attempt++ {
		r.say(2, "", "asking %s to divide the work, attempt %d", m.Name(), attempt)
		began := time.Now()
		var p planDraft
		kept, handed := r.handed("planner", plannerSystem, prompt+why)
		_, u, err := model.AskJSON(ctx, m, plannerSystem, prompt+why, &p)
		r.spend(u)
		r.cost(artifact.Cost{Area: 2, What: fmt.Sprintf("the plan, attempt %d", attempt), Actor: m.Name(), Turns: 1,
			Handed: handed, Kept: kept}, u, time.Since(began))
		if errors.Is(err, model.ErrUnreadable) {
			why = "\n\nYour last answer was not one JSON object: " + err.Error()
			continue
		}
		if err != nil {
			return nil, nil, "the planner could not be asked: " + err.Error()
		}
		units, problem := r.unitsOf(p, all, byTests)
		if problem != "" {
			r.say(2, "", "the plan is sent back: %s", problem)
			why = "\n\nYour last plan cannot be used: " + problem + ". Answer again."
			continue
		}
		approach := []artifact.Part{{Title: "How", Text: p.Approach.Summary}, {Title: "In order", Points: p.Approach.Steps},
			{Title: "What could go wrong", Points: p.Approach.Risks}}
		return units, approach, m.Name()
	}
	return nil, nil, "the planner's answers could not be used (" + strings.TrimSpace(strings.TrimPrefix(why, "\n\nYour last plan cannot be used: ")) + ")"
}

// unitsOf holds a planner's answer to the method and numbers its units:
// tests first, each scope within what it may touch, every criterion carried.
func (r *Run) unitsOf(p planDraft, all, byTests []string) ([]unit, string) {
	var tests, code []unit
	for _, pu := range p.Units {
		u := unit{title: pu.Title, class: r.spec.Class, scope: pu.Scope, brief: pu.brief(), tests: pu.Kind == "tests"}
		if !u.tests {
			u.provides = pu.Provides
		}
		if len(u.scope) == 0 || strings.TrimSpace(pu.Objective) == "" || len(pu.Tasks) == 0 {
			return nil, fmt.Sprintf("unit %q has no scope, no objective or no tasks", pu.Title)
		}
		allowed := r.spec.Scope
		if u.tests {
			allowed = r.spec.NewTests
		} else if pu.Kind != "code" {
			return nil, fmt.Sprintf("unit %q is of kind %q, which is neither tests nor code", pu.Title, pu.Kind)
		}
		for _, s := range u.scope {
			if !slices.Contains(allowed, s) {
				return nil, fmt.Sprintf("unit %q would touch %s, which a %s unit may not", pu.Title, s, pu.Kind)
			}
		}
		for _, c := range pu.Criteria {
			if !slices.Contains(all, c) {
				return nil, fmt.Sprintf("unit %q names %s, which is not a criterion", pu.Title, c)
			}
		}
		u.criteria = pu.Criteria
		if u.tests {
			tests = append(tests, u)
		} else {
			code = append(code, u)
		}
	}
	if len(code) == 0 {
		return nil, "it has no code unit"
	}
	if len(r.spec.NewTests) > 0 {
		var written []string
		for _, u := range tests {
			written = append(written, u.scope...)
		}
		for _, t := range r.spec.NewTests {
			if !slices.Contains(written, t) {
				return nil, "no tests unit writes " + t
			}
		}
	}
	covered := map[string]bool{}
	for _, u := range code {
		for _, c := range u.criteria {
			covered[c] = true
		}
	}
	// A criterion the planner left to no code unit is the last code unit's:
	// that unit completes the change, and the suite it must pass judges
	// every criterion. Said, not silently fixed.
	for _, c := range all {
		if !covered[c] {
			last := &code[len(code)-1]
			last.criteria = append(last.criteria, c)
			last.brief += "\nAlso serves " + c + ", which the planner gave to no unit; this unit completes the change.\n"
			r.say(2, "", "%s was carried by no code unit; it is given to the last one, %q", c, last.title)
		}
	}
	// The tests units judge exactly the criteria the new tests judge.
	for i := range tests {
		tests[i].criteria = slices.DeleteFunc(tests[i].criteria, func(c string) bool { return !slices.Contains(byTests, c) })
		if len(tests[i].criteria) == 0 {
			tests[i].criteria = byTests
		}
	}
	return r.number(append(tests, code...)), ""
}

// number gives units their ids and their order: every tests unit prepares
// the first code unit, and each code unit depends on the unit before it.
func (r *Run) number(units []unit) []unit {
	for i := range units {
		units[i].id = r.id("WU", i+1)
	}
	firstCode := slices.IndexFunc(units, func(u unit) bool { return !u.tests })
	var testIDs []string
	for i := range units {
		switch {
		case units[i].tests:
			units[i].preparatoryFor = units[firstCode].id
			testIDs = append(testIDs, units[i].id)
		case i == firstCode:
			units[i].dependsOn = joinIDs(testIDs)
		default:
			units[i].dependsOn = units[i-1].id
		}
	}
	return units
}

// derivedUnits is the division made without a planner: the new tests as one
// unit, the whole scope as the other.
func (r *Run) derivedUnits(all, byTests []string) []unit {
	d := r.spec
	var units []unit
	if len(d.NewTests) > 0 {
		units = append(units, unit{title: "the tests that judge the change", class: d.Class, scope: d.NewTests, criteria: byTests, tests: true,
			brief: "Write the tests that decide the criteria named, as the specification states them."})
	}
	units = append(units, unit{title: "the change", class: d.Class, scope: d.Scope, criteria: all,
		brief: "Make the criteria hold: " + r.statement})
	return r.number(units)
}

// planText is a plan as a reviewer reads it: the approach, then every unit
// with its scope and brief.
func planText(units []unit, approach []artifact.Part) string {
	var b strings.Builder
	for _, p := range approach {
		if p.Text != "" {
			b.WriteString(p.Title + ": " + p.Text + "\n")
		}
		for _, pt := range p.Points {
			b.WriteString(p.Title + ": " + pt + "\n")
		}
	}
	for _, u := range units {
		kind := "code"
		if u.tests {
			kind = "tests"
		}
		fmt.Fprintf(&b, "\n## %s — %s (%s)\nScope: %s\nCriteria: %s\n\n%s", u.id, u.title, kind, strings.Join(u.scope, ", "), joinIDs(u.criteria), u.brief)
	}
	return b.String()
}

// splitIDs is a list of identifiers written "A, B".
func splitIDs(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
