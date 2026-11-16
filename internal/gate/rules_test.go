package gate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/estate"
)

// These are the gate's arbiters. One change is built that passes, from the
// shapes the method defines, and then each rule is broken in it one at a
// time: the gate must name exactly the rule that was broken. A rule whose
// breaking nothing here notices is not being tested — which is what
// iflow check --mutate finds out by suppressing each rule and running these.

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixture builds an estate holding one landed change that passes, and
// returns the estate's root and the change's folder.
func fixture(t *testing.T) (root, dir string) {
	t.Helper()
	root = t.TempDir()
	write(t, filepath.Join(root, "src", "a.go"), "package src\n\nfunc A() int { return 1 }\n")
	write(t, filepath.Join(root, "src", "a_test.go"), "package src\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {\n\tif A() != 1 {\n\t\tt.Fatal()\n\t}\n}\n")
	dir = filepath.Join(root, "changes", "c")
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(dir, 0o755))
	must(artifact.Write(dir, artifact.Intent, artifact.IntentDoc{Name: "c", ID: "INT-001", StatedBy: "a person", Authority: "a person",
		Priority: "normal", Deadline: "none", Statement: "A answers one.", Status: "open"}.Markdown()))
	must(artifact.Write(dir, artifact.Specification, artifact.SpecDoc{Name: "c", ID: "SPEC-001", Intent: "INT-001", Class: "C3",
		Criteria: []artifact.CriterionRow{
			{ID: "CR-001-01", Criterion: "The answer is one", When: "A is asked", Then: "it answers one", Plan: "required", Procedure: "machine", Evidence: "test-run", Threshold: "exactly"},
			{ID: "CR-001-02", Criterion: "Nothing else moves", When: "the suite is run", Then: "it stands", Plan: "not-required", Procedure: "machine", Evidence: "test-run", Threshold: "identical"},
		},
		Termination: [][2]string{{"one cannot be answered", "stop"}},
		Included:    []string{"src/a.go"}, Excluded: []string{"src/a_test.go"},
		Arbiter: []artifact.ArbiterRow{{Path: "src/a_test.go", Arbitrates: "CR-001-01, CR-001-02"}},
		Inputs:  [][2]string{{"src/a.go", "yes"}},
	}.Markdown()))
	must(artifact.Write(dir, artifact.Plan, artifact.PlanDoc{Name: "c", Spec: "SPEC-001", Phases: "1", Intermediate: "yes", Residue: "none",
		Coverage: "The unit covers both criteria.",
		Units:    []artifact.UnitRow{{ID: "WU-001-01", Phase: "1", Class: "C3", Scope: "src/a.go", AreaOfEffect: "src, derived, high", Criteria: "CR-001-01, CR-001-02"}},
	}.Markdown()))
	digest, err := artifact.DigestFile(filepath.Join(dir, "01-specification.md"))
	must(err)
	write(t, filepath.Join(dir, "baseline", "a.go.before"), "package src\n")
	must(artifact.Write(dir, artifact.Admission, artifact.AdmissionDoc{Name: "c", Digest: digest,
		Review:       [4]string{"yes", "a person", "2026-11-15", ""},
		Decisions:    [][4]string{{"WU-001-01", "admitted", "permission", "high"}},
		EvidencePlan: [][6]string{{"CR-001-01", "src/a.go", "the test, before and after", "it answers as before", "the area of effect of src/a.go, derived, high", "2026-11-15"}},
		Captured:     [][3]string{{"CR-001-01", "src/a.go as it stood", "baseline/a.go.before"}},
	}.Markdown()))
	write(t, filepath.Join(dir, "candidates", "CA-001-01", "a.go"), "package src\n\nfunc A() int { return 1 }\n")
	must(artifact.Write(dir, artifact.Execution, artifact.ExecutionDoc{Name: "c",
		Runs:       []artifact.RunRow{{Unit: "WU-001-01", Executor: "an agent", Version: "1", Terminal: "candidate", Candidate: "CA-001-01"}},
		Candidates: []artifact.CandidateRow{{ID: "CA-001-01", Unit: "WU-001-01", StoredAt: "candidates/CA-001-01/", Artefacts: "the answer", ProducedAt: "2026-11-15"}},
	}.Markdown()))
	must(artifact.Write(dir, artifact.Assurance, artifact.AssuranceDoc{Name: "c",
		Evidence: []artifact.EvidenceRow{{ID: "EV-001-01-01", Subject: "candidate", Kind: "test-run", Producer: "`go test ./...`",
			Independence: "independent-by-precommitment", How: "written and failing before the candidate", Repeatable: "yes", ObtainedAt: "2026-11-15"}},
		Verdicts: []artifact.VerdictRow{
			{ID: "VE-001-01", Unit: "WU-001-01", Criterion: "CR-001-01", Outcome: "met", Evidence: "EV-001-01-01", State: "settled"},
			{ID: "VE-001-02", Unit: "WU-001-01", Criterion: "CR-001-02", Outcome: "met", Evidence: "EV-001-01-01", State: "settled"},
		},
	}.Markdown()))
	must(artifact.Write(dir, artifact.Landing, artifact.LandingDoc{Name: "c",
		Entries: []artifact.EntryRow{{Order: "1", Unit: "WU-001-01", VerdictState: "settled", StillValid: "yes", EnteredAt: "2026-11-15"}},
	}.Markdown()))
	rec := artifact.Record{Name: "c", Dir: dir}
	for _, s := range []artifact.Crossing{{At: "2026-11-15", Stage: "intent", Object: "INT-001"}, {At: "2026-11-15", Stage: "specification", Object: "SPEC-001"},
		{At: "2026-11-15", Stage: "plan", Object: "SPEC-001"}, {At: "2026-11-15", Stage: "admission", Object: "WU-001-01"}, {At: "2026-11-15", Stage: "execution", Object: "CA-001-01"},
		{At: "2026-11-15", Stage: "assurance", Object: "VE-001-01"}, {At: "2026-11-15", Stage: "landing", Object: "WU-001-01"}} {
		must(rec.Append(s))
	}
	return root, dir
}

