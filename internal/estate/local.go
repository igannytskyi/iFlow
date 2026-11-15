package estate

import (
	"context"
	"sync"

	"github.com/igannytskyi/iflow/internal/reader"
)

// Local answers the gate's questions from the estate on disk, one Repo per
// root, each derived at most once for the life of the Local. It is safe for
// concurrent use: changes checked side by side share the index of the
// repository they are carried against.
type Local struct {
	mu    sync.Mutex
	repos map[string]*Repo
}

// NewLocal returns an estate read from disk.
func NewLocal() *Local { return &Local{repos: map[string]*Repo{}} }

// Repo returns the repository at root, opened once.
func (l *Local) Repo(root string) *Repo {
	l.mu.Lock()
	defer l.mu.Unlock()
	if r, ok := l.repos[root]; ok {
		return r
	}
	r := Open(root)
	l.repos[root] = r
	return r
}

// Affects implements Estate.
func (l *Local) Affects(ctx context.Context, root string, paths []string) ([]Reach, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rows, err := l.Repo(root).Affects(paths)
	return reaches(rows), err
}

// NamedBy implements Estate.
func (l *Local) NamedBy(ctx context.Context, root string, paths []string) ([]Reach, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rows, err := l.Repo(root).NamedBy(paths)
	return reaches(rows), err
}

func reaches(rows []Reached) []Reach {
	out := make([]Reach, len(rows))
	for i, r := range rows {
		out[i] = Reach{File: r.File, Confidence: r.Confidence}
	}
	return out
}

// Regions implements Estate.
func (l *Local) Regions(ctx context.Context, root string) ([]Region, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rows, err := l.Repo(root).EveryRegion()
	out := make([]Region, len(rows))
	for i, r := range rows {
		out[i] = Region{Path: r.Region, Verdict: r.Verdict, TestGround: r.TestGround}
	}
	return out, err
}

// Corrections implements Estate.
func (l *Local) Corrections(ctx context.Context, root string) ([]Correction, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var out []Correction
	for _, c := range l.Repo(root).Corrections() {
		out = append(out, Correction{By: c.By})
	}
	return out, nil
}

// Readable implements Estate.
func (l *Local) Readable(_ context.Context, path string) (bool, error) {
	return reader.Readable(path), nil
}
