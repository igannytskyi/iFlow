package flow

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/gate"
	"github.com/igannytskyi/iflow/internal/meta"
)

// admit is area 3: conflict, allowance and permission settled together, the
// evidence plan fixed from the area of effect, the prior state captured.
func (r *Run) admit(ctx context.Context) error {
	r.set(3, Working)
	// The specification as admitted is its JSON: R1 compares against this
	// digest ever after.
	digest, err := artifact.DigestFile(artifact.JSONFile(r.dir, artifact.Specification))
	if err != nil {
		return r.fail(3, "the specification could not be read: %v", err)
	}
	doc := meta.Admission{Schema: meta.SchemaVersion, Name: r.slug, Digest: digest,
		Reviews: []meta.Review{{Reviewed: "yes", By: r.cfg.Person, At: today(), Note: "accepted in the workbench before any work"}}}
	for i, u := range r.units {
		cond := "permission"
		if i > 0 {
			cond = "conflict"
		}
		doc.Decisions = append(doc.Decisions, meta.Decision{Unit: u.id, Outcome: meta.Admitted, Condition: cond, Confidence: "medium"})
	}
	if len(r.units) > 1 {
		var ids []string
		for _, u := range r.units {
			ids = append(ids, u.id)
		}
		doc.Conflicts = append(doc.Conflicts, meta.Conflict{Units: joinIDs(ids), Region: "none", Invalidates: "none", Resolution: "ordering"})
	}
	// Other changes in flight in the same estate whose scope meets this
	// one's: whichever lands second finds the ground moved under it, and the
	// landing says so rather than undoing the first.
	for _, other := range r.inFlight() {
		doc.Conflicts = append(doc.Conflicts, meta.Conflict{Units: r.id("SPEC") + ", " + other.name, Region: strings.Join(other.shared, ", "),
			Invalidates: "the evidence of whichever lands second", Resolution: "ordering: the second landing merges onto the ground as it is then, and stops if their edits overlap"})
		r.say(3, "", "in flight beside it: %s, which may also change %s; whichever lands second checks the ground first", other.name, strings.Join(other.shared, ", "))
	}
	observed, err := r.observed()
	if err != nil {
		return r.fail(3, "the estate could not be asked: %v", err)
	}
	for i, c := range r.spec.Criteria {
		if c.Plan == "required" {
			doc.Plans = append(doc.Plans, meta.EvidencePlan{Criterion: r.id("CR", i+1), Observed: observed,
				Method: "the tests written first, run once before the change and once after", Unchanged: "the behaviour stands as it does now",
				DerivedFrom: "the area of effect of " + strings.Join(r.spec.Scope, ", ") + ", derived, medium", FixedAt: today()})
		}
	}
	base := filepath.Join(r.dir, "baseline")
	if err := os.MkdirAll(base, 0o755); err != nil {
		return r.fail(3, "the prior state could not be captured: %v", err)
	}
	for _, s := range r.spec.Scope {
		src, err := os.ReadFile(filepath.Join(r.repo, s))
		to, what := "baseline/"+path.Base(s)+".before", s+" as it stood"
		if err != nil {
			// A file the change creates has a prior state too: it was not
			// there. Saying so is what keeps the comparison from being made
			// against something the change itself wrote.
			src, to, what = nil, "baseline/"+path.Base(s)+".absent", s+": did not exist before this change"
		}
		if err := os.WriteFile(filepath.Join(r.dir, to), src, 0o644); err != nil {
			return r.fail(3, "the prior state could not be captured: %v", err)
		}
		doc.Prior = append(doc.Prior, meta.Capture{Criterion: r.id("CR", 1), Artefact: what, CapturedTo: to})
	}
	if err := artifact.Save(r.dir, artifact.Admission, doc); err != nil {
		return r.fail(3, "the admission could not be written: %v", err)
	}
	for _, u := range r.units {
		if err := r.record("admission", u.id, "admitted"); err != nil {
			return err
		}
		r.say(3, u.id, "%s admitted", u.id)
	}
	if rep, err := r.check(ctx, 3); err != nil || !rep.Passed() {
		return r.fail(3, "the admission does not pass the gate")
	}
	r.set(3, Done)
	return nil
}

// observed is what an evidence plan observes: the scope, and every region
// the change reaches firmly that nothing already watches.
func (r *Run) observed() (string, error) {
	repo := r.cfg.Estate.Repo(r.repo)
	reach, err := repo.Affects(r.spec.Scope)
	if err != nil {
		return "", err
	}
	regions, err := repo.EveryRegion()
	if err != nil {
		return "", err
	}
	watched := map[string]bool{}
	for _, g := range regions {
		if g.Verdict != "unclaimed" || g.TestGround {
			watched[g.Region] = true
		}
	}
	parts := slices.Clone(r.spec.Scope)
	for _, x := range reach {
		if dir := path.Dir(x.File); x.Confidence != "low" && !watched[dir] && dir != "." && !slices.Contains(parts, dir) {
			parts = append(parts, dir)
		}
	}
	return strings.Join(parts, ", ") + " — the behaviour the criterion names", nil
}

// flight is another change in flight in the same estate, and the paths it
// may change that this one may too.
type flight struct {
	name   string
	shared []string
}

// inFlight is every other change of the estate not yet landed nor closed
// whose scope meets this one's.
func (r *Run) inFlight() []flight {
	dirs, _ := filepath.Glob(filepath.Join(r.repo, "changes", "*", string(artifact.Specification)+".md"))
	var out []flight
	for _, spec := range dirs {
		dir := filepath.Dir(spec)
		if dir == r.dir {
			continue
		}
		c, err := artifact.Load(dir)
		if err != nil || c.Meta.Specification == nil || c.Has(artifact.Landing) {
			continue
		}
		if _, closed := gate.Closed(c); closed {
			continue
		}
		var shared []string
		for _, p := range c.Meta.Specification.Scope.Included {
			if p = strings.TrimSpace(p); p != "" && inScope(p, r.spec.Scope) {
				shared = append(shared, p)
			}
		}
		if len(shared) > 0 {
			out = append(out, flight{filepath.Base(dir), shared})
		}
	}
	return out
}