func gateOf(t *testing.T, dir string, opts ...Option) Report {
	t.Helper()
	c, err := artifact.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return New(estate.NewLocal(), append([]Option{WithHome(t.TempDir())}, opts...)...).Check(context.Background(), c)
}

func edit(t *testing.T, path, old, new string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), old) {
		t.Fatalf("%s does not hold %q", filepath.Base(path), old)
	}
	write(t, path, strings.Replace(string(b), old, new, 1))
}

func rules(fs []Finding) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Rule)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func needGo(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("the fixture's evidence names go, which is not installed here")
	}
}

func TestAChangeBuiltFromTheShapesPasses(t *testing.T) {
	needGo(t)
	_, dir := fixture(t)
	if rep := gateOf(t, dir); !rep.Passed() {
		t.Fatalf("the fixture fails: %v", rep.Problems())
	}
}

func TestEachRuleIsNamedWhenBroken(t *testing.T) {
	needGo(t)
	file := func(dir string, s artifact.Stage) string { return filepath.Join(dir, string(s)+".md") }
	cases := []struct {
		rule   string
		break_ func(t *testing.T, root, dir string)
	}{
		{"B1", func(t *testing.T, _, dir string) { os.Remove(file(dir, artifact.Plan)) }},
		{"R23", func(t *testing.T, _, dir string) {
			if err := artifact.AppendCost(dir, artifact.Cost{Area: 1, What: "the specification", Input: 100, Output: 10,
				Provenance: "derived", Handed: 90, Kept: "handed/01-author.md"}); err != nil {
				t.Fatal(err)
			}
		}},
		{"R19", func(t *testing.T, _, dir string) {
			edit(t, file(dir, artifact.Specification), "| The answer is one |", "| src/a.go returns one |")
		}},
		{"R16", func(t *testing.T, _, dir string) {
			edit(t, file(dir, artifact.Admission), "| CR-001-01 | src/a.go | the test", "| CR-001-09 | src/a.go | the test")
		}},
		{"CONVENTIONS", func(t *testing.T, _, dir string) {
			edit(t, file(dir, artifact.Specification), "| required | machine |", "| required | robot |")
		}},
		{"R7b", func(t *testing.T, _, dir string) {
			edit(t, file(dir, artifact.Admission), "| CR-001-01 | src/a.go as it stood | baseline/a.go.before |", "| CR-001-01 | nothing | nowhere |")
		}},
		{"R11", func(t *testing.T, _, dir string) {
			edit(t, file(dir, artifact.Specification), "| src/a_test.go | CR-001-01, CR-001-02 |", "| src/a_test.go | CR-001-01 |")
		}},
		{"R1", func(t *testing.T, _, dir string) {
			edit(t, file(dir, artifact.Specification), "| Default class | C3 |", "| Default class | C2 |")
		}},
		{"R2", func(t *testing.T, _, dir string) { os.RemoveAll(filepath.Join(dir, "candidates")) }},
		{"R7", func(t *testing.T, _, dir string) {
			edit(t, file(dir, artifact.Specification), "| src/a_test.go | CR-001-01, CR-001-02 |", "| src/a_test.go | CR-001-01, CR-001-02 |\n| src/a.go | CR-001-01 |")
		}},
		{"R3", func(t *testing.T, _, dir string) {
			edit(t, file(dir, artifact.Assurance), "| written and failing before the candidate |", "|  |")
		}},
		{"R3b", func(t *testing.T, _, dir string) {
			edit(t, file(dir, artifact.Assurance), "| `go test ./...` |", "| `go test ./...` over src/a.go |")
		}},
		{"B2", func(t *testing.T, _, dir string) {
			edit(t, file(dir, artifact.Assurance), "| met | EV-001-01-01 | settled |", "| met | EV-001-09-09 | settled |")
		}},
		{"R17", func(t *testing.T, _, dir string) {
			b, _ := os.ReadFile(file(dir, artifact.Assurance))
			write(t, file(dir, artifact.Assurance), string(b)+"\n## Refusals\n\n| Id | Object | Boundary | Refused by | Ground | At |\n|---|---|---|---|---|---|\n| RF-001-01 | CA-001-01 | execution to assurance | person | not what was asked | 2026-11-15 |\n")
		}},
		{"R18", func(t *testing.T, _, dir string) {
			edit(t, file(dir, artifact.Admission), "| yes | a person |", "| maybe | a person |")
		}},
		{"R13", func(t *testing.T, _, dir string) {
			edit(t, file(dir, artifact.Assurance), "| `go test ./...` |", "| the suite, run by hand |")
		}},
		{"R14", func(t *testing.T, _, dir string) {
			edit(t, file(dir, artifact.Assurance), "| yes | 2026-11-15 |", "| no | 2026-11-15 |")
		}},
		{"R6", func(t *testing.T, _, dir string) {
			edit(t, file(dir, artifact.Assurance), "| met | EV-001-01-01 | settled |", "| met | EV-001-01-01 | deferred |")
		}},
		{"A5", func(t *testing.T, _, dir string) {
			edit(t, file(dir, artifact.Assurance), "| VE-001-02 | WU-001-01 | CR-001-02 | met | EV-001-01-01 | settled |  |  |\n", "")
		}},
		{"A6", func(t *testing.T, _, dir string) {
			edit(t, file(dir, artifact.Assurance), "| CR-001-02 | met |", "| CR-001-02 | failed |")
		}},
		{"R10", func(t *testing.T, _, dir string) {
			edit(t, file(dir, artifact.Assurance), "| CR-001-02 | met |", "| CR-001-02 | undecided |")
		}},
		{"R4", func(t *testing.T, _, dir string) { edit(t, filepath.Join(dir, "record.md"), "| 2 |", "| 9 |") }},
		{"R8", func(t *testing.T, _, dir string) {
			edit(t, file(dir, artifact.Execution), "| the answer |", "| the answer, and src/a_test.go |")
		}},
		{"B4", func(t *testing.T, _, dir string) {
			edit(t, filepath.Join(dir, "record.md"), "| landing | WU-001-01 |", "| landed | WU-001-01 |")
		}},
		{"R21", func(t *testing.T, root, dir string) {
			write(t, filepath.Join(root, "other", "b.go"), "package other\n\nfunc B() {}\n")
			write(t, filepath.Join(dir, "candidates", "CA-001-01", "b.go"), "package other\n\nfunc B() {}\n")
		}},
		{"R22", func(t *testing.T, root, _ string) {
			// An estate that is a framework is asked whether its arbiters ran.
			write(t, filepath.Join(root, "internal", "gate", "gate.go"), "package gate\n")
		}},
	}
	covered := map[string]bool{}
	for _, tc := range cases {
		covered[tc.rule] = true
		t.Run(tc.rule, func(t *testing.T) {
			root, dir := fixture(t)
			tc.break_(t, root, dir)
			rep := gateOf(t, dir)
			if got := rules(rep.Problems()); !slices.Contains(got, tc.rule) {
				t.Errorf("broke %s, the gate said %v", tc.rule, rep.Problems())
			}
		})
	}
	for _, r := range Rules() {
		if !covered[r] {
			t.Errorf("rule %s is checked by the gate and broken by no case here", r)
		}
	}
}

