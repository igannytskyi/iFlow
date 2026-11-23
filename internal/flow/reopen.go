package flow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/meta"
)

// Reopen prepares the change in dir to be carried again from one of its
// areas, in its own folder and under its own number, with what the person
// says is to change: from the specification (1), whose author is handed the
// specification as it was and the person's words, or from the plan (2),
// whose planner is handed them. What the areas from there on wrote is kept
// under superseded/, nothing deleted, and the record says the change was
// reopened. Start carries it; a change that lands again lands on the estate
// as it is, its earlier landing included.
func Reopen(cfg Config, repo, dir string, from int, notes string, changed func()) (*Run, error) {
	notes = strings.TrimSpace(notes)
	switch {
	case from != 1 && from != 2:
		return nil, fmt.Errorf("a change is reopened from its specification (1) or its plan (2), not area %d", from)
	case notes == "":
		return nil, errors.New("a change is reopened to change something; say what")
	case Running(dir):
		return nil, errors.New("the change is being carried now; stop it first")
	}
	c, err := artifact.Load(dir)
	if err != nil {
		return nil, err
	}
	in, spec := c.Meta.Intent, c.Meta.Specification
	if in == nil || spec == nil {
		return nil, errors.New("only a change with an intent and a specification can be reopened")
	}
	n, err := strconv.Atoi(strings.TrimPrefix(in.ID, "INT-"))
	if err != nil {
		return nil, fmt.Errorf("the intent's id %q names no number", in.ID)
	}
	r := New(cfg, repo, in.Statement, changed)
	r.dir, r.slug, r.n, r.from = c.Dir, c.Name(), n, from
	if abs, err := filepath.Abs(r.dir); err == nil {
		r.dir = abs
	}
	switch from {
	case 1:
		r.reopened = "\n\n# Your previous specification\n\n" + r.specText() +
			"\n# The person reopened the change, saying\n\n" + notes +
			"\n\nWrite the specification again, whole, as the person says; what they do not ask to change stays as it was."
	case 2:
		r.spec = draftOf(in, spec, repo)
		r.reopened = "\n\n# The person reopened the change after its plan, saying\n\n" + notes +
			"\n\nPlan the change again, whole, as the person says."
	}
	if err := supersede(r.dir, from); err != nil {
		return nil, err
	}
	if err := r.record("reopened", in.ID, fmt.Sprintf("carried again from area %d by %s: %s", from, cfg.Person, notes)); err != nil {
		return nil, err
	}
	_ = decide(repo, r.slug, notes)
	return r, nil
}

// draftOf is a specification in the model as a run holds one.
func draftOf(in *meta.Intent, s *meta.Specification, repo string) draft {
	d := draft{Slug: s.Name, Name: s.Name, Statement: in.Statement, Class: string(s.Class), Prose: s.Prose,
		Scope: s.Scope.Included, NewTests: s.NewTests, SuiteTests: s.SuiteTests}
	byNew := map[string]bool{}
	if len(d.NewTests) == 0 && len(d.SuiteTests) == 0 {
		// A specification written before the model said which tests are
		// written first says it all the same: they are the arbiters it keeps
		// out of its scope that the estate does not hold yet, and the
		// criteria they arbitrate are judged by them.
		for _, a := range s.Arbiters {
			if !slices.Contains(s.Scope.Excluded, a.Path) {
				continue
			}
			if fileExists(filepath.Join(repo, a.Path)) {
				d.SuiteTests = append(d.SuiteTests, a.Path)
				continue
			}
			d.NewTests = append(d.NewTests, a.Path)
			for _, id := range splitIDs(a.Arbitrates) {
				byNew[id] = true
			}
		}
	}
	for _, c := range s.Criteria {
		c.New = c.New || byNew[c.ID]
		d.Criteria = append(d.Criteria, draftCriterion{c.Criterion, c.When, c.Then, string(c.Plan), c.Threshold, c.New, string(c.Procedure)})
	}
	for _, t := range s.Termination {
		d.Termination = append(d.Termination, struct {
			Condition string `json:"condition"`
			Action    string `json:"action"`
		}{t.Condition, t.Action})
	}
	return d
}

