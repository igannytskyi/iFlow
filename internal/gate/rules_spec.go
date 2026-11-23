package gate

import (
	"path/filepath"
	"slices"
	"strings"

	a "github.com/igannytskyi/iflow/internal/artifact"
)

// criteriaVocabulary — R19: a criterion naming something the change may touch
// can be met by editing that thing. The probe is crude — a path appearing in
// the text — and crude is enough, because the mistake it catches is not
// subtle.
func criteriaVocabulary(r *run) {
	touchable := slices.DeleteFunc(scope(r.c), func(p string) bool {
		return p == "" || !strings.ContainsAny(p, "/.")
	})
	if len(touchable) == 0 {
		return
	}
	for _, c := range r.c.Meta.Specification.Criteria {
		said := c.Criterion + " " + c.When + " " + c.Then
		if i := slices.IndexFunc(touchable, func(p string) bool { return strings.Contains(said, p) }); i >= 0 {
			r.fail("R19", "%s names %s, which the change may touch: it can be met by editing what it names", or(c.ID), touchable[i])
		}
	}
}

// arbiterInputs — R7b: an arbiter input inside the scope needs its prior
// state captured.
func arbiterInputs(r *run) {
	// The prior state is captured at admission: a change that has not
	// reached it owes none yet.
	if !r.has(a.Specification) || !r.has(a.Admission) {
		return
	}
	reads := newOrdered[string]()
	for _, in := range r.c.Meta.Specification.Inputs {
		reads.put(in.Input, in.InScope)
	}
	if reads.len() == 0 {
		r.note("R7b", "this specification declares no arbiter inputs — it predates the obligation rather than violating it")
		return
	}
	var captured strings.Builder
	for _, p := range r.c.Meta.Admission.Prior {
		captured.WriteString(strings.Join(nonEmpty([]string{p.Criterion, p.Artefact, p.CapturedTo}), " ") + " ")
	}
	for _, in := range reads.keys {
		if strings.HasPrefix(reads.vals[in], "yes") && !strings.Contains(captured.String(), in) {
			r.fail("R7b", "%s is read by the arbiter and inside the scope, with no prior state captured at admission", in)
		}
	}
}

// everyCriterionArbitrated — R11: a criterion nothing claims to arbitrate is
// not being tested.
func everyCriterionArbitrated(r *run) {
	var claims []string
	if r.c.Meta.Specification != nil {
		for _, a := range r.c.Meta.Specification.Arbiters {
			if a.Path != "" {
				claims = append(claims, a.Arbitrates)
			}
		}
	}
	claimed := strings.Join(claims, " ")
	for _, crit := range criteria(r.c) {
		if !strings.Contains(claimed, crit) {
			r.fail("R11", "%s is claimed by no arbiter, so nothing tests it", crit)
		}
	}
}

// scopeArbiterDisjoint — R7: what judges a change lies outside what it may
// touch.
func scopeArbiterDisjoint(r *run) {
	included, judges := scope(r.c), arbiter(r.c)
	overlap := slices.DeleteFunc(slices.Clone(included), func(p string) bool { return !slices.Contains(judges, p) })
	if len(overlap) > 0 {
		r.fail("R7", "scope intersects the arbiter: %s", quoteList(overlap))
	}
	if len(included) > 0 && len(judges) == 0 {
		r.note("R7", "no arbiter paths declared — nothing to be disjoint from")
	}
}

// specImmutable — R1. Immutability protects criteria from being shaped by
// what execution produced. Before a candidate exists there is nothing to be
// shaped by, so an edit then is a re-admission — free, but never silent.
func specImmutable(r *run) {
	if !r.has(a.Admission) || !r.has(a.Specification) {
		return
	}
	recorded := r.c.Meta.Admission.Digest
	if recorded == "" {
		r.fail("R1", "no specification digest recorded at admission")
		return
	}
	actual := r.c.SpecDigest
	switch {
	case recorded == actual:
	case recorded == r.c.LegacySpecDigest && migratedAs(r.c, actual):
		// Unchanged from admission to migration — the Markdown kept under
		// legacy/ is what was admitted — and since: the JSON is what the
		// migration wrote.
	case len(candidates(r.c)) > 0:
		r.fail("R1", "specification changed after a candidate existed (recorded %s, now %s)", recorded, actual)
	case readmissionRecorded(r.c):
		r.note("R1", "the specification differs from the digest taken at admission and no candidate exists — a re-admission, recorded")
	default:
		r.fail("R1", "specification changed with no candidate and no record of a re-admission (recorded %s, now %s)", recorded, actual)
	}
}

// candidatesStored — R2: a candidate exists apart from the live tree.
func candidatesStored(r *run) {
	if r.c.Meta.Execution == nil {
		return
	}
	for _, ca := range r.c.Meta.Execution.Candidates {
		id := or(ca.ID)
		switch where := ca.StoredAt; {
		case where == "":
			r.fail("R2", "%s names no store", id)
		case !exists(filepath.Join(r.c.Dir, where)):
			r.fail("R2", "%s is not stored at %s", id, where)
		}
	}
}

// arbiterChangesSeparate — R8: only a unit that is preparatory for another
// may touch the arbiter.
func arbiterChangesSeparate(r *run) {
	if !r.has(a.Execution) || !r.has(a.Specification) {
		return
	}
	judges := nonEmpty(arbiter(r.c))
	if len(judges) == 0 {
		return
	}
	preparatory := map[string]bool{}
	if r.c.Meta.Plan != nil {
		for _, u := range r.c.Meta.Plan.Units {
			if len(u.PreparatoryFor) > 0 {
				preparatory[u.ID] = true
			}
		}
	}
	for _, ca := range r.c.Meta.Execution.Candidates {
		i := slices.IndexFunc(judges, func(p string) bool { return strings.Contains(ca.Artefacts, p) })
		if i >= 0 && !preparatory[ca.Unit] {
			r.fail("R8", "%s touches the arbiter (%s) and is nobody's preparatory work", or(ca.Unit), judges[i])
		}
	}
}
