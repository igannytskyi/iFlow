package service

import (
	"os"
	"path/filepath"
	"slices"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/gate"
	"github.com/igannytskyi/iflow/internal/method"
	"github.com/igannytskyi/iflow/internal/protocol"
	"github.com/igannytskyi/iflow/internal/workspace"
)

// Catalog is the projects at paths and their changes, each as its folder
// says it stands, newest first.
func Catalog(paths []string) []protocol.Project {
	out := make([]protocol.Project, 0, len(paths))
	for _, path := range paths {
		p := protocol.Project{Path: path, Name: filepath.Base(path)}
		for _, dir := range workspace.Folders(filepath.Join(path, "changes")) {
			if c, ok := changeOf(dir); ok {
				p.Changes = append(p.Changes, c)
			}
		}
		slices.SortStableFunc(p.Changes, func(a, b protocol.Change) int { return b.Modified.Compare(a.Modified) })
		out = append(out, p)
	}
	return out
}

// changeOf is the change in dir as its folder says it stands.
func changeOf(dir string) (protocol.Change, bool) {
	c, err := artifact.Load(dir)
	if err != nil {
		return protocol.Change{}, false
	}
	out := protocol.Change{Dir: dir, Name: c.Name()}
	if c.Meta.Intent != nil {
		out.Statement = c.Meta.Intent.Statement
	}
	for _, s := range c.Present() {
		out.Reached = max(out.Reached, method.StageArea[s])
	}
	if st, err := os.Stat(dir); err == nil {
		out.Modified = st.ModTime()
	}
	_, closed := gate.Closed(c)
	switch {
	case flow.Running(dir):
		out.State = protocol.Carried
	case closed:
		out.State = protocol.Closed
	case out.Reached == 6:
		out.State = protocol.Landed
	default:
		out.State = protocol.Stopped
	}
	return out, true
}

// record is a change's record, as its folder keeps it.
func record(dir string) ([]protocol.Crossing, error) {
	rec, err := artifact.ReadRecord(dir)
	if err != nil {
		return nil, err
	}
	out := make([]protocol.Crossing, len(rec))
	for i, x := range rec {
		out[i] = protocol.Crossing{N: x.N, At: x.At, Stage: x.Stage, Object: x.Object, Note: x.Note}
	}
	return out, nil
}
