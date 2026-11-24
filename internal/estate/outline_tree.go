package estate

import (
	"cmp"
	"slices"
	"strings"

	"github.com/igannytskyi/iflow/internal/reader"
)

// TreeOutline is a file of any language its tree is read in, as an agent
// deciding or changing code needs it without reading it, as GoOutline is a
// Go file: each definition as it is declared, without its body, with the
// fields and the definitions within it beneath it — a class with its fields
// and methods — and each with the functions and methods it calls.
func TreeOutline(f reader.File) string {
	return outlineWithin(f, 0, 1<<30)
}

// outlineWithin is the outline of what f defines between the lines from
// and to.
func outlineWithin(f reader.File, from, to int) string {
	type item struct {
		reader.Symbol
		def bool
	}
	var items []item
	for _, s := range f.Symbols {
		if s.Start >= from && s.End <= to {
			items = append(items, item{s, true})
		}
	}
	for _, s := range f.Fields {
		if s.Start >= from && s.End <= to {
			items = append(items, item{s, false})
		}
	}
	// Outer first: by where each begins, the wider of two at one line first.
	slices.SortStableFunc(items, func(a, b item) int {
		return cmp.Or(cmp.Compare(a.Start, b.Start), cmp.Compare(b.End, a.End), cmp.Compare(btoi(b.def), btoi(a.def)))
	})
	defs := slices.DeleteFunc(slices.Clone(items), func(i item) bool { return !i.def })
	within := func(in, of item) bool {
		return in.Symbol != of.Symbol && of.Start <= in.Start && in.End <= of.End && (of.Start < in.Start || in.End < of.End || !in.def)
	}
	// innermost is the narrowest definition around a line, or -1.
	innermost := func(l int) int {
		at := -1
		for i, d := range defs {
			if d.Start <= l && l <= d.End && (at < 0 || d.End-d.Start <= defs[at].End-defs[at].Start) {
				at = i
			}
		}
		return at
	}
	calls := make([][]string, len(defs))
	for _, c := range f.Calls {
		if c.Kind == reader.Import {
			continue
		}
		if i := innermost(c.Line); i >= 0 && !slices.Contains(calls[i], c.Name) && len(calls[i]) < 12 {
			calls[i] = append(calls[i], c.Name)
		}
	}
	seen := map[reader.Symbol]bool{}
	var b strings.Builder
	for _, it := range items {
		if seen[it.Symbol] {
			continue
		}
		seen[it.Symbol] = true
		depth := 0
		for _, d := range defs {
			if within(it, d) {
				depth++
			}
		}
		b.WriteString(strings.Repeat("  ", depth+1) + cmp.Or(it.Head, it.Name))
		if it.def {
			if i := slices.IndexFunc(defs, func(d item) bool { return d.Symbol == it.Symbol }); i >= 0 && len(calls[i]) > 0 {
				b.WriteString("   — calls " + strings.Join(calls[i], ", "))
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// treeFile is what the index holds of a source, as the reader read it.
func (r *Repo) treeFile(rel string) (reader.File, bool) {
	ix, err := r.Index()
	if err != nil || ix.Files[rel] == nil {
		return reader.File{}, false
	}
	rec := ix.Files[rel]
	return reader.File{Symbols: rec.Symbols, Fields: rec.Fields, Calls: rec.Calls}, true
}
