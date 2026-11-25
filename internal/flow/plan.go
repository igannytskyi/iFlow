package flow

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/estate"
	"github.com/igannytskyi/iflow/internal/harness"
	"github.com/igannytskyi/iflow/internal/meta"
	"github.com/igannytskyi/iflow/internal/model"
)

// unit is one work unit as the plan derives it.
type unit struct {
	id, class, aoe string
	title, brief   string
	kind           meta.UnitKind
	// provides is the names it gives the others; uses, the names of
	// theirs it builds on. Together they are the contract.
	provides []meta.Provision
	uses     []string
	// reads is the files of the estate outside its scope it builds on.
	reads           []string
	scope, criteria []string
	preparatoryFor  string
	dependsOn       string
}

// isTests says the unit writes the tests that judge the change.
func (u unit) isTests() bool { return u.kind == meta.TestsUnit }

// isContract says the unit declares the contract every other unit builds on.
func (u unit) isContract() bool { return u.kind == meta.ContractUnit }

// names is every name the unit gives the others.
func (u unit) names() []string {
	var out []string
	for _, p := range u.provides {
		if p.Kind != meta.Described && !slices.Contains(out, p.Name) {
			out = append(out, p.Name)
		}
	}
	return out
}

// gives says the unit provides what a use names: a name bare, or a field or
// a method by its type and name — Config.Floor — however its package is
// written before them.
func (u unit) gives(use string) bool {
	return slices.ContainsFunc(u.provides, func(p meta.Provision) bool {
		if p.Kind == meta.Described {
			return false
		}
		if use == p.Name {
			return true
		}
		q := p.Of + "." + p.Name
		return p.Of != "" && (use == q || strings.HasSuffix(use, "."+q))
	})
}

// ambiguous says where a unit uses a bare name that units give to more than
// one type — Config.Floor and OpenAI.Floor are both Floor — which no order
// can be read from: the unit names it with its type.
func ambiguous(units []unit) string {
	for _, u := range units {
		for _, use := range u.uses {
			if strings.Contains(use, ".") {
				continue
			}
			var as []string
			for _, o := range units {
				for _, p := range o.provides {
					if p.Name == use && p.Kind != meta.Described {
						if q := cmp.Or(p.Of+".", "") + p.Name; !slices.Contains(as, q) {
							as = append(as, q)
						}
					}
				}
			}
			if len(as) > 1 {
				return fmt.Sprintf("unit %q uses %s, and the plan gives %s: name the one it uses with its type, as %s, in its uses", u.title, use, strings.Join(as, " and "), as[0])
			}
		}
	}
	return ""
}

const plannerSystem = `You divide a specified change into units of work. Each unit is carried out by an agent that sees its own unit — the brief you write, the files it may change, the code around them — and the specification's decisions; it never sees the person or the other units' briefs. You answer with one JSON object and nothing else.

Rules you must follow:
- Tests come first: one or more units of kind "tests", whose scope is only among the new test files you are given. They judge the change from outside, calling the code by the names the code units will give it; they never define what the change itself provides — only fakes of what it depends on.
- Then units of kind "code", whose scope is only among the files the change may touch. Their order is derived from what each provides and uses: a unit is carried after every unit whose names it uses. Say in "uses" every name of another unit's provides a unit builds on, and nothing more — a field or a method by its type and name, as Config.Floor. A unit never defines what another unit provides. Keep a unit to one coherent piece; wiring something into what exists is a unit of its own when it touches another file.
- "provides" of a code unit is the contract: every name it declares that another unit or the tests use, each as {"file": where it is declared, "name": the name it is called by, "kind": one of "function", "method", "type", "field", "variable", "constant", "of": for a field or a method, the type it belongs to, "signature": its declaration in the estate's language, without a body} — for Go, {"file": "server.go", "name": "NewServer", "kind": "function", "signature": "func NewServer(c *Catalog, clock Clock) *Server"}. A field added to a type is a "field" whose signature is the field as the type declares it — in Python and JavaScript, "name: type" and "name" — and whose "of" is the type. The framework declares every provision itself, before the tests, from exactly what you write here. Provides lists only names the unit adds: a name that exists already, whose body or type the unit changes, is not provided — what it changes belongs in its tasks. Every agent is handed every unit's provides, and the tests call exactly these.
- "uses" of a unit is the names, from other units' provides, that it builds on — exactly as they are named there. A unit uses only what an earlier unit provides. A test is never in "uses": which tests a code unit must pass, the framework knows from the tests units' ledger.
- "reads" of a unit is the files outside its scope that it builds on — a helper it calls, a test whose setup it follows — each a file that exists, or one another unit of the plan writes. Its agent is handed each whole: what exists as it is, what another unit writes as that unit left it.
- A unit fits what one agent is handed: its brief, the specification's decisions, and the files it changes, whole, within the budget you are told, with each file's weight. An agent does its best work on the least it needs: a unit whose files to change do not fit is two units. A file it reads is handed whole where it fits beside them, by its outline where not; read a file only when the unit builds on it.
- Every criterion is assigned to at least one code unit; the last code unit carries the criterion that nothing else moves. A tests unit carries the criteria its tests judge. A criterion the person decides needs no test: assign it to the code unit that makes it true.
- A unit relies only on what exists — as the estate you are shown says — or on what an earlier unit provides. Never on a helper, method or ability you assume is there: if it is not in what you are shown, a unit provides it first, or the unit does without it.
- Each unit is written as a ticket complete for someone who has read nothing else: its objective; the requirements (FR-n) and criteria it serves; what it relies on that exists or that earlier units provide; its tasks in order; the names and signatures it provides, exactly — the same in the tests units and the code units; its constraints (what not to touch or change); its edge cases; and when it is done.

The JSON object has exactly these keys:
- "approach": {"summary": one paragraph, "steps": points in order, "risks": points}
- "units": a list of {"title", "kind": "tests" or "code", "scope": [paths], "criteria": [criterion ids], "requirements": [FR ids], "objective": one paragraph, "relies_on": [points], "tasks": [points, in order], "provides": [{"file", "name", "kind", "of", "signature"}], "uses": [names], "reads": [paths], "constraints": [points], "edge_cases": [points], "done_when": [points]}`