// supersede moves what the areas from one on wrote under superseded/<n>/:
// their documents, and the objects they made.
func supersede(dir string, from int) error {
	var moving []string
	for _, s := range artifact.Stages {
		if s.Index() >= from {
			moving = append(moving, string(s)+".json", string(s)+".md")
		}
	}
	if from <= 1 {
		moving = append(moving, artifact.ChangeFile)
	}
	moving = append(moving, artifact.PlanFile, artifact.UnitsDir, "candidates", "evidence", "baseline")
	return moveAside(dir, moving)
}

// moveAside moves what is named, where it is there, under the next
// superseded/<n>/.
func moveAside(dir string, moving []string) error {
	n := 1
	for ; ; n++ {
		if _, err := os.Stat(filepath.Join(dir, "superseded", strconv.Itoa(n))); errors.Is(err, os.ErrNotExist) {
			break
		}
	}
	to := filepath.Join(dir, "superseded", strconv.Itoa(n))
	for _, name := range moving {
		src := filepath.Join(dir, name)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := os.MkdirAll(to, 0o755); err != nil {
			return err
		}
		if err := os.Rename(src, filepath.Join(to, name)); err != nil {
			return err
		}
	}
	return nil
}

// Resume carries a change that stopped — its process ended, it was stopped,
// it failed — on from where it stopped, in its own folder, without paying
// again for what it finished: a specification written and not yet accepted
// is put to the person again as it is; a plan made is carried on from
// admission; units all carried are judged and landed. What the areas it
// carries again wrote is kept under superseded/, and the record says it was
// resumed. A plan written before the model kept its units whole is made
// again.
func Resume(cfg Config, repo, dir string, changed func()) (*Run, error) {
	if Running(dir) {
		return nil, errors.New("the change is being carried now")
	}
	c, err := artifact.Load(dir)
	if err != nil {
		return nil, err
	}
	m := c.Meta
	switch {
	case m.Intent == nil:
		return nil, errors.New("only a change with an intent can be resumed")
	case m.Landing != nil:
		return nil, errors.New("the change has landed; reopen it to change it")
	}
	n, err := strconv.Atoi(strings.TrimPrefix(m.Intent.ID, "INT-"))
	if err != nil {
		return nil, fmt.Errorf("the intent's id %q names no number", m.Intent.ID)
	}
	r := New(cfg, repo, m.Intent.Statement, changed)
	r.dir, r.slug, r.n = c.Dir, c.Name(), n
	if abs, err := filepath.Abs(r.dir); err == nil {
		r.dir = abs
	}
	units, whole := unitsOf(m.Plan)
	switch {
	case m.Specification == nil:
		r.from = 1 // nothing to keep: the specification is written from the start
	case m.Plan == nil:
		r.from, r.resumed = 1, true
		r.spec = draftOf(m.Intent, m.Specification, repo)
	case !whole:
		r.from = 2
		r.spec = draftOf(m.Intent, m.Specification, repo)
	case m.Execution != nil && executed(m.Execution, units):
		r.from = 5
		r.spec, r.units = draftOf(m.Intent, m.Specification, repo), units
		for _, ca := range m.Execution.Candidates {
			r.cands[ca.Unit] = filepath.Join(r.dir, ca.StoredAt, "files")
		}
		r.ledger = m.Execution.Tests
	default:
		r.from = 3
		r.spec, r.units = draftOf(m.Intent, m.Specification, repo), units
		r.mend = mendOf(dir)
	}
	if r.from > 1 { // a specification put to the person again is kept where it is
		if err := supersedeFrom(r.dir, r.from); err != nil {
			return nil, err
		}
	}
	if err := r.record("resumed", m.Intent.ID, fmt.Sprintf("carried on from area %d by %s, after it stopped", r.from, cfg.Person)); err != nil {
		return nil, err
	}
	return r, nil
}

