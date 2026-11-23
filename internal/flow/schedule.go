package flow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/meta"
)

// Area 4 is a graph of units, carried by one scheduler. Each unit waits
// for the units it depends on — computed once, from what each writes and
// shares (dependencies) — and is carried as soon as they are done, at once
// with every other unit that is ready. What comes of a unit is one of a few
// outcomes, and what each leads to is the table in next:
//
//	outcome          what follows
//	candidate        it is kept, and the units waiting on it may go
//	holds            the change asks for what holds already: area 4 ends
//	contract acts    the contract is declared again, once, with why, and the tests written again
//	tests wrong      the tests unit blamed is written again, once, with why, and the unit carried again;
//	                 wrong again, the specification they were written from goes back to its author, once
//	cannot           the plan goes back to its planner, once; where what
//	                 must change is outside the specification's scope, the
//	                 specification goes back to its author, once
//	failed           area 4 stops, saying why
//
// A unit sent to be carried again while it is still being carried is
// carried again once that carrying ends; what that carrying produced is
// set aside, never kept.

// outcome is what came of carrying a unit.
type outcome int

const (
	outCandidate outcome = iota
	outHolds
	outContractActs
	outTestsWrong
	outCannot
	outSentBack
	outFailed
)

// unitState is where a unit stands in area 4.
type unitState int

const (
	waiting unitState = iota
	running
	done
)

// dependencies is, for every unit, the units it waits for: the contract
// first; the tests on the contract; a code unit on the contract, the
// tests, the unit before it in its chain, and every earlier code unit
// whose files it reads or whose names it calls; the last unit on all.
func (r *Run) dependencies() [][]int {
	deps := make([][]int, len(r.units))
	var contract, tests []int
	for i, u := range r.units {
		switch u.kind {
		case meta.ContractUnit:
			contract = append(contract, i)
		case meta.TestsUnit:
			tests = append(tests, i)
		}
	}
	first := len(contract) + len(tests)
	last := len(r.units) - 1
	for _, i := range tests {
		deps[i] = slices.Clone(contract)
	}
	for i := first; i < last; i++ {
		deps[i] = append(slices.Clone(contract), tests...)
	}
	for _, chain := range r.chains(first, last) {
		for k := 1; k < len(chain); k++ {
			deps[chain[k]] = append(deps[chain[k]], chain[k-1])
		}
	}
	// A code unit that reads a file an earlier one writes, or calls a name
	// it gives, waits for it: the contract declares the name, but only the
	// earlier unit's candidate gives it behaviour, and the tests the later
	// unit must pass ask for that behaviour.
	for i := first; i < last; i++ {
		for j := first; j < i; j++ {
			if slices.Contains(deps[i], j) {
				continue
			}
			reads := slices.ContainsFunc(r.units[i].reads, func(f string) bool { return slices.Contains(r.units[j].scope, f) })
			uses := slices.ContainsFunc(r.units[i].uses, func(n string) bool { return slices.Contains(r.units[j].names(), n) })
			if reads || uses {
				deps[i] = append(deps[i], j)
			}
		}
	}
	if last >= first {
		for i := range last {
			deps[last] = append(deps[last], i)
		}
	}
	return deps
}

// finished is one carrying ended: which unit, in which of its carryings.
type finished struct {
	unit, generation int
	carried          carried
	err              error
}

// scheduler carries the plan's units through area 4.
type scheduler struct {
	r       *Run
	prof    profile
	ok      bool
	deps    [][]int
	state   []unitState
	gen     []int // a unit's carrying, raised whenever it is sent to be carried again
	stale   []bool
	results []carried
	doc     meta.Execution
	ends    chan finished
	running int
	// again is the units already sent to be carried again for a reason
	// that may send each once.
	again map[int]bool
}

func (r *Run) execute(ctx context.Context) error {
	r.set(4, Working)
	prof, ok := detect(r.repo)
	n := len(r.units)
	s := &scheduler{r: r, prof: prof, ok: ok, deps: r.dependencies(), state: make([]unitState, n), gen: make([]int, n),
		stale: make([]bool, n), results: make([]carried, n), ends: make(chan finished), again: map[int]bool{},
		doc: meta.Execution{Schema: meta.SchemaVersion, Name: r.slug}}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := func(err error) error {
		cancel()
		for s.running > 0 { // nothing is left writing once area 4 has ended
			<-s.ends
			s.running--
		}
		return err
	}
	for {
		s.startReady(ctx)
		if s.running == 0 {
			break
		}
		f := <-s.ends
		s.running--
		if err := s.next(ctx, f); err != nil {
			return stop(err)
		}
	}
	if i := slices.Index(s.state, waiting); i >= 0 {
		return r.fail(4, "%s could not be carried: what it waits for did not come", r.units[i].id)
	}
	if rep, err := r.check(ctx, 4); err != nil || !rep.Passed() {
		return r.fail(4, "the execution does not pass the gate")
	}
	r.set(4, Done)
	return nil
}