// planDraft is what the planner answers.
type planDraft struct {
	Approach struct {
		Summary string   `json:"summary"`
		Steps   []string `json:"steps"`
		Risks   []string `json:"risks"`
	} `json:"approach"`
	Units tickets `json:"units"`
}

// ticket is one unit as the planner writes it: everything its agent needs,
// and nothing about the other units but what it relies on from them.
type ticket struct {
	Title        string           `json:"title"`
	Kind         string           `json:"kind"`
	Scope        []string         `json:"scope"`
	Criteria     []string         `json:"criteria"`
	Requirements []string         `json:"requirements"`
	Objective    string           `json:"objective"`
	ReliesOn     []string         `json:"relies_on"`
	Tasks        []string         `json:"tasks"`
	Provides     []meta.Provision `json:"provides"`
	Uses         []string         `json:"uses"`
	Reads        []string         `json:"reads"`
	Constraints  []string         `json:"constraints"`
	EdgeCases    []string         `json:"edge_cases"`
	DoneWhen     []string         `json:"done_when"`
}

// providedAlready says where a unit provides a name its file declares
// already: provides is what a unit adds; what it changes of what exists is
// its tasks', and a contract that declared it again would break its users.
func (r *Run) providedAlready(u unit) string {
	for _, p := range u.provides {
		if filepath.Ext(p.File) == ".go" {
			// In Go a name is its package's, not its file's: a top-level
			// name once a package, a field or method once a type.
			if at := goTaken(filepath.Join(r.repo, filepath.FromSlash(path.Dir(p.File))), p); at != "" {
				return fmt.Sprintf("it provides %s, which %s declares already in the same package — provides names only what the unit adds, under a name its package does not hold; what it changes of what exists belongs in its tasks", provisionName(p), path.Join(path.Dir(p.File), at))
			}
			continue
		}
		src, err := os.ReadFile(filepath.Join(r.repo, filepath.FromSlash(p.File)))
		if err != nil {
			continue // a file the unit creates
		}
		if names, _ := declaredIn(p.File, src); names[p.Name] {
			return fmt.Sprintf("it provides %s, which %s declares already — provides names only what the unit adds; what it changes of what exists belongs in its tasks", p.Name, p.File)
		}
	}
	return ""
}

