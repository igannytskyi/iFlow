package flow

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/estate"
	"github.com/igannytskyi/iflow/internal/harness"
	"github.com/igannytskyi/iflow/internal/meta"
	"github.com/igannytskyi/iflow/internal/model"
)

// codeScope is every file the code units of the plan may change, in plan
// order, each once.
func (r *Run) codeScope() []string {
	var out []string
	for _, u := range r.units {
		if u.isTests() {
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

// behaves is tests written first that pass on the contract and not on the
// estate without it: the contract does what it was only to declare.
type behaves struct{ tests []string }

func (b *behaves) Error() string {
	return "the contract carries behaviour: the tests " + strings.Join(b.tests, ", ") + " already pass on its declarations"
}

// sentBack is a decision a later step cannot build on: the change goes back
// to the area that made it — its specification (1) or its plan (2) — with
// why, once a run for each.
type sentBack struct {
	area      int
	unit, why string
}

func (e *sentBack) Error() string {
	return fmt.Sprintf("%s: sent back to area %d — %s", e.unit, e.area, e.why)
}

// blamed is the tests unit whose tests a code unit's failure blames, or
// -1: by the tests its agent held wrong, by name, or the files its check
// failed in or its agent named, by path — each found in the test ledger,
// exactly, never by a likeness.
func (r *Run) blamed(c carried, _ []carried) int {
	if c.res.Terminal != harness.FailedTask {
		return -1
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	unitOf := func(match func(meta.TestRecord) bool) int {
		for _, t := range r.ledger {
			if match(t) {
				return slices.IndexFunc(r.units, func(u unit) bool { return u.id == t.Unit })
			}
		}
		return -1
	}
	for _, name := range c.res.BlamedTests {
		if j := unitOf(func(t meta.TestRecord) bool { return t.Test == name }); j >= 0 {
			return j
		}
	}
	for _, f := range c.res.Blames {
		f = path.Clean(strings.TrimPrefix(filepath.ToSlash(f), "./"))
		if j := unitOf(func(t meta.TestRecord) bool { return t.File == f }); j >= 0 {
			return j
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
	tests     []meta.TestRecord // a tests unit's part of the ledger
}

// carry hands one unit to its agent in a copy of the estate holding the
// candidates it is carried on, and keeps what comes of it as a candidate.
func (r *Run) carry(ctx context.Context, i int, on []onto, prof profile, ok bool) (carried, error) {
	u := r.units[i]
	c := carried{ca: r.id("CA", i+1)}
	overlays := make([]string, len(on))
	for k, o := range on {
		overlays[k] = o.files
	}
	ws, err := harness.Snapshot(r.repo, overlays...)
	if err != nil {
		return c, fmt.Errorf("a workspace could not be made: %v", err)
	}
	defer os.RemoveAll(ws)
	before, err := harness.Manifest(ws)
	if err != nil {
		return c, fmt.Errorf("the workspace could not be read: %v", err)
	}
	c.store = filepath.Join(r.dir, "candidates", c.ca)
	if err := os.MkdirAll(c.store, 0o755); err != nil {
		return c, fmt.Errorf("%s: the candidate could not be kept: %v", u.id, err)
	}
	scope := u.scope
	if i == len(r.units)-1 && !u.isTests() {
		// The last unit completes the change: the whole suite must pass
		// after it, and what keeps it from passing may be in any code the
		// change writes. It may mend all of that code — never the tests.
		scope = r.codeScope()
	}
	if u.isContract() && canDeclare(u) {
		// The contract is the plan's provisions, declared: data the
		// framework writes, never a model's to write without behaviour.
		res, err := r.declareContract(ctx, ws, u, prof, ok)
		if err != nil {
			return c, err
		}
		c.res = res
	} else {
		res, err := r.byAgent(ctx, i, ws, on, scope, prof, ok, c)
		c.res = res
		if err != nil {
			return c, err
		}
	}
	res := c.res
	var breaks []string
	if u.isTests() {
		// The ledger is the unit's answer, not part of the change: read,
		// held to what was written, and taken out of the copy.
		c.tests, breaks = r.readLedger(ws, u)
		_ = os.Remove(filepath.Join(ws, LedgerFile))
	}
	changes, err := harness.Since(ws, before)
	if err != nil {
		return c, fmt.Errorf("%s: what changed could not be read: %v", u.id, err)
	}
	// What the agent was given beyond the unit's scope is its scope too,
	// and is said: the plan was a hint, and this is where it was wrong.
	scope = append(slices.Clone(scope), res.Widened...)
	if len(res.Widened) > 0 {
		r.say(4, u.id, "%s changed %s beyond what the plan gave it, within the specification's scope", u.id, strings.Join(res.Widened, ", "))
		r.mu.Lock()
		if r.widened == nil {
			r.widened = map[string][]string{}
		}
		r.widened[u.id] = res.Widened
		r.mu.Unlock()
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
	// A unit that finds its work done already — a contract whose names are
	// there, a code unit an earlier one left nothing for — changes nothing,
	// and that is its answer, its check passed and its names declared. Tests
	// that change nothing are no tests.
	if res.Terminal != harness.Candidate || (len(changes) == 0 && u.isTests()) {
		return c, fmt.Errorf("%s ended %s with %d change(s): %s", u.id, res.Terminal, len(changes), res.Summary)
	}
	if len(breaks) > 0 {
		return c, fmt.Errorf("%s wrote tests whose ledger does not hold: %s", u.id, strings.Join(breaks, "; "))
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
		var names []string
		for _, t := range c.tests {
			names = append(names, t.Test)
		}
		slices.Sort(names)
		names = slices.Compact(names)
		if len(names) == 0 {
			return c, fmt.Errorf("%s wrote no test this estate's runner can name", u.id)
		}
		run := runTests(ctx, ws, prof, names)
		r.cost(artifact.Cost{Area: 4, Unit: u.id, What: "the tests run against the estate as it stands", Actor: prof.name}, model.Usage{}, run.Took)
		_ = os.WriteFile(filepath.Join(c.store, "failing-before.txt"), []byte(run.Out), 0o644)
		if run.Unbuilt != "" {
			return c, fmt.Errorf("%s: its tests do not build on the contract, so they judge nothing yet: %s", u.id, lastLines(run.Unbuilt, 6))
		}
		if missed := run.notRun(names); len(missed) > 0 {
			// The ledger names them, the runner did not run them: they are
			// where it does not look, and judge nothing there.
			return c, fmt.Errorf("%s: the estate's own runner (%s) did not run %s — they are where it does not look; it said: %s",
				u.id, prof.name, strings.Join(missed, ", "), lastLines(run.Out, 3))
		}
		if !run.Reported && run.Passed && nothingRan(run.Out, names) {
			return c, fmt.Errorf("%s: the estate's own runner (%s) ran none of the tests %s — they are where it does not look; it said: %s",
				u.id, shown(prof.only(names)), strings.Join(names, ", "), lastLines(run.Out, 3))
		}
		passed := len(run.failed(names)) == 0
		if passed && len(on) > 0 {
			// They pass on the contract. What was asked holds already only
			// if they pass on the estate as it stood, without it; if not,
			// the contract does what it was only to declare.
			if bare, err := harness.Snapshot(r.repo, filepath.Join(c.store, "files")); err == nil {
				still := runTests(ctx, bare, prof, names)
				_ = os.RemoveAll(bare)
				if len(still.failed(names)) > 0 {
					return c, &behaves{tests: names}
				}
			}
		}
		if passed {
			return c, &holds{unit: u.id, tests: names}
		}
		r.say(4, c.ca, "the tests %s fail against the estate as it stands, as they must", strings.Join(names, ", "))
	}
	r.mu.Lock()
	r.cands[u.id] = filepath.Join(c.store, "files")
	if u.isTests() {
		r.ledger = append(slices.DeleteFunc(r.ledger, func(t meta.TestRecord) bool { return t.Unit == u.id }), c.tests...)
	}
	r.mu.Unlock()
	r.say(4, c.ca, "%s: %s — %s", c.ca, strings.Join(c.paths, ", "), res.Summary)
	return c, nil
}

// byAgent hands a unit to the executor's agent in ws, handed what its copy
// of the estate holds, and says what came of it.
func (r *Run) byAgent(ctx context.Context, i int, ws string, on []onto, scope []string, prof profile, ok bool, c carried) (harness.Result, error) {
	u := r.units[i]
	// What the agent is handed, and what it asks of the estate as it works,
	// is the copy it works in: the estate with exactly the candidates it is
	// carried on, and no others.
	here := r.cfg.Estate.Repo(r.repo).Over(ws)
	goal := r.goal(u)
	if u.isTests() {
		goal += ledgerAsk(u)
		scope = append(slices.Clone(scope), LedgerFile)
	}
	pack := r.pack(u, here, on, r.cfg.packBudget()-estate.Tokens(goal))
	handed := estate.Tokens(goal) + pack.Tokens()
	_ = os.WriteFile(filepath.Join(c.store, "pack.md"), []byte("# Goal\n\n"+goal+"\n\n"+pack.String()), 0o644)
	task := harness.Task{Workspace: ws, Goal: goal, Role: string(u.kind), Scope: scope, Context: pack.String(), Held: pack.Held().String(), BudgetUSD: r.left()}
	if !u.isTests() && !u.isContract() {
		task.Widen = func(path string) string { return r.widen(i, path) }
		task.Tests = r.criterionTests(i)
		if i == len(r.units)-1 {
			task.Tests = r.newTestNames(prof)
		}
	}
	switch {
	case ok && u.isTests() && prof.vet != nil:
		// Tests written first build against the contract, declared before
		// them: what they call is there, without its behaviour. They are
		// vetted — built, run none — and must build; and their ledger must
		// hold.
		task.Check = resolve(ws, prof.vet(u.scope))
		task.Judge = r.ledgerJudge(ws, u)
	case !ok || u.isTests():
		// Without a runner to vet with, tests written first are judged
		// afterwards, by failing against the estate as it stands.
	case i == len(r.units)-1:
		task.Check = resolve(ws, prof.suite) // the last unit completes the change
		// The change's own tests run first: seconds, where the whole suite
		// takes minutes, and most of what fails fails there.
		if names := r.newTestNames(prof); len(names) > 0 {
			task.First = resolve(ws, prof.only(names))
		}
	case len(r.criterionTests(i)) > 0:
		// The criteria it serves last are judged after it, by their own
		// tests: the estate builds, and those tests pass.
		task.First = resolve(ws, prof.build)
		task.Check = resolve(ws, prof.only(r.criterionTests(i)))
	case len(prof.build) > 0:
		task.Check = resolve(ws, prof.build) // an earlier one leaves the estate building
	}
	if !u.isTests() {
		provides := u.provides
		task.Unprovided = func(ws string) []string { return undeclared(ws, provides) }
	}
	task.Graph = estateGraph{repo: here, root: ws}
	// The language of what the unit writes reads its check's faults; the
	// estate's language says where its libraries can be read.
	if l, ok := scopeLanguage(scope); ok {
		task.Faults = faultsOf(l, scope)
	}
	if l, ok := languageOfEstate(r.repo); ok && l.libraries != nil {
		task.Libraries = l.libraries()
	}
	if ok && !u.isTests() && prof.quick != nil {
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
	r.say(4, u.id, "%s handed to %s: %d tokens, of %d an agent is handed at most", u.id, r.cfg.Executor.Name(), handed, r.cfg.packBudget())
	res, err := r.cfg.Executor.Run(model.Within(ctx, "unit "+u.id), task, func(e harness.Event) {
		r.mu.Lock()
		r.agent[u.id] = append(r.agent[u.id], e)
		r.mu.Unlock()
		r.changed()
	})
	r.spend(res.Usage)
	r.cost(artifact.Cost{Area: 4, Unit: u.id, What: "execution of " + u.id, Actor: r.cfg.Executor.Name() + " " + r.cfg.Executor.Version(),
		Turns: res.Turns, Steps: res.Steps, Handed: handed, Kept: "candidates/" + c.ca + "/pack.md", Note: res.Terminal}, res.Usage, res.Elapsed)
	r.writeTrace(c.store, u.id) // kept whether or not a candidate comes of it
	if err != nil {
		return res, fmt.Errorf("%s: the executor failed: %v", u.id, err)
	}
	if !u.isTests() && ok && (res.Terminal == harness.Exhausted || res.Terminal == harness.Terminated) {
		// A code unit that spent its steps while the change's own tests still
		// fail may be failing tests that ask for what cannot be: where they
		// fail is said, so that the tests are carried again with it, once.
		names := r.criterionTests(i)
		if i == len(r.units)-1 {
			names = r.newTestNames(prof)
		}
		if len(names) > 0 {
			run := runTests(ctx, ws, prof, names)
			if failed := run.failed(names); len(failed) > 0 && run.Reported {
				// The report names the tests that fail; the ledger, the files
				// they are in and the units that wrote them.
				res.Terminal, res.Blames = harness.FailedTask, r.ledgerFiles(failed)
				res.Summary = "its agent spent its steps and the change's own tests " + strings.Join(failed, ", ") + " in " + strings.Join(res.Blames, ", ") +
					" still fail — they may ask for what no code can give: " + failedSaid(run, failed, 6)
			}
		}
	}
	return res, nil
}

// pack is what one unit's agent is handed of the copy it works in, within
// budget tokens: the tests it must pass, whole; the files it may change and
// those the plan says it reads, whole, or by their outline where whole would
// not fit; around them only signatures and names; and, by path alone, what
// the units before it wrote, which it can read for itself.
func (r *Run) pack(u unit, here *estate.Repo, on []onto, budget int) estate.Pack {
	var read []string
	for _, f := range u.reads {
		if !slices.Contains(read, f) && !slices.Contains(u.scope, f) {
			read = append(read, f)
		}
	}
	var mustPass []estate.Section
	var before []string
	for _, o := range on {
		prev := r.units[o.unit]
		_ = filepath.WalkDir(o.files, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || d.Name() == "CHANGES" {
				return err
			}
			rel, _ := filepath.Rel(o.files, p)
			rel = filepath.ToSlash(rel)
			switch {
			case prev.isTests() && !u.isTests() && r.judges(u, prev.id, rel):
				// The tests of the criteria it serves, whole: the ledger says
				// which they are, and which of them must pass after it.
				if b, err := os.ReadFile(p); err == nil {
					title := "Judges the criteria you serve, written by " + prev.id + ": " + rel
					if must := r.mustPassIn(u, rel); len(must) > 0 {
						title = "Must pass after you — " + strings.Join(must, ", ") + " — written by " + prev.id + ": " + rel
					}
					mustPass = append(mustPass, estate.Section{Title: title, Body: "```\n" + string(b) + "```"})
					budget -= estate.Tokens(string(b))
				}
			case !slices.Contains(u.scope, rel) && !slices.Contains(read, rel):
				before = append(before, rel+"   ("+prev.id+")")
			}
			return nil
		})
	}
	p := here.PackFor(u.scope, read, budget)
	p.Sections = append(mustPass, p.Sections...)
	if u.isTests() {
		// The code the tests exercise, as the contract leaves it: how it is
		// called, by its declarations. Its bodies are not what a test is
		// written from; a file the plan says the unit reads is read whole.
		if d := here.Outlined(r.spec.Scope, budget/4); d != "" {
			p.Sections = append(p.Sections, estate.Section{Title: "The code these tests exercise, as declared — read a file whole where its body matters", Body: d})
		}
	}
	if len(before) > 0 {
		p.Sections = append(p.Sections, estate.Section{Title: "Written by the units before you — in your copy; read what you need",
			Body: strings.Join(before, "\n")})
	}
	return p
}

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
	// Every agent builds from the same decisions — the specification's own
	// words, not a retelling of them — so that the tests and the code, written
	// apart, agree: a detail one agent would otherwise invent and another
	// invent otherwise is decided once, here.
	if d := specDecisions(r.dir, u.isTests()); d != "" {
		brief += "\n\n## What the specification decides — build exactly this; a detail it decides is decided for every unit, and none may decide it otherwise\n\n" + d
	}
	// Every agent is handed the contract between the units — what each code
	// unit provides, exactly — so that the tests call what the code defines
	// and no unit defines what another provides.
	var contract []string
	for _, o := range r.units {
		if o.isContract() {
			continue // it declares what the code units provide: each is said once, by its provider
		}
		for _, p := range signatures(o.provides) {
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
	if why := r.againFor(u.id); u.isTests() && why != "" {
		brief += "\n\n## Why you are asked again\n\nTests written for this unit before were found wrong by the code that had to pass them; it could not, because:\n\n" + why +
			"\n\nWrite them again so that they call only what exists or the contract promises, and check only what the criteria say."
	}
	if r.mend != "" && !u.isTests() {
		brief += "\n\n## Why you are asked again\n\n" + r.cfg.Person + " read the change this unit made before it could land, and did not affirm it:\n\n" + r.mend +
			"\n\nMend exactly that; what they did not name stands as it was."
	}
	if u.isContract() {
		if why := r.againFor(u.id); why != "" {
			brief += "\n\n## Why you are asked again\n\nWhat you declared before did more than declare: " + why +
				". Declare again, with no behaviour at all — every function and method returns its zero values or fails as not yet done."
		}
		return brief + "\n\nDeclare only: no unit's behaviour is yours to write, and no test is written yet."
	}
	if u.isTests() {
		return brief + "\n\nWrite only the tests. Each judges whether its criterion's goal is reached — not how: any implementation that reaches it passes, and a test that another correct implementation would fail is wrong. " +
			"So judge the behaviour where it is met, through what a person or a caller of the estate uses that exists already — its command, its server, its public functions, what it shows or writes — and assert the observable outcome the criterion states, nothing the specification left open. " +
			"A helper's name, a private field, an internal order of calls is the code's to choose and never asserted. " +
			"Call a name of the contract below only where nothing that exists reaches the behaviour — a new public function the criterion is about — and leave it undefined: the code units define it, to pass your tests. " +
			"Fakes of what the code depends on (a mailer, a clock, a payment provider) belong in the tests; what the code itself provides never does. " +
			"The tests fail against the estate as it stands, if only because what they call is not there yet. Name each test after what it checks. " +
			"No test waits longer than five seconds for anything: wait on a channel or poll a condition with a short deadline, so that a failing test fails in seconds, not minutes."
	}
	return brief + "\n\nTests that decide the criteria were written first; do not change them."
}

// judges says whether file, written by the tests unit named, holds a test
// of a criterion u serves — for the last unit, any test.
func (r *Run) judges(u unit, by, file string) bool {
	last := len(r.units) > 0 && r.units[len(r.units)-1].id == u.id
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.ContainsFunc(r.ledger, func(t meta.TestRecord) bool {
		return t.Unit == by && t.File == file && (last || slices.Contains(u.criteria, t.Criterion))
	})
}

// mustPassIn is the tests in file that must pass after u: of the criteria
// it serves last — for the last unit, all of them.
func (r *Run) mustPassIn(u unit, file string) []string {
	i := slices.IndexFunc(r.units, func(x unit) bool { return x.id == u.id })
	last := i == len(r.units)-1
	crit := r.judgedAt(i)
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, t := range r.ledger {
		if t.File == file && (last || slices.Contains(crit, t.Criterion)) {
			out = append(out, t.Test)
		}
	}
	return out
}

// criterionTests is the tests the ledger says judge the criteria the code
// unit i serves last: what must pass after it.
func (r *Run) criterionTests(i int) []string {
	crit := r.judgedAt(i)
	if len(crit) == 0 {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var names []string
	for _, t := range r.ledger {
		if slices.Contains(crit, t.Criterion) {
			names = append(names, t.Test)
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// ledgerFiles is the files the ledger says the tests named are in.
func (r *Run) ledgerFiles(tests []string) []string {
	var files []string
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.ledger {
		if slices.Contains(tests, t.Test) {
			files = append(files, t.File)
		}
	}
	slices.Sort(files)
	return slices.Compact(files)
}

// failedSaid is what the first n of the tests named that failed said, as
// the report holds it, each by its name.
func failedSaid(run testRun, failed []string, n int) string {
	if run.Unbuilt != "" {
		return "they do not build: " + lastLines(run.Unbuilt, 6)
	}
	var out []string
	for _, name := range failed[:min(n, len(failed))] {
		o, ok := run.Results[name]
		if !ok {
			out = append(out, name+": did not run")
			continue
		}
		out = append(out, name+": "+lastLines(strings.TrimSpace(o.Output), 4))
	}
	return strings.Join(out, "\n")
}

// specDecisions is what the specification of the change in dir decides that
// every unit builds from: its requirements, its design, its edge cases —
// and, for the tests, its test strategy — as change.md states them.
func specDecisions(dir string, tests bool) string {
	b, err := os.ReadFile(filepath.Join(dir, "change.md"))
	if err != nil {
		return ""
	}
	want := []string{"Functional requirements", "Non-functional requirements", "Data model", "Interfaces", "Existing names it builds on", "Components and files", "Edge cases and errors"}
	if tests {
		want = append(want, "Test strategy")
	}
	var out []string
	for _, sec := range strings.Split("\n"+string(b), "\n#") {
		heading, body, _ := strings.Cut(sec, "\n")
		title := strings.TrimSpace(strings.TrimLeft(heading, "#"))
		if slices.Contains(want, title) && strings.TrimSpace(body) != "" {
			out = append(out, "### "+title+"\n\n"+strings.TrimSpace(body))
		}
	}
	return strings.Join(out, "\n\n")
}

// chains divides the code units from to to (to not included) into chains
// that can be carried at once: units that change a file in common are in
// one chain, in the plan's order; without a contract declared first, so is
// a unit and one whose names it uses.
func (r *Run) chains(from, to int) [][]int {
	if to-from < 2 {
		return nil
	}
	group := make([]int, len(r.units))
	for i := range group {
		group[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if group[i] != i {
			group[i] = find(group[i])
		}
		return group[i]
	}
	contract := len(r.units) > 0 && r.units[0].isContract()
	for i := from; i < to; i++ {
		for j := from; j < i; j++ {
			shared := slices.ContainsFunc(r.units[i].scope, func(f string) bool { return slices.Contains(r.units[j].scope, f) })
			// Without a contract declared first, a unit that uses another's
			// names waits for it.
			uses := !contract && slices.ContainsFunc(r.units[i].uses, r.units[j].gives)
			if shared || uses {
				group[find(i)] = find(j)
			}
		}
	}
	byRoot := map[int][]int{}
	var roots []int
	for i := from; i < to; i++ {
		root := find(i)
		if _, seen := byRoot[root]; !seen {
			roots = append(roots, root)
		}
		byRoot[root] = append(byRoot[root], i)
	}
	var out [][]int
	for _, root := range roots {
		out = append(out, byRoot[root])
	}
	return out
}

// newTestNames is every test the tests units of the change wrote, as the
// test ledger holds them.
func (r *Run) newTestNames(prof profile) []string {
	return r.ledgerTests()
}

// writeTrace keeps every step the executor took, with what each model turn
// cost, beside the candidate it produced.
func (r *Run) writeTrace(store, unit string) {
	r.mu.Lock()
	events := slices.Clone(r.agent[unit])
	r.mu.Unlock()
	_ = harness.WriteTrace(filepath.Join(store, harness.TraceFile), events)
}

// widen is why the code unit i may not be given path beyond its scope, or
// "" where it may: a file of the specification's scope that nothing judges,
// and that no other unit writes unless it was carried before this one —
// what a unit carried beside it holds, it is not given.
func (r *Run) widen(i int, path string) string {
	switch {
	case !inScope(path, r.spec.Scope):
		return "it is outside the specification's scope; name it with cannot, and the specification goes back to have it"
	case slices.Contains(r.spec.NewTests, path) || slices.Contains(r.spec.SuiteTests, path) || r.declaresTests(path) || slices.ContainsFunc(r.ledgerOfAll(), func(t meta.TestRecord) bool { return t.File == path }):
		return "it judges the change, and what judges a change is never changed by it"
	}
	before := ancestors(r.dependencies(), i)
	for j, u := range r.units {
		if j != i && inScope(path, u.scope) && !before[j] {
			return fmt.Sprintf("%s writes it, carried apart from this unit", u.id)
		}
	}
	return ""
}

// ancestors is every unit i waits for, near and far.
func ancestors(deps [][]int, i int) map[int]bool {
	seen := map[int]bool{}
	var walk func(int)
	walk = func(j int) {
		for _, d := range deps[j] {
			if !seen[d] {
				seen[d] = true
				walk(d)
			}
		}
	}
	walk(i)
	return seen
}
