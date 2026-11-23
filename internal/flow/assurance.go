package flow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/harness"
	"github.com/igannytskyi/iflow/internal/meta"
	"github.com/igannytskyi/iflow/internal/model"
)

// refused is a finished change the person read and did not affirm: its
// units are carried again with what the person said.
type refused struct{ why string }

func (e *refused) Error() string { return "the person did not affirm the change: " + e.why }

// maxRefusals is how many times a change the person did not affirm is
// carried again before the run stops and leaves it to them.
const maxRefusals = 2

// assure is area 5: evidence gathered in a copy of the estate holding every
// candidate, by what judges the change, and a verdict on every criterion.
func (r *Run) assure(ctx context.Context) error {
	r.set(5, Working)
	prof, ok := detect(r.repo)
	if !ok {
		return r.fail(5, "nothing here says how this estate's tests are run, so no evidence can be gathered: every criterion would be undecided")
	}
	var overlays []string
	for _, u := range r.units {
		overlays = append(overlays, r.cands[u.id])
	}
	ws, err := harness.Snapshot(r.repo, overlays...)
	if err != nil {
		return r.fail(5, "a workspace could not be made: %v", err)
	}
	defer os.RemoveAll(ws)
	doc := meta.Assurance{Schema: meta.SchemaVersion, Name: r.slug}
	passed := map[string]bool{} // evidence id → passed
	byCriterion := map[string]string{}
	for i, prep := range r.units {
		if !prep.isTests() {
			continue
		}
		ca := r.id("CA", i+1)
		names := r.ledgerTests(prep.id)
		cmd := prof.only(names)
		ok, out, took := runIn(ctx, ws, resolve(ws, cmd))
		ev := r.id("EV", i+1, 1)
		r.cost(artifact.Cost{Area: 5, Unit: prep.id, What: ev + ": the tests written first", Actor: prof.name}, model.Usage{}, took)
		passed[ev] = ok
		r.keepOutput(ev, out)
		doc.Evidence = append(doc.Evidence, meta.Evidence{ID: ev, Subject: meta.OfCandidate, Kind: meta.TestRun, Producer: shown(cmd),
			Independence: meta.ByPrecommitment,
			How:          "written by " + prep.id + " and failed against the unchanged estate before the change existed — candidates/" + ca + "/failing-before.txt",
			Repeatable:   meta.Yes, ObtainedAt: today(), ValidUntil: "next change to " + strings.Join(r.spec.Scope, ", ")})
		for _, c := range prep.criteria {
			byCriterion[c] = ev
		}
		r.say(5, ev, "%s: %s %s", ev, shown(cmd), verdictWord(ok))
	}
	ok2, out, took := runIn(ctx, ws, resolve(ws, prof.suite))
	suite := r.id("EV", len(r.units), 1)
	r.cost(artifact.Cost{Area: 5, What: suite + ": the whole suite", Actor: prof.name}, model.Usage{}, took)
	passed[suite] = ok2
	r.keepOutput(suite, out)
	doc.Evidence = append(doc.Evidence, meta.Evidence{ID: suite, Subject: meta.OfCandidate, Kind: meta.TestRun, Producer: shown(prof.suite),
		Independence: meta.ByExecutor, How: "the estate's own suite, written before this change and outside everything it touched",
		Repeatable: meta.Yes, ObtainedAt: today(), ValidUntil: "next change to the estate"})
	r.say(5, suite, "%s: %s %s", suite, shown(prof.suite), verdictWord(ok2))
	// What only a person can see, the person affirms, reading the finished
	// change before it lands: one question, every such criterion in it.
	var byPerson []string
	for i, c := range r.spec.Criteria {
		if c.Procedure == "human" {
			id := r.id("CR", i+1)
			byPerson = append(byPerson, fmt.Sprintf("%s: when %s, then %s", id, c.When, c.Then))
			byCriterion[id] = r.id("EV", len(r.units), 2)
		}
	}
	var refusal string
	if len(byPerson) > 0 {
		ev := r.id("EV", len(r.units), 2)
		got, err := r.ask(ctx, 5, "Does the change do what only you can judge? Read it — the candidates' files under Execution — and accept if each of these holds, or send it back saying which does not:\n\n"+strings.Join(byPerson, "\n"))
		if err != nil {
			return err
		}
		passed[ev] = got.ok
		how := "affirmed by " + r.cfg.Person + ", who did not write the candidate, reading it before it landed"
		if !got.ok {
			how = "refused by " + r.cfg.Person + ": " + got.ground
		}
		doc.Evidence = append(doc.Evidence, meta.Evidence{ID: ev, Subject: meta.OfCandidate, Kind: meta.HumanAffirmation,
			Producer: r.cfg.Person, Independence: meta.ByExecutor, How: how, Repeatable: meta.No, ObtainedAt: today()})
		r.say(5, ev, "%s: %s %s", ev, r.cfg.Person, map[bool]string{true: "affirmed it", false: "did not affirm it: " + got.ground}[got.ok])
		if !got.ok {
			refusal = got.ground
		}
	}
	n := 0
	allMet := true
	for _, u := range r.units {
		for _, c := range u.criteria {
			ev, ok := byCriterion[c]
			if !ok {
				ev = suite
			}
			n++
			outcome := meta.Met
			if !passed[ev] {
				outcome, allMet = meta.Failed, false
			}
			ve := r.id("VE", n)
			doc.Verdicts = append(doc.Verdicts, meta.Verdict{ID: ve, Unit: u.id, Criterion: c, Outcome: outcome, Evidence: []string{ev}, State: meta.Settled})
			r.say(5, ve, "%s: %s on %s — %s, on %s", ve, u.id, c, outcome, ev)
		}
	}
	if allMet {
		for _, ref := range r.settles {
			doc.Settled = append(doc.Settled, meta.Settlement{Escalation: ref, ResolvedBy: "this change", Note: "every criterion met; started to close it"})
			r.say(5, ref, "%s is closed by this change", ref)
		}
	}
	if err := artifact.Save(r.dir, artifact.Assurance, doc); err != nil {
		return r.fail(5, "the assurance could not be written: %v", err)
	}
	for _, v := range doc.Verdicts {
		if err := r.record("assurance", v.ID, string(v.Outcome)); err != nil {
			return err
		}
	}
	if rep, err := r.check(ctx, 5); err != nil || !rep.Passed() {
		return r.fail(5, "the assurance does not pass the gate")
	}
	if !allMet && refusal != "" && r.refusals < maxRefusals {
		return &refused{why: refusal}
	}
	if !allMet {
		return r.fail(5, "a criterion failed, so nothing enters")
	}
	r.set(5, Done)
	return nil
}

func verdictWord(ok bool) string {
	if ok {
		return "passed"
	}
	return "failed"
}

func (r *Run) keepOutput(ev, out string) {
	dir := filepath.Join(r.dir, "evidence")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, ev+".txt"), []byte(out), 0o644)
}
