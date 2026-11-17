package gate

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/ledger"
)

// Standing is one change at a glance: how far it got, and what holds it.
type Standing struct {
	Name      string
	Reached   string
	Problems  int
	Held      int
	Waiting   int
	Undecided int
	Owed      int
}

// changeDirs lists the change folders directly under where, sorted.
func changeDirs(where string) []string {
	entries, err := os.ReadDir(where)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && exists(filepath.Join(where, e.Name(), string(artifact.Intent)+".md")) {
			out = append(out, e.Name())
		}
	}
	slices.Sort(out)
	return out
}

// Status checks every change under where, concurrently and at most one per
// processor, and returns them in name order.
func (g *Gate) Status(ctx context.Context, where string) ([]Standing, error) {
	names := changeDirs(where)
	out := make([]Standing, len(names))
	errs := make([]error, len(names))
	limit := make(chan struct{}, runtime.GOMAXPROCS(0))
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Go(func() {
			limit <- struct{}{}
			defer func() { <-limit }()
			c, err := artifact.Load(filepath.Join(where, name))
			if err != nil {
				errs[i] = err
				return
			}
			out[i] = g.standing(ctx, c)
		})
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	// What a change still owes is what it raised that no change — itself, a
	// later one, a person's decision — has settled: the ledger's to say.
	if abs, err := filepath.Abs(where); err == nil {
		if l, err := ledger.Read(filepath.Dir(abs)); err == nil {
			owed := map[string]int{}
			for _, o := range l.Owed {
				name, _, _ := strings.Cut(o.Ref, "#")
				owed[name]++
			}
			for i := range out {
				out[i].Owed = owed[out[i].Name]
			}
		}
	}
	return out, ctx.Err()
}

func (g *Gate) standing(ctx context.Context, c *artifact.Change) Standing {
	s := Standing{Name: c.Name(), Reached: "-", Problems: len(g.Check(ctx, c).Problems())}
	if p := c.Present(); len(p) > 0 {
		s.Reached = string(p[len(p)-1])
	}
	if ad := c.Meta.Admission; ad != nil {
		for _, d := range ad.Decisions {
			switch d.Outcome {
			case "held":
				s.Held++
			case "awaiting-authority":
				s.Waiting++
			}
		}
	}
	if as := c.Meta.Assurance; as != nil {
		for _, v := range as.Verdicts {
			if v.Outcome == "undecided" {
				s.Undecided++
			}
		}
	}
	return s
}