func TestASuppressedRuleIsNotSaid(t *testing.T) {
	needGo(t)
	_, dir := fixture(t)
	edit(t, filepath.Join(dir, "record.md"), "| 2 |", "| 9 |")
	if got := rules(gateOf(t, dir, Suppress("R4")).Problems()); slices.Contains(got, "R4") {
		t.Errorf("a suppressed rule was said: %v", got)
	}
}

// A closed change owes nothing more: what it still owed is kept as notes,
// and it is held to its record — which must say why it was closed.
func TestAClosedChangeIsHeldToItsRecord(t *testing.T) {
	needGo(t)
	_, dir := fixture(t)
	edit(t, filepath.Join(dir, string(artifact.Specification)+".md"), "| src/a_test.go | CR-001-01, CR-001-02 |", "| src/a_test.go | CR-001-01 |")
	if gateOf(t, dir).Passed() {
		t.Fatal("the broken fixture passes before it is closed")
	}
	rec := artifact.Record{Name: "x", Dir: dir}
	if err := rec.Append(artifact.Crossing{At: "2026-11-16", Stage: "closed", Object: "INT-001", Note: "it will not be finished"}); err != nil {
		t.Fatal(err)
	}
	rep := gateOf(t, dir)
	if !rep.Passed() {
		t.Errorf("a closed change is still held to what it owed: %v", rep.Problems())
	}
	if !slices.ContainsFunc(rep.Notes(), func(f Finding) bool { return f.Rule == "R11" }) {
		t.Error("what the closed change owed is not kept as a note")
	}
	if err := rec.Append(artifact.Crossing{At: "2026-11-16", Stage: "closed", Object: "INT-001", Note: ""}); err != nil {
		t.Fatal(err)
	}
	if gateOf(t, dir).Passed() {
		t.Error("a change closed without saying why passes")
	}
}