// startReady carries every unit that waits on nothing left undone.
func (s *scheduler) startReady(ctx context.Context) {
	for i := range s.r.units {
		if s.state[i] != waiting || !s.ready(i) {
			continue
		}
		s.state[i], s.running = running, s.running+1
		overlays, gen := s.overlays(i), s.gen[i]
		go func() {
			c, err := s.r.carry(ctx, i, overlays, s.prof, s.ok)
			s.ends <- finished{i, gen, c, err}
		}()
	}
}

func (s *scheduler) ready(i int) bool {
	return !slices.ContainsFunc(s.deps[i], func(d int) bool { return s.state[d] != done })
}

// onto is one candidate a unit is carried on: the unit that made it, and
// where its files are kept.
type onto struct {
	unit  int
	files string
}

// overlays is the candidates a unit is carried on: those of every unit it
// waits for, near and far, in the plan's order.
func (s *scheduler) overlays(i int) []onto {
	under := map[int]bool{}
	var walk func(int)
	walk = func(j int) {
		for _, d := range s.deps[j] {
			if !under[d] {
				under[d] = true
				walk(d)
			}
		}
	}
	walk(i)
	var out []onto
	for j := range s.r.units {
		if under[j] && s.results[j].store != "" {
			out = append(out, onto{j, filepath.Join(s.results[j].store, "files")})
		}
	}
	return out
}

// next applies the table: what follows from a carrying that ended.
func (s *scheduler) next(ctx context.Context, f finished) error {
	r, i := s.r, f.unit
	if f.generation != s.gen[i] || s.stale[i] {
		// Sent to be carried again while it was carried: what it made is
		// set aside, and it is carried again.
		s.stale[i], s.state[i] = false, waiting
		return nil
	}
	s.results[i], s.state[i] = f.carried, waiting // ended; what follows says where it stands
	switch out, blamed := s.outcomeOf(i, f); out {
	case outCandidate:
		s.state[i] = done
		return s.keep(i)
	case outHolds:
		var h *holds
		errors.As(f.err, &h)
		return h
	case outContractActs:
		c := slices.IndexFunc(r.units, unit.isContract)
		if c < 0 || s.again[c] {
			return r.fail(4, "%v", f.err)
		}
		s.again[c] = true
		r.say(4, r.units[c].id, "%s is carried again: %v", r.units[c].id, f.err)
		r.carriedAgainFor(r.units[c].id, f.err.Error())
		s.redo(c)
		for t, u := range r.units {
			if u.isTests() {
				s.redo(t)
			}
		}
		return nil
	case outTestsWrong:
		if s.again[blamed] && !r.back[1] {
			// Written again and still asking for what the code cannot give:
			// what they were written from is at fault, not how.
			return &sentBack{area: 1, unit: r.units[blamed].id, why: fmt.Sprintf(
				"the tests %s wrote, written twice from the specification, still ask for what %s cannot give: %s",
				r.units[blamed].id, r.units[i].id, f.carried.res.Summary)}
		}
		if s.again[blamed] {
			return r.fail(4, "%v", f.err)
		}
		s.again[blamed] = true
		r.say(4, r.units[blamed].id, "%s is carried again: %s found its tests wrong", r.units[blamed].id, r.units[i].id)
		r.carriedAgainFor(r.units[blamed].id, f.carried.res.Summary)
		s.redo(blamed)
		s.redo(i)
		return nil
	case outSentBack:
		var sb *sentBack
		errors.As(f.err, &sb)
		return sb
	case outCannot:
		r.say(4, r.units[i].id, "%s cannot be done as it was planned: %s", r.units[i].id, f.carried.res.Cannot)
		if out := r.outsideScope(f.carried.res.Blames); len(out) > 0 && !r.back[1] {
			// What must change lies outside what the specification lets the
			// change touch: no plan can mend that, its scope must.
			return &sentBack{area: 1, unit: r.units[i].id, why: fmt.Sprintf(
				"it needs %s changed, which the specification's scope does not hold: %s",
				strings.Join(out, ", "), f.carried.res.Cannot)}
		}
		return &sentBack{area: 2, unit: r.units[i].id, why: "it cannot be done as it was planned: " + f.carried.res.Cannot}
	default:
		return r.fail(4, "%v", f.err)
	}
}

