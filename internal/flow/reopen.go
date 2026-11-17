package flow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
		if len(spec.NewTests) == 0 && len(spec.SuiteTests) == 0 {
			return nil, errors.New("this change was specified before the data model kept which tests judge it; reopen it from its specification")
		}
		r.spec = draftOf(in, spec)
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
func draftOf(in *meta.Intent, s *meta.Specification) draft {
	d := draft{Slug: s.Name, Name: s.Name, Statement: in.Statement, Class: string(s.Class), Prose: s.Prose,
		Scope: s.Scope.Included, NewTests: s.NewTests, SuiteTests: s.SuiteTests}
	for _, c := range s.Criteria {
		d.Criteria = append(d.Criteria, struct {
			Criterion string `json:"criterion"`
			When      string `json:"when"`
			Then      string `json:"then"`
			Plan      string `json:"plan"`
			Threshold string `json:"threshold"`
			New       bool   `json:"judged_by_new_tests"`
		}{c.Criterion, c.When, c.Then, string(c.Plan), c.Threshold, c.New})
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
	n := 1
	for ; ; n++ {
		if _, err := os.Stat(filepath.Join(dir, "superseded", strconv.Itoa(n))); errors.Is(err, os.ErrNotExist) {
			break
		}
	}
	to := filepath.Join(dir, "superseded", strconv.Itoa(n))
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
