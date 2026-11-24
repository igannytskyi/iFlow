package flow

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/gate"
	"github.com/igannytskyi/iflow/internal/meta"
	"github.com/igannytskyi/iflow/internal/model"
)

// arbitrate runs the framework's own tests against it as it is now, where
// the change landed on the framework itself and nothing has run them since
// it last changed: R22 holds such a landing until they pass, and running
// them is the framework's work, not the person's.
func (r *Run) arbitrate(ctx context.Context) {
	// The framework the change landed in, wherever its copy is: the one at
	// work here, or another the bench laid out.
	home := r.repo
	if !gate.IsFramework(home) {
		return
	}
	if _, ok := gate.Verified(home)[gate.ArbitersKey(home)]; ok {
		return
	}
	r.say(6, r.id("SPEC"), "the framework changed, so its own tests are run against it as it is now (R22)")
	began := time.Now()
	run, err := gate.Arbiters(ctx, home)
	r.cost(artifact.Cost{Area: 6, What: "the framework's own tests, run against it as it is now", Actor: "go test"}, model.Usage{}, time.Since(began))
	switch {
	case err != nil:
		r.say(6, r.id("SPEC"), "the framework's tests could not be run: %v", err)
	case !run.Passed:
		r.say(6, r.id("SPEC"), "the framework's own tests fail against it as it is now")
	}
}

// land is area 6: each accepted candidate entered into the estate, in the
// order the plan gave, while its evidence still holds.
func (r *Run) land(ctx context.Context) error {
	r.set(6, Working)
	doc := meta.Landing{Schema: meta.SchemaVersion, Name: r.slug}
	// The estate may have moved since admission: another change landed on
	// the same ground. Entering whole files now would undo it, and the
	// evidence was gathered against ground that is no longer there. Where
	// the two changes' edits are apart, they are merged and the whole suite
	// is run on the estate as it is now with the change on it; where they
	// overlap, or the suite fails, the change stops.
	merged, moved, why := r.rebased(ctx)
	if why != "" {
		_ = r.record("landing", r.id("SPEC"), "stopped: the estate moved under it — "+why)
		return r.fail(6, "the estate moved since this change was admitted (%s), and it cannot be laid on the estate as it is now: %s. Carry it again from the estate as it is now", strings.Join(moved, "; "), why)
	}
	if len(merged) > 0 {
		r.say(6, r.id("SPEC"), "the estate moved under it (%s); merged with what moved it, the whole suite passes on the estate as it is now", strings.Join(moved, "; "))
		_ = r.record("landing", r.id("SPEC"), "merged onto the estate as it is now: "+strings.Join(moved, "; ")+"; the whole suite passed on the merged ground")
	}
	for i, u := range r.units {
		if err := copyInto(r.cands[u.id], r.repo); err != nil {
			return r.fail(6, "%s could not be entered: %v", u.id, err)
		}
		for rel, b := range merged {
			if inScope(rel, u.scope) || slices.Contains(r.codeScope(), rel) {
				if err := os.WriteFile(filepath.Join(r.repo, filepath.FromSlash(rel)), b, 0o644); err != nil {
					return r.fail(6, "%s could not be entered merged: %v", rel, err)
				}
			}
		}
		doc.Entries = append(doc.Entries, meta.Entry{Order: i + 1, Unit: u.id, VerdictState: "settled",
			StillValid: "yes", EnteredAt: today(), Invalidated: "nothing the evidence relies on"})
		r.say(6, u.id, "%s entered", u.id)
	}
	if err := artifact.Save(r.dir, artifact.Landing, doc); err != nil {
		return r.fail(6, "the landing could not be written: %v", err)
	}
	for _, u := range r.units {
		if err := r.record("landing", u.id, "entered"); err != nil {
			return err
		}
	}
	c, err := artifact.Load(r.dir)
	if err == nil {
		began := time.Now()
		var kept func(string) (string, bool)
		if r.cfg.RepeatOnce {
			kept = func(ev string) (string, bool) {
				// The report a run wrote, where it wrote one: compared test
				// by test. What it said, where not.
				if b, err := os.ReadFile(filepath.Join(r.dir, "evidence", ev+".report")); err == nil {
					return string(b), true
				}
				b, err := os.ReadFile(filepath.Join(r.dir, "evidence", ev+".txt"))
				return string(b), err == nil
			}
		}
		reps, err := r.cfg.Gate.RepeatAgainst(ctx, c, kept)
		r.cost(artifact.Cost{Area: 6, What: "every claim to repeat, run twice", Actor: "the gate"}, model.Usage{}, time.Since(began))
		if err == nil {
			for _, rp := range reps {
				if rp.Same {
					r.say(6, rp.Evidence, "%s repeated: %s gave the same result twice", rp.Evidence, rp.Shown())
				} else {
					r.fail(6, "%s repeated: %s did not give the same result twice — %s", rp.Evidence, rp.Shown(), rp.Differs)
				}
				// The framework's whole suite, repeated here on the landed
				// framework and passing, is the arbiters' run: R22 is told
				// so, and it is not run a third time on the same content.
				if rp.Same && rp.Passed && gate.IsArbiterRun(rp.Command) && gate.IsFramework(r.repo) {
					_ = gate.NoteVerified(r.repo, gate.ArbitersKey(r.repo), "every arbiter passed against this framework, run as "+rp.Evidence+"'s repeat")
				}
			}
		}
	}
	r.arbitrate(ctx)
	if rep, err := r.check(ctx, 6); err != nil || !rep.Passed() {
		return r.fail(6, "the landing does not pass the gate")
	}
	r.set(6, Done)
	return nil
}

func copyInto(from, to string) error {
	return filepath.WalkDir(from, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() == "CHANGES" {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		dst := filepath.Join(to, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, b, 0o644)
	})
}
