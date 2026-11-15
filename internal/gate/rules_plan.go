package gate

import (
	"path/filepath"
	"slices"
	"strings"

	a "github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/estate"
)

// evidencePlan — R16: a criterion that needs a plan has one, and the plan says
// what it was derived from. A plan derived from the candidate rather than from
// the estate is the failure this exists to prevent, so the derivation is the
// one field that cannot be left empty.
func evidencePlan(r *run) {
	spec := r.doc(a.Specification)
	if !spec.HasTable("Plan", "Criterion") {
		if r.has(a.Specification) {
			r.note("R16", "this specification does not say which criteria need an evidence plan — it predates the obligation rather than violating it")
		}
		return
	}
	var needs []string
	for _, row := range spec.Rows("Plan", "Criterion") {
		v := row.Cell("Plan")
		if v != "" && !Codes["plan"].allows(v) {
			r.fail("CONVENTIONS", "01-specification: Plan = %s is not a defined value", quote(v))
		}
		if v == "required" {
			needs = append(needs, row.Cell("Id"))
		}
	}
	needs = distinct(needs)
	planned := map[string]a.Row{}
	for _, row := range r.doc(a.Admission).Rows("Observed", "Derived from") {
		planned[row.Cell("Criterion")] = row
	}
	for _, crit := range needs {
		row, ok := planned[crit]
		if !ok {
			if r.has(a.Execution) {
				r.fail("R16", "%s needs an evidence plan and was executed against without one", crit)
			} else {
				r.fail("R16", "%s needs an evidence plan and has none", crit)
			}
			continue
		}
		for _, field := range []string{"Observed", "Method", "Derived from"} {
			if row.Cell(field) == "" {
				r.fail("R16", "the plan for %s leaves %s empty; a plan that does not say what it was derived from was derived from the change", crit, quote(field))
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
func planAgainstEstate(r *run, needs []string, planned map[string]a.Row) {
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
		row, ok := planned[crit]
		if !ok {
			continue
		}
		for _, field := range []string{"Observed", "Derived from"} {
			for _, p := range a.PathsIn(row.Cell(field)) {
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
func planCoversReach(r *run, needs []string, planned map[string]a.Row) {
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
	for _, row := range planned {
		for _, field := range []string{"Observed", "Derived from"} {
			for _, p := range a.PathsIn(row.Cell(field)) {
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
