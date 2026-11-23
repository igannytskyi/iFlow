package flow

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/igannytskyi/iflow/internal/meta"
)

// The ledger holds only as the tests unit wrote its tests: each test in it
// declared in a file of the unit, judging a criterion of the unit, once;
// every test the files declare in it; every criterion judged.
func TestTheLedgerIsHeldToWhatWasWritten(t *testing.T) {
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
