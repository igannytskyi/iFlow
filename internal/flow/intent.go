package flow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/estate"
	"github.com/igannytskyi/iflow/internal/meta"
	"github.com/igannytskyi/iflow/internal/model"
)

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

// draft is what area 1 asks a model to write, before it is checked.
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
	Later    []string         `json:"later"`
	Criteria []draftCriterion `json:"criteria"`
	// Relies are the names the change builds on that exist already, each
	// as the estate declares it.
	Relies      []meta.Provision `json:"relies_on"`
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
- Every entry of "scope", "new_tests" and "existing_tests" is the path of a file and nothing else — "test_greeting.py", never "test_greeting.py::GreetTest" or a path with words after it.
- Keep the change to what a few agents, each with one unit, can carry out. If the task is larger, specify its first coherent part — one that is worth landing alone — and list in "later" the rest, each written as a task of its own, in the order they should come.
- "existing_tests" lists the estate's existing test files that judge that nothing else moves.
- "relies_on" lists every name that exists already and the change calls, changes or builds on, each as {"file" it is declared in, "name", "kind": one of function, method, type, field, variable, constant, "of": the type a method or field belongs to}. The framework writes each one's declaration from the estate; you do not copy it. You are shown how the names near the task are declared; one you are not shown, you do not guess — leave it out or say in "open_questions" what is unknown. Every other part of the specification states these names as declared here: a decision about an existing name never contradicts its declaration.
- "slug" is a few lowercase words joined by hyphens.

