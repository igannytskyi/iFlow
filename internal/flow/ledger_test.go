package flow

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/igannytskyi/iflow/internal/estate"
	"github.com/igannytskyi/iflow/internal/harness"
	"github.com/igannytskyi/iflow/internal/meta"
)

// The ledger holds only as the tests unit wrote its tests: each test in it
// declared in a file of the unit, judging a criterion of the unit, once;
// every test the files declare in it; every criterion judged.
func TestTheLedgerIsHeldToWhatWasWritten(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	src := "package greet\n\nimport \"testing\"\n\nfunc TestShout(t *testing.T) {}\n\nfunc TestWhisper(t *testing.T) {}\n\nfunc FuzzShout(f *testing.F) {}\n\nfunc TestHelper(n int) {}\n\nfunc helper(t *testing.T) {}\n"
	if err := os.WriteFile(filepath.Join(ws, "shout_test.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	u := unit{id: "WU-001-02", kind: meta.TestsUnit, scope: []string{"shout_test.go"}, criteria: []string{"CR-001-01", "CR-001-02"}}
	whole := []ledgerEntry{
		{Test: "TestShout", File: "shout_test.go", Criterion: "CR-001-01"},
		{Test: "FuzzShout", File: "./shout_test.go", Criterion: "CR-001-01"},
		{Test: "TestWhisper", File: "shout_test.go", Criterion: "CR-001-02"},
	}
	got, breaks := ledgerOf(ws, u, whole)
	if len(breaks) > 0 || len(got) != 3 || got[1].File != "shout_test.go" || got[2].Unit != "WU-001-02" {
		t.Fatalf("a ledger that holds was refused: %v %+v", breaks, got)
	}
	for _, c := range []struct {
		entries []ledgerEntry
		want    string
	}{
		{whole[:2], "TestWhisper in shout_test.go is not in the ledger"},
		{whole[:2], "no test in the ledger judges CR-001-02"},
		{append(whole, ledgerEntry{Test: "TestLoud", File: "shout_test.go", Criterion: "CR-001-01"}), "shout_test.go does not declare a test TestLoud"},
		{append(whole, ledgerEntry{Test: "TestHelper", File: "shout_test.go", Criterion: "CR-001-01"}), "does not declare a test TestHelper"},
		{append(whole[:2:2], ledgerEntry{Test: "TestWhisper", File: "shout_test.go", Criterion: "CR-001-09"}), `"CR-001-09", which is not one of this unit's criteria`},
		{append(whole, ledgerEntry{Test: "TestShout", File: "shout_test.go", Criterion: "CR-001-02"}), "TestShout in shout_test.go is in the ledger twice"},
		{append(whole, ledgerEntry{Test: "TestShout", File: "greet.go", Criterion: "CR-001-01"}), "greet.go, which is not a file this unit writes"},
		{nil, "the ledger is empty"},
	} {
		if _, breaks := ledgerOf(ws, u, c.entries); !strings.Contains(strings.Join(breaks, "\n"), c.want) {
			t.Errorf("not said: %q; said: %q", c.want, breaks)
		}
	}
	if _, breaks := (&Run{}).readLedger(ws, u); len(breaks) != 1 || !strings.Contains(breaks[0], "there is no "+LedgerFile) {
		t.Errorf("a missing ledger was not said: %q", breaks)
	}
}

// A criterion is judged at the code unit that serves it last, by the tests
// the ledger says judge it; that unit waits for every other that serves it.
func TestACriterionIsJudgedWhereItIsServedLast(t *testing.T) {
	t.Parallel()
	r := &Run{units: []unit{
		{id: "C", kind: meta.ContractUnit, scope: []string{"greet.go"}},
		{id: "T", kind: meta.TestsUnit, scope: []string{"greet_test.go"}, criteria: []string{"CR-1", "CR-2", "CR-3"}},
		{id: "A", kind: meta.CodeUnit, scope: []string{"a.go"}, criteria: []string{"CR-1", "CR-2"}},
		{id: "B", kind: meta.CodeUnit, scope: []string{"b.go"}, criteria: []string{"CR-2"}},
		{id: "L", kind: meta.CodeUnit, scope: []string{"l.go"}, criteria: []string{"CR-3"}},
	}, ledger: []meta.TestRecord{
		{Test: "TestOne", File: "greet_test.go", Criterion: "CR-1", Unit: "T"},
		{Test: "TestTwo", File: "greet_test.go", Criterion: "CR-2", Unit: "T"},
		{Test: "TestThree", File: "greet_test.go", Criterion: "CR-3", Unit: "T"},
	}}
	for i, want := range map[int][]string{2: {"CR-1"}, 3: {"CR-2"}, 1: nil} {
		if got := r.judgedAt(i); !slices.Equal(got, want) {
			t.Errorf("unit %s judges %v, want %v", r.units[i].id, got, want)
		}
	}
	if got := r.criterionTests(2); !slices.Equal(got, []string{"TestOne"}) {
		t.Errorf("A must pass %v", got)
	}
	if got := r.criterionTests(3); !slices.Equal(got, []string{"TestTwo"}) {
		t.Errorf("B must pass %v", got)
	}
	if deps := r.dependencies(); !slices.Contains(deps[3], 2) {
		t.Errorf("B, judging CR-2, does not wait for A, which serves it too: %v", deps[3])
	}
}

// A failure is blamed on a tests unit through the ledger, exactly: by a
// test's name or a file's path, never a file of a like name.
func TestBlameGoesThroughTheLedger(t *testing.T) {
	t.Parallel()
	r := &Run{units: []unit{
		{id: "T1", kind: meta.TestsUnit, scope: []string{"a/greet_test.go"}},
		{id: "T2", kind: meta.TestsUnit, scope: []string{"b/greet_test.go"}},
		{id: "C", kind: meta.CodeUnit},
	}, ledger: []meta.TestRecord{
		{Test: "TestOne", File: "a/greet_test.go", Criterion: "CR-1", Unit: "T1"},
		{Test: "TestTwo", File: "b/greet_test.go", Criterion: "CR-2", Unit: "T2"},
	}}
	failed := func(tests, files []string) carried {
		return carried{res: harness.Result{Terminal: harness.FailedTask, BlamedTests: tests, Blames: files}}
	}
	for _, c := range []struct {
		c    carried
		want int
	}{
		{failed([]string{"TestTwo"}, nil), 1},
		{failed(nil, []string{"./a/greet_test.go"}), 0},
		{failed(nil, []string{"greet_test.go"}), -1}, // two files of that name: neither is guessed
		{failed([]string{"TestNowhere"}, nil), -1},
	} {
		if got := r.blamed(c.c, nil); got != c.want {
			t.Errorf("%v %v blamed unit %d, want %d", c.c.res.BlamedTests, c.c.res.Blames, got, c.want)
		}
	}
}

// Each criterion stands by its own tests: one failing fails it and no
// other; one that new tests were to judge, with none in the ledger, is not
// met whatever else passed.
func TestEachCriterionStandsByItsOwnTests(t *testing.T) {
	t.Parallel()
	holds := map[string]bool{"CR-1": true, "CR-2": false}
	for _, c := range []struct {
		crit          string
		passed, byNew bool
		want          meta.VerdictOutcome
	}{
		{"CR-1", false, true, meta.Met},   // its tests passed, though another of its unit failed
		{"CR-2", true, true, meta.Failed}, // one of its tests failed, though the run's others passed
		{"CR-3", true, true, meta.Failed}, // new tests were to judge it; the ledger has none
		{"CR-4", true, false, meta.Met},   // judged by the suite, which passed
		{"CR-5", false, false, meta.Failed},
	} {
		if got, _ := verdictOn(c.crit, holds, c.passed, c.byNew); got != c.want {
			t.Errorf("%s: %s, want %s", c.crit, got, c.want)
		}
	}
}

// A code unit is given a file beyond its plan where the specification's
// scope holds it, nothing judges it, and no unit carried apart from it
// writes it.
func TestAUnitIsGivenWhatThePlanMissed(t *testing.T) {
	t.Parallel()
	r := &Run{spec: draft{Scope: []string{"internal/ui/"}, NewTests: []string{"internal/ui/feed_test.go"}}, units: []unit{
		{id: "C", kind: meta.ContractUnit, scope: []string{"internal/ui/feed.go"}},
		{id: "T", kind: meta.TestsUnit, scope: []string{"internal/ui/feed_test.go"}},
		{id: "A", kind: meta.CodeUnit, scope: []string{"internal/ui/remote.go"}},
		{id: "B", kind: meta.CodeUnit, scope: []string{"internal/ui/feed.go"}, reads: []string{"internal/ui/remote.go"}},
		{id: "D", kind: meta.CodeUnit, scope: []string{"internal/ui/app.go"}},
		{id: "L", kind: meta.CodeUnit, scope: []string{"internal/ui/view.go"}},
	}}
	for _, c := range []struct {
		unit int
		path string
		want string
	}{
		{3, "internal/ui/kit.go", ""},                         // nobody's: given
		{3, "internal/ui/remote.go", ""},                      // A's, which B waits for: given
		{3, "internal/ui/app.go", "D writes it"},              // D's, carried apart
		{3, "internal/ui/feed_test.go", "judges the change"},  // a test
		{3, "cmd/iflow/main.go", "outside the specification"}, // beyond the scope
	} {
		got := r.widen(c.unit, c.path)
		if (c.want == "") != (got == "") || !strings.Contains(got, c.want) {
			t.Errorf("%s given %s: %q, want %q", r.units[c.unit].id, c.path, got, c.want)
		}
	}
}

// A planner is handed Go as it is: a struct with its fields, an interface
// with its methods, a function with what it calls.
func TestThePlannerSeesFieldsMethodsAndCalls(t *testing.T) {
	t.Parallel()
	src := []byte("package model\n\ntype Model interface {\n\tName() string\n}\n\ntype UI struct {\n\trunOf func() int\n}\n\nfunc (o OpenAI) Stream(n int) error {\n\treq := o.body(n)\n\treturn send(req)\n}\n")
	got := estate.GoOutline(src)
	for _, want := range []string{"type Model interface", "    Name() string", "type UI struct", "    runOf func() int", "func (o OpenAI) Stream(n int) error   — calls o.body, send"} {
		if !strings.Contains(got, want) {
			t.Errorf("the outline lacks %q:\n%s", want, got)
		}
	}
}
