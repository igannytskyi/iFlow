package flow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/estate"
	"github.com/igannytskyi/iflow/internal/gate"
	"github.com/igannytskyi/iflow/internal/harness"
	"github.com/igannytskyi/iflow/internal/meta"
	"github.com/igannytskyi/iflow/internal/model"
)

// draft is what area 1 asks a model to write, before it is checked.
// draftCriterion is one criterion as the author states it.
type draftCriterion struct {
	Criterion string `json:"criterion"`
	When      string `json:"when"`
	Then      string `json:"then"`
	Plan      string `json:"plan"`
	Threshold string `json:"threshold"`
	New       bool   `json:"judged_by_new_tests"`
	// Procedure is who decides it: "machine", by tests, or "human", the
	// person reading the finished change.
	Procedure string `json:"procedure"`
}

type draft struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Statement string `json:"statement"`
	Class     string `json:"class"`
	Prose     string `json:"prose"`
	// Document is the change described for a reader, as a product and a
	// design document would describe it.
	Document document `json:"document"`
	// Later are the parts of a task too large for one change: each a task
	// of its own, to be specified when this one has landed.
	Later       []string         `json:"later"`
	Criteria    []draftCriterion `json:"criteria"`
	Scope       []string         `json:"scope"`
	NewTests    []string         `json:"new_tests"`
	SuiteTests  []string         `json:"existing_tests"`
	Termination list[struct {
		Condition string `json:"condition"`
		Action    string `json:"action"`
	}] `json:"termination"`
}

const intentSystem = `You state a change to software so that it can be carried out and judged without its author. You answer with one JSON object and nothing else.

The specification you write is the whole specification of the change, as the best engineering organisations write one — a product requirements document, a design document and a request for comments together. A person reads it before accepting it; agents who never talk to that person build from it, each seeing only its part. Leave nothing to be guessed. In "document":
- "summary": one paragraph — what changes, for whom, and why, readable alone.
- "context": the problem and its motivation, and what exists now in the estate.
- "goals", "non_goals": points — what the change achieves; what could reasonably have been a goal and deliberately is not.
- "users": points — who meets the change, and in what role.
- "user_stories": points — "As a <user>, I want <action>, so that <outcome>".
- "flows": points — how a person uses it, step by step, for every path: the ordinary one and each alternative.
- "requirements": points — the functional requirements, one testable statement each; numbered FR-1, FR-2… in order.
- "non_functional": points — security, privacy, error handling, data integrity, limits, performance, accessibility, as far as they matter here.
- "architecture": points — the parts and how they talk to each other and to what exists.
- "data_model": points — what is kept, its fields and types, where and for how long.
- "interfaces": points — every route, command or function another part calls: inputs, outputs, status codes and errors, with signatures in the estate's language.
- "components": points — which file holds what.
- "technology": points — languages, libraries, protocols and services, and why, as far as the estate does not already decide them; never one the estate does not have without saying so.
- "edge_cases": points — wrong input, repetition, expiry, absence, refusal, concurrency, failure of what it depends on — each with what happens.
- "alternatives": points — designs weighed and why not chosen.
- "dependencies", "assumptions": points — what it relies on, inside the estate or out; what is taken as true without proof.
- "risks": points — what could go wrong or be misread, and how it is contained.
- "observability": points — what is logged or measured to see it work in use.
- "rollout": points — how it is shipped, migrated if anything is, and undone.
- "test_strategy": points — which tests judge which criteria, at which entry point, with which fakes.
- "success_metrics": points — how, once in use, it is known to have achieved its goals.
- "open_questions": points — only what the person who asked must decide; never guess at these, and never leave the design vague instead of asking.
"prose" is one sentence summing it up.

Rules you must follow:
- A criterion is in the vocabulary of the intent, never of the implementation: "when these conditions hold, then this observable fact holds". It never names a file, function or line.
- Each criterion is decided the cheapest way that truly decides it, and judges the behaviour where a person meets it — through the estate's own entry point (its HTTP server, its command, its public API) — so that what is built is reachable, not only a part working alone. Where tests through an entry point the estate already has can observe it, "procedure": "machine". Where what must hold is how something looks or reads to a person — a colour, a layout, a wording — and no test the estate has, or that one unit could write against what exists, observes it, "procedure": "human": the person affirms it on the finished change, and no test is written for it. Never ask for test machinery the estate does not have. Mark "plan": "required" for a criterion about new or changed behaviour, "not-required" for "nothing else moves".
- Always include a last criterion that nothing else in the estate moves: the whole existing suite stands as it did. It is always "procedure": "machine", decided by the existing tests.
- "class" is one of C1P (behaviour-preserving, proved), C1T (behaviour-preserving, tested), C2 (defect repair), C3 (a stated contract), C4 (observed effect), C5 (judgement).
- "scope" lists the files the change may create or edit, by path. It never includes tests, nor the scripts that run them. Keep it to what the task needs.
- "new_tests" lists test files that will be written first to judge the change — new files, never in scope, placed where the language expects tests for the scope (in Go, beside the code in the same package directory). Criteria they judge have "judged_by_new_tests": true.
- Keep the change to what a few agents, each with one unit, can carry out. If the task is larger, specify its first coherent part — one that is worth landing alone — and list in "later" the rest, each written as a task of its own, in the order they should come.
- "existing_tests" lists the estate's existing test files that judge that nothing else moves.
- "slug" is a few lowercase words joined by hyphens.

The JSON object has exactly these keys: slug, name, statement, class, prose, document (the keys above), later, criteria (criterion, when, then, plan, threshold, judged_by_new_tests, procedure), scope, new_tests, existing_tests, termination (condition, action: stop or escalate).`

// overview is what a model is shown of the estate to state a change: every
// source file and the names it defines, where the words of the task land,
// and how the tests are run — never the code itself, which the agents that
// change it are handed, each only its part.
func (r *Run) overview() string {
	repo := r.cfg.Estate.Repo(r.repo)
	var b strings.Builder
	b.WriteString("The files of the estate, and what each defines:\n\n" + repo.Map(r.cfg.packBudget()/2) + "\n")
	if found, _, err := repo.Survey(r.statement, 4); err == nil && len(found) > 0 {
		b.WriteString("Where the words of the task land:\n\n")
		for _, l := range found {
			var at []string
			for _, p := range l.Places {
				at = append(at, fmt.Sprintf("%s:%d", p.File, p.Start))
			}
			fmt.Fprintf(&b, "- %q %s %s — %s\n", l.Word, l.How, strings.Join(l.Names, ", "), strings.Join(at, ", "))
		}
		b.WriteString("\n")
	}
	if p, ok := detect(r.repo); ok {
		b.WriteString("Its tests are run with " + shown(p.suite) + ".\n")
	}
	if d := decisions(r.repo, r.cfg.packBudget()/8); d != "" {
		b.WriteString("\nWhat the person has already decided — take it as settled, and do not ask it again:\n\n" + d + "\n")
	}
	return b.String()
}

// document is the prose of a specification: what the change is for, how a
// person uses it, what it must do, how it is built, and what is still open.
type document struct {
	Summary       string   `json:"summary"`
	Context       string   `json:"context"`
	Goals         []string `json:"goals"`
	NonGoals      []string `json:"non_goals"`
	Users         []string `json:"users"`
	Stories       []string `json:"user_stories"`
	Flows         []string `json:"flows"`
	Requirements  []string `json:"requirements"`
	Quality       []string `json:"non_functional"`
	Architecture  []string `json:"architecture"`
	DataModel     []string `json:"data_model"`
	Interfaces    []string `json:"interfaces"`
	Components    []string `json:"components"`
	Technology    []string `json:"technology"`
	EdgeCases     []string `json:"edge_cases"`
	Alternatives  []string `json:"alternatives"`
	Dependencies  []string `json:"dependencies"`
	Assumptions   []string `json:"assumptions"`
	Risks         []string `json:"risks"`
	Observability []string `json:"observability"`
	Rollout       []string `json:"rollout"`
	Testing       []string `json:"test_strategy"`
	Metrics       []string `json:"success_metrics"`
	Questions     []string `json:"open_questions"`
}

