package gate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/estate"
	"github.com/igannytskyi/iflow/internal/meta"
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
	must(artifact.Save(dir, artifact.Intent, meta.Intent{Schema: meta.SchemaVersion, Name: "c", ID: "INT-001", StatedBy: "a person", Authority: "a person",
		Priority: "normal", Deadline: "none", Statement: "A answers one.", Status: meta.Open}))
	must(artifact.Save(dir, artifact.Specification, meta.Specification{Schema: meta.SchemaVersion, Name: "c", ID: "SPEC-001", Intent: "INT-001", Class: "C3",
		Criteria: []meta.Criterion{
			{ID: "CR-001-01", Criterion: "The answer is one", When: "A is asked", Then: "it answers one", Plan: meta.Required, Procedure: meta.Machine, Evidence: meta.TestRun, Threshold: "exactly"},
			{ID: "CR-001-02", Criterion: "Nothing else moves", When: "the suite is run", Then: "it stands", Plan: meta.NotRequired, Procedure: meta.Machine, Evidence: meta.TestRun, Threshold: "identical"},
		},
		Termination: []meta.Termination{{Condition: "one cannot be answered", Action: "stop"}},
		Scope:       meta.Scope{Included: []string{"src/a.go"}, Excluded: []string{"src/a_test.go"}},
		Arbiters:    []meta.Arbiter{{Path: "src/a_test.go", Arbitrates: "CR-001-01, CR-001-02"}},
		Inputs:      []meta.ArbiterInput{{Input: "src/a.go", InScope: "yes"}},
	}))
	must(artifact.Save(dir, artifact.Plan, meta.Plan{Schema: meta.PlanVersion, Name: "c", Specification: "SPEC-001", Phases: 1, IntermediateValid: "yes", Residue: "none",
		CoverageText: "The unit covers both criteria.",
		Units: []meta.WorkUnit{{ID: "WU-001-01", Phase: 1, Class: "C3", Kind: meta.CodeUnit, Scope: []string{"src/a.go"}, AreaOfEffect: "src, derived, high", Criteria: []string{"CR-001-01", "CR-001-02"},
			Brief: "Make one of src/a.go's answers right."}},
	}))
	digest, err := artifact.DigestFile(artifact.JSONFile(dir, artifact.Specification))
	must(err)
	write(t, filepath.Join(dir, "baseline", "a.go.before"), "package src\n")
	must(artifact.Save(dir, artifact.Admission, meta.Admission{Schema: meta.SchemaVersion, Name: "c", Digest: digest,
		Reviews:   []meta.Review{{Reviewed: "yes", By: "a person", At: "2026-11-15"}},
		Decisions: []meta.Decision{{Unit: "WU-001-01", Outcome: meta.Admitted, Condition: "permission", Confidence: "high"}},
		Plans: []meta.EvidencePlan{{Criterion: "CR-001-01", Observed: "src/a.go", Method: "the test, before and after", Unchanged: "it answers as before",
			DerivedFrom: "the area of effect of src/a.go, derived, high", FixedAt: "2026-11-15"}},
		Prior: []meta.Capture{{Criterion: "CR-001-01", Artefact: "src/a.go as it stood", CapturedTo: "baseline/a.go.before"}},
	}))
	write(t, filepath.Join(dir, "candidates", "CA-001-01", "a.go"), "package src\n\nfunc A() int { return 1 }\n")
	must(artifact.Save(dir, artifact.Execution, meta.Execution{Schema: meta.SchemaVersion, Name: "c",
		Runs:       []meta.Run{{Unit: "WU-001-01", Executor: "an agent", Version: "1", Terminal: meta.CandidateEnd, Candidate: "CA-001-01"}},
		Candidates: []meta.Candidate{{ID: "CA-001-01", Unit: "WU-001-01", StoredAt: "candidates/CA-001-01/", Artefacts: "the answer", ProducedAt: "2026-11-15"}},
	}))
	must(artifact.Save(dir, artifact.Assurance, meta.Assurance{Schema: meta.SchemaVersion, Name: "c",
		Evidence: []meta.Evidence{{ID: "EV-001-01-01", Subject: meta.OfCandidate, Kind: meta.TestRun, Producer: "`go test ./...`",
			Independence: meta.ByPrecommitment, How: "written and failing before the candidate", Repeatable: meta.Yes, ObtainedAt: "2026-11-15"}},
		Verdicts: []meta.Verdict{
			{ID: "VE-001-01", Unit: "WU-001-01", Criterion: "CR-001-01", Outcome: meta.Met, Evidence: []string{"EV-001-01-01"}, State: meta.Settled},
			{ID: "VE-001-02", Unit: "WU-001-01", Criterion: "CR-001-02", Outcome: meta.Met, Evidence: []string{"EV-001-01-01"}, State: meta.Settled},
		},
	}))
	must(artifact.Save(dir, artifact.Landing, meta.Landing{Schema: meta.SchemaVersion, Name: "c",
		Entries: []meta.Entry{{Order: 1, Unit: "WU-001-01", VerdictState: "settled", StillValid: "yes", EnteredAt: "2026-11-15"}},
	}))
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

