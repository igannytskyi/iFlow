package estate

import (
	"path"
	"slices"
	"strings"

	"github.com/igannytskyi/iflow/internal/reader"
)

func stem(rel string) string {
	base := path.Base(rel)
	if ext := path.Ext(base); ext != "" && ext != base {
		return strings.TrimSuffix(base, ext)
	}
	return base
}

func parentName(rel string) string {
	dir := path.Dir(rel)
	if dir == "." {
		return ""
	}
	return path.Base(dir)
}

type edgeSet = map[string]map[string]bool

func add(m edgeSet, from, to string) {
	if m[from] == nil {
		m[from] = map[string]bool{}
	}
	m[from][to] = true
}

// Importers is which file imports which: for each file, the files importing
// it. An import statement is one destination and several guesses at it, so
// both go in: the most specific tail naming somewhere here is the firm edge,
// the coarser ones are weak, and which an answer uses is the answer's
// business. A graph narrowed to suit one query is no longer of the estate.
func (r *Repo) Importers() (all, firm edgeSet, err error) {
	ix, err := r.Index()
	if err != nil {
		return nil, nil, err
	}
	r.graphOnce.Do(func() { r.edges, r.firm = r.importers(ix) })
	return r.edges, r.firm, nil
}

func (r *Repo) importers(ix *Index) (edgeSet, edgeSet) {
	// A module is a file in some languages and a directory in others, and an
	// import naming a place reaches what is there rather than everything of
	// that name anywhere — so every tail of every path is a key.
	byModule := map[string]map[string]bool{}
	put := func(k, rel string) {
		if byModule[k] == nil {
			byModule[k] = map[string]bool{}
		}
		byModule[k][rel] = true
	}
	for _, rel := range r.Sources() {
		put(stem(rel), rel)
		put(parentName(rel), rel)
		parts := strings.Split(rel, "/")
		for i := 0; i < len(parts)-1; i++ {
			put(strings.Join(parts[i:len(parts)-1], "/"), rel)
			put(strings.Join(append(slices.Clone(parts[i:len(parts)-1]), stem(rel)), "/"), rel)
		}
	}
	type match struct {
		key     string
		targets map[string]bool
	}
	// Everywhere a name could mean, longest tail first.
	resolve := func(name string) []match {
		var out []match
		if t, ok := byModule[name]; ok {
			out = append(out, match{name, t})
		}
		parts := strings.Split(name, "/")
		for i := 1; i < len(parts); i++ {
			tail := strings.Join(parts[i:], "/")
			if t, ok := byModule[tail]; ok {
				out = append(out, match{tail, t})
			}
		}
		return out
	}
	edges, firm := edgeSet{}, edgeSet{}
	for _, rel := range r.Sources() {
		rec := ix.Files[rel]
		if rec == nil {
			continue
		}
		byLine := map[int][]string{}
		var lines []int
		for _, c := range rec.Calls {
			if c.Kind != reader.Import {
				continue
			}
			if _, ok := byLine[c.Line]; !ok {
				lines = append(lines, c.Line)
			}
			byLine[c.Line] = append(byLine[c.Line], c.Name)
		}
		for _, l := range lines {
			names := slices.Clone(byLine[l])
			slices.SortStableFunc(names, func(a, b string) int {
				return strings.Count(b, "/") - strings.Count(a, "/")
			})
			best := ""
			for _, name := range names {
				for _, m := range resolve(name) {
					for _, target := range sortedSet(m.targets) {
						if target == rel {
							continue
						}
						add(edges, target, rel)
						if best == "" || m.key == best {
							best = m.key
							add(firm, target, rel)
						}
					}
				}
			}
		}
	}
	return edges, firm
}

func sortedSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// Transitive is everything reached from the seeds over the edges, with the
// fewest hops it takes. Reach is not one hop: reporting only what touches a
// change directly understates the blast radius, and understating it is the
// dangerous direction — what is not reported is what nobody re-tests.
func Transitive(seeds []string, edges edgeSet) map[string]int {
	hops := map[string]int{}
	seen := map[string]bool{}
	frontier := slices.Clone(seeds)
	for _, s := range seeds {
		seen[s] = true
	}
	for depth := 1; len(frontier) > 0; depth++ {
		var next []string
		for _, cur := range frontier {
			for _, n := range sortedSet(edges[cur]) {
				if !seen[n] {
					seen[n] = true
					hops[n] = depth
					next = append(next, n)
				}
			}
		}
		frontier = next
	}
	return hops
}