// requirementText is a requirement without the number an author wrote
// before it, and the colon or dash after that number: the number is
// written once, by the document.
func requirementText(q string, n int) string {
	q = strings.TrimSpace(q)
	q = strings.TrimPrefix(q, fmt.Sprintf("FR-%d", n))
	return strings.TrimSpace(strings.TrimLeft(q, " :.—–-"))
}

// parts is the specification in the order a reader needs it, as product
// requirements, design document and request for comments write it: why,
// for whom, what it must do, how it is built, what else was weighed, how it
// is shipped and watched, how it is judged — its functional requirements
// numbered so that units, reviews and criteria can cite them.
func (d document) parts() []artifact.Part {
	var reqs []string
	for i, q := range d.Requirements {
		reqs = append(reqs, fmt.Sprintf("FR-%d %s", i+1, requirementText(q, i+1)))
	}
	return []artifact.Part{
		{Title: "Summary", Text: d.Summary},
		{Title: "Context and problem", Text: d.Context},
		{Title: "Goals", Points: d.Goals},
		{Title: "Non-goals", Points: d.NonGoals},
		{Title: "Users", Points: d.Users},
		{Title: "User stories", Points: d.Stories},
		{Title: "How a person uses it", Points: d.Flows},
		{Title: "Functional requirements", Points: reqs},
		{Title: "Non-functional requirements", Points: d.Quality},
		{Title: "Design", Level: 2, Group: true},
		{Title: "Architecture", Points: d.Architecture, Level: 3},
		{Title: "Data model", Points: d.DataModel, Level: 3},
		{Title: "Interfaces", Points: d.Interfaces, Level: 3},
		{Title: "Components and files", Points: d.Components, Level: 3},
		{Title: "Technology", Points: d.Technology, Level: 3},
		{Title: "Edge cases and errors", Points: d.EdgeCases},
		{Title: "Alternatives considered", Points: d.Alternatives},
		{Title: "Dependencies", Points: d.Dependencies},
		{Title: "Assumptions", Points: d.Assumptions},
		{Title: "Risks", Points: d.Risks},
		{Title: "Observability", Points: d.Observability},
		{Title: "Rollout and rollback", Points: d.Rollout},
		{Title: "Test strategy", Points: d.Testing},
		{Title: "Success metrics", Points: d.Metrics},
		{Title: "Open questions", Points: d.Questions},
	}
}

var slugRE = regexp.MustCompile(`[^a-z0-9-]+`)

// intent is area 1: from a sentence to an intent and a specification a
// person accepts, checked by the gate before the person is asked.
func (r *Run) intent(ctx context.Context) error {
	r.set(1, Working)
	r.mu.Lock()
	if r.n == 0 {
		r.n = nextNumber(r.repo)
	}
	r.mu.Unlock()
	prompt := "The task, as a person wrote it:\n\n" + r.statement + "\n\n" + r.overview()
	feedback := r.reopened
	for sent := 0; ; sent++ {
		d := r.spec
		if sent > 0 || !r.resumed {
			var err error
			if d, err = r.specified(ctx, prompt, feedback); err != nil {
				return err
			}
		}
		r.spec = d
		r.say(1, r.id("SPEC"), "specification written: %d criteria, class %s, scope %s", len(d.Criteria), d.Class, strings.Join(d.Scope, ", "))
		got, err := r.ask(ctx, 1, "Is this the change you want? Read the specification and its criteria; accepting them fixes them, and from here nothing moves them.")
		if err != nil {
			return err
		}
		if got.ok {
			r.say(1, r.id("SPEC"), "%s accepted the criteria", r.cfg.Person)
			r.set(1, Done)
			return nil
		}
		// Sent back, the specification is its author's failure, recorded as
		// one; a new author writes it again from the person's notes, and it
		// is reviewed again before the person is asked again.
		_ = r.record("specification", r.id("SPEC"), "sent back by "+r.cfg.Person+": "+got.ground)
		_ = decide(r.repo, r.slug, got.ground)
		r.say(1, r.id("SPEC"), "%s sent the specification back: %s", r.cfg.Person, got.ground)
		if sent+1 >= personRounds {
			return r.fail(1, "the specification was sent back %d times; the last time: %s", personRounds, got.ground)
		}
		r.set(1, Working)
		spec := r.specText()
		feedback = "\n\n# Your previous specification\n\n" + spec +
			"\n# The person who asked for the change sent it back, saying\n\n" + got.ground +
			"\n\nWrite the specification again, whole, as the person says. Where they answered an open question, the answer is decided: take it out of the open questions and build on it."
	}
}

// personRounds is how many times a person may send a specification back
// before the task stops.
const personRounds = 5

// specified is a specification an author wrote and reviewers read: drafts
// until one is readable and passes the gate, then reviews, each sending it
// back with what is missing, reviewRounds times at most.
func (r *Run) specified(ctx context.Context, prompt, feedback string) (draft, error) {
	var d draft
	var last review
	for round := 1; ; round++ {
		var err error
		if d, err = r.drafted(ctx, prompt, feedback); err != nil {
			return d, err
		}
		if round > reviewRounds {
			break
		}
		spec := r.specText()
		v, err := r.reviewed(ctx, 1, "specification", specReviewSystem,
			"# The specification\n\n"+spec+"\n# The estate, as names — no code\n\n"+r.cfg.Estate.Repo(r.repo).Map(r.cfg.packBudget()/4), round)
		if err != nil {
			r.say(1, "", "the review could not be had: %v", err)
			break
		}
		last = v
		if v.sufficient() {
			break
		}
		feedback = "\n\n# Your previous specification\n\n" + spec +
			"\n# What a reviewer found in it\n\n" + v.forAuthor() +
			"\nWrite the specification again, whole, resolving every blocking and major finding. Questions for the person stay open questions; do not answer them yourself."
	}
	r.mu.Lock()
	r.reviewLeft = last
	r.mu.Unlock()
	return d, nil
}

// drafted asks the author for a specification until one is readable and
// passes the gate, three times at most.
func (r *Run) drafted(ctx context.Context, prompt, feedback string) (draft, error) {
	r.say(1, "", "asking %s to state the change and its criteria", r.cfg.Model.Name())
	note := ""
	for attempt := 1; attempt <= 3; attempt++ {
		var d draft
		began := time.Now()
		kept, handed := r.handed("author", intentSystem, prompt+feedback+note)
		answer, u, err := model.AskJSON(ctx, r.cfg.Model, intentSystem, prompt+feedback+note, &d)
		r.spend(u)
		r.cost(artifact.Cost{Area: 1, What: fmt.Sprintf("the specification, attempt %d", attempt),
			Actor: r.cfg.Model.Name(), Turns: 1, Handed: handed, Kept: kept}, u, time.Since(began))
		if errors.Is(err, model.ErrUnreadable) {
			note = "\n\nYour last answer was not one JSON object: " + err.Error()
			r.say(1, "", "the draft is sent back: it is not one JSON object (%v); it began %q and ended %q", err, head(answer, 80), tail(answer, 80))
			continue
		}
		if err != nil {
			return d, r.fail(1, "the model could not be asked: %v", err)
		}
		if why := d.problem(); why != "" {
			note = "\n\nYour last answer cannot be used: " + why + ". Answer again."
			r.say(1, "", "the draft is sent back: %s", why)
			continue
		}
		if err := r.writeIntent(d); err != nil {
			return d, r.fail(1, "the specification could not be written: %v", err)
		}
		rep, err := r.check(ctx, 1)
		if err != nil {
			return d, r.fail(1, "the gate could not be asked: %v", err)
		}
		if rep.Passed() {
			return d, nil
		}
		var found, fixes []string
		for _, f := range rep.Problems() {
			found = append(found, f.String())
			if fix, ok := remedies[f.Rule]; ok && !slices.Contains(fixes, fix) {
				fixes = append(fixes, fix)
			}
		}
		note = "\n\nThe gate refused your last specification: " + strings.Join(found, "; ") + "."
		if len(fixes) > 0 {
			note += "\n\nHow to mend it:\n- " + strings.Join(fixes, "\n- ")
		}
		note += "\n\nAnswer again, with that mended and the rest as it was."
		r.say(1, "", "the gate refused the draft: %s", strings.Join(found, "; "))
	}
	return draft{}, r.fail(1, "after three drafts there is still no specification that is readable and passes the gate")
}

