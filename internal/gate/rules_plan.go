package gate

import (
	"path/filepath"
	"slices"
	"strings"

	a "github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/estate"
	"github.com/igannytskyi/iflow/internal/meta"
)

// evidencePlan — R16: a criterion that needs a plan has one, and the plan says
// what it was derived from. A plan derived from the candidate rather than from
// the estate is the failure this exists to prevent, so the derivation is the
// one field that cannot be left empty.
func evidencePlan(r *run) {
	spec := r.c.Meta.Specification
	if spec == nil {
		return
	}
	if !slices.ContainsFunc(spec.Criteria, func(c meta.Criterion) bool { return c.Plan != "" }) {
		r.note("R16", "this specification does not say which criteria need an evidence plan — it predates the obligation rather than violating it")
		return
	}
	var needs []string
	for _, c := range spec.Criteria {
		if c.Plan == meta.Required {
			needs = append(needs, c.ID)
		}
	}
	needs = distinct(needs)
	if !r.has(a.Admission) {
		return // the evidence plan is fixed at admission, which is not reached
	}
	planned := map[string]meta.EvidencePlan{}
	for _, p := range r.c.Meta.Admission.Plans {
		planned[p.Criterion] = p
	}
	for _, crit := range needs {
		p, ok := planned[crit]
		if !ok {
			if r.has(a.Execution) {
				r.fail("R16", "%s needs an evidence plan and was executed against without one", crit)
			} else {
				r.fail("R16", "%s needs an evidence plan and has none", crit)
			}
			continue
		}
		for _, f := range []struct{ name, v string }{{"Observed", p.Observed}, {"Method", p.Method}, {"Derived from", p.DerivedFrom}} {
			if f.v == "" {
				r.fail("R16", "the plan for %s leaves %s empty; a plan that does not say what it was derived from was derived from the change", crit, quote(f.name))
			}
		}
	}
	planAgainstEstate(r, needs, planned)
	planCoversReach(r, needs, planned)
}

// planAgainstEstate — R16, the half that could not be checked until there was
// a model. Ground the change provably does not reach is not an area of
// effect, and a plan that names it was derived from something else. Where the
// estate cannot answer, the gate says so, because an unasked question and an
// answered one look the same in a log otherwise.
func planAgainstEstate(r *run, needs []string, planned map[string]meta.EvidencePlan) {
	within := nonEmpty(scope(r.c))
	if len(needs) == 0 || len(within) == 0 {
		return
	}
	est := r.gate.estate
	reach, err := est.Affects(r.ctx, r.c.Root, within)
	var named []estate.Reach
	if err == nil {
		named, err = est.NamedBy(r.ctx, r.c.Root, within)
	}
	if err != nil {
		r.note("R16", "the estate could not be asked about this scope (%s), so what the plan names is recorded and not verified", estate.Kind(err))
		return
	}
	known := map[string]bool{}
	for _, x := range slices.Concat(reach, named) {
		known[x.File] = true
	}
	for _, p := range within {
		known[p] = true
	}
	reachedBelow := func(p string) bool {
		for k := range known {
			if strings.HasPrefix(k, p) {
				return true
			}
		}
		return false
	}
	for _, crit := range needs {
		plan, ok := planned[crit]
		if !ok {
			continue
		}
		for _, f := range []struct{ field, v string }{{"Observed", plan.Observed}, {"Derived from", plan.DerivedFrom}} {
			field := f.field
			for _, p := range a.PathsIn(f.v) {
				if known[p] || reachedBelow(p) || !exists(filepath.Join(r.c.Root, p)) {
					continue // reached, or not a path here to judge
				}
				r.fail("R16", "the plan for %s names %s under %s, and nothing in the scope reaches it — an area of effect names what the change touches, so this was derived from something other than the estate", crit, p, quote(field))
			}
		}
	}
}

// planCoversReach — R16, the other half: a plan can name nothing wrong and
// still miss everything that matters. The demand is narrow: ground the change
// reaches firmly, where nothing already names a symbol in a test, and which
// no plan observes. Anywhere else, something is watching; there, nothing is.
func planCoversReach(r *run, needs []string, planned map[string]meta.EvidencePlan) {
	within := nonEmpty(scope(r.c))
	if len(needs) == 0 || len(within) == 0 {
		return
	}
	reach, err := r.gate.estate.Affects(r.ctx, r.c.Root, within)
	if err != nil {
		return // the half above has said so
	}
	regions, err := r.gate.estate.Regions(r.ctx, r.c.Root)
	if err != nil {
		return
	}
	// Only somewhere that defines something can be left judged by nothing.
	defining, watched := map[string]bool{}, map[string]bool{}
	for _, reg := range regions {
		defining[reg.Path] = true
		if reg.Watched() {
			watched[reg.Path] = true
		}
	}
	covered := map[string]bool{}
	for _, plan := range planned {
		for _, v := range []string{plan.Observed, plan.DerivedFrom} {
			for _, p := range a.PathsIn(v) {
				covered[regionOf(r.c.Root, p)] = true
			}
		}
	}
	for _, p := range within {
		covered[regionOf(r.c.Root, p)] = true
	}
	var firm []string
	for _, x := range reach {
		if reg := regionOf(r.c.Root, x.File); x.Firm() && defining[reg] && !watched[reg] && !covered[reg] {
			firm = append(firm, reg)
		}
	}
	for _, reg := range distinct(firm) {
		r.fail("R16", "the change reaches %s firmly, nothing there is named by a test, and no plan observes it — a plan that covers none of an unwatched area leaves it judged by nothing", reg)
	}
}

// planInvariants — R24: a plan holds to the invariants that it and its
// specification decide alone (docs/method/execution.md), however it was
// written: by the planner, by hand, or changed after. Every breach is named
// by its invariant. A plan whose contract was described in prose, carried
// before provisions were typed, is held to the rest, and that is noted.
func planInvariants(r *run) {
	p, s := r.c.Meta.Plan, r.c.Meta.Specification
	if p == nil || s == nil {
		return
	}
	if p.UpgradedFrom > 0 {
		r.note("R24", "this plan was written in version %d of the model and carried forward; it predates the invariants rather than breaking them", p.UpgradedFrom)
		return
	}
	for _, b := range meta.PlanBreaches(*p, *s) {
		r.fail("R24", "%s", b)
	}
	if slices.ContainsFunc(p.Units, func(u meta.WorkUnit) bool {
		return slices.ContainsFunc(u.Provides, func(pr meta.Provision) bool { return pr.Kind == meta.Described })
	}) {
		r.note("R24", "the contract of this plan is described in prose — it was carried before provisions were typed — so what it provides is not held to P6, P8 and P9")
	}
}