// signatures is each provision as its agent reads it: its declaration, and
// where.
func signatures(provides []meta.Provision) []string {
	var out []string
	for _, p := range provides {
		if p.File == "" {
			out = append(out, p.Signature)
			continue
		}
		out = append(out, p.Signature+" — in "+p.File)
	}
	return out
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
	}{{"Relies on", t.ReliesOn}, {"Tasks", t.Tasks}, {"Provides", signatures(t.Provides)}, {"Constraints", t.Constraints},
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
		units, approach, planner = r.planned(ctx, byTests, feedback)
		if r.scopeFault != nil {
			sb := r.scopeFault
			r.scopeFault = nil
			return sb
		}
		if units == nil || round > r.cfg.reviewRounds() {
			break
		}
		spec := r.specText()
		text := planText(units, approach)
		v, err := r.reviewed(ctx, 2, "plan", planReviewSystem, "# The specification\n\n"+spec+"\n# The plan\n\n"+text+
			"\n# What exists where the change works — every definition by its first line\n\n"+r.signatures(), round)
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
	if err := ctx.Err(); err != nil {
		// Stopped while the plan was asked for: nothing is planned in its
		// place, and the run stops here.
		return err
	}
	if units == nil {
		// No plan was made that holds to the method; none is made up in its
		// place. The person reopens the change from its plan, saying more.
		return r.fail(2, "no plan could be made: %s", planner)
	}
	units = r.withContract(units)
	repo := r.cfg.Estate.Repo(r.repo)
	totalFirm, totalReach := 0, 0
	for i := range units {
		if units[i].isTests() {
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
	// What each unit waits for is written as the scheduler will carry it —
	// shared files, what it reads and uses, a criterion's last server — so
	// that the plan's record and every screen's graph say what happens.
	for i, deps := range r.dependencies() {
		var ids []string
		for _, d := range deps {
			ids = append(ids, units[d].id)
		}
		slices.Sort(ids)
		units[i].dependsOn = joinIDs(slices.Compact(ids))
	}
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
	record := r.planRecord(units)
	record.IntermediateValid, record.Residue, record.CoverageText, record.Planner = doc.Intermediate, doc.Residue, doc.Coverage, planner
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

// planRecord is units as the plan's record in the data model keeps them.
func (r *Run) planRecord(units []unit) meta.Plan {
	p := meta.Plan{Schema: meta.PlanVersion, Name: r.slug, Specification: r.id("SPEC"), Phases: 1}
	for _, u := range units {
		p.Units = append(p.Units, meta.WorkUnit{ID: u.id, Phase: 1, Class: meta.Class(u.class), Scope: u.scope,
			AreaOfEffect: u.aoe, Criteria: u.criteria, DependsOn: splitIDs(u.dependsOn), PreparatoryFor: splitIDs(u.preparatoryFor),
			Kind: u.kind, Title: u.title, Brief: u.brief, Provides: u.provides, Uses: u.uses, Reads: u.reads})
	}
	return p
}

// breaches is every invariant the units, with their contract, break against
// the specification as it was accepted (meta.PlanBreaches), said for the
// planner; or nothing.
func (r *Run) breaches(units []unit) string {
	var spec meta.Specification
	if ok, err := artifact.LoadJSON(r.dir, artifact.Specification, &spec); !ok || err != nil {
		return fmt.Sprintf("the specification could not be read (%v)", err)
	}
	units = r.withContract(units)
	var out []string
	for _, b := range meta.PlanBreaches(r.planRecord(units), spec) {
		// The planner knows its units by their titles, not by the ids they
		// are given after it answers.
		if i := slices.IndexFunc(units, func(u unit) bool { return u.id == b.Unit }); i >= 0 {
			b.Unit = strconv.Quote(units[i].title)
		}
		out = append(out, b.String())
	}
	return strings.Join(out, "; ")
}

// planned asks the planner, up to three times, for units the method can
// accept; it returns nil units, and why, when none came.
func (r *Run) planned(ctx context.Context, byTests []string, feedback string) ([]unit, []artifact.Part, string) {
	spec := r.specText()
	if spec == "" {
		return nil, nil, "the specification could not be read"
	}
	repo := r.cfg.Estate.Repo(r.repo)
	var b strings.Builder
	b.WriteString("# The specification\n\n" + spec + "\n# What the units may touch\n\n")
	b.WriteString("New test files, for tests units: " + strings.Join(r.spec.NewTests, ", ") + "\n")
	b.WriteString("Files the change may touch, for code units: " + strings.Join(r.weighed(r.spec.Scope), ", ") + "\n")
	fmt.Fprintf(&b, "What one agent is handed: at most %d tokens — the specification's decisions weigh %d of them; the rest is its brief and its files.\n",
		r.cfg.packBudget(), estate.Tokens(specDecisions(r.dir, true)))
	b.WriteString("Criteria judged by the new tests: " + strings.Join(byTests, ", ") + "\n")
	if r.cfg.Whole {
		b.WriteString("\nThis change is carried whole: exactly one tests unit, writing every new test file and judging every criterion the new tests judge, and exactly one code unit, whose scope is every file the change may touch and which serves every criterion.\n")
	}
	b.WriteString("\n# The estate, as names — no code\n\n" + repo.Map(r.cfg.packBudget()/4))
	b.WriteString("\n# What exists where the change works — every definition by its first line, tests and their helpers among them. A unit relies on these, or on what an earlier unit provides, and on nothing else.\n\n" + r.signatures())
	prompt := b.String() + feedback
	var why string
	m := r.cfg.planner()
	// base is the last plan that read, which a mend is laid over.
	var base *planDraft
	mend := planMendWhole
	if freeForm(m) {
		mend = planMend
	}
	for attempt := 1; attempt <= 3; attempt++ {
		r.say(2, "", "asking %s to divide the work, attempt %d", m.Name(), attempt)
		began := time.Now()
		var p planDraft
		if base != nil {
			p = base.clone()
		}
		kept, handed := r.handed("planner", plannerSystem, prompt+why)
		// The criteria a unit serves are the specification's: chosen among
		// its ids, never written.
		_, u, turns, err := r.study(model.Within(ctx, "planner"), 2, m, plannerSystem, prompt+why, &p, map[string][]string{"units.criteria": r.criterionIDs()})
		r.spend(u)
		r.cost(artifact.Cost{Area: 2, What: fmt.Sprintf("the plan, attempt %d", attempt), Actor: m.Name(), Turns: turns,
			Handed: handed, Kept: kept}, u, time.Since(began))
		if errors.Is(err, model.ErrUnreadable) {
			why = "\n\nYour last answer was not one JSON object: " + err.Error()
			continue
		}
		if err != nil {
			return nil, nil, "the planner could not be asked: " + err.Error()
		}
		base = &p
		// A plan that must write outside the specification's scope says the
		// scope is wrong, not the plan: the planner read the code and found
		// what the change touches. It goes back to the specification's
		// author, with what and why, and to the person — never round the
		// planner, who may not widen what the person accepted.
		if outside := r.planOutsideScope(p); len(outside) > 0 && !r.back[1] {
			r.scopeFault = &sentBack{area: 1, unit: "the plan", why: "the planner, reading the code, found the change must touch " +
				strings.Join(outside, ", ") + ", which the specification's scope does not hold; the plan it made: " + p.Approach.Summary}
			return nil, nil, "the specification's scope is too narrow for the change"
		}
		units, problem := r.unitsOf(p)
		if problem == "" {
			problem = r.breaches(units)
		}
		if problem != "" {
			r.say(2, "", "the plan is sent back: %s", problem)
			// It is handed what it answered and exactly what breaks, to mend
			// that and keep the rest: asked afresh, it would plan afresh, and
			// break something else.
			why = "\n\n# Your last plan\n\n```json\n" + p.json() + "\n```\n\nIts units, by number:\n\n" + p.numbered() +
				"\nIt cannot be used: " + problem + ".\n\n" + mend
			continue
		}
		approach := []artifact.Part{{Title: "How", Text: p.Approach.Summary}, {Title: "In order", Points: p.Approach.Steps},
			{Title: "What could go wrong", Points: p.Approach.Risks}}
		return units, approach, m.Name()
	}
	return nil, nil, "the planner's answers could not be used (" + strings.TrimSpace(strings.TrimPrefix(why, "\n\nYour last plan cannot be used: ")) + ")"
}

// unitsOf reads a planner's answer into units, numbered, tests first, and
// holds it to what only the estate as it stands can decide: a file read
// exists or another unit writes it (P11), a name provided is not declared
// already (P7), a name used exists or an earlier unit provides it (P10),
// and a unit fits one agent (P13). What the plan and the specification
// decide alone is meta.PlanBreaches', at breaches.
func (r *Run) unitsOf(p planDraft) ([]unit, string) {
	var tests, code []unit
	for _, pu := range p.Units {
		u := unit{title: pu.Title, class: r.spec.Class, scope: pu.Scope, brief: pu.brief(), kind: meta.UnitKind(pu.Kind),
			uses: pu.Uses, reads: pu.Reads, criteria: pu.Criteria}
		if strings.TrimSpace(pu.Objective) == "" || len(pu.Tasks) == 0 {
			return nil, fmt.Sprintf("unit %q has no objective or no tasks", pu.Title)
		}
		// The plan is what the agents follow, and the planner reads the code
		// before it plans (harness.Study): where the estate shows the plan
		// wrong, it goes back to the planner, told where, to mend — never to
		// the agents with the fault in it. Only a unit's weight is a hint:
		// what does not fit is handed by its outline and read by parts.
		for _, f := range u.reads {
			if fileExists(filepath.Join(r.repo, filepath.FromSlash(f))) {
				continue
			}
			written := slices.ContainsFunc(p.Units, func(o ticket) bool {
				if u.kind == meta.TestsUnit {
					return slices.ContainsFunc(o.Provides, func(pr meta.Provision) bool { return pr.File == f })
				}
				return o.Title != pu.Title && slices.Contains(o.Scope, f)
			})
			if !written {
				return nil, fmt.Sprintf("unit %q reads %s, which neither exists nor is written before it", pu.Title, f)
			}
		}
		if why := r.oversized(u); why != "" {
			r.say(2, "", "a hint: %q %s — its agent is handed outlines where whole files do not fit", pu.Title, why)
		}
		switch u.kind {
		case meta.TestsUnit:
			tests = append(tests, u)
		case meta.CodeUnit:
			u.provides = pu.Provides
			code = append(code, u)
		default:
			return nil, fmt.Sprintf("unit %q is of kind %q, which is neither tests nor code", pu.Title, pu.Kind)
		}
	}
	if r.cfg.Whole && (len(tests) != 1 || len(code) != 1) {
		return nil, fmt.Sprintf("the change is carried whole: one tests unit and one code unit, not %d and %d", len(tests), len(code))
	}
	if problem := r.canonical(tests, code); problem != "" {
		return nil, problem
	}
	if problem := ambiguous(slices.Concat(tests, code)); problem != "" {
		return nil, problem
	}
	for _, u := range code {
		if why := r.providedAlready(u); why != "" {
			return nil, fmt.Sprintf("unit %q: %s", u.title, why)
		}
	}
	code, problem := orderByUse(code)
	if problem != "" {
		return nil, problem
	}
	if problem := r.orderOf(code); problem != "" {
		return nil, problem
	}
	return r.number(append(tests, code...)), ""
}

// qualifiers is what a name declared in file may be qualified by: its
// package — the folder's name and, in Go, the name its files declare — and,
// in a language whose modules are files, the file's own name.
func (r *Run) qualifiers(file string) []string {
	dir := path.Dir(file)
	q := []string{path.Base(dir), strings.TrimSuffix(path.Base(file), path.Ext(file))}
	if strings.HasSuffix(file, ".go") {
		if pkg, err := goPackageOf(filepath.Join(r.repo, filepath.FromSlash(dir))); err == nil {
			q = append(q, pkg)
		}
	}
	return q
}

// canonical writes every name of the plan as the package it is declared in
// writes it: a name the planner qualified by that package — model.Step,
// models.Config — is the package's own Step or Config. A field or method
// said to belong to a type of another package is what no unit can add.
func (r *Run) canonical(tests, code []unit) string {
	// What the units provide first: the names the others use are matched
	// against them as their packages write them.
	var provided []meta.Provision
	for i := range code {
		var kept []meta.Provision
		for _, p := range code[i].provides {
			if q, base, ok := cutLast(p.Name, "."); ok && slices.Contains(r.qualifiers(p.File), q) {
				p.Name = base
			}
			q, base, ok := cutLast(p.Of, ".")
			switch {
			case !ok:
			case slices.Contains(r.qualifiers(p.File), q):
				p.Of = base
			default:
				return fmt.Sprintf("unit %q gives %s to %s, a type of another package than %s's: a unit adds fields and methods only to the types of the package it writes in",
					code[i].title, p.Name, p.Of, p.File)
			}
			kept = append(kept, p)
		}
		code[i].provides = kept
		provided = append(provided, code[i].provides...)
	}
	repo := r.cfg.Estate.Repo(r.repo)
	declaredIn := func(base string) []string {
		var files []string
		for _, p := range provided {
			if p.Name == base {
				files = append(files, p.File)
			}
		}
		if places, err := repo.WhereDefined(base); err == nil {
			for _, pl := range places {
				files = append(files, pl.File)
			}
		}
		return files
	}
	for _, units := range [][]unit{tests, code} {
		for i := range units {
			for j, n := range units[i].uses {
				q, base, ok := cutLast(n, ".")
				if !ok {
					continue
				}
				for _, file := range declaredIn(base) {
					if slices.Contains(r.qualifiers(file), q) {
						units[i].uses[j] = base
						break
					}
				}
			}
		}
	}
	return ""
}

// orderByUse is the code units in an order in which every unit comes after
// those that provide what it uses: of the units free to go next, always the
// one the planner put first. The order is derived, not asked of the
// planner; a circle of uses is the one thing it cannot derive.
func orderByUse(code []unit) ([]unit, string) {
	needs := make([][]int, len(code))
	for i, u := range code {
		for _, name := range u.uses {
			for j, o := range code {
				if j != i && o.gives(name) && !slices.Contains(needs[i], j) {
					needs[i] = append(needs[i], j)
				}
			}
		}
	}
	placed := make([]bool, len(code))
	out := make([]unit, 0, len(code))
	for len(out) < len(code) {
		found := -1
		for i := range code {
			if placed[i] {
				continue
			}
			if !slices.ContainsFunc(needs[i], func(j int) bool { return !placed[j] }) {
				found = i
				break
			}
		}
		if found < 0 {
			// Each unit left waits for another left: say who for whom, by
			// the name it waits on.
			var waits []string
			for i, u := range code {
				if placed[i] {
					continue
				}
				for _, name := range u.uses {
					for j, o := range code {
						if j != i && !placed[j] && o.gives(name) {
							waits = append(waits, fmt.Sprintf("%q uses %s, which %q provides", u.title, name, o.title))
						}
					}
				}
			}
			return nil, "the units wait for one another in a circle — " + strings.Join(waits, "; ") + " — and none can go first: a unit uses only what units before it provide"
		}
		placed[found] = true
		out = append(out, code[found])
	}
	return out, ""
}

// cutLast is s cut around its last sep.
func cutLast(s, sep string) (before, after string, found bool) {
	if i := strings.LastIndex(s, sep); i >= 0 {
		return s[:i], s[i+len(sep):], true
	}
	return s, "", false
}

// handedWhole is the files of the estate a unit's agent must be handed
// whole: those it may change, for it edits what it sees. What it reads is
// handed whole where it fits beside them, by its outline where not, and
// does not decide whether a unit fits; the code tests exercise is handed
// them by its declarations.
func (r *Run) handedWhole(u unit) []string {
	files := slices.Clone(u.scope)
	slices.Sort(files)
	return slices.Compact(files)
}

// weight is what a file of the estate weighs as an agent reads it, in
// tokens; a file not yet written weighs nothing.
func (r *Run) weight(rel string) int {
	b, err := os.ReadFile(filepath.Join(r.repo, filepath.FromSlash(rel)))
	if err != nil {
		return 0
	}
	return estate.Tokens(string(b))
}

// weighed is files, each with its weight.
func (r *Run) weighed(files []string) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = fmt.Sprintf("%s (%d tokens)", f, r.weight(f))
	}
	return out
}

// oversized says why a unit does not fit what one agent is handed — its
// brief, the specification's decisions and its files whole — or nothing.
// A unit of one file that alone does not fit is let be: no plan can divide
// it, and its agent is handed its outline, to read by parts.
func (r *Run) oversized(u unit) string {
	files := r.handedWhole(u)
	total := estate.Tokens(u.brief) + estate.Tokens(specDecisions(r.dir, u.isTests()))
	for _, f := range files {
		total += r.weight(f)
	}
	if total <= r.cfg.packBudget() || len(files) <= 1 {
		return ""
	}
	return fmt.Sprintf("would be handed %d tokens — its brief, the specification's decisions and %s, the files it changes, whole — more than the %d one agent is handed; divide it, so that each unit changes fewer files",
		total, strings.Join(r.weighed(files), ", "), r.cfg.packBudget())
}

// orderOf says where a code unit uses a name it cannot build on: one a
// unit after it provides, or one no unit provides and the estate does not
// define.
func (r *Run) orderOf(code []unit) string {
	repo := r.cfg.Estate.Repo(r.repo)
	for _, u := range code {
		for _, name := range u.uses {
			// Code units are in the order of their uses already (orderByUse):
			// what is left to say is a name nothing provides.
			if slices.ContainsFunc(code, func(o unit) bool { return o.gives(name) }) {
				continue
			}
			if places, err := repo.WhereDefined(name); err == nil && len(places) == 0 {
				return fmt.Sprintf("unit %q uses %s, which no unit provides and the estate does not define — a unit provides it first, or the unit does without it", u.title, name)
			}
		}
	}
	return ""
}

// number gives units their ids and their order: every tests unit prepares
// the first code unit, and each code unit depends on the unit before it.
func (r *Run) number(units []unit) []unit {
	for i := range units {
		units[i].id = r.id("WU", i+1)
	}
	firstCode := slices.IndexFunc(units, func(u unit) bool { return !u.isTests() && !u.isContract() })
	var testIDs []string
	for i := range units {
		switch {
		case units[i].isContract():
		case units[i].isTests():
			if firstCode >= 0 { // a plan with no code is sent back (P1), not numbered wrong
				units[i].preparatoryFor = units[firstCode].id
			}
			testIDs = append(testIDs, units[i].id)
		}
	}
	return units
}

// withContract puts first, where the tests and the code are written apart
// and the code provides names, a unit that declares every one of them
// without behaviour, each in the file it is provided in. The tests are written against that interface
// and the code units fill it in, apart and at once where they can: none
// guesses what another will declare.
func (r *Run) withContract(units []unit) []unit {
	var code, tests []unit
	for _, u := range units {
		switch u.kind {
		case meta.ContractUnit:
			return units // already there
		case meta.TestsUnit:
			tests = append(tests, u)
		default:
			code = append(code, u)
		}
	}
	if len(code) == 0 || len(tests) == 0 {
		return units // nothing is written apart that must agree
	}
	var scope []string
	var provides []meta.Provision
	var b strings.Builder
	for _, u := range code {
		for _, p := range u.provides {
			if p.Kind == meta.Described {
				continue
			}
			if !slices.Contains(scope, p.File) {
				scope = append(scope, p.File)
			}
			provides = append(provides, p)
			b.WriteString("- " + p.Signature + " — in " + p.File + ", for " + u.title + "\n")
		}
	}
	if len(provides) == 0 {
		return units
	}
	// What the contract is judged by is what the estate's own suite judges:
	// that, declared, nothing else moves. The new tests judge behaviour,
	// which the contract has none of.
	byTests := map[string]bool{}
	for _, u := range tests {
		for _, c := range u.criteria {
			byTests[c] = true
		}
	}
	var criteria []string
	for _, u := range code {
		for _, c := range u.criteria {
			if !byTests[c] && !slices.Contains(criteria, c) {
				criteria = append(criteria, c)
			}
		}
	}
	if len(criteria) == 0 {
		criteria = code[len(code)-1].criteria
	}
	contract := unit{kind: meta.ContractUnit, criteria: criteria, class: r.spec.Class, scope: scope, provides: provides,
		title: "the contract — every name the units give each other, declared without behaviour",
		brief: "Declare exactly these names, each in the file given, as the estate's language declares them, and nothing else:\n\n" + b.String() +
			"\nNo behaviour: a function or method returns its zero values, or fails as not yet done, in the way the language does; a type has its fields; a constant or variable its declared type. " +
			"Keep what each file already holds working, and the estate building. The tests and the code units are written after you, against exactly what you declare.\n"}
	return r.number(append(append([]unit{contract}, tests...), code...))
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
		if u.isTests() {
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

// signatures is what exists where the change works, for the planner and its
// reviewer: the scope's directories and the new tests', every definition by
// its first line.
func (r *Run) signatures() string {
	paths := append(slices.Clone(r.spec.Scope), r.spec.NewTests...)
	return r.cfg.Estate.Repo(r.repo).Outlined(paths, r.cfg.packBudget()/2)
}

// provisionName is a provision's name as its package knows it: a field or
// method by its type.
func provisionName(p meta.Provision) string {
	if p.Of != "" {
		return bareType(p.Of) + "." + p.Name
	}
	return p.Name
}

// bareType is a type named by its package or as a pointer, as its own
// package names it.
func bareType(of string) string {
	of = strings.TrimLeft(strings.TrimSpace(of), "*")
	if _, base, ok := cutLast(of, "."); ok {
		return base
	}
	return of
}

// goTaken is the file of the Go package in dir that already holds the name
// p would declare — a top-level name, or a field or method of its type —
// or "" where none does.
func goTaken(dir string, p meta.Provision) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	of := bareType(p.Of)
	member := (p.Kind == meta.Method || p.Kind == meta.Field) && of != ""
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, n), nil, parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && !member && d.Name.Name == p.Name {
					return n
				}
				if d.Recv != nil && member && d.Name.Name == p.Name && len(d.Recv.List) == 1 && receiverType(d.Recv.List[0].Type) == of {
					return n
				}
			case *ast.GenDecl:
				for _, sp := range d.Specs {
					switch sp := sp.(type) {
					case *ast.TypeSpec:
						if !member && sp.Name.Name == p.Name {
							return n
						}
						if st, ok := sp.Type.(*ast.StructType); ok && member && sp.Name.Name == of {
							for _, fd := range st.Fields.List {
								for _, id := range fd.Names {
									if id.Name == p.Name {
										return n
									}
								}
							}
						}
					case *ast.ValueSpec:
						for _, id := range sp.Names {
							if !member && id.Name == p.Name {
								return n
							}
						}
					}
				}
			}
		}
	}
	return ""
}

