package flow

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/igannytskyi/iflow/internal/meta"
)

// LedgerFile is where a tests unit states its part of the test ledger, in
// the copy it works in: for each test it wrote, its name, its file and the
// criterion it judges. It is read and checked by the framework, kept in the
// execution document, and never lands.
const LedgerFile = "iflow-tests.json"

// ledgerEntry is one line of a tests unit's ledger, as it writes it.
type ledgerEntry struct {
	Test      string `json:"test"`
	File      string `json:"file"`
	Criterion string `json:"criterion"`
}

// ledgerAsk is what a tests unit is told of the ledger it writes.
func ledgerAsk(u unit) string {
	return "\n\n## The test ledger\n\nWhen your tests are written, write `" + LedgerFile + "` at the root of the repository: a JSON array with one object for every test you wrote, " +
		`{"test": its name as the test runner names it, "file": the file it is in, "criterion": the one criterion it judges}. ` +
		"The criteria are " + strings.Join(u.criteria, ", ") + "; each has at least one test, and every test judges one of them. " +
		"The framework holds the ledger to what you wrote: a test not in it, or in it and not in its file, keeps the unit from being done. It is how every later step knows which test judges what; it is not part of the change."
}

// readLedger is a tests unit's ledger as it left it in ws, checked against
// what it wrote: the records, or every way it does not hold.
func (r *Run) readLedger(ws string, u unit) ([]meta.TestRecord, []string) {
	b, err := os.ReadFile(filepath.Join(ws, LedgerFile))
	if err != nil {
		return nil, []string{"there is no " + LedgerFile + ": write it, one object for every test you wrote"}
	}
	var entries []ledgerEntry
	if err := json.Unmarshal(b, &entries); err != nil {
		return nil, []string{LedgerFile + " is not a JSON array of {test, file, criterion}: " + err.Error()}
	}
	return ledgerOf(ws, u, entries)
}

// ledgerOf is entries, a tests unit's ledger, held to what the unit wrote
// in ws: each test declared, as its language declares a test, in a file of
// the unit's; each criterion one of the unit's; every test the unit's files
// declare in it, once; every criterion of the unit judged by one at least.
func ledgerOf(ws string, u unit, entries []ledgerEntry) ([]meta.TestRecord, []string) {
	var breaks []string
	declared := map[string][]string{} // file → the tests it declares
	for _, f := range u.scope {
		src, err := os.ReadFile(filepath.Join(ws, filepath.FromSlash(f)))
		if err != nil {
			continue // a file of the scope not written: no tests of it
		}
		declared[f] = declaredTests(f, src)
	}
	seen := map[[2]string]bool{}
	judged := map[string]bool{}
	var out []meta.TestRecord
	for _, e := range entries {
		e.File = path.Clean(filepath.ToSlash(strings.TrimSpace(e.File)))
		e.Test, e.Criterion = strings.TrimSpace(e.Test), strings.TrimSpace(e.Criterion)
		key := [2]string{e.File, e.Test}
		switch {
		case !slices.Contains(u.scope, e.File):
			breaks = append(breaks, fmt.Sprintf("%s is said to be in %s, which is not a file this unit writes (%s)", e.Test, e.File, strings.Join(u.scope, ", ")))
			continue
		case !slices.Contains(declared[e.File], e.Test):
			breaks = append(breaks, fmt.Sprintf("%s does not declare a test %s", e.File, e.Test))
			continue
		case !slices.Contains(u.criteria, e.Criterion):
			breaks = append(breaks, fmt.Sprintf("%s is said to judge %q, which is not one of this unit's criteria (%s)", e.Test, e.Criterion, strings.Join(u.criteria, ", ")))
			continue
		case seen[key]:
			breaks = append(breaks, fmt.Sprintf("%s in %s is in the ledger twice; a test judges one criterion", e.Test, e.File))
			continue
		}
		seen[key], judged[e.Criterion] = true, true
		out = append(out, meta.TestRecord{Test: e.Test, File: e.File, Criterion: e.Criterion, Unit: u.id})
	}
	for _, f := range u.scope {
		for _, t := range declared[f] {
			if !seen[[2]string{f, t}] {
				breaks = append(breaks, fmt.Sprintf("%s in %s is not in the ledger: say which criterion it judges", t, f))
			}
		}
	}
	for _, c := range u.criteria {
		if !judged[c] {
			breaks = append(breaks, fmt.Sprintf("no test in the ledger judges %s", c))
		}
	}
	if len(entries) == 0 {
		breaks = append(breaks, "the ledger is empty")
	}
	return out, breaks
}

// declaredTests is the tests file declares, read by its language's own
// parser where the framework has one: for Go, every top-level function
// named Test or Fuzz that takes *testing.T or *testing.F.
func declaredTests(file string, src []byte) []string {
	if filepath.Ext(file) != ".go" {
		return testsIn(file, string(src))
	}
	f, err := parser.ParseFile(token.NewFileSet(), file, src, parser.SkipObjectResolution)
	if err != nil {
		return testsIn(file, string(src)) // a file that does not parse fails its build, which says why
	}
	var out []string
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
			continue
		}
		name := fn.Name.Name
		want := ""
		switch {
		case strings.HasPrefix(name, "Test"):
			want = "T"
		case strings.HasPrefix(name, "Fuzz"):
			want = "F"
		default:
			continue
		}
		if star, ok := fn.Type.Params.List[0].Type.(*ast.StarExpr); ok {
			if sel, ok := star.X.(*ast.SelectorExpr); ok && sel.Sel.Name == want {
				out = append(out, name)
			}
		}
	}
	return out
}

// ledgerJudge is a tests unit's check, judged also by its ledger: the tests
// build, and the ledger holds.
func (r *Run) ledgerJudge(ws string, u unit) func(bool, string) (bool, string) {
	return func(passed bool, out string) (bool, string) {
		if !passed {
			return false, ""
		}
		if _, breaks := r.readLedger(ws, u); len(breaks) > 0 {
			return false, "The tests build, but the test ledger " + LedgerFile + " does not hold:\n\n- " + strings.Join(breaks, "\n- ") +
				"\n\nMend the ledger, or the tests, so that each test is in it once with the criterion it judges."
		}
		return true, ""
	}
}

// ledgerTests is the names of the tests in the ledger kept so far, of the
// units named, or of every tests unit where none are.
func (r *Run) ledgerTests(units ...string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var names []string
	for _, t := range r.ledger {
		if len(units) == 0 || slices.Contains(units, t.Unit) {
			names = append(names, t.Test)
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// ledgerOf is the records of the ledger the tests unit named wrote.
func (r *Run) ledgerOf(unit string) []meta.TestRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []meta.TestRecord
	for _, t := range r.ledger {
		if t.Unit == unit {
			out = append(out, t)
		}
	}
	return out
}
