package gate

import (
	"slices"
	"strings"

	a "github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/meta"
)

// evidenceIndependence — R3, R3b and B2: evidence says on what ground it is
// independent, evidence produced from inside the scope is not, and nothing is
// met on evidence that is not or that does not exist.
func evidenceIndependence(r *run) {
	as := r.c.Meta.Assurance
	if as == nil {
		return
	}
	ev := newOrdered[meta.Evidence]()
	for _, e := range as.Evidence {
		ev.put(e.ID, e)
	}
	within := scope(r.c)
	for _, id := range ev.keys {
		e := ev.vals[id]
		ind := string(e.Independence)
		switch {
		case ind == "":
			r.fail("R3", "%s does not state independence", id)
		case strings.HasPrefix(ind, "independent") && e.How == "":
			r.fail("R3", "%s claims independence without saying how", id)
		}
		i := slices.IndexFunc(within, func(p string) bool { return p != "" && strings.Contains(e.Producer, p) })
		if i >= 0 && e.Independence != meta.NotIndependent {
			r.fail("R3b", "%s is produced from inside the scope (%s) and cannot be %s", id, within[i], ind)
		}
	}
	for _, v := range as.Verdicts {
		if v.Outcome != meta.Met {
			continue
		}
		for _, cited := range v.Evidence {
			src, ok := ev.get(cited)
			switch {
			case !ok:
				r.fail("B2", "%s cites undefined evidence %s", or(v.ID), cited)
			case src.Independence == meta.NotIndependent:
				r.fail("R3", "%s is met on non-independent %s", or(v.ID), cited)
			}
		}
	}
}

// refusals — R17: a refusal names what it returns and why, and nothing
// downstream of a refused object is accepted. A refusal is the producer's
// failure, not a request to try again.
func refusals(r *run) {
	as := r.c.Meta.Assurance
	if as == nil {
		return
	}
	refused := newOrdered[string]()
	for _, f := range as.Refusals {
		id := or(f.ID)
		if f.Object == "" {
			r.fail("R17", "%s refuses nothing in particular", id)
			continue
		}
		if f.Ground == "" {
			r.fail("R17", "%s refuses %s without a ground", id, f.Object)
		}
		refused.put(f.Object, f.ID)
	}
	if refused.len() == 0 {
		return
	}
	for _, v := range as.Verdicts {
		if v.Outcome != meta.Met {
			continue
		}
		said := strings.Join(append([]string{v.ID, v.Unit, v.Criterion, string(v.Outcome), string(v.State), v.Window, v.Baseline}, v.Evidence...), " ")
		for _, obj := range refused.keys {
			if strings.Contains(said, obj) {
				r.fail("R17", "%s accepts %s, which %s refused", or(v.ID), obj, refused.vals[obj])
			}
		}
	}
	if r.c.Meta.Execution == nil {
		return
	}
	for _, ca := range r.c.Meta.Execution.Candidates {
		if _, was := refused.get(ca.ID); ca.ID == "" || !was {
			continue
		}
		for _, v := range as.Verdicts {
			if v.Unit == ca.Unit && v.Outcome == meta.Met {
				r.fail("R17", "%s accepts work whose candidate %s was refused", or(v.ID), ca.ID)
			}
		}
	}
}

// criteriaReview — R18: whether a person read the criteria before admission
// is recorded either way. Recording no is legitimate; saying nothing is not.
func criteriaReview(r *run) {
	ad := r.c.Meta.Admission
	if ad == nil {
		return
	}
	if len(ad.Reviews) == 0 {
		r.note("R18", "this change does not say whether the criteria were reviewed before admission — it predates the obligation rather than violating it")
		return
	}
	for _, v := range ad.Reviews {
		switch {
		case !meta.Allowed("Reviewed", string(v.Reviewed)):
			r.fail("R18", "criteria review recorded as %s, which is not a defined value", quote(string(v.Reviewed)))
		case v.Reviewed == "yes" && v.By == "":
			r.fail("R18", "the criteria are recorded as reviewed and nobody is named as having reviewed them")
		}
	}
}

// repeatability — R13 and R14: a record of a run says whether the run can be
// produced again, and a result does not rest on one that cannot alone.
func repeatability(r *run) {
	as := r.c.Meta.Assurance
	if as == nil {
		return
	}
	if !stated(r.c) {
		r.note("R13", "this change's evidence does not state repeatability — it predates the obligation rather than violating it")
		return
	}
	ev := repeatable(r.c)
	for _, id := range ev.keys {
		switch v := ev.vals[id]; {
		case v == "":
			r.fail("R13", "%s does not say whether the run behind it repeats", id)
		case !meta.Allowed("Repeatable", v):
			r.fail("R13", "%s: Repeatable = %s is not a defined value", id, quote(v))
		case v == "yes" && ProducerCommand(r.c, id) == nil && r.has(a.Landing) && producerNamed(r.c, id):
			// It named what to re-run, and that has gone since the change
			// landed — the code it ran was rewritten. The claim stands as it
			// was tested then; nothing now can test it again.
			r.note("R13", "%s named what to re-run, which is gone since the change landed; the claim stands as it was tested then", id)
		case v == "yes" && ProducerCommand(r.c, id) == nil:
			// A claim to repeat is worth nothing unless it says what to re-run.
			r.fail("R13", "%s claims to repeat and names nothing that can be re-run, so the claim cannot be tested", id)
		}
	}
	for _, v := range as.Verdicts {
		if v.Outcome != meta.Met {
			continue
		}
		known := slices.DeleteFunc(slices.Clone(v.Evidence), func(id string) bool { _, ok := ev.get(id); return !ok })
		unrepeatable := func(id string) bool { x := ev.vals[id]; return x == "no" || x == "unknown" }
		if len(known) > 0 && !slices.ContainsFunc(known, func(id string) bool { return !unrepeatable(id) }) {
			r.fail("R14", "%s is met only on runs that cannot be produced again (%s)", or(v.ID), strings.Join(known, ", "))
		}
	}
}

// deferredComplete — R6: a deferred verdict names its window and its
// baseline.
func deferredComplete(r *run) {
	if r.c.Meta.Assurance == nil {
		return
	}
	for _, v := range r.c.Meta.Assurance.Verdicts {
		if v.State == meta.Deferred && (v.Window == "" || v.Baseline == "") {
			r.fail("R6", "%s is deferred without a window and a baseline", or(v.ID))
		}
	}
}

// verdictCoverage — A5: every criterion has a verdict.
func verdictCoverage(r *run) {
	if !r.has(a.Assurance) || !r.has(a.Specification) {
		return
	}
	var judged []string
	for _, v := range r.c.Meta.Assurance.Verdicts {
		judged = append(judged, v.Criterion)
	}
	missing := slices.DeleteFunc(criteria(r.c), func(c string) bool { return slices.Contains(judged, c) })
	if len(missing) > 0 {
		r.fail("A5", "no verdict for %s", quoteList(missing))
	}
}