// alter changes one document of the fixture as a hand would: its JSON
// rewritten, nothing checked on the way, which is what the gate is for.
func alter[T any](t *testing.T, dir string, s artifact.Stage, f func(*T)) {
	t.Helper()
	var d T
	if ok, err := artifact.LoadJSON(dir, s, &d); err != nil || !ok {
		t.Fatalf("%s does not read: %v", s, err)
	}
	f(&d)
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	write(t, artifact.JSONFile(dir, s), string(b))
}

// alterRecord changes the record as a hand would.
func alterRecord(t *testing.T, dir string, f func([]meta.Crossing)) {
	t.Helper()
	cs, err := artifact.ReadRecord(dir)
	if err != nil {
		t.Fatal(err)
	}
	f(cs)
	var b strings.Builder
	for _, c := range cs {
		line, _ := json.Marshal(c)
		b.Write(append(line, '\n'))
	}
	write(t, filepath.Join(dir, artifact.RecordFile), b.String())
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
	type (
		spec = meta.Specification
		adm  = meta.Admission
		ex   = meta.Execution
		as   = meta.Assurance
	)
	cases := []struct {
		rule   string
		break_ func(t *testing.T, root, dir string)
	}{
		{"B1", func(t *testing.T, _, dir string) {
			os.Remove(artifact.JSONFile(dir, artifact.Plan))
			os.Remove(filepath.Join(dir, string(artifact.Plan)+".md"))
		}},
		{"R23", func(t *testing.T, _, dir string) {
			if err := artifact.AppendCost(dir, artifact.Cost{Area: 1, What: "the specification", Input: 100, Output: 10,
				Provenance: "derived", Handed: 90, Kept: "handed/01-author.md"}); err != nil {
				t.Fatal(err)
			}
		}},
		{"R24", func(t *testing.T, _, dir string) {
			alter(t, dir, artifact.Plan, func(d *meta.Plan) { d.Units[0].Criteria = d.Units[0].Criteria[:1] })
		}},
		{"R19", func(t *testing.T, _, dir string) {
			alter(t, dir, artifact.Specification, func(d *spec) { d.Criteria[0].Criterion = "src/a.go returns one" })
		}},
		{"R16", func(t *testing.T, _, dir string) {
			alter(t, dir, artifact.Admission, func(d *adm) { d.Plans[0].Criterion = "CR-001-09" })
		}},
		{"CONVENTIONS", func(t *testing.T, _, dir string) {
			alter(t, dir, artifact.Specification, func(d *spec) { d.Criteria[0].Procedure = "robot" })
		}},
		{"R7b", func(t *testing.T, _, dir string) {
			alter(t, dir, artifact.Admission, func(d *adm) {
				d.Prior[0] = meta.Capture{Criterion: "CR-001-01", Artefact: "nothing", CapturedTo: "nowhere"}
			})
		}},
		{"R11", func(t *testing.T, _, dir string) {
			alter(t, dir, artifact.Specification, func(d *spec) { d.Arbiters[0].Arbitrates = "CR-001-01" })
		}},
		{"R1", func(t *testing.T, _, dir string) {
			alter(t, dir, artifact.Specification, func(d *spec) { d.Class = "C2" })
		}},
		{"R2", func(t *testing.T, _, dir string) { os.RemoveAll(filepath.Join(dir, "candidates")) }},
		{"R7", func(t *testing.T, _, dir string) {
			alter(t, dir, artifact.Specification, func(d *spec) {
				d.Arbiters = append(d.Arbiters, meta.Arbiter{Path: "src/a.go", Arbitrates: "CR-001-01"})
			})
		}},
		{"R3", func(t *testing.T, _, dir string) {
			alter(t, dir, artifact.Assurance, func(d *as) { d.Evidence[0].How = "" })
		}},
		{"R3b", func(t *testing.T, _, dir string) {
			alter(t, dir, artifact.Assurance, func(d *as) { d.Evidence[0].Producer = "`go test ./...` over src/a.go" })
		}},
		{"B2", func(t *testing.T, _, dir string) {
			alter(t, dir, artifact.Assurance, func(d *as) { d.Verdicts[0].Evidence = []string{"EV-001-09-09"} })
		}},
		{"R17", func(t *testing.T, _, dir string) {
			alter(t, dir, artifact.Assurance, func(d *as) {
				d.Refusals = []meta.Refusal{{ID: "RF-001-01", Object: "CA-001-01", Boundary: "execution to assurance", By: "person", Ground: "not what was asked", At: "2026-11-15"}}
			})
		}},
		{"R18", func(t *testing.T, _, dir string) {
			alter(t, dir, artifact.Admission, func(d *adm) { d.Reviews[0].Reviewed = "maybe" })
		}},
		{"R13", func(t *testing.T, _, dir string) {
			alter(t, dir, artifact.Assurance, func(d *as) { d.Evidence[0].Producer = "the suite, run by hand" })
		}},
		{"R14", func(t *testing.T, _, dir string) {
			alter(t, dir, artifact.Assurance, func(d *as) { d.Evidence[0].Repeatable = meta.No })
		}},
		{"R6", func(t *testing.T, _, dir string) {
			alter(t, dir, artifact.Assurance, func(d *as) { d.Verdicts[0].State = meta.Deferred })
		}},
		{"A5", func(t *testing.T, _, dir string) {
			alter(t, dir, artifact.Assurance, func(d *as) { d.Verdicts = d.Verdicts[:1] })
		}},
		{"A6", func(t *testing.T, _, dir string) {
			alter(t, dir, artifact.Assurance, func(d *as) { d.Verdicts[1].Outcome = meta.Failed })
		}},
		{"R10", func(t *testing.T, _, dir string) {
			alter(t, dir, artifact.Assurance, func(d *as) { d.Verdicts[1].Outcome = meta.Undecided })
		}},
		{"R4", func(t *testing.T, _, dir string) { alterRecord(t, dir, func(cs []meta.Crossing) { cs[1].N = 9 }) }},
		{"R8", func(t *testing.T, _, dir string) {
			alter(t, dir, artifact.Execution, func(d *ex) { d.Candidates[0].Artefacts = "the answer, and src/a_test.go" })
		}},
		{"B4", func(t *testing.T, _, dir string) {
			alterRecord(t, dir, func(cs []meta.Crossing) { cs[len(cs)-1].Stage = "landed" })
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
	alterRecord(t, dir, func(cs []meta.Crossing) { cs[1].N = 9 })
	if got := rules(gateOf(t, dir, Suppress("R4")).Problems()); slices.Contains(got, "R4") {
		t.Errorf("a suppressed rule was said: %v", got)
	}
}

// A closed change owes nothing more: what it still owed is kept as notes,
// and it is held to its record — which must say why it was closed.
func TestAClosedChangeIsHeldToItsRecord(t *testing.T) {
	needGo(t)
	_, dir := fixture(t)
	alter(t, dir, artifact.Specification, func(d *meta.Specification) { d.Arbiters[0].Arbitrates = "CR-001-01" })
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

// The command evidence names is the same command wherever the gate is run
// from, so that its repeat is the same note from the terminal and from the
// workbench.
func TestAProducerIsNamedAbsolutely(t *testing.T) {
	root, dir := fixture(t)
	write(t, filepath.Join(root, "run.sh"), "#!/bin/sh\n")
	alter(t, dir, artifact.Assurance, func(d *meta.Assurance) { d.Evidence[0].Producer = "`run.sh --all`" })
	c, err := artifact.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	cmd := ProducerCommand(c, "EV-001-01-01")
	if len(cmd) != 2 || !filepath.IsAbs(cmd[0]) {
		t.Errorf("named %v", cmd)
	}
}

// What was handed and is kept under superseded/, where a change carried
// again keeps what it had made, is kept.
func TestHandedKeptUnderSuperseded(t *testing.T) {
	needGo(t)
	_, dir := fixture(t)
	if err := artifact.AppendCost(dir, artifact.Cost{Area: 4, What: "execution of WU-001-01", Input: 100, Output: 10,
		Provenance: "derived", Handed: 90, Kept: "candidates/CA-001-09/pack.md"}); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "superseded", "1", "candidates", "CA-001-09", "pack.md"), "handed")
	if rep := gateOf(t, dir); !rep.Passed() {
		t.Errorf("a handover kept under superseded/ was not found: %v", rep.Problems())
	}
}

// While the arbiters are being run against the framework as it is, R22
// says so as a note, not as a change that entered unverified; once their
// run is over, or the framework has moved since it began, it holds again.
func TestR22WaitsForArbitersBeingRun(t *testing.T) {
	needGo(t)
	root, dir := fixture(t)
	write(t, filepath.Join(root, "internal", "gate", "gate.go"), "package gate\n")
	home := t.TempDir()
	c, err := artifact.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	unrun := func() bool {
		for _, p := range New(estate.NewLocal(), WithHome(home)).Check(context.Background(), c).Problems() {
			if strings.Contains(fmt.Sprint(p), "before the arbiters were run") {
				return true
			}
		}
		return false
	}
	if !unrun() {
		t.Fatal("a landing with no arbiters run passed R22")
	}
	// The arbiters are those of the framework the change entered: its root.
	write(t, arbitersRunning(root), ArbitersKey(root))
	if unrun() {
		t.Error("R22 failed while the arbiters were being run")
	}
	write(t, filepath.Join(root, "internal", "moved.go"), "package moved\n")
	if !unrun() {
		t.Error("a run against the framework as it was stood for the framework as it is")
	}
}

// A claim to repeat set against what its evidence kept is run once more,
// and agrees with the kept output only where the two say the same.
func TestARepeatAgainstWhatWasKept(t *testing.T) {
	needGo(t)
	_, dir := fixture(t)
	c, err := artifact.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	g := New(estate.NewLocal(), WithHome(t.TempDir()))
	ids, _ := Producers(c)
	if len(ids) == 0 {
		t.Fatal("the fixture claims nothing repeats")
	}
	cmd := ProducerCommand(c, ids[0])
	now := runOnce(context.Background(), c.Root, cmd).raw
	for kept, same := range map[string]bool{now: true, "FAIL\tsomething else\n": false} {
		reps, err := g.RepeatAgainst(context.Background(), c, func(string) (string, bool) { return kept, true })
		if err != nil {
			t.Fatal(err)
		}
		if reps[0].Same != same {
			t.Errorf("kept %q: same %v, want %v", kept, reps[0].Same, same)
		}
	}
}
