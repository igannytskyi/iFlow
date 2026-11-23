package service

import (
	"path/filepath"

	"github.com/igannytskyi/iflow/internal/flow"
)

// FlowMaker makes runs as the path carries any, with what Config answers:
// asked again for every run, so that a run takes the models chosen when it
// starts.
type FlowMaker struct{ Config func() flow.Config }

// New starts a task on a project.
func (m FlowMaker) New(project, statement string, settles []string, changed func()) (Run, error) {
	r := flow.New(m.Config(), project, statement, changed)
	r.Settles(settles...)
	return r, nil
}

// Resume carries a stopped change on.
func (m FlowMaker) Resume(dir string, settles []string, changed func()) (Run, error) {
	r, err := flow.Resume(m.Config(), projectOf(dir), dir, changed)
	if err != nil {
		return nil, err
	}
	r.Settles(settles...)
	return r, nil
}

// Reopen carries a change again from its specification or its plan.
func (m FlowMaker) Reopen(dir string, from int, notes string, settles []string, changed func()) (Run, error) {
	r, err := flow.Reopen(m.Config(), projectOf(dir), dir, from, notes, changed)
	if err != nil {
		return nil, err
	}
	r.Settles(settles...)
	return r, nil
}

var _ Maker = FlowMaker{}

// projectOf is the estate a change's folder belongs to: <estate>/changes/<slug>.
func projectOf(dir string) string { return filepath.Dir(filepath.Dir(dir)) }