The JSON object has exactly these keys: slug, name, statement, class, prose, document (the keys above), later, criteria (criterion, when, then, plan, threshold, judged_by_new_tests, procedure), relies_on (file, name, kind, of), scope, new_tests, existing_tests, termination (condition, action: stop or escalate).`

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
		var names []string
		for _, l := range found {
			var at []string
			for _, p := range l.Places {
				at = append(at, fmt.Sprintf("%s:%d", p.File, p.Start))
			}
			fmt.Fprintf(&b, "- %q %s %s — %s\n", l.Word, l.How, strings.Join(l.Names, ", "), strings.Join(at, ", "))
			names = append(names, l.Names...)
		}
		if decl := r.declarations(names, 40); len(decl) > 0 {
			b.WriteString("\nHow those names are declared:\n\n" + strings.Join(decl, "\n") + "\n")
		}
		b.WriteString("\n")
	}
	if p, ok := detect(r.repo); ok {
		b.WriteString("Its tests are run with " + shown(p.suite) + "; a new test goes where that runs it, or it judges nothing.\n")
		// A runner the estate keeps as a script says itself where it looks.
		if script, err := os.ReadFile(filepath.Join(r.repo, p.suite[0])); err == nil && len(script) < 2000 {
			b.WriteString("It reads:\n\n```\n" + strings.TrimSpace(string(script)) + "\n```\n")
		}
	}
	if a := aims(r.repo, r.cfg.packBudget()/4); a != "" {
		b.WriteString("\nWhat the project is for and how its work is done, in its own words — a change serves this, not only the task:\n\n" + a + "\n")
	}
	if d := decisions(r.repo, r.cfg.packBudget()/8); d != "" {
		b.WriteString("\nWhat the person has already decided — take it as settled, and do not ask it again:\n\n" + d + "\n")
	}
	return b.String()
}

// aimsFiles are where a project says what it is for and how work on it is
// done, in the order they are read.
var aimsFiles = []string{"README.md", "AGENTS.md"}

// aims is what the project says of itself, within budget tokens: each of
// aimsFiles that it has, from its beginning, by whole paragraphs.
func aims(repo string, budget int) string {
	var out []string
	for _, f := range aimsFiles {
		b, err := os.ReadFile(filepath.Join(repo, f))
		if err != nil {
			continue
		}
		var kept []string
		for _, para := range strings.Split(strings.TrimSpace(string(b)), "\n\n") {
			if budget -= estate.Tokens(para); budget < 0 {
				break
			}
			kept = append(kept, para)
		}
		if len(kept) > 0 {
			out = append(out, "## "+f+"\n\n"+strings.Join(kept, "\n\n"))
		}
		if budget < 0 {
			break
		}
	}
	return strings.Join(out, "\n\n")
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
			r.say(1, r.id("SPEC"), "%s accepted the criteria", r.who(got))
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
		if round > r.cfg.reviewRounds() {
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
		answer, u, err := model.AskJSON(model.Within(ctx, "author"), r.cfg.Model, intentSystem, prompt+feedback+note, &d)
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
		breaks := d.problems()
		relied, whys := r.relied(d.Relies)
		if breaks = append(breaks, whys...); len(whys) == 0 {
			d.Relies = relied
		}
		if why := strings.Join(breaks, "; "); why != "" {
			// The author is handed what it wrote and every way it breaks, to
			// mend those and keep the rest: asked afresh, it would write
			// afresh; told of one break, it would mend it and break another.
			note = "\n\n# Your last answer\n\n```json\n" + strings.TrimSpace(answer) + "\n```\n\nIt cannot be used, for each of these reasons:\n\n- " + strings.Join(breaks, "\n- ") +
				"\n\nAnswer with that answer, mended exactly where it breaks, and the rest as it was."
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
		note = "\n\n# Your last answer\n\n```json\n" + strings.TrimSpace(answer) + "\n```\n\nThe gate refused it: " + strings.Join(found, "; ") + "."
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

// filePath is what a path of the estate looks like: no spaces, no test
// named after it, no words said of it.
var filePath = regexp.MustCompile(`^[\w./@+-]+$`)

func (d draft) problem() string {
	return strings.Join(d.problems(), "; ")
}

// problems is every way the draft cannot be used, all at once: an author
// told of one mends it and breaks another.
func (d draft) problems() []string {
	var out []string
	if d.Slug == "" || d.Statement == "" {
		out = append(out, "it has no slug or no statement")
	}
	if len(d.Criteria) == 0 {
		out = append(out, "it has no criteria: \"criteria\" lists each as {criterion, when, then, plan, threshold, judged_by_new_tests, procedure}, and the tests and the person judge those alone")
	}
	if len(d.Scope) == 0 {
		out = append(out, "it names no scope")
	}
	if d.Document.Summary == "" || len(d.Document.Flows) == 0 || len(d.Document.Requirements) == 0 || len(d.Document.Interfaces) == 0 || len(d.Document.Components) == 0 {
		out = append(out, "it does not specify the change: summary, flows, requirements, interfaces or components are empty")
	}
	if !meta.Allowed("Class", d.Class) {
		out = append(out, "class "+d.Class+" is not one of C1P, C1T, C2, C3, C4, C5")
	}
	for _, group := range []struct {
		name  string
		paths []string
	}{{"scope", d.Scope}, {"new_tests", d.NewTests}, {"existing_tests", d.SuiteTests}} {
		for _, p := range group.paths {
			if !filePath.MatchString(p) {
				out = append(out, fmt.Sprintf("%s holds %q, which is not the path of a file: each entry is a path alone — no test named in it, nothing said of it", group.name, p))
			}
		}
	}
	for _, t := range d.NewTests {
		if slices.Contains(d.Scope, t) {
			out = append(out, t+" is both in scope and a test that judges it")
		}
	}
	// What the change makes true that is not true now is judged by tests
	// that fail before it: the existing tests pass before it, and judge
	// nothing of it.
	for i, c := range d.Criteria {
		if c.Plan == "required" && c.Procedure != "human" && (!c.New || len(d.NewTests) == 0) {
			out = append(out, fmt.Sprintf("criterion %d (%s) is about behaviour the change makes, and no new test judges it: the existing tests pass before the change and cannot; name a new test file in new_tests and mark the criterion judged_by_new_tests", i+1, c.Criterion))
		}
	}
	return out
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
	spec := meta.Specification{Schema: meta.SchemaVersion, Name: r.slug, ID: r.id("SPEC"), Intent: r.id("INT"), Class: meta.Class(d.Class), Prose: d.Prose, Relies: d.Relies,
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
	var relies []string
	for _, p := range d.Relies {
		relies = append(relies, p.Signature+" — "+p.File)
	}
	parts := append(d.Document.parts(), artifact.Part{Title: "Existing names it builds on", Points: relies},
		artifact.Part{Title: "Acceptance criteria", Points: crit},
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

// declarations is how each of names is declared in the estate, at most n
// of them: where, and its first line.
func (r *Run) declarations(names []string, n int) []string {
	repo := r.cfg.Estate.Repo(r.repo)
	var out []string
	for _, name := range slices.Compact(slices.Sorted(slices.Values(names))) {
		places, _ := repo.WhereDefined(name)
		for _, p := range places {
			if len(out) == n {
				return out
			}
			if ex := repo.Excerpt(p.File, p.Start, p.Start, 1); len(ex) == 1 {
				out = append(out, fmt.Sprintf("%s:%d  %s", p.File, p.Start, estate.Signature(ex[0].Text)))
			}
		}
	}
	return out
}

// relied is what a specification relies on, each name with the declaration
// the estate holds for it — the framework copies it, the author only names
// it — or why one cannot be: a name its file does not declare, or one its
// file declares more than once and the author's words do not choose.
func (r *Run) relied(relies []meta.Provision) ([]meta.Provision, []string) {
	repo := r.cfg.Estate.Repo(r.repo)
	out := make([]meta.Provision, 0, len(relies))
	var whys []string
	for _, p := range relies {
		switch {
		case p.Name == "" || p.File == "":
			whys = append(whys, fmt.Sprintf("relies_on holds %q without its file or its name", p.Name+p.File))
			continue
		case !meta.Allowed("DeclKind", string(p.Kind)) || p.Kind == meta.Described:
			whys = append(whys, fmt.Sprintf("relies_on gives %s the kind %q, which is not one of function, method, type, field, variable, constant — an interface, a struct or a class is a type", p.Name, p.Kind))
			continue
		}
		if !identifier.MatchString(p.Name) || (p.Of != "" && !identifier.MatchString(strings.TrimLeft(p.Of, "*"))) {
			whys = append(whys, fmt.Sprintf("relies_on names %q: a name is the name alone, as the code spells it — where it is declared goes in \"file\", the type it belongs to in \"of\"", p.Name))
			continue
		}
		// A method or field may be named by its type, Config.Model, or its
		// package, models.Config: the estate knows it by its own name.
		name, of := p.Name, bareType(p.Of)
		if q, base, ok := cutLast(name, "."); ok {
			name = base
			if of == "" {
				of = bareType(q)
			}
		}
		if filepath.Ext(p.File) == ".go" {
			// Go is read by its own parser: a method by its receiver, a
			// field by its struct, an interface's method by its interface;
			// a name qualified by its file's package is that package's.
			src, err := os.ReadFile(filepath.Join(r.repo, filepath.FromSlash(p.File)))
			decl, all := "", []string(nil)
			if err == nil {
				if pkg := goPackage(src); pkg != "" && of == pkg {
					of = ""
				}
				decl, all = goDeclarationOf(src, name, of)
			}
			switch {
			case decl != "":
				p.Signature = decl
				out = append(out, p)
			case len(all) > 0:
				whys = append(whys, fmt.Sprintf("relies_on names %s in %s, which declares %s: name the type it belongs to in \"of\"", p.Name, p.File, strings.Join(quoted(all), " and ")))
			default:
				whys = append(whys, fmt.Sprintf("relies_on names %s in %s, which does not declare it%s; a name the change adds is not relied on, and one that exists is relied on where it is", p.Name, p.File, map[bool]string{true: " as a member of " + of, false: ""}[of != ""]))
			}
			continue
		}
		places, _ := repo.WhereDefined(name)
		var lines []string
		for _, pl := range places {
			if pl.File != p.File {
				continue
			}
			if ex := repo.Excerpt(pl.File, pl.Start, pl.Start, 1); len(ex) == 1 {
				lines = append(lines, estate.Signature(ex[0].Text))
			}
		}
		line, why := chosen(lines, p.Signature, of)
		switch {
		case len(lines) == 0:
			whys = append(whys, fmt.Sprintf("relies_on names %s in %s, which the estate does not declare there; a name the change adds is not relied on, and one that exists is relied on where it is", p.Name, p.File))
			continue
		case why != "":
			whys = append(whys, fmt.Sprintf("relies_on names %s in %s, which declares %s: %s", p.Name, p.File, strings.Join(quoted(lines), " and "), why))
			continue
		}
		p.Signature = line
		out = append(out, p)
	}
	if len(whys) > 0 {
		return nil, whys
	}
	return out, nil
}

// chosen is the one of lines, a file's declarations of one name, that the
// author means: the only one, the one its signature states, or the one of
// the type it names.
func chosen(lines []string, signature, of string) (string, string) {
	if len(lines) == 1 {
		return lines[0], ""
	}
	for _, l := range lines {
		if signature != "" && declares(l, signature) {
			return l, ""
		}
	}
	if of != "" {
		var mine []string
		for _, l := range lines {
			if strings.Contains(l, " "+of+")") || strings.Contains(l, "*"+of+")") || strings.Contains(l, "class "+of) {
				mine = append(mine, l)
			}
		}
		if len(mine) == 1 {
			return mine[0], ""
		}
	}
	return "", "name the type it belongs to in \"of\""
}

func quoted(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = strconv.Quote(l)
	}
	return out
}

// declares says whether line, a declaration's first line, declares what
// signature states: the same but for its body, the block it opens, and its
// spacing.
func declares(line, signature string) bool {
	norm := func(s string) string { return strings.TrimRight(strings.Join(strings.Fields(s), " "), " {:") }
	l, s := norm(estate.Signature(line)), norm(signature)
	rest, ok := strings.CutPrefix(l, s)
	return ok && (rest == "" || strings.HasPrefix(strings.TrimSpace(rest), "="))
}

// identifier is a name as code spells it, perhaps qualified: Config,
// models.Config, Config.Model.
var identifier = regexp.MustCompile(`^[\pL_][\pL\pN_]*(\.[\pL_][\pL\pN_]*)*$`)

// goDeclarationOf is the declaration, as its first line reads, of name in
// the Go file src: a member of the type of where of is given — a method by
// its receiver, a field of a struct, a method of an interface — a top-level
// name where not. Where it is not found as asked, all is every declaration
// of that name in the file, for the author to choose among.
func goDeclarationOf(src []byte, name, of string) (decl string, all []string) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.SkipObjectResolution)
	if err != nil {
		return "", nil
	}
	line := func(n ast.Node) string {
		from, to := fset.Position(n.Pos()).Offset, fset.Position(n.End()).Offset
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Body != nil {
			to = fset.Position(fn.Body.Lbrace).Offset // its signature, not its body
		}
		text := string(src[from:to])
		if i := strings.IndexByte(text, '\n'); i >= 0 {
			text = text[:i]
		}
		return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), "{"))
	}
	found := func(owner string, n ast.Node) {
		l := line(n)
		all = append(all, l)
		if decl == "" && owner == of {
			decl = l
		}
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Name.Name != name {
				continue
			}
			owner := ""
			if d.Recv != nil && len(d.Recv.List) == 1 {
				owner = receiverType(d.Recv.List[0].Type)
			}
			found(owner, d)
		case *ast.GenDecl:
			for _, sp := range d.Specs {
				switch sp := sp.(type) {
				case *ast.TypeSpec:
					if sp.Name.Name == name {
						found("", sp)
					}
					var fields *ast.FieldList
					switch t := sp.Type.(type) {
					case *ast.StructType:
						fields = t.Fields
					case *ast.InterfaceType:
						fields = t.Methods
					}
					if fields == nil {
						continue
					}
					for _, fd := range fields.List {
						for _, id := range fd.Names {
							if id.Name == name {
								found(sp.Name.Name, fd)
							}
						}
					}
				case *ast.ValueSpec:
					for _, id := range sp.Names {
						if id.Name == name {
							found("", sp)
						}
					}
				}
			}
		}
	}
	return decl, all
}

// goPackage is the name of the package the Go file src is in.
func goPackage(src []byte) string {
	f, err := parser.ParseFile(token.NewFileSet(), "", src, parser.PackageClauseOnly)
	if err != nil {
		return ""
	}
	return f.Name.Name
}
