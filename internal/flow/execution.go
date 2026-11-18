package flow

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/estate"
	"github.com/igannytskyi/iflow/internal/harness"
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

type replan struct{ unit, why string }

func (e *replan) Error() string { return e.unit + " cannot be done as it was planned: " + e.why }

// blamed is the tests unit at the head of the plan whose files a code
// unit's failure blames, or -1.
func (r *Run) blamed(c carried, results []carried) int {
	if c.res.Terminal != harness.FailedTask {
		return -1
	}
	// By a file the unit's agent or its check named: its path, or, said
	// without its folder, its name where only one tests unit wrote a file
	// of that name.
	for _, blamed := range c.res.Blames {
		blamed = filepath.ToSlash(filepath.Clean(blamed))
		byName := -1
		for j, u := range r.units {
			if !u.isTests() {
				continue
			}
			for _, p := range results[j].paths {
				switch {
				case p == blamed:
					return j
				case path.Base(p) == blamed && byName == -1:
					byName = j
				case path.Base(p) == blamed:
					byName = -2 // the name is not one file's
				}
			}
		}
		if byName >= 0 {
			return byName
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
	if i == len(r.units)-1 && !u.isTests() {
		// The last unit completes the change: the whole suite must pass
		// after it, and what keeps it from passing may be in any code the
		// change writes. It may mend all of that code — never the tests.
		scope = r.codeScope()
	}
	task := harness.Task{Workspace: ws, Goal: r.goal(u), Role: string(u.kind), Scope: scope, Context: pack.String(), BudgetUSD: r.cfg.BudgetUSD}
	switch {
	case ok && u.isTests() && prof.vet != nil:
		// Tests written first build against the contract, declared before
		// them: what they call is there, without its behaviour. They are
		// vetted — built, run none — and must build.
		task.Check = resolve(ws, prof.vet(u.scope))
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
	case len(prof.build) > 0:
		task.Check = resolve(ws, prof.build) // an earlier one leaves the estate building
	}
	if !u.isTests() {
		task.Provides = u.names()
	}
	task.Graph = estateGraph{repo: r.cfg.Estate.Repo(r.repo), root: r.repo}
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
	if !u.isTests() && ok && (res.Terminal == harness.Exhausted || res.Terminal == harness.Terminated) {
		// A code unit that spent its steps while the change's own tests still
		// fail may be failing tests that ask for what cannot be: where they
		// fail is said, so that the tests are carried again with it, once.
		if names := r.newTestNames(prof); len(names) > 0 {
			if passed, out, _ := runIn(ctx, ws, resolve(ws, prof.only(names))); !passed {
				if at := testFaults(out); at != "" {
					res.Terminal, res.Blames = harness.FailedTask, r.failingTestFiles(out)
					res.Summary = "its agent spent its steps and the change's own tests in " + strings.Join(res.Blames, ", ") +
						" still fail — they may ask for what no code can give: " + at
					c.res = res
				}
			}
		}
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
	// A contract with nothing to declare — every name it lists is there
	// already — changes nothing, and that is its answer.
	if res.Terminal != harness.Candidate || (len(changes) == 0 && !u.isContract()) {
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
		names := r.testNames(c.store)
		if len(names) == 0 {
			return c, fmt.Errorf("%s wrote no test this estate's runner can name", u.id)
		}
		passed, out, took := runIn(ctx, ws, resolve(ws, prof.only(names)))
		r.cost(artifact.Cost{Area: 4, Unit: u.id, What: "the tests run against the estate as it stands", Actor: prof.name}, model.Usage{}, took)
		_ = os.WriteFile(filepath.Join(c.store, "failing-before.txt"), []byte(out), 0o644)
		if passed && nothingRan(out, names) {
			// Not one of them was run: the estate's own runner does not look
			// where they were written. That they pass says nothing.
			return c, fmt.Errorf("%s: the estate's own runner (%s) ran none of the tests %s — they are where it does not look; it said: %s",
				u.id, shown(prof.only(names)), strings.Join(names, ", "), lastLines(out, 3))
		}
		if passed && len(overlays) > 0 {
			// They pass on the contract. What was asked holds already only
			// if they pass on the estate as it stood, without it; if not,
			// the contract does what it was only to declare.
			if bare, err := harness.Snapshot(r.repo, filepath.Join(c.store, "files")); err == nil {
				still, _, _ := runIn(ctx, bare, resolve(bare, prof.only(names)))
				_ = os.RemoveAll(bare)
				if !still {
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
	if u.isTests() {
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
		if prev.id == u.id {
			break
		}
		if u.isTests() && !prev.isContract() {
			continue // tests units rely on nothing another unit writes but the contract
		}
		dir := r.cands[prev.id]
		_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || d.Name() == "CHANGES" {
				return err
			}
			rel, _ := filepath.Rel(dir, p)
			if b, err := os.ReadFile(p); err == nil {
				title := "Written by " + prev.id + ", which comes before you: " + filepath.ToSlash(rel)
				if prev.isTests() {
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
		return brief + "\n\nWrite only the tests. Assert what the criteria and the specification's decisions state, and nothing they leave open: a test that checks a detail the specification did not decide is a guess the code cannot know. The behaviour does not exist yet, and neither does the code that will provide it: " +
			"call it by the names the brief gives, and leave it undefined — the code units define it, to pass your tests. " +
			"Fakes of what the code depends on (a mailer, a clock, a payment provider) belong in the tests; what the code itself provides never does. " +
			"The tests fail against the estate as it stands, if only because what they call is not there yet. Name each test after what it checks. " +
			"No test waits longer than five seconds for anything: wait on a channel or poll a condition with a short deadline, so that a failing test fails in seconds, not minutes."
	}
	return brief + "\n\nTests that decide the criteria were written first; do not change them."
}

// failingTestFiles is the files of the tests units whose tests a run's
// output says failed, each by its path.
func (r *Run) failingTestFiles(out string) []string {
	failed := map[string]bool{}
	for _, n := range failedTests(out) {
		failed[n] = true
	}
	var files []string
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.units {
		dir, ok := r.cands[u.id]
		if !u.isTests() || !ok {
			continue
		}
		for _, n := range r.testNames(filepath.Dir(dir)) {
			if failed[n] {
				files = append(files, u.scope...)
				break
			}
		}
	}
	return files
}

// specDecisions is what the specification of the change in dir decides that
// every unit builds from: its requirements, its design, its edge cases —
// and, for the tests, its test strategy — as change.md states them.
func specDecisions(dir string, tests bool) string {
	b, err := os.ReadFile(filepath.Join(dir, "change.md"))
	if err != nil {
		return ""
	}
	want := []string{"Functional requirements", "Non-functional requirements", "Data model", "Interfaces", "Components and files", "Edge cases and errors"}
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
			uses := !contract && slices.ContainsFunc(r.units[i].uses, func(n string) bool { return slices.Contains(r.units[j].names(), n) })
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

// nothingRan says a run of the tests named shows that none of them ran:
// none is named in what it said, and it says nothing was run — Go's "no
// tests to run", unittest's "Ran 0 tests", pytest's "no tests ran", node's
// "tests 0".
func nothingRan(out string, names []string) bool {
	for _, n := range names {
		if strings.Contains(out, n) {
			return false
		}
	}
	return notRun.MatchString(out)
}

var notRun = regexp.MustCompile(`(?m)\[no tests to run\]|\[no test files\]|^Ran 0 tests|no tests ran|^\S*\s*tests 0\s*$`)

// newTestNames is every test the tests units of the change wrote.
func (r *Run) newTestNames(prof profile) []string {
	var names []string
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.units {
		if dir, ok := r.cands[u.id]; ok && u.isTests() {
			names = append(names, r.testNames(filepath.Dir(dir))...)
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

func (r *Run) testNames(store string) []string {
	var names []string
	_ = filepath.WalkDir(filepath.Join(store, "files"), func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if b, err := os.ReadFile(p); err == nil {
				names = append(names, testsIn(p, string(b))...)
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
