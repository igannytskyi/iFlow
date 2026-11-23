// Package ledger is the method's debt to itself: every escalation raised by a
// change, minus those a later change has settled. An escalation is addressed
// as <slug>#<n>, because its number is an ordinal local to the change that
// raised it.
package ledger

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/igannytskyi/iflow/internal/artifact"
)

// Owed is an escalation still open, and its ground.
type Owed struct {
	Ref    string
	Ground string
}

// Ledger is what every change under a repository raised and settled.
type Ledger struct {
	// Owed is every escalation still open, by address.
	Owed []Owed
	// Settled maps an escalation to the change that settled it.
	Settled map[string]string
	// Problems are breaches of R15: settling twice, or settling what was
	// never owed.
	Problems []string
}

// Read builds the ledger from the changes under root/changes.
func Read(root string) (*Ledger, error) {
	type raised struct{ standing, ground string }
	owedBy := map[string]raised{}
	l := &Ledger{Settled: map[string]string{}}
	var order []string
	entries, err := os.ReadDir(filepath.Join(root, "changes"))
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, "changes", e.Name())
		if _, err := os.Stat(filepath.Join(dir, string(artifact.Assurance)+".md")); errors.Is(err, fs.ErrNotExist) {
			if _, err := os.Stat(artifact.JSONFile(dir, artifact.Assurance)); errors.Is(err, fs.ErrNotExist) {
				continue // nothing raised or settled here
			}
		}
		c, err := artifact.Load(dir)
		if err != nil {
			return nil, err
		}
		as := c.Meta.Assurance
		if as == nil {
			continue
		}
		for _, x := range as.Raised {
			if x.N > 0 {
				owedBy[fmt.Sprintf("%s#%d", e.Name(), x.N)] = raised{x.Standing, x.Ground}
			}
		}
		for _, x := range as.Settled {
			ref := x.Escalation
			if ref == "" {
				continue
			}
			if prev, twice := l.Settled[ref]; twice {
				l.Problems = append(l.Problems, fmt.Sprintf("R15: %s is settled twice — in %s and in %s", ref, prev, e.Name()))
			} else {
				order = append(order, ref)
			}
			l.Settled[ref] = e.Name()
		}
	}
	for _, ref := range order {
		if _, ok := owedBy[ref]; !ok {
			l.Problems = append(l.Problems, fmt.Sprintf("R15: %s settles %s, which was never owed", l.Settled[ref], ref))
		}
	}
	for _, ref := range slices.Sorted(maps.Keys(owedBy)) {
		if r := owedBy[ref]; r.standing == "open" {
			if _, settled := l.Settled[ref]; !settled {
				l.Owed = append(l.Owed, Owed{ref, r.ground})
			}
		}
	}
	return l, nil
}

var ruleRef = regexp.MustCompile(`\b(R\d+b?|B\d)\b`)

// Shared is every rule named by more than one escalation still owed, which is
// how one defect looks like several.
func (l *Ledger) Shared() []Shared {
	by := map[string][]string{}
	for _, o := range l.Owed {
		rules := ruleRef.FindAllString(o.Ground, -1)
		slices.Sort(rules)
		for _, r := range slices.Compact(rules) {
			by[r] = append(by[r], o.Ref)
		}
	}
	var out []Shared
	for rule, refs := range by {
		if len(refs) > 1 {
			out = append(out, Shared{rule, refs})
		}
	}
	slices.SortFunc(out, func(a, b Shared) int { return cmp.Compare(a.Rule, b.Rule) })
	return out
}

// Shared is a rule and the escalations that name it.
type Shared struct {
	Rule string
	Refs []string
}