// receiverType is the name of the type a method's receiver is of.
func receiverType(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return receiverType(t.X)
	case *ast.IndexExpr:
		return receiverType(t.X)
	case *ast.IndexListExpr:
		return receiverType(t.X)
	case *ast.Ident:
		return t.Name
	}
	return ""
}

// criterionIDs is the specification's criteria, by id.
func (r *Run) criterionIDs() []string {
	ids := make([]string, len(r.spec.Criteria))
	for i := range r.spec.Criteria {
		ids[i] = r.id("CR", i+1)
	}
	return ids
}

// study has m take a decision of area — the specification, the plan — and
// reads its answer into into. A model that holds a conversation studies
// the estate first, by its graph and then its files, as far as the
// decision needs (harness.Study); any other is asked once, with what it is
// handed. The answer, what it cost, and the turns it took.
func (r *Run) study(ctx context.Context, area int, m model.Model, system, prompt string, into any, among map[string][]string) (string, model.Usage, int, error) {
	c, ok := chatter(m)
	if !ok {
		answer, u, err := model.AskJSONOf(ctx, m, system, prompt, into, among)
		return answer, u, 1, err
	}
	repo := r.cfg.Estate.Repo(r.repo)
	study := harness.Study{Dir: r.repo, Graph: estateGraph{repo: repo, root: r.repo}, System: system, Prompt: prompt, BudgetUSD: r.studyBudget()}
	turns := 0
	answer, u, _, err := study.Run(ctx, c, func(e harness.Event) {
		switch e.Kind {
		case harness.Spent:
			turns++
		case harness.Called:
			r.say(area, "", "reading the estate: %s %s", e.Tool, e.Detail)
		}
	})
	if err != nil {
		return answer, u, turns, err
	}
	if err := model.JSON(answer, into); err != nil {
		return answer, u, turns, fmt.Errorf("%w: %v", model.ErrUnreadable, err)
	}
	return answer, u, turns, nil
}

