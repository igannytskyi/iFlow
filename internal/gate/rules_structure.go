package gate

import (
	"os"
	"path/filepath"

	a "github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/meta"
)

// stageOrder — B1: the stages are written in order, from the intent.
func stageOrder(r *run) {
	present := r.c.Present()
	if len(present) == 0 {
		r.fail("B1", "no stage files found")
		return
	}
	if present[0] != a.Intent {
		r.fail("B1", "%s present without 00-intent", present[0])
	}
	for i := 1; i < len(present); i++ {
		if present[i].Index() != present[i-1].Index()+1 {
			r.fail("B1", "%s present but %s is missing", present[i], a.Stages[present[i].Index()-1])
		}
	}
}

// readable — B1: every document reads into the data model whole. A table or
// a section a document cannot place, or JSON that does not decode, is a
// document the checks after this one would judge in part.
func readable(r *run) {
	for _, p := range r.c.Meta.Problems {
		r.fail("B1", "%s", p)
	}
}

// vocabulary — every code holds one of its defined values, every identifier
// has its defined form: the data model, checked.
func vocabulary(r *run) {
	m := r.c.Meta
	for _, d := range []struct {
		stage a.Stage
		doc   any
		ok    bool
	}{
		{a.Intent, m.Intent, m.Intent != nil}, {a.Specification, m.Specification, m.Specification != nil},
		{a.Plan, m.Plan, m.Plan != nil}, {a.Admission, m.Admission, m.Admission != nil},
		{a.Execution, m.Execution, m.Execution != nil}, {a.Assurance, m.Assurance, m.Assurance != nil},
		{a.Landing, m.Landing, m.Landing != nil},
	} {
		if !d.ok {
			continue
		}
		for _, p := range meta.Validate(d.doc) {
			r.fail("CONVENTIONS", "%s: %s", d.stage, p)
		}
	}
}

// recordComplete — B4: every stage present has its crossing recorded.
func recordComplete(r *run) {
	if len(r.c.Meta.Record) == 0 {
		r.fail("B4", "no record — crossings are not recorded")
		return
	}
	logged := map[string]bool{}
	for _, c := range r.c.Meta.Record {
		logged[c.Stage] = true
	}
	for _, s := range r.c.Present() {
		if !logged[s.Name()] {
			r.fail("B4", "stage %s present but not recorded", s.Name())
		}
	}
}

// recordAppendOnly — R4: ordinals form 1..N, in order, with no gap and no
// repeat. A line removed from the tail is not detected: that needs a digest
// chain the record does not yet carry, and rules.md says so.
func recordAppendOnly(r *run) {
	var seen []int
	for _, c := range r.c.Meta.Record {
		seen = append(seen, c.N)
	}
	for i, n := range seen {
		if n != i+1 {
			r.fail("R4", "the record is not append-only: ordinals %s are not 1..%d in order", intList(seen), len(seen))
			return
		}
	}
}

// handedKept — R23: model work says what it was handed, and what it was
// handed is kept beside the change.
func handedKept(r *run) {
	costs, err := a.Costs(r.c.Dir)
	if err != nil {
		r.fail("R23", "costs.jsonl does not read: %v", err)
		return
	}
	unmeasured := 0
	for _, c := range costs {
		if c.Input+c.CacheRead+c.CacheWrite+c.Output == 0 {
			continue // no model read or wrote anything: a test run, the gate
		}
		if c.Kept != "" && !kept(r.c.Dir, c.Kept) {
			r.fail("R23", "%q says what it was handed is kept at %s, and nothing is there", c.What, c.Kept)
		}
		if c.Handed == 0 {
			unmeasured++
		}
	}
	if unmeasured > 0 {
		r.note("R23", "%d cost record(s) of model work say nothing of what the model was handed — work measured before handovers were", unmeasured)
	}
}

// kept reports whether what was handed is kept where its cost says — or
// under superseded/, where a change carried again keeps what the areas it
// carried again had made.
func kept(dir, at string) bool {
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(at))); err == nil {
		return true
	}
	again, _ := filepath.Glob(filepath.Join(dir, "superseded", "*", filepath.FromSlash(at)))
	return len(again) > 0
}