// remedies say, for each rule a specification can break, how its author
// mends it: the rule's words alone say what is wrong, not what to write.
var remedies = map[string]string{
	"R19":         "a criterion never names a file the change may touch: say what is observed — \"the whole existing suite passes\", \"the greeting reads Hola, Ada!\" — not which file runs or holds it; and keep tests, and the scripts that run them, out of the scope",
	"R7":          "the scope and the arbiter never share a file: a test, or a script that runs the tests, is never in the scope",
	"R11":         "every criterion is judged by something: the new tests, the existing tests, or the person (procedure human)",
	"CONVENTIONS": "use only the defined values: plan is required or not-required; procedure is machine or human; class is C1P, C1T, C2, C3, C4 or C5",
}

// place names the change's folder: the slug the model gave, made safe, or
// change-<n> where nothing of it survives, never a folder that is taken.
func (r *Run) place(given string) (slug, dir string) {
	slug = strings.Trim(slugRE.ReplaceAllString(strings.ToLower(given), "-"), "-")
	if slug == "" {
		slug = fmt.Sprintf("change-%03d", r.n)
	}
	base := slug
	for i := 2; ; i++ {
		dir = filepath.Join(r.repo, "changes", slug)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return slug, dir
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

func (d draft) problem() string {
	switch {
	case d.Slug == "" || d.Statement == "":
		return "it has no slug or no statement"
	case len(d.Criteria) == 0:
		return "it has no criteria"
	case len(d.Scope) == 0:
		return "it names no scope"
	case d.Document.Summary == "" || len(d.Document.Flows) == 0 || len(d.Document.Requirements) == 0 || len(d.Document.Interfaces) == 0 || len(d.Document.Components) == 0:
		return "it does not specify the change: summary, flows, requirements, interfaces or components are empty"
	case !meta.Allowed("Class", d.Class):
		return "class " + d.Class + " is not one of C1P, C1T, C2, C3, C4, C5"
	}
	for _, t := range d.NewTests {
		if slices.Contains(d.Scope, t) {
			return t + " is both in scope and a test that judges it"
		}
	}
	return ""
}

func (r *Run) writeIntent(d draft) error {
	r.mu.Lock()
	if r.dir == "" {
		r.slug, r.dir = r.place(d.Slug)
	}
	dir := r.dir
	r.mu.Unlock()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := shelter(filepath.Join(r.repo, "changes")); err != nil {
		return err
	}
	_ = os.WriteFile(filepath.Join(dir, RunningFile), []byte(fmt.Sprint(os.Getpid())), 0o644)
	r.keepCosts()
	first := !fileExists(filepath.Join(dir, "00-intent.md"))
	err := artifact.Save(dir, artifact.Intent, meta.Intent{Schema: meta.SchemaVersion,
		Name: r.slug, ID: r.id("INT"), StatedBy: r.cfg.Person, Authority: r.cfg.Person,
		Priority: "normal", Deadline: "none", Statement: d.Statement, Status: meta.Open})
	if err != nil {
		return err
	}
	spec := meta.Specification{Schema: meta.SchemaVersion, Name: r.slug, ID: r.id("SPEC"), Intent: r.id("INT"), Class: meta.Class(d.Class), Prose: d.Prose,
		Scope:    meta.Scope{Included: d.Scope, Excluded: append(slices.Clone(d.NewTests), d.SuiteTests...)},
		NewTests: d.NewTests, SuiteTests: d.SuiteTests,
	}
	var byNew, bySuite, byPerson []string
	for i, c := range d.Criteria {
		id := r.id("CR", i+1)
		plan := meta.NotRequired
		if c.Plan == "required" {
			plan = meta.Required
		}
		proc, evidence := meta.Machine, meta.TestRun
		if c.Procedure == "human" {
			proc, evidence = meta.Human, meta.HumanAffirmation
		}
		spec.Criteria = append(spec.Criteria, meta.Criterion{ID: id, Criterion: c.Criterion, When: c.When,
			Then: c.Then, Plan: plan, Procedure: proc, Evidence: evidence, Threshold: c.Threshold, New: c.New && proc == meta.Machine})
		switch {
		case proc == meta.Human:
			byPerson = append(byPerson, id)
		case c.New && len(d.NewTests) > 0:
			byNew = append(byNew, id)
		default:
			bySuite = append(bySuite, id)
		}
	}
	for _, t := range d.Termination {
		action := t.Action
		if action != "escalate" {
			action = "stop"
		}
		spec.Termination = append(spec.Termination, meta.Termination{Condition: t.Condition, Action: action})
	}
	for _, t := range d.NewTests {
		spec.Arbiters = append(spec.Arbiters, meta.Arbiter{Path: t, Arbitrates: joinIDs(byNew)})
	}
	suite := d.SuiteTests
	if len(suite) == 0 {
		if p, ok := detect(r.repo); ok {
			suite = []string{p.name}
		}
	}
	for _, t := range suite {
		spec.Arbiters = append(spec.Arbiters, meta.Arbiter{Path: t, Arbitrates: joinIDs(bySuite)})
	}
	if len(byPerson) > 0 {
		// What only a person can see is judged by the person, reading the
		// finished change before it lands.
		spec.Arbiters = append(spec.Arbiters, meta.Arbiter{Path: "the person: " + r.cfg.Person, Arbitrates: joinIDs(byPerson)})
	}
	for _, s := range d.Scope {
		spec.Inputs = append(spec.Inputs, meta.ArbiterInput{Input: s, InScope: "yes — what the tests judge is what changes"})
	}
	if err := artifact.Save(dir, artifact.Specification, spec); err != nil {
		return err
	}
	// The change described is a document of its own, beside the method's
	// record of it: what is wanted and how, for a person to read and the
	// agents to build from; the specification's tables are what judge it.
	var crit []string
	for i, c := range d.Criteria {
		crit = append(crit, fmt.Sprintf("%s %s — when %s, then %s", r.id("CR", i+1), c.Criterion, c.When, c.Then))
	}
	parts := append(d.Document.parts(), artifact.Part{Title: "Acceptance criteria", Points: crit},
		artifact.Part{Title: "Termination", Points: terms(d)},
		artifact.Part{Title: "What comes after", Points: d.Later})
	if err := os.WriteFile(filepath.Join(dir, artifact.ChangeFile), []byte(artifact.ChangeDoc{Name: r.slug, Statement: d.Statement, Summary: d.Prose, Parts: parts}.Markdown()), 0o644); err != nil {
		return err
	}
	if first {
		if err := r.record("intent", r.id("INT"), r.statement); err != nil {
			return err
		}
		return r.record("specification", r.id("SPEC"), fmt.Sprintf("class %s; %d criteria", d.Class, len(d.Criteria)))
	}
	return r.record("specification", r.id("SPEC"), "written again before admission, after the gate refused it")
}

// admit is area 3: conflict, allowance and permission settled together, the
// evidence plan fixed from the area of effect, the prior state captured.
func (r *Run) admit(ctx context.Context) error {
	r.set(3, Working)
	// The specification as admitted is its JSON: R1 compares against this
	// digest ever after.
	digest, err := artifact.DigestFile(artifact.JSONFile(r.dir, artifact.Specification))
	if err != nil {
		return r.fail(3, "the specification could not be read: %v", err)
	}
	doc := meta.Admission{Schema: meta.SchemaVersion, Name: r.slug, Digest: digest,
		Reviews: []meta.Review{{Reviewed: "yes", By: r.cfg.Person, At: today(), Note: "accepted in the workbench before any work"}}}
	for i, u := range r.units {
		cond := "permission"
		if i > 0 {
			cond = "conflict"
		}
		doc.Decisions = append(doc.Decisions, meta.Decision{Unit: u.id, Outcome: meta.Admitted, Condition: cond, Confidence: "medium"})
	}
	if len(r.units) > 1 {
		var ids []string
		for _, u := range r.units {
			ids = append(ids, u.id)
		}
		doc.Conflicts = append(doc.Conflicts, meta.Conflict{Units: joinIDs(ids), Region: "none", Invalidates: "none", Resolution: "ordering"})
	}
	// Other changes in flight in the same estate whose scope meets this
	// one's: whichever lands second finds the ground moved under it, and the
	// landing says so rather than undoing the first.
	for _, other := range r.inFlight() {
		doc.Conflicts = append(doc.Conflicts, meta.Conflict{Units: r.id("SPEC") + ", " + other.name, Region: strings.Join(other.shared, ", "),
			Invalidates: "the evidence of whichever lands second", Resolution: "ordering: the second landing merges onto the ground as it is then, and stops if their edits overlap"})
		r.say(3, "", "in flight beside it: %s, which may also change %s; whichever lands second checks the ground first", other.name, strings.Join(other.shared, ", "))
	}
	observed, err := r.observed()
	if err != nil {
		return r.fail(3, "the estate could not be asked: %v", err)
	}
	for i, c := range r.spec.Criteria {
		if c.Plan == "required" {
			doc.Plans = append(doc.Plans, meta.EvidencePlan{Criterion: r.id("CR", i+1), Observed: observed,
				Method: "the tests written first, run once before the change and once after", Unchanged: "the behaviour stands as it does now",
				DerivedFrom: "the area of effect of " + strings.Join(r.spec.Scope, ", ") + ", derived, medium", FixedAt: today()})
		}
	}
	base := filepath.Join(r.dir, "baseline")
	if err := os.MkdirAll(base, 0o755); err != nil {
		return r.fail(3, "the prior state could not be captured: %v", err)
	}
	for _, s := range r.spec.Scope {
		src, err := os.ReadFile(filepath.Join(r.repo, s))
		to, what := "baseline/"+path.Base(s)+".before", s+" as it stood"
		if err != nil {
			// A file the change creates has a prior state too: it was not
			// there. Saying so is what keeps the comparison from being made
			// against something the change itself wrote.
			src, to, what = nil, "baseline/"+path.Base(s)+".absent", s+": did not exist before this change"
		}
		if err := os.WriteFile(filepath.Join(r.dir, to), src, 0o644); err != nil {
			return r.fail(3, "the prior state could not be captured: %v", err)
		}
		doc.Prior = append(doc.Prior, meta.Capture{Criterion: r.id("CR", 1), Artefact: what, CapturedTo: to})
	}
	if err := artifact.Save(r.dir, artifact.Admission, doc); err != nil {
		return r.fail(3, "the admission could not be written: %v", err)
	}
	for _, u := range r.units {
		if err := r.record("admission", u.id, "admitted"); err != nil {
			return err
		}
		r.say(3, u.id, "%s admitted", u.id)
	}
	if rep, err := r.check(ctx, 3); err != nil || !rep.Passed() {
		return r.fail(3, "the admission does not pass the gate")
	}
	r.set(3, Done)
	return nil
}

// observed is what an evidence plan observes: the scope, and every region
// the change reaches firmly that nothing already watches.
func (r *Run) observed() (string, error) {
	repo := r.cfg.Estate.Repo(r.repo)
	reach, err := repo.Affects(r.spec.Scope)
	if err != nil {
		return "", err
	}
	regions, err := repo.EveryRegion()
	if err != nil {
		return "", err
	}
	watched := map[string]bool{}
	for _, g := range regions {
		if g.Verdict != "unclaimed" || g.TestGround {
			watched[g.Region] = true
		}
	}
	parts := slices.Clone(r.spec.Scope)
	for _, x := range reach {
		if dir := path.Dir(x.File); x.Confidence != "low" && !watched[dir] && dir != "." && !slices.Contains(parts, dir) {
			parts = append(parts, dir)
		}
	}
	return strings.Join(parts, ", ") + " — the behaviour the criterion names", nil
}

// execute is area 4: each unit carried out by the executor in a copy of the
// estate holding the candidates before it, its changes kept as a candidate.
func (r *Run) execute(ctx context.Context) error {
	r.set(4, Working)
	prof, ok := detect(r.repo)
	doc := meta.Execution{Schema: meta.SchemaVersion, Name: r.slug}
	// The tests units at the head of the plan rely on nothing any unit
	// writes, so they are carried out at once, each by its own agent in its
	// own copy of the estate; the code units after them each rely on the one
	// before, and go one at a time.
	head := 0
	for head < len(r.units) && r.units[head].tests {
		head++
	}
	results := make([]carried, len(r.units))
	errs := make([]error, len(r.units))
	var wg sync.WaitGroup
	for i := range head {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = r.carry(ctx, i, nil, prof, ok)
		}()
	}
	wg.Wait()
	var overlays []string
	keep := func(i int) {
		c, u := results[i], r.units[i]
		run := meta.Run{Unit: u.id, Executor: r.cfg.Executor.Name(), Version: r.cfg.Executor.Version(), Terminal: meta.Terminal(c.res.Terminal), Candidate: c.ca}
		cand := meta.Candidate{ID: c.ca, Unit: u.id, StoredAt: "candidates/" + c.ca + "/", Artefacts: strings.Join(c.paths, ", "), ProducedAt: today()}
		if j := slices.IndexFunc(doc.Runs, func(x meta.Run) bool { return x.Unit == u.id }); j >= 0 {
			doc.Runs[j], doc.Candidates[j] = run, cand // carried again: the last carrying stands
			return
		}
		doc.Runs = append(doc.Runs, run)
		doc.Candidates = append(doc.Candidates, cand)
		overlays = append(overlays, filepath.Join(c.store, "files"))
	}
	carriedAgain := map[int]bool{}
	for i := 0; i < len(r.units); i++ {
		if i >= head {
			results[i], errs[i] = r.carry(ctx, i, overlays, prof, ok)
			// Code that cannot pass because the tests written first are
			// wrong — they call what no unit can define, or check what the
			// specification does not say — sends those tests back, once,
			// with what it found, and is carried again on what comes back.
			if j := r.blamed(results[i], head, results); errs[i] != nil && j >= 0 && !carriedAgain[j] {
				carriedAgain[j] = true
				r.say(4, r.units[j].id, "%s is carried again: %s found its tests wrong", r.units[j].id, r.units[i].id)
				if r.again == nil {
					r.again = map[string]string{}
				}
				r.again[r.units[j].id] = results[i].res.Summary
				_ = os.RemoveAll(results[j].store)
				results[j], errs[j] = r.carry(ctx, j, nil, prof, ok)
				if errs[j] != nil {
					return r.fail(4, "%v", errs[j])
				}
				keep(j)
				i--
				continue
			}
		}
		var h *holds
		if errors.As(errs[i], &h) {
			return h // what was asked holds already: there is nothing to change
		}
		if errs[i] != nil {
			if why, ok := strings.CutPrefix(results[i].res.Summary, harness.BlockedPrefix); ok && !r.replanned {
				// Its agent says the unit cannot be done as it was planned:
				// the plan goes back to its planner with why, once.
				r.say(4, r.units[i].id, "%s cannot be done as it was planned: %s", r.units[i].id, why)
				return &replan{unit: r.units[i].id, why: why}
			}
			return r.fail(4, "%v", errs[i])
		}
		c := results[i]
		keep(i)
		if err := artifact.Save(r.dir, artifact.Execution, doc); err != nil {
			return r.fail(4, "the execution could not be written: %v", err)
		}
		if err := r.record("execution", c.ca, c.res.Summary); err != nil {
			return err
		}
	}
	if rep, err := r.check(ctx, 4); err != nil || !rep.Passed() {
		return r.fail(4, "the execution does not pass the gate")
	}
	r.set(4, Done)
	return nil
}

// createdBy reports whether an import path names a directory a code unit
// of the plan writes into.
func createdBy(path string, created map[string]bool) bool {
	for dir := range created {
		if dir != "." && (path == dir || strings.HasSuffix(path, "/"+dir)) {
			return true
		}
	}
	return false
}

// arbitrate runs the framework's own tests against it as it is now, where
// the change landed on the framework itself and nothing has run them since
// it last changed: R22 holds such a landing until they pass, and running
// them is the framework's work, not the person's.
func (r *Run) arbitrate(ctx context.Context) {
	home := gate.Home()
	if home == "" || filepath.Clean(home) != filepath.Clean(r.repo) {
		return
	}
	if _, ok := gate.Verified(home)[gate.ArbitersKey(home)]; ok {
		return
	}
	r.say(6, r.id("SPEC"), "the framework changed, so its own tests are run against it as it is now (R22)")
	began := time.Now()
	run, err := gate.Arbiters(ctx, home)
	r.cost(artifact.Cost{Area: 6, What: "the framework's own tests, run against it as it is now", Actor: "go test"}, model.Usage{}, time.Since(began))
	switch {
	case err != nil:
		r.say(6, r.id("SPEC"), "the framework's tests could not be run: %v", err)
	case !run.Passed:
		r.say(6, r.id("SPEC"), "the framework's own tests fail against it as it is now")
	}
}

// codeScope is every file the code units of the plan may change, in plan
// order, each once.
func (r *Run) codeScope() []string {
	var out []string
	for _, u := range r.units {
		if u.tests {
			continue
		}
		for _, p := range u.scope {
			if !slices.Contains(out, p) {
				out = append(out, p)
			}
		}
	}
	return out
}

// holds is tests written first that pass on the estate as it stands: what
// they judge is true already.
type holds struct {
	unit  string
	tests []string
}

func (h *holds) Error() string {
	return h.unit + ": its tests " + strings.Join(h.tests, ", ") + " pass on the estate as it stands"
}

// replan is a unit its agent says cannot be done as it was planned, which
// sends the change back to its planner.
// refused is a finished change the person read and did not affirm: its
// units are carried again with what the person said.
type refused struct{ why string }

func (e *refused) Error() string { return "the person did not affirm the change: " + e.why }

// maxRefusals is how many times a change the person did not affirm is
// carried again before the run stops and leaves it to them.
const maxRefusals = 2

type replan struct{ unit, why string }

func (e *replan) Error() string { return e.unit + " cannot be done as it was planned: " + e.why }

// blamed is the tests unit at the head of the plan whose files a code
// unit's failure blames, or -1.
func (r *Run) blamed(c carried, head int, results []carried) int {
	if c.res.Terminal != harness.FailedTask {
		return -1
	}
	// By the path the failure names; by the file's name only where no other
	// tests unit wrote a file of that name.
	byName := map[string][]int{}
	for j := range head {
		for _, p := range results[j].paths {
			if strings.Contains(c.res.Summary, p) {
				return j
			}
			byName[filepath.Base(p)] = append(byName[filepath.Base(p)], j)
		}
	}
	for name, js := range byName {
		if len(js) == 1 && strings.Contains(c.res.Summary, name) {
			return js[0]
		}
	}
	return -1
}

// carried is one unit carried out: its candidate, where it is kept, how the
// executor ended, and what it changed.
type carried struct {
	ca, store string
	res       harness.Result
	paths     []string
}

// carry hands one unit to its agent in a copy of the estate holding the
// candidates in overlays, and keeps what comes of it as a candidate.
func (r *Run) carry(ctx context.Context, i int, overlays []string, prof profile, ok bool) (carried, error) {
	u := r.units[i]
	c := carried{ca: r.id("CA", i+1)}
	ws, err := harness.Snapshot(r.repo, overlays...)
	if err != nil {
		return c, fmt.Errorf("a workspace could not be made: %v", err)
	}
	defer os.RemoveAll(ws)
	before, err := harness.Manifest(ws)
	if err != nil {
		return c, fmt.Errorf("the workspace could not be read: %v", err)
	}
	pack := r.pack(u)
	c.store = filepath.Join(r.dir, "candidates", c.ca)
	if err := os.MkdirAll(c.store, 0o755); err != nil {
		return c, fmt.Errorf("%s: the candidate could not be kept: %v", u.id, err)
	}
	_ = os.WriteFile(filepath.Join(c.store, "pack.md"), []byte("# Goal\n\n"+r.goal(u)+"\n\n"+pack.String()), 0o644)
	scope := u.scope
	if i == len(r.units)-1 && !u.tests {
		// The last unit completes the change: the whole suite must pass
		// after it, and what keeps it from passing may be in any code the
		// change writes. It may mend all of that code — never the tests.
		scope = r.codeScope()
	}
	task := harness.Task{Workspace: ws, Goal: r.goal(u), Scope: scope, Context: pack.String(), BudgetUSD: r.cfg.BudgetUSD}
	switch {
	case ok && u.tests && prof.vet != nil:
		// Tests written first cannot build, since what they call does not
		// exist yet — but only that may keep them from building. They are
		// vetted, and every failure that is not a name the units' contract
		// promises goes back to their agent: a function of a library that
		// has none, the wrong arguments to one that exists.
		cmd := prof.vet(u.scope)
		if overlay, done := r.contractOverlay(ws); overlay != "" && prof.name == "go test" && len(cmd) > 2 && cmd[1] == "test" {
			// What the code units will create is stubbed from the contract,
			// so that the compiler says all the tests got wrong.
			defer done()
			cmd = append([]string{cmd[0], cmd[1], overlay}, cmd[2:]...)
		} else {
			done()
		}
		task.Check = resolve(ws, cmd)
		task.Judge = r.judgeTests()
	case !ok || u.tests:
		// Without a runner to vet with, tests written first are judged
		// afterwards, by failing against the estate as it stands.
	case i == len(r.units)-1:
		task.Check = resolve(ws, prof.suite) // the last unit completes the change
	case len(prof.build) > 0:
		task.Check = resolve(ws, prof.build) // an earlier one leaves the estate building
	}
	if !u.tests {
		task.Provides = providedNames(u.provides)
	}
	if ok && !u.tests && prof.quick != nil {
		task.Quick = func(files []string) []string {
			if cmd := prof.quick(files); cmd != nil {
				return resolve(ws, cmd)
			}
			return nil
		}
	}
	if ok && prof.focus != nil {
		task.Focus = func(pattern, path string) []string { return resolve(ws, prof.focus(pattern, path)) }
	}
	r.say(4, u.id, "%s handed to %s with %d tokens of the estate", u.id, r.cfg.Executor.Name(), pack.Tokens())
	res, err := r.cfg.Executor.Run(ctx, task, func(e harness.Event) {
		r.mu.Lock()
		r.agent[u.id] = append(r.agent[u.id], e)
		r.mu.Unlock()
		r.changed()
	})
	c.res = res
	r.spend(res.Usage)
	r.cost(artifact.Cost{Area: 4, Unit: u.id, What: "execution of " + u.id, Actor: r.cfg.Executor.Name() + " " + r.cfg.Executor.Version(),
		Turns: res.Turns, Steps: res.Steps, Handed: pack.Tokens(), Kept: "candidates/" + c.ca + "/pack.md", Note: res.Terminal}, res.Usage, res.Elapsed)
	r.writeTrace(c.store, u.id) // kept whether or not a candidate comes of it
	if err != nil {
		return c, fmt.Errorf("%s: the executor failed: %v", u.id, err)
	}
	changes, err := harness.Since(ws, before)
	if err != nil {
		return c, fmt.Errorf("%s: what changed could not be read: %v", u.id, err)
	}
	var outside []string
	for _, ch := range changes {
		if !inScope(ch.Path, scope) {
			outside = append(outside, ch.Path)
		}
		c.paths = append(c.paths, ch.Path)
	}
	if len(outside) > 0 {
		return c, fmt.Errorf("%s changed %s, outside its scope; the candidate is refused", u.id, strings.Join(outside, ", "))
	}
	if res.Terminal != harness.Candidate || len(changes) == 0 {
		return c, fmt.Errorf("%s ended %s with %d change(s): %s", u.id, res.Terminal, len(changes), res.Summary)
	}
	if prof.format != nil {
		// Form is not the agent's to spend turns on: what it wrote is put in
		// the estate's one form before it is kept, and what cannot be is
		// left as written, for the check to say why.
		var written []string
		for _, ch := range changes {
			if !ch.Gone {
				written = append(written, ch.Path)
			}
		}
		if cmd := prof.format(written); cmd != nil {
			_, _, _ = runIn(ctx, ws, resolve(ws, cmd))
		}
	}
	if err := harness.Keep(ws, changes, filepath.Join(c.store, "files")); err != nil {
		return c, fmt.Errorf("%s: the candidate could not be kept: %v", u.id, err)
	}
	_ = os.WriteFile(filepath.Join(c.store, "summary.txt"), []byte(res.Summary+"\n"), 0o644)
	if u.preparatoryFor != "" && ok {
		// What judges the change must fail before it, and fail for the stated
		// reason: otherwise it judges nothing.
		names := r.testNames(c.store, prof)
		if len(names) == 0 {
			return c, fmt.Errorf("%s wrote no test this estate's runner can name", u.id)
		}
		passed, out, took := runIn(ctx, ws, resolve(ws, prof.only(names)))
		r.cost(artifact.Cost{Area: 4, Unit: u.id, What: "the tests run against the estate as it stands", Actor: prof.name}, model.Usage{}, took)
		_ = os.WriteFile(filepath.Join(c.store, "failing-before.txt"), []byte(out), 0o644)
		if passed {
			return c, &holds{unit: u.id, tests: names}
		}
		r.say(4, c.ca, "the tests %s fail against the estate as it stands, as they must", strings.Join(names, ", "))
	}
	r.mu.Lock()
	r.cands[u.id] = filepath.Join(c.store, "files")
	r.mu.Unlock()
	r.say(4, c.ca, "%s: %s — %s", c.ca, strings.Join(c.paths, ", "), res.Summary)
	return c, nil
}

// pack is what the estate gives one unit's agent: the files it may change,
// the code its tests exercise or the tests it must satisfy, and around them
// only signatures and names.
func (r *Run) pack(u unit) estate.Pack {
	repo := r.cfg.Estate.Repo(r.repo)
	var read []string
	if u.tests {
		read = r.spec.Scope // the code the tests will exercise, as it stands
	}
	// A file the unit's ticket names, that exists, is read whole: a ticket
	// that says "as render_test.go does" asks for what an agent with no
	// tools would otherwise never see.
	for _, f := range r.named(u) {
		if !slices.Contains(read, f) && !slices.Contains(u.scope, f) {
			read = append(read, f)
		}
	}
	p := repo.PackFor(u.scope, read, r.cfg.packBudget())
	// What earlier units wrote is not in the estate yet, only in their
	// candidates; a unit that depends on them reads them from there.
	var earlier []estate.Section
	r.mu.Lock()
	for _, prev := range r.units {
		if prev.id == u.id || u.tests {
			break // tests units rely on nothing another unit writes
		}
		dir := r.cands[prev.id]
		_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || d.Name() == "CHANGES" {
				return err
			}
			rel, _ := filepath.Rel(dir, p)
			if b, err := os.ReadFile(p); err == nil {
				title := "Written by " + prev.id + ", which comes before you: " + filepath.ToSlash(rel)
				if prev.tests {
					title = "Must pass, written by " + prev.id + ": " + filepath.ToSlash(rel)
				}
				earlier = append(earlier, estate.Section{Title: title, Body: "```\n" + string(b) + "```"})
			}
			return nil
		})
	}
	r.mu.Unlock()
	p.Sections = append(earlier, p.Sections...)
	return p
}

