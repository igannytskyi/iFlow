package measure

import (
	"math/rand/v2"
	"path"
	"slices"
	"strings"

	"github.com/igannytskyi/iflow/internal/estate"
)

// What the estate model is worth, measured rather than asserted. Two figures,
// both about the index rather than about an estate: reach, for comparing two
// indexes and nothing more; and quotation, the share of reported call sites
// whose named line holds the call — what an answer handed to someone to act
// on rests on.

const (
	sample = 40
	seed   = 3
)

// Quotation is how many of the call sites the index reports name the line
// the call is on.
type Quotation struct {
	Sites, Right int
}

// Share is the right share, or -1 where nothing was sampled.
func (q Quotation) Share() float64 {
	if q.Sites == 0 {
		return -1
	}
	return float64(int(1000*float64(q.Right)/float64(q.Sites)+0.5)) / 10
}

// Quote samples names from the index — those in files ending ext, when ext
// is not empty — and checks where their call sites point.
func Quote(r *estate.Repo, ext string) (Quotation, error) {
	ix, err := r.Index()
	if err != nil {
		return Quotation{}, err
	}
	var names []string
	for _, n := range ix.Names {
		if len(n) > 3 && (ext == "" || slices.ContainsFunc(ix.Defines[n], func(f string) bool { return strings.HasSuffix(f, ext) })) {
			names = append(names, n)
		}
	}
	rng := rand.New(rand.NewPCG(seed, seed))
	rng.Shuffle(len(names), func(i, j int) { names[i], names[j] = names[j], names[i] })
	var q Quotation
	for _, name := range names[:min(sample, len(names))] {
		sites, err := r.CallSites(name)
		if err != nil {
			return q, err
		}
		for _, s := range sites[:min(5, len(sites))] {
			if s.Kind != "name" && s.Kind != "attribute" {
				continue
			}
			if ext != "" && !strings.HasSuffix(s.File, ext) {
				continue
			}
			q.Sites++
			if l := r.Excerpt(s.File, s.Line, s.Line, 1); len(l) == 1 && strings.Contains(l[0].Text, name) {
				q.Right++
			}
		}
	}
	return q, nil
}

// Languages are the extensions at least twenty definitions are written in.
func Languages(r *estate.Repo) ([]string, error) {
	ix, err := r.Index()
	if err != nil {
		return nil, err
	}
	count := map[string]int{}
	for _, files := range ix.Defines {
		for _, f := range files {
			count[path.Ext(f)]++
		}
	}
	var out []string
	for ext, n := range count {
		if n >= 20 {
			out = append(out, ext)
		}
	}
	slices.SortFunc(out, func(a, b string) int { return count[b] - count[a] })
	return out, nil
}
