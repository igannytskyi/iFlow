package estate

import (
	"bytes"
	"cmp"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/igannytskyi/iflow/internal/fsx"
	"github.com/igannytskyi/iflow/internal/reader"
)

// Grade is how far a statement is to be trusted, and why.
type Grade struct {
	Provenance string
	Confidence string
}

// The grades an answer can carry.
var (
	derivedTotal   = Grade{"derived", "high"}   // the parser saw it and could not be wrong
	derivedPartial = Grade{"derived", "medium"} // the parser saw it, within known blind spots
	matched        = Grade{"matched", "medium"} // resolved by a name, which can collide
	matchedWeak    = Grade{"matched", "low"}    // resolved by a name several things answer to
)

var rank = map[string]int{"high": 0, "medium": 1, "low": 2}

// Reached is one file a change reaches, through what, and how firmly.
type Reached struct {
	File       string   `json:"file"`
	Through    []string `json:"through"`
	Provenance string   `json:"provenance"`
	Confidence string   `json:"confidence"`
	ImportsIt  bool     `json:"imports it"`
	Why        string   `json:"why,omitempty"`
	How        string   `json:"how,omitempty"`
}

func wants(paths []string) map[string]bool {
	out := map[string]bool{}
	for _, p := range paths {
		out[filepath.ToSlash(filepath.Clean(p))] = true
	}
	return out
}

func underAny(f string, wanted map[string]bool) bool {
	for w := range wanted {
		if f == w || strings.HasPrefix(f, w) {
			return true
		}
	}
	return false
}