// named is every file of the estate the unit's ticket names, by its path
// or, in a folder of the unit's scope, by its name alone.
func (r *Run) named(u unit) []string {
	var out []string
	for _, m := range fileName.FindAllString(u.title+"\n"+u.brief, -1) {
		m = strings.Trim(m, "./")
		candidates := []string{m}
		if !strings.Contains(m, "/") {
			candidates = nil
			for _, s := range u.scope {
				candidates = append(candidates, path.Join(path.Dir(s), m))
			}
		}
		for _, c := range candidates {
			if fileExists(filepath.Join(r.repo, filepath.FromSlash(c))) && !slices.Contains(out, c) {
				out = append(out, c)
			}
		}
	}
	return out
}

var fileName = regexp.MustCompile(`[\w./-]*\w\.(?:go|py|ts|tsx|js|jsx|rs|java|kt|rb|swift|c|h|cpp|cs|php)\b`)

func inScope(p string, scope []string) bool {
	return slices.ContainsFunc(scope, func(s string) bool {
		s = strings.TrimSuffix(s, "/")
		return p == s || strings.HasPrefix(p, s+"/")
	})
}

func (r *Run) goal(u unit) string {
	var crit []string
	for i, c := range r.spec.Criteria {
		id := r.id("CR", i+1)
		if slices.Contains(u.criteria, id) {
			crit = append(crit, fmt.Sprintf("- %s: when %s, then %s (%s)", id, c.When, c.Then, c.Criterion))
		}
	}
	brief := "## What to do: " + u.title + "\n\n" + u.brief + "\n\n## The criteria this unit serves\n\n" + strings.Join(crit, "\n")
	// Every agent is handed the contract between the units — what each code
	// unit provides, exactly — so that the tests call what the code defines
	// and no unit defines what another provides.
	var contract []string
	for _, o := range r.units {
		for _, p := range o.provides {
			who := o.id
			if o.id == u.id {
				who += ", you"
			}
			contract = append(contract, "- "+p+"   ("+who+")")
		}
	}
	if len(contract) > 0 {
		brief += "\n\n## The contract between the units — use these names and signatures exactly; define only those marked you\n\n" + strings.Join(contract, "\n")
	}
	if why := r.again[u.id]; u.tests && why != "" {
		brief += "\n\n## Why you are asked again\n\nTests written for this unit before were found wrong by the code that had to pass them; it could not, because:\n\n" + why +
			"\n\nWrite them again so that they call only what exists or the contract promises, and check only what the criteria say."
	}
	if r.mend != "" && !u.tests {
		brief += "\n\n## Why you are asked again\n\n" + r.cfg.Person + " read the change this unit made before it could land, and did not affirm it:\n\n" + r.mend +
			"\n\nMend exactly that; what they did not name stands as it was."
	}
	if u.tests {
		return brief + "\n\nWrite only the tests. The behaviour does not exist yet, and neither does the code that will provide it: " +
			"call it by the names the brief gives, and leave it undefined — the code units define it, to pass your tests. " +
			"Fakes of what the code depends on (a mailer, a clock, a payment provider) belong in the tests; what the code itself provides never does. " +
			"The tests fail against the estate as it stands, if only because what they call is not there yet. Name each test after what it checks."
	}
	return brief + "\n\nTests that decide the criteria were written first; do not change them."
}