// outcomeOf is what came of a carrying, and, where its tests are wrong,
// which tests unit wrote them.
func (s *scheduler) outcomeOf(i int, f finished) (outcome, int) {
	var h *holds
	var b *behaves
	var sb *sentBack
	switch {
	case f.err == nil:
		return outCandidate, -1
	case errors.As(f.err, &sb):
		return outSentBack, -1
	case errors.As(f.err, &h):
		return outHolds, -1
	case errors.As(f.err, &b):
		return outContractActs, -1
	}
	if j := s.r.blamed(f.carried, s.results); j >= 0 {
		return outTestsWrong, j
	}
	// A unit that cannot be done goes back to the step at fault while that
	// step has not been gone back to: the specification, where what must
	// change is outside its scope; the plan, where not.
	if c := f.carried.res; c.Cannot != "" && (!s.r.back[2] || (!s.r.back[1] && len(s.r.outsideScope(c.Blames)) > 0)) {
		return outCannot, -1
	}
	return outFailed, -1
}

// redo sends a unit to be carried again: now, if it is not being carried;
// once its carrying ends, if it is.
func (s *scheduler) redo(i int) {
	s.gen[i]++
	switch s.state[i] {
	case running:
		s.stale[i] = true
	default:
		if s.results[i].store != "" {
			_ = os.RemoveAll(s.results[i].store)
		}
		s.results[i], s.state[i] = carried{}, waiting
	}
}

// keep records a unit's candidate: in the execution document, the last
// carrying of a unit standing for it, and as a crossing of the record.
func (s *scheduler) keep(i int) error {
	r, c, u := s.r, s.results[i], s.r.units[i]
	run := meta.Run{Unit: u.id, Executor: r.cfg.Executor.Name(), Version: r.cfg.Executor.Version(), Terminal: meta.Terminal(c.res.Terminal), Candidate: c.ca}
	cand := meta.Candidate{ID: c.ca, Unit: u.id, StoredAt: "candidates/" + c.ca + "/", Artefacts: strings.Join(c.paths, ", "), ProducedAt: today()}
	if j := slices.IndexFunc(s.doc.Runs, func(x meta.Run) bool { return x.Unit == u.id }); j >= 0 {
		s.doc.Runs[j], s.doc.Candidates[j] = run, cand
	} else {
		s.doc.Runs, s.doc.Candidates = append(s.doc.Runs, run), append(s.doc.Candidates, cand)
	}
	if err := artifact.Save(r.dir, artifact.Execution, s.doc); err != nil {
		return r.fail(4, "the execution could not be written: %v", err)
	}
	return r.record("execution", c.ca, c.res.Summary)
}

// carriedAgainFor keeps why a unit is carried again, for its agent's brief.
func (r *Run) carriedAgainFor(unit, why string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.again == nil {
		r.again = map[string]string{}
	}
	r.again[unit] = why
}

// againFor is why a unit is carried again, if it is.
func (r *Run) againFor(unit string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.again[unit]
}

// outsideScope is those of files, named by an agent as keeping its unit
// from being done, that exist, judge nothing, and lie outside the
// specification's scope: what the change must touch and may not.
func (r *Run) outsideScope(files []string) []string {
	var out []string
	for _, f := range files {
		f = filepath.ToSlash(filepath.Clean(strings.TrimSpace(f)))
		if f == "." || inScope(f, r.spec.Scope) || slices.Contains(r.spec.NewTests, f) || slices.Contains(r.spec.SuiteTests, f) || r.declaresTests(f) {
			continue
		}
		if _, err := os.Stat(filepath.Join(r.repo, f)); err == nil && !slices.Contains(out, f) {
			out = append(out, f)
		}
	}
	return out
}

// declaresTests says whether file, in the estate, declares a test in its
// language's own form: a file that judges, which no scope holds.
func (r *Run) declaresTests(file string) bool {
	l, ok := languageOfFile(file)
	if !ok || l.tests == nil {
		return false
	}
	b, err := os.ReadFile(filepath.Join(r.repo, file))
	return err == nil && l.tests.Match(b)
}
