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
	for _, row := range r.doc(a.Specification).Rows("Criterion", "Procedure") {
		said := row.Cell("Criterion") + " " + row.Cell("When") + " " + row.Cell("Then")
		if i := slices.IndexFunc(touchable, func(p string) bool { return strings.Contains(said, p) }); i >= 0 {
			r.fail("R19", "%s names %s, which the change may touch: it can be met by editing what it names",
				orNone(row.Get("Id")), touchable[i])
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
	for _, row := range r.doc(a.Specification).Rows("Input", "In scope?") {
		reads.put(row.Cell("Input"), row.Cell("In scope?"))
	}
	if reads.len() == 0 {
		r.note("R7b", "this specification declares no arbiter inputs — it predates the obligation rather than violating it")
		return
	}
	var captured strings.Builder
	for _, t := range r.doc(a.Admission).All() {
		if !t.Has("Artefact", "Captured to") {
			continue
		}
		var parts []string
		for _, row := range t.Rows {
			if !row.Placeholder() {
				parts = append(parts, nonEmpty(row.Values())...)
			}
		}
		captured.WriteString(strings.Join(parts, " "))
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
	claimed := strings.Join(r.doc(a.Specification).Column("What it arbitrates", "Path"), " ")
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
	var recorded string
	for _, t := range r.doc(a.Admission).All() {
		for _, row := range t.Rows {
			if !row.Placeholder() && strings.HasPrefix(row.Cell("Field"), "Specification digest") {
				recorded = row.Cell("Value")
			}
		}
	}
	if recorded == "" {
		r.fail("R1", "no specification digest recorded at admission")
		return
	}
	actual := r.c.SpecDigest
	switch {
	case recorded == actual:
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
	for _, row := range r.doc(a.Execution).Rows("Stored at") {
		id := orNone(row.Get("Id"))
		switch where := row.Cell("Stored at"); {
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
	for _, row := range r.doc(a.Plan).Rows("Preparatory for") {
		if row.Cell("Preparatory for") != "" {
			preparatory[row.Cell("Id")] = true
		}
	}
	for _, row := range r.doc(a.Execution).Rows("Artefacts", "Unit") {
		i := slices.IndexFunc(judges, func(p string) bool { return strings.Contains(row.Cell("Artefacts"), p) })
		if i >= 0 && !preparatory[row.Cell("Unit")] {
			r.fail("R8", "%s touches the arbiter (%s) and is nobody's preparatory work", orNone(row.Get("Unit")), judges[i])
		}
	}
}