func (r *Run) testNames(store string, prof profile) []string {
	var names []string
	_ = filepath.WalkDir(filepath.Join(store, "files"), func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if b, err := os.ReadFile(p); err == nil {
				for _, m := range prof.names.FindAllStringSubmatch(string(b), -1) {
					names = append(names, m[1])
				}
			}
		}
		return nil
	})
	slices.Sort(names)
	return slices.Compact(names)
}

// writeTrace keeps every step the executor took, with what each model turn
// cost, beside the candidate it produced.
func (r *Run) writeTrace(store, unit string) {
	r.mu.Lock()
	events := slices.Clone(r.agent[unit])
	r.mu.Unlock()
	_ = harness.WriteTrace(filepath.Join(store, harness.TraceFile), events)
}

// assure is area 5: evidence gathered in a copy of the estate holding every
// candidate, by what judges the change, and a verdict on every criterion.
func (r *Run) assure(ctx context.Context) error {
	r.set(5, Working)
	prof, ok := detect(r.repo)
	if !ok {
		return r.fail(5, "nothing here says how this estate's tests are run, so no evidence can be gathered: every criterion would be undecided")
	}
	var overlays []string
	for _, u := range r.units {
		overlays = append(overlays, r.cands[u.id])
	}
	ws, err := harness.Snapshot(r.repo, overlays...)
	if err != nil {
		return r.fail(5, "a workspace could not be made: %v", err)
	}
	defer os.RemoveAll(ws)
	doc := meta.Assurance{Schema: meta.SchemaVersion, Name: r.slug}
	passed := map[string]bool{} // evidence id → passed
	byCriterion := map[string]string{}
	for i, prep := range r.units {
		if !prep.tests {
			continue
		}
		ca := r.id("CA", i+1)
		names := r.testNames(filepath.Join(r.dir, "candidates", ca), prof)
		cmd := prof.only(names)
		ok, out, took := runIn(ctx, ws, resolve(ws, cmd))
		ev := r.id("EV", i+1, 1)
		r.cost(artifact.Cost{Area: 5, Unit: prep.id, What: ev + ": the tests written first", Actor: prof.name}, model.Usage{}, took)
		passed[ev] = ok
		r.keepOutput(ev, out)
		doc.Evidence = append(doc.Evidence, meta.Evidence{ID: ev, Subject: meta.OfCandidate, Kind: meta.TestRun, Producer: shown(cmd),
			Independence: meta.ByPrecommitment,
			How:          "written by " + prep.id + " and failed against the unchanged estate before the change existed — candidates/" + ca + "/failing-before.txt",
			Repeatable:   meta.Yes, ObtainedAt: today(), ValidUntil: "next change to " + strings.Join(r.spec.Scope, ", ")})
		for _, c := range prep.criteria {
			byCriterion[c] = ev
		}
		r.say(5, ev, "%s: %s %s", ev, shown(cmd), verdictWord(ok))
	}
	ok2, out, took := runIn(ctx, ws, resolve(ws, prof.suite))
	suite := r.id("EV", len(r.units), 1)
	r.cost(artifact.Cost{Area: 5, What: suite + ": the whole suite", Actor: prof.name}, model.Usage{}, took)
	passed[suite] = ok2
	r.keepOutput(suite, out)
	doc.Evidence = append(doc.Evidence, meta.Evidence{ID: suite, Subject: meta.OfCandidate, Kind: meta.TestRun, Producer: shown(prof.suite),
		Independence: meta.ByExecutor, How: "the estate's own suite, written before this change and outside everything it touched",
		Repeatable: meta.Yes, ObtainedAt: today(), ValidUntil: "next change to the estate"})
	r.say(5, suite, "%s: %s %s", suite, shown(prof.suite), verdictWord(ok2))
	// What only a person can see, the person affirms, reading the finished
	// change before it lands: one question, every such criterion in it.
	var byPerson []string
	for i, c := range r.spec.Criteria {
		if c.Procedure == "human" {
			id := r.id("CR", i+1)
			byPerson = append(byPerson, fmt.Sprintf("%s: when %s, then %s", id, c.When, c.Then))
			byCriterion[id] = r.id("EV", len(r.units), 2)
		}
	}
	var refusal string
	if len(byPerson) > 0 {
		ev := r.id("EV", len(r.units), 2)
		got, err := r.ask(ctx, 5, "Does the change do what only you can judge? Read it — the candidates' files under Execution — and accept if each of these holds, or send it back saying which does not:\n\n"+strings.Join(byPerson, "\n"))
		if err != nil {
			return err
		}
		passed[ev] = got.ok
		how := "affirmed by " + r.cfg.Person + ", who did not write the candidate, reading it before it landed"
		if !got.ok {
			how = "refused by " + r.cfg.Person + ": " + got.ground
		}
		doc.Evidence = append(doc.Evidence, meta.Evidence{ID: ev, Subject: meta.OfCandidate, Kind: meta.HumanAffirmation,
			Producer: r.cfg.Person, Independence: meta.ByExecutor, How: how, Repeatable: meta.No, ObtainedAt: today()})
		r.say(5, ev, "%s: %s %s", ev, r.cfg.Person, map[bool]string{true: "affirmed it", false: "did not affirm it: " + got.ground}[got.ok])
		if !got.ok {
			refusal = got.ground
		}
	}
	n := 0
	allMet := true
	for _, u := range r.units {
		for _, c := range u.criteria {
			ev, ok := byCriterion[c]
			if !ok {
				ev = suite
			}
			n++
			outcome := meta.Met
			if !passed[ev] {
				outcome, allMet = meta.Failed, false
			}
			ve := r.id("VE", n)
			doc.Verdicts = append(doc.Verdicts, meta.Verdict{ID: ve, Unit: u.id, Criterion: c, Outcome: outcome, Evidence: []string{ev}, State: meta.Settled})
			r.say(5, ve, "%s: %s on %s — %s, on %s", ve, u.id, c, outcome, ev)
		}
	}
	if allMet {
		for _, ref := range r.settles {
			doc.Settled = append(doc.Settled, meta.Settlement{Escalation: ref, ResolvedBy: "this change", Note: "every criterion met; started to close it"})
			r.say(5, ref, "%s is closed by this change", ref)
		}
	}
	if err := artifact.Save(r.dir, artifact.Assurance, doc); err != nil {
		return r.fail(5, "the assurance could not be written: %v", err)
	}
	for _, v := range doc.Verdicts {
		if err := r.record("assurance", v.ID, string(v.Outcome)); err != nil {
			return err
		}
	}
	if rep, err := r.check(ctx, 5); err != nil || !rep.Passed() {
		return r.fail(5, "the assurance does not pass the gate")
	}
	if !allMet && refusal != "" && r.refusals < maxRefusals {
		return &refused{why: refusal}
	}
	if !allMet {
		return r.fail(5, "a criterion failed, so nothing enters")
	}
	r.set(5, Done)
	return nil
}