// chatter is m as a model that holds a conversation, where it is one.
func chatter(m model.Model) (harness.Chatter, bool) {
	c, ok := m.(harness.Chatter)
	if t, taped := m.(interface{ Chats() bool }); taped && !t.Chats() {
		ok = false // a tape around a model that holds no conversation
	}
	return c, ok
}

// freeForm says m's answer is shaped by nothing but what it is asked: it is
// studied in a conversation, or asked with no schema. Only then can it be
// asked to mend by answering what it changes alone — a strict schema has
// every key written, and what it leaves would come back emptied.
func freeForm(m model.Model) bool {
	if _, ok := chatter(m); ok {
		return true
	}
	_, shaped := m.(model.Shaped)
	return !shaped
}

// planOutsideScope is the files the planner's code units would write that
// lie outside the specification's scope and judge nothing: what the change
// must touch, by the planner's reading of the code, and was not given.
func (r *Run) planOutsideScope(p planDraft) []string {
	var out []string
	for _, pu := range p.Units {
		if meta.UnitKind(pu.Kind) != meta.CodeUnit {
			continue
		}
		for _, f := range pu.Scope {
			f = path.Clean(strings.TrimPrefix(filepath.ToSlash(f), "./"))
			if inScope(f, r.spec.Scope) || slices.Contains(r.spec.NewTests, f) || slices.Contains(r.spec.SuiteTests, f) || r.declaresTests(f) || slices.Contains(out, f) {
				continue
			}
			out = append(out, f)
		}
	}
	return out
}

// studyFraction is how much of a change's budget one decision may spend
// reading the code before it answers: deciding is a small part of doing.
const studyFraction = 0.15

// studyBudget is what one decision — a specification, a plan — may spend on
// reading, of what is left of the change's budget.
func (r *Run) studyBudget() float64 {
	if r.cfg.BudgetUSD <= 0 {
		return 0
	}
	return min(r.left(), r.cfg.BudgetUSD*studyFraction)
}
