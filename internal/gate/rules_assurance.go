package gate

import (
	"slices"
	"strings"

	a "github.com/igannytskyi/iflow/internal/artifact"
)

// evidenceIndependence — R3, R3b and B2: evidence says on what ground it is
// independent, evidence produced from inside the scope is not, and nothing is
// met on evidence that is not or that does not exist.
func evidenceIndependence(r *run) {
	if !r.has(a.Assurance) {
		return
	}
	ev := newOrdered[a.Row]()
	for _, row := range r.doc(a.Assurance).Rows("Independence") {
		ev.put(row.Cell("Id"), row)
	}
	within := scope(r.c)
	for _, id := range ev.keys {
		row := ev.vals[id]
		ind := row.Cell("Independence")
		switch {
		case ind == "":
			r.fail("R3", "%s does not state independence", id)
		case strings.HasPrefix(ind, "independent") && row.Cell("How established") == "":
			r.fail("R3", "%s claims independence without saying how", id)
		}
		producer := row.Cell("Producer")
		i := slices.IndexFunc(within, func(p string) bool { return p != "" && strings.Contains(producer, p) })
		if i >= 0 && ind != "not-independent" {
			r.fail("R3b", "%s is produced from inside the scope (%s) and cannot be %s", id, within[i], ind)
		}
	}
	for _, row := range r.doc(a.Assurance).Rows("Outcome", "Evidence") {
		if row.Cell("Outcome") != "met" {
			continue
		}
		for _, cited := range a.IDs(row.Cell("Evidence")) {
			src, ok := ev.get(cited)
			switch {
			case !ok:
				r.fail("B2", "%s cites undefined evidence %s", orNone(row.Get("Id")), cited)
			case src.Cell("Independence") == "not-independent":
				r.fail("R3", "%s is met on non-independent %s", orNone(row.Get("Id")), cited)
			}
		}
	}
}

// refusals — R17: a refusal names what it returns and why, and nothing
// downstream of a refused object is accepted. A refusal is the producer's
// failure, not a request to try again.
func refusals(r *run) {
	refused := newOrdered[string]()
	for _, s := range a.Stages {
		for _, row := range r.doc(s).Rows("Refused by", "Ground") {
			id := orNone(row.Get("Id"))
			obj := row.Cell("Object")
			if obj == "" {
				r.fail("R17", "%s refuses nothing in particular", id)
				continue
			}
			if row.Cell("Ground") == "" {
				r.fail("R17", "%s refuses %s without a ground", id, obj)
			}
			if by := row.Cell("Refused by"); by != "" && !Codes["refused_by"].allows(by) {
				r.fail("CONVENTIONS", "Refused by = %s is not a defined value", quote(by))
			}
			refused.put(obj, row.Cell("Id"))
		}
	}
	if refused.len() == 0 {
		return
	}
	verdicts := r.doc(a.Assurance).Rows("Outcome", "Unit")
	for _, v := range verdicts {
		if v.Cell("Outcome") != "met" {
			continue
		}
		for _, obj := range refused.keys {
			if strings.Contains(v.Joined(), obj) {
				r.fail("R17", "%s accepts %s, which %s refused", orNone(v.Get("Id")), obj, refused.vals[obj])
			}
		}
	}
	for _, cand := range r.doc(a.Execution).Rows("Stored at") {
		id, ok := cand.Get("Id")
		if _, was := refused.get(id); !ok || !was {
			continue
		}
		for _, v := range verdicts {
			if v.Cell("Unit") == cand.Cell("Unit") && v.Cell("Outcome") == "met" {
				r.fail("R17", "%s accepts work whose candidate %s was refused", orNone(v.Get("Id")), id)
			}
		}
	}
}

// criteriaReview — R18: whether a person read the criteria before admission
// is recorded either way. Recording no is legitimate; saying nothing is not.
func criteriaReview(r *run) {
	if !r.has(a.Admission) {
		return
	}
	for _, t := range r.doc(a.Admission).All() {
		if !t.Has("Reviewed before admission") {
			continue
		}
		for _, row := range t.Rows {
			if row.Placeholder() {
				continue
			}
			switch v := row.Cell("Reviewed before admission"); {
			case !Codes["reviewed"].allows(v):
				r.fail("R18", "criteria review recorded as %s, which is not a defined value", quote(v))
			case v == "yes" && row.Cell("By") == "":
				r.fail("R18", "the criteria are recorded as reviewed and nobody is named as having reviewed them")
			}
		}
		return
	}
	r.note("R18", "this change does not say whether the criteria were reviewed before admission — it predates the obligation rather than violating it")
}

// repeatability — R13 and R14: a record of a run says whether the run can be
// produced again, and a result does not rest on one that cannot alone.
func repeatability(r *run) {
	if !r.has(a.Assurance) {
		return
	}
	if !r.doc(a.Assurance).HasTable("Repeatable") {
		r.note("R13", "this change's evidence does not state repeatability — it predates the obligation rather than violating it")
		return
	}
	ev := repeatable(r.c)
	for _, id := range ev.keys {
		switch v := ev.vals[id]; {
		case v == "":
			r.fail("R13", "%s does not say whether the run behind it repeats", id)
		case !Codes["repeatable"].allows(v):
			r.fail("R13", "%s: Repeatable = %s is not a defined value", id, quote(v))
		case v == "yes" && ProducerCommand(r.c, id) == nil:
			// A claim to repeat is worth nothing unless it says what to re-run.
			r.fail("R13", "%s claims to repeat and names nothing that can be re-run, so the claim cannot be tested", id)
		}
	}
	for _, row := range r.doc(a.Assurance).Rows("Outcome", "Evidence") {
		if row.Cell("Outcome") != "met" {
			continue
		}
		known := slices.DeleteFunc(a.IDs(row.Cell("Evidence")), func(id string) bool { _, ok := ev.get(id); return !ok })
		unrepeatable := func(id string) bool { v := ev.vals[id]; return v == "no" || v == "unknown" }
		if len(known) > 0 && !slices.ContainsFunc(known, func(id string) bool { return !unrepeatable(id) }) {
			r.fail("R14", "%s is met only on runs that cannot be produced again (%s)", orNone(row.Get("Id")), strings.Join(known, ", "))
		}
	}
}

// deferredComplete — R6: a deferred verdict names its window and its
// baseline.
func deferredComplete(r *run) {
	for _, row := range r.doc(a.Assurance).Rows("State") {
		if row.Cell("State") == "deferred" && (row.Cell("Window") == "" || row.Cell("Baseline") == "") {
			r.fail("R6", "%s is deferred without a window and a baseline", orNone(row.Get("Id")))
		}
	}
}

// verdictCoverage — A5: every criterion has a verdict.
func verdictCoverage(r *run) {
	if !r.has(a.Assurance) || !r.has(a.Specification) {
		return
	}
	judged := r.doc(a.Assurance).Column("Criterion", "Outcome")
	missing := slices.DeleteFunc(criteria(r.c), func(c string) bool { return slices.Contains(judged, c) })
	if len(missing) > 0 {
		r.fail("A5", "no verdict for %s", quoteList(missing))
	}
}