func verdictWord(ok bool) string {
	if ok {
		return "passed"
	}
	return "failed"
}

func (r *Run) keepOutput(ev, out string) {
	dir := filepath.Join(r.dir, "evidence")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, ev+".txt"), []byte(out), 0o644)
}

// land is area 6: each accepted candidate entered into the estate, in the
// order the plan gave, while its evidence still holds.
func (r *Run) land(ctx context.Context) error {
	r.set(6, Working)
	doc := meta.Landing{Schema: meta.SchemaVersion, Name: r.slug}
	// The estate may have moved since admission: another change landed on
	// the same ground. Entering whole files now would undo it, and the
	// evidence was gathered against ground that is no longer there. Where
	// the two changes' edits are apart, they are merged and the whole suite
	// is run on the estate as it is now with the change on it; where they
	// overlap, or the suite fails, the change stops.
	merged, moved, why := r.rebased(ctx)
	if why != "" {
		_ = r.record("landing", r.id("SPEC"), "stopped: the estate moved under it — "+why)
		return r.fail(6, "the estate moved since this change was admitted (%s), and it cannot be laid on the estate as it is now: %s. Carry it again from the estate as it is now", strings.Join(moved, "; "), why)
	}
	if len(merged) > 0 {
		r.say(6, r.id("SPEC"), "the estate moved under it (%s); merged with what moved it, the whole suite passes on the estate as it is now", strings.Join(moved, "; "))
		_ = r.record("landing", r.id("SPEC"), "merged onto the estate as it is now: "+strings.Join(moved, "; ")+"; the whole suite passed on the merged ground")
	}
	for i, u := range r.units {
		if err := copyInto(r.cands[u.id], r.repo); err != nil {
			return r.fail(6, "%s could not be entered: %v", u.id, err)
		}
		for rel, b := range merged {
			if inScope(rel, u.scope) || slices.Contains(r.codeScope(), rel) {
				if err := os.WriteFile(filepath.Join(r.repo, filepath.FromSlash(rel)), b, 0o644); err != nil {
					return r.fail(6, "%s could not be entered merged: %v", rel, err)
				}
			}
		}
		doc.Entries = append(doc.Entries, meta.Entry{Order: i + 1, Unit: u.id, VerdictState: "settled",
			StillValid: "yes", EnteredAt: today(), Invalidated: "nothing the evidence relies on"})
		r.say(6, u.id, "%s entered", u.id)
	}
	if err := artifact.Save(r.dir, artifact.Landing, doc); err != nil {
		return r.fail(6, "the landing could not be written: %v", err)
	}
	for _, u := range r.units {
		if err := r.record("landing", u.id, "entered"); err != nil {
			return err
		}
	}
	c, err := artifact.Load(r.dir)
	if err == nil {
		began := time.Now()
		reps, err := r.cfg.Gate.Repeat(ctx, c)
		r.cost(artifact.Cost{Area: 6, What: "every claim to repeat, run twice", Actor: "the gate"}, model.Usage{}, time.Since(began))
		if err == nil {
			for _, rp := range reps {
				r.say(6, rp.Evidence, "%s repeated: %s gave the same result twice: %v", rp.Evidence, rp.Shown(), rp.Same)
			}
		}
	}
	r.arbitrate(ctx)
	if rep, err := r.check(ctx, 6); err != nil || !rep.Passed() {
		return r.fail(6, "the landing does not pass the gate")
	}
	r.set(6, Done)
	return nil
}