// Affects is what a change to these paths reaches, and how far to trust each
// answer. What a landing showed comes first and cannot be outranked: it
// happened.
func (r *Repo) Affects(paths []string) ([]Reached, error) {
	ix, err := r.Index()
	if err != nil {
		return nil, err
	}
	all, _, err := r.Importers()
	if err != nil {
		return nil, err
	}
	wanted := wants(paths)
	changed := map[string]bool{}
	for name, where := range ix.Defines {
		if slices.ContainsFunc(where, func(w string) bool { return underAny(w, wanted) }) {
			changed[name] = true
		}
	}
	// Two files can share a method name and have nothing to do with each
	// other. Code that is never imported is code that cannot be called, so
	// the import graph separates a dependency from a coincidence of
	// vocabulary — without deleting the coincidences, since a framework may
	// load what nothing imports.
	linked := Transitive(sortedSet(wanted), all)
	type hit struct {
		name  string
		grade Grade
	}
	reached := map[string][]hit{}
	for _, ref := range ix.Refs {
		if !changed[ref.Name] || wanted[ref.From] {
			continue
		}
		where := ix.Defines[ref.Name]
		_, here := linked[ref.From]
		var g Grade
		switch {
		case ref.Kind == reader.Import:
			g = derivedPartial
		case ref.Kind == reader.Typed:
			// The file said what the receiver is, so this is read off it
			// rather than guessed: the firmest thing short of an import.
			g = Grade{"derived", "medium"}
			if here && len(where) == 1 {
				g = derivedTotal
			}
		case ref.Kind == reader.Attribute:
			g = matchedWeak // the receiver's type is unknown
		case len(where) > 1:
			g = matchedWeak // several things answer to this name
		case here:
			g = matched
		default:
			g = matchedWeak // nothing imports it: the name may be all
		}
		reached[ref.From] = append(reached[ref.From], hit{ref.Name, g})
	}
	var out []Reached
	taught := map[string]bool{}
	learnt, err := r.Learned(paths)
	if err != nil {
		return nil, err
	}
	for _, t := range learnt {
		if !wanted[t.File] && !taught[t.File] {
			taught[t.File] = true
			out = append(out, t)
		}
	}
	for _, rel := range sortedKeys(reached) {
		if taught[rel] {
			continue
		}
		hits := reached[rel]
		var names []string
		best := hits[0].grade
		for _, h := range hits {
			names = append(names, h.name)
			if rank[h.grade.Confidence] < rank[best.Confidence] {
				best = h.grade
			}
		}
		slices.Sort(names)
		_, imports := linked[rel]
		out = append(out, Reached{File: rel, Through: slices.Compact(names), Provenance: best.Provenance,
			Confidence: best.Confidence, ImportsIt: imports})
	}
	slices.SortStableFunc(out, func(a, b Reached) int {
		return cmp.Or(cmp.Compare(rank[a.Confidence], rank[b.Confidence]), cmp.Compare(a.File, b.File))
	})
	return out, nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// NamedBy is every file naming these paths as text rather than calling into
// them. The strongest dependency can be invisible to a parser — a gate run as
// a process, addressed by a path in a string — and what is found this way is
// weaker, so it is labelled as what it is.
func (r *Repo) NamedBy(paths []string) ([]Reached, error) {
	wanted := wants(paths)
	var needles []string
	for w := range wanted {
		needles = append(needles, w, filepath.Base(w))
	}
	var out []Reached
	for _, rel := range r.Sources() {
		if wanted[rel] {
			continue
		}
		text, err := os.ReadFile(r.abs(rel))
		if err != nil {
			return nil, &Error{Kind: "OSError"}
		}
		hits := map[string]bool{}
		for _, n := range needles {
			if bytes.Contains(text, []byte(n)) {
				hits[n] = true
			}
		}
		if len(hits) > 0 {
			out = append(out, Reached{File: rel, Through: sortedKeys(hits), Provenance: "matched",
				Confidence: "low", How: "named as text, not called"})
		}
	}
	return out, nil
}

// ------------------------------------------------- what landings taught
//
// Everything above is derived, and derivation has a blind spot it cannot see
// from inside: a dependency nothing writes down. A landing that had to touch
// ground the model never connected to its scope is reality saying an edge
// exists — the one correction this representation can take. So this is the
// one thing here kept rather than derived: delete the cache and it is
// rebuilt; delete these and what the estate learned from its history is gone.

// CorrectionRecord is one edge a landing showed.
type CorrectionRecord struct {
	From       []string `json:"from"`
	To         []string `json:"to"`
	By         string   `json:"by"`
	Why        string   `json:"why"`
	Provenance string   `json:"provenance"`
	Confidence string   `json:"confidence"`
	At         string   `json:"at"`
}

func (r *Repo) correctionsFile() string { return filepath.Join(r.Home(), "corrections.json") }

// Corrections is every edge landings showed here.
func (r *Repo) Corrections() []CorrectionRecord {
	var out []CorrectionRecord
	b, err := os.ReadFile(r.correctionsFile())
	if err != nil || json.Unmarshal(b, &out) != nil {
		return nil
	}
	return out
}

// Correct records that work on this ground had to touch that, though
// nothing here says so. Stated as what it is: observed, not derived.
func (r *Repo) Correct(scope, reached []string, by, why string) ([]CorrectionRecord, error) {
	held := r.Corrections()
	e := CorrectionRecord{From: sortedCopy(scope), To: sortedCopy(reached), By: by, Why: why,
		Provenance: "observed", Confidence: "high", At: time.Now().Format("2006-01-02")}
	for _, c := range held {
		if slices.Equal(c.From, e.From) && slices.Equal(c.To, e.To) && c.By == by {
			return held, nil
		}
	}
	held = append(held, e)
	b, err := json.MarshalIndent(held, "", " ")
	if err != nil {
		return nil, err
	}
	return held, fsx.WriteAtomic(r.correctionsFile(), b, 0o644)
}

func sortedCopy(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	return out
}

// Learned is what landings have shown a change to these paths reaches.
func (r *Repo) Learned(paths []string) ([]Reached, error) {
	wanted := wants(paths)
	var out []Reached
	for _, c := range r.Corrections() {
		if !slices.ContainsFunc(c.From, func(f string) bool { return underAny(f, wanted) }) {
			continue
		}
		for _, to := range c.To {
			out = append(out, Reached{File: to, Through: []string{"landed with " + c.By}, Provenance: "observed",
				Confidence: "high", ImportsIt: true, Why: c.Why})
		}
	}
	return out, nil
}