// mendOf is what the person said when they last did not affirm the change,
// if it has not been admitted again since: a run stopped while carrying it
// again is resumed with it.
func mendOf(dir string) string {
	rec, _ := artifact.ReadRecord(dir)
	var why string
	for _, c := range rec {
		switch {
		case c.Stage == "admission":
			why = ""
		case strings.HasPrefix(c.Note, mendNote):
			why = strings.TrimPrefix(c.Note, mendNote)
		}
	}
	return why
}

// unitsOf is a plan's units as a run holds them, and whether the plan kept
// them whole: a plan written before the model kept each unit's ticket did
// not.
func unitsOf(p *meta.Plan) ([]unit, bool) {
	if p == nil || len(p.Units) == 0 {
		return nil, false
	}
	var out []unit
	for _, u := range p.Units {
		if u.Brief == "" {
			return nil, false
		}
		out = append(out, unit{id: u.ID, class: string(u.Class), aoe: u.AreaOfEffect, title: u.Title, brief: u.Brief,
			kind: u.Kind, provides: u.Provides, uses: u.Uses, reads: u.Reads, scope: u.Scope, criteria: u.Criteria,
			preparatoryFor: strings.Join(u.PreparatoryFor, ", "), dependsOn: strings.Join(u.DependsOn, ", ")})
	}
	return out, true
}

// executed reports whether every unit was carried to a candidate, every
// tests unit with its part of the ledger.
func executed(e *meta.Execution, units []unit) bool {
	done := map[string]bool{}
	for _, run := range e.Runs {
		done[run.Unit] = run.Terminal == meta.CandidateEnd && run.Candidate != ""
	}
	inLedger := map[string]bool{}
	for _, t := range e.Tests {
		inLedger[t.Unit] = true
	}
	for _, u := range units {
		// A tests unit carried before the ledger was kept left no record of
		// which test judges what: it is carried again, not guessed at.
		if !done[u.id] || (u.isTests() && !inLedger[u.id]) {
			return false
		}
	}
	return len(units) > 0
}

// supersedeFrom keeps what the areas from one on wrote, and what they made,
// under superseded/ — the candidates only when they are made again.
func supersedeFrom(dir string, from int) error {
	if from >= 5 {
		return supersedeOnly(dir, string(artifact.Assurance), string(artifact.Landing), "evidence")
	}
	if from >= 3 {
		return supersedeOnly(dir, string(artifact.Admission), string(artifact.Execution), string(artifact.Assurance),
			string(artifact.Landing), "candidates", "evidence", "baseline")
	}
	return supersede(dir, from)
}

func supersedeOnly(dir string, names ...string) error {
	var moving []string
	for _, n := range names {
		if strings.HasPrefix(n, "0") {
			moving = append(moving, n+".json", n+".md")
		} else {
			moving = append(moving, n)
		}
	}
	return moveAside(dir, moving)
}

// Close records that a person closed the change in dir: it will not be
// finished, nothing it made entered the estate, and nothing it wrote is
// deleted. A change being carried is stopped first.
func Close(dir, person, why string) error {
	why = strings.TrimSpace(why)
	if why == "" {
		why = "no longer needed"
	}
	if Running(dir) {
		return errors.New("the change is being carried now; stop it first")
	}
	c, err := artifact.Load(dir)
	if err != nil {
		return err
	}
	object := c.Name()
	if c.Meta.Intent != nil {
		object = c.Meta.Intent.ID
	}
	return artifact.Record{Name: c.Name(), Dir: c.Dir}.Append(artifact.Crossing{At: today(), Stage: "closed", Object: object,
		Note: "closed by " + person + ": " + why})
}