func copyInto(from, to string) error {
	return filepath.WalkDir(from, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() == "CHANGES" {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		dst := filepath.Join(to, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, b, 0o644)
	})
}

func head(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}

// list is a list a model may also write as its one element alone — a
// termination condition given as an object where a list of them was asked
// for — which says the same and is read as the same.
type list[T any] []T

func (l *list[T]) UnmarshalJSON(b []byte) error {
	if t := bytes.TrimSpace(b); len(t) > 0 && t[0] == '{' {
		var one T
		if err := json.Unmarshal(t, &one); err != nil {
			return err
		}
		*l = list[T]{one}
		return nil
	}
	var many []T
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*l = many
	return nil
}

// specText is the specification as an agent reads it: the change described,
// then the method's record of how it is judged.
func (r *Run) specText() string {
	doc, _ := os.ReadFile(filepath.Join(r.dir, artifact.ChangeFile))
	meta, err := os.ReadFile(filepath.Join(r.dir, string(artifact.Specification)+".md"))
	if err != nil {
		return ""
	}
	return string(doc) + "\n# How it is judged — the specification's record\n\n" + string(meta)
}

func terms(d draft) []string {
	var out []string
	for _, t := range d.Termination {
		out = append(out, t.Action+" when "+t.Condition)
	}
	return out
}

// flight is another change in flight in the same estate, and the paths it
// may change that this one may too.
type flight struct {
	name   string
	shared []string
}

// inFlight is every other change of the estate not yet landed nor closed
// whose scope meets this one's.
func (r *Run) inFlight() []flight {
	dirs, _ := filepath.Glob(filepath.Join(r.repo, "changes", "*", string(artifact.Specification)+".md"))
	var out []flight
	for _, spec := range dirs {
		dir := filepath.Dir(spec)
		if dir == r.dir {
			continue
		}
		c, err := artifact.Load(dir)
		if err != nil || c.Meta.Specification == nil || c.Has(artifact.Landing) {
			continue
		}
		if _, closed := gate.Closed(c); closed {
			continue
		}
		var shared []string
		for _, p := range c.Meta.Specification.Scope.Included {
			if p = strings.TrimSpace(p); p != "" && inScope(p, r.spec.Scope) {
				shared = append(shared, p)
			}
		}
		if len(shared) > 0 {
			out = append(out, flight{filepath.Base(dir), shared})
		}
	}
	return out
}

// judgeTests accepts a vet of tests written before their code when every
// failure it names is a name the units' contract promises, and returns the
// rest to the tests' agent.
func (r *Run) judgeTests() func(bool, string) (bool, string) {
	promised := map[string]bool{}
	for _, u := range r.units {
		for _, p := range u.provides {
			for _, w := range identifier.FindAllString(p, -1) {
				promised[w] = true
			}
		}
	}
	// Only a package the code units write into may lack a name, and only a
	// package a code unit creates may be missing whole: the tests import it
	// before it exists.
	local := harness.ScopePackages(r.codeScope())
	created := map[string]bool{}
	for _, p := range r.codeScope() {
		created[filepath.ToSlash(filepath.Dir(p))] = true
	}
	return func(passed bool, out string) (bool, string) {
		if passed {
			return true, ""
		}
		var theirs []string
		tolerated := 0
		for _, line := range strings.Split(out, "\n") {
			if !faultLine.MatchString(line) {
				continue
			}
			if strings.Contains(line, stubFile) {
				tolerated++ // the contract's own stub: not the tests' to mend
				continue
			}
			// What the code units will define may be missing; nothing
			// else may — least of all a name of a library, which no unit
			// can define.
			if m := undefinedName.FindStringSubmatch(line); m != nil && promised[m[1]+m[2]+m[3]] && harness.MissingHere(line, local) {
				tolerated++
				continue
			}
			if m := missingPackage.FindStringSubmatch(line); m != nil && createdBy(m[1]+m[2], created) {
				tolerated++
				continue
			}
			theirs = append(theirs, strings.TrimSpace(line))
		}
		if len(theirs) == 0 && tolerated == 0 {
			// It fails, and nothing it says is a line of the tests: they
			// are not there to judge — a folder missing, a file not written.
			return false, "The check fails, and not because of a line of your tests: they are not all written yet. Write every file in your scope.\n\n" + lastLines(out, 12)
		}
		if len(theirs) == 0 {
			return true, ""
		}
		return false, "Your tests do not build for reasons of their own, not because what they call is still to be written — only names the contract promises may be missing:\n\n" +
			strings.Join(theirs, "\n") + "\n\nUse only what exists, by the signatures you were shown, and what the contract promises. A library's source can be read where the brief says."
	}
}

// lastLines is the last n lines of out.
func lastLines(out string, n int) string {
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	return strings.Join(lines[max(0, len(lines)-n):], "\n")
}

var (
	identifier     = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
	faultLine      = regexp.MustCompile(`^\s*(?:vet: )?[\w./-]+\.\w+:\d+:\d+: `)
	missingPackage = regexp.MustCompile(`no required module provides package ([^\s;]+)|could not import ([^\s;]+)`)
	undefinedName  = regexp.MustCompile(`undefined: (?:\w+\.)?(\w+)|has no field or method (\w+)|unknown field (\w+) in struct literal`)
)
