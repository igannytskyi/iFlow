package flow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/estate"
	"github.com/igannytskyi/iflow/internal/gate"
	"github.com/igannytskyi/iflow/internal/harness"
	"github.com/igannytskyi/iflow/internal/meta"
	"github.com/igannytskyi/iflow/internal/model"
)

// stubModel answers area 1 with a fixed specification, and keeps what it
// was handed where asked to.
type stubModel struct {
	answer string
	handed *[]string
}

func (s stubModel) Name() string { return "stub" }
func (s stubModel) Ask(_ context.Context, system, prompt string) (string, model.Usage, error) {
	if s.handed != nil {
		*s.handed = append(*s.handed, prompt)
	}
	switch system {
	case plannerSystem:
		return plan, model.Usage{Model: "stub", Input: 80, Output: 40}, nil
	case specReviewSystem:
		// The first review sends the specification back; the second finds
		// nothing that stops the work.
		if !strings.Contains(prompt, "What a reviewer found") && s.handed != nil && countReviews(*s.handed) == 1 {
			return `{"verdict":"insufficient","findings":[{"severity":"major","where":"Edge cases","finding":"an empty name is not covered","suggestion":"say what shouting nothing gives"}],"questions_for_the_person":["should shouting add an exclamation mark?"]}`, model.Usage{Model: "stub"}, nil
		}
		return `{"verdict":"sufficient","findings":[{"severity":"minor","where":"Risks","finding":"thin","suggestion":""}],"questions_for_the_person":[]}`, model.Usage{Model: "stub"}, nil
	case planReviewSystem:
		return `{"verdict":"sufficient","findings":[],"questions_for_the_person":[]}`, model.Usage{Model: "stub"}, nil
	}
	return s.answer, model.Usage{Model: "stub", Input: 100, Output: 50}, nil
}

// countReviews is how many reviews of a specification were asked for.
func countReviews(prompts []string) int {
	n := 0
	for _, p := range prompts {
		if strings.HasPrefix(p, "# The specification") && !strings.Contains(p, "# The plan") && !strings.Contains(p, "# What the units may touch") {
			n++
		}
	}
	return n
}

// plan is what the planner answers: the tests, then the change.
const plan = `{"approach":{"summary":"Add Shout beside Greet.","steps":["write the test","add Shout"],"risks":["none"]},
"units":[
 {"title":"the shout test","kind":"tests","scope":["shout_test.go"],"criteria":["CR-001-01"],"requirements":["FR-1"],"objective":"Judge shouting from outside.","tasks":["Test that Shout(\"a\") is HELLO, A."],"provides":["TestShout"],"done_when":["it fails before Shout exists"]},
 {"title":"Shout","kind":"code","scope":["greet.go"],"criteria":["CR-001-01","CR-001-02"],"requirements":["FR-1"],"objective":"Shout greets in capitals.","tasks":["Add func Shout(n string) string returning Greet(n) in capitals."],"provides":["func Shout(n string) string"],"constraints":["leave Greet as it is"],"done_when":["the suite passes"]}]}`

// stubExecutor writes what each unit's scope asks for: the test first, then
// the repair; and keeps the pack each unit was handed where asked to.
type stubExecutor struct {
	files map[string]string
	packs *[]string
	goals *[]string
	// meanwhile, where set, is written into the estate itself while a code
	// unit runs: another change landing on the same ground.
	meanwhile func()
}

func (s stubExecutor) Name() string    { return "stub" }
func (s stubExecutor) Version() string { return "1" }
func (s stubExecutor) Run(_ context.Context, t harness.Task, emit func(harness.Event)) (harness.Result, error) {
	if s.packs != nil {
		*s.packs = append(*s.packs, t.Context)
	}
	if s.goals != nil {
		*s.goals = append(*s.goals, t.Goal)
	}
	if s.meanwhile != nil && !strings.HasSuffix(t.Scope[0], "_test.go") {
		s.meanwhile()
	}
	for _, p := range t.Scope {
		emit(harness.Event{At: time.Now(), Kind: harness.Called, Tool: "write_file", Detail: p})
		body := s.files[p]
		if d, ok := s.files["declared "+p]; ok && strings.Contains(t.Goal, "Declare exactly these names") {
			body = d // a contract declares, and does nothing
		}
		if err := os.WriteFile(filepath.Join(t.Workspace, p), []byte(body), 0o644); err != nil {
			return harness.Result{Terminal: harness.FailedTask}, err
		}
	}
	return harness.Result{Terminal: harness.Candidate, Summary: "wrote " + strings.Join(t.Scope, ", "), Steps: len(t.Scope)}, nil
}

const spec = `{"slug":"shout","name":"shout","statement":"The greeting can be shouted.","class":"C3","prose":"A shouted greeting is in capitals.",
"document":{"summary":"A greeting can be shouted.","context":"Greetings are only spoken.","goals":["shout"],"non_goals":["whisper"],"users":["a caller"],"user_stories":["As a caller, I want to shout a greeting, so that it is heard."],"flows":["Shouting a name greets it in capitals."],"requirements":["A shouted greeting is the greeting in capitals."],"interfaces":["func Shout(n string) string"],"components":["greet.go holds Shout beside Greet."],"edge_cases":["an empty name shouts HELLO, "],"risks":["none"],"open_questions":[],"test_strategy":["shout_test.go calls Shout"]},
"criteria":[
 {"criterion":"A shouted greeting is in capitals","when":"a greeting is shouted","then":"every letter is a capital","plan":"required","threshold":"exact text","judged_by_new_tests":true},
 {"criterion":"Nothing else moves","when":"the whole suite is run","then":"it stands as before","plan":"not-required","threshold":"identical","judged_by_new_tests":false}],
"scope":["greet.go"],"new_tests":["shout_test.go"],"existing_tests":["greet_test.go"],
"termination":[{"condition":"shouting needs a new dependency","action":"stop"}]}`

func estateFixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain to judge the fixture with")
	}
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":        "module example.com/greet\n\ngo 1.22\n",
		"greet.go":      "package greet\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n",
		"greet_test.go": "package greet\n\nimport \"testing\"\n\nfunc TestGreet(t *testing.T) {\n\tif Greet(\"a\") != \"Hello, a\" {\n\t\tt.Fatal()\n\t}\n}\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestATaskTravelsThePath(t *testing.T) {
	repo := estateFixture(t)
	est := estate.NewLocal()
	var prompts, packs, goals []string
	var landed string
	cfg := Config{
		Model: stubModel{spec, &prompts},
		Executor: stubExecutor{packs: &packs, goals: &goals, files: map[string]string{
			"shout_test.go": "package greet\n\nimport \"testing\"\n\nfunc TestShout(t *testing.T) {\n\tif Shout(\"a\") != \"HELLO, A\" {\n\t\tt.Fatal()\n\t}\n}\n",
			"greet.go":      "package greet\n\nimport \"strings\"\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n\nfunc Shout(n string) string { return strings.ToUpper(Greet(n)) }\n",
		}},
		Estate: est, Gate: gate.New(est), Person: "a test",
	}
	defer func() {
		if len(goals) != 2 || !strings.Contains(goals[1], "Add func Shout") {
			t.Errorf("the units were not handed their briefs: %q", goals)
		}
		if b, _ := os.ReadFile(filepath.Join(landed, "plan.md")); !strings.Contains(string(b), "## The units") || !strings.Contains(string(b), "Planned by stub") {
			t.Errorf("the plan does not describe its units:\n%s", b)
		}
		if b, _ := os.ReadFile(filepath.Join(landed, "units", "WU-001-02.md")); !strings.Contains(string(b), "Add func Shout") {
			t.Errorf("the code unit's ticket is not kept:\n%s", b)
		}
		// Each agent is handed the least it needs: the model stating the
		// intent sees what the estate defines, never its code; the agent
		// writing the tests sees the code they will exercise; the agent
		// changing the code sees the tests it must pass.
		if len(prompts) == 0 || !strings.Contains(prompts[0], "greet.go: Greet") || strings.Contains(prompts[0], `"Hello, "`) {
			t.Errorf("the intent was handed the wrong view of the estate:\n%s", prompts)
		}
		// The author is asked, a reviewer sends the specification back, the
		// author writes it again with what was found, a second review lets it
		// through; then the planner, and a reviewer of the plan.
		var planner, redraft string
		for _, p := range prompts {
			if strings.Contains(p, "# What the units may touch") {
				planner = p
			}
			if strings.Contains(p, "What a reviewer found in it") && strings.Contains(p, "an empty name is not covered") && strings.HasPrefix(p, "The task") {
				redraft = p
			}
		}
		if redraft == "" {
			t.Error("the author was never handed what the reviewer found")
		}
		if !strings.Contains(planner, "## How a person uses it") || !strings.Contains(planner, "FR-1 ") || strings.Contains(planner, `"Hello, "`) {
			t.Errorf("the planner was not handed the specification as a document, and no code:\n%s", planner)
		}
		for _, f := range []string{"specification-1.md", "specification-2.md", "plan-1.md"} {
			if _, err := os.Stat(filepath.Join(landed, "reviews", f)); err != nil {
				t.Errorf("review %s was not kept", f)
			}
		}
		if len(packs) != 2 {
			t.Fatalf("%d packs handed", len(packs))
		}
		for i, want := range [][]string{{"May change: shout_test.go", "Read: greet.go", `"Hello, "`}, {"Must pass, written by WU-001-01", "TestShout", "May change: greet.go"}} {
			for _, w := range want {
				if !strings.Contains(packs[i], w) {
					t.Errorf("unit %d was not handed %q:\n%s", i+1, w, packs[i])
				}
			}
		}
	}()
	var mu sync.Mutex
	asked := 0
	var run *Run
	run = New(cfg, repo, "make it possible to shout the greeting", func() {
		if v := run.View(); v.Question != "" {
			mu.Lock()
			asked++
			mu.Unlock()
			go run.Accept()
		}
	})
	run.Settles("earlier#1")
	if err := run.Start(context.Background()); err != nil {
		for _, s := range run.View().Steps {
			t.Logf("%d %s", s.Area, s.Text)
		}
		t.Fatalf("the task stopped: %v", err)
	}
	v := run.View()
	landed = v.Dir
	for n := 1; n <= 6; n++ {
		if v.Areas[n] != Done {
			t.Errorf("area %d ended %s", n, v.Areas[n])
		}
	}
	if asked == 0 {
		t.Error("the person was never asked to accept the criteria")
	}
	c, err := artifact.Load(v.Dir)
	if err != nil {
		t.Fatal(err)
	}
	// Every document the run wrote is in the data model: JSON beside its
	// Markdown, read back whole and breaking nothing of the model.
	for _, st := range artifact.Stages {
		if _, err := os.Stat(artifact.JSONFile(v.Dir, st)); err != nil {
			t.Errorf("%s was not written in the data model: %v", st, err)
		}
	}
	if len(c.Meta.Problems) > 0 || c.Meta.Landing == nil || len(c.Meta.Record) == 0 || c.Meta.Specification.Criteria[0].ID != "CR-001-01" {
		t.Errorf("the change does not read back in the model: %v", c.Meta.Problems)
	}
	for _, d := range []any{c.Meta.Intent, c.Meta.Specification, c.Meta.Plan, c.Meta.Admission, c.Meta.Execution, c.Meta.Assurance, c.Meta.Landing} {
		if ps := meta.Validate(d); len(ps) > 0 {
			t.Errorf("a document breaks the model: %v", ps)
		}
	}
	rep := cfg.Gate.Check(context.Background(), c)
	if !rep.Passed() {
		t.Errorf("the landed change does not pass the gate: %v", rep.Problems())
	}
	if b, _ := os.ReadFile(filepath.Join(repo, "greet.go")); !strings.Contains(string(b), "Shout") {
		t.Error("the repair did not enter the estate")
	}
	if b, _ := os.ReadFile(filepath.Join(v.Dir, "candidates", "CA-001-01", "failing-before.txt")); !strings.Contains(string(b), "FAIL") {
		t.Errorf("the reproduction was not shown to fail before the change: %s", b)
	}
	if b, _ := os.ReadFile(filepath.Join(v.Dir, "05-assurance.md")); !strings.Contains(string(b), "| earlier#1 | this change |") {
		t.Errorf("the escalation the task was started to close is not recorded closed:\n%s", b)
	}
	// The estate's own runner, run where a person runs it, sees the estate
	// and not the copies of candidates its change folders keep.
	cmd := exec.Command("go", "test", "-count=1", "./...")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("the estate's own tests fail after the landing:\n%s", out)
	}
}

func TestARefusalStopsThePath(t *testing.T) {
	repo := estateFixture(t)
	est := estate.NewLocal()
	cfg := Config{Model: stubModel{answer: spec}, Executor: stubExecutor{}, Estate: est, Gate: gate.New(est), Person: "a test"}
	var run *Run
	run = New(cfg, repo, "make it possible to shout", func() {
		if v := run.View(); v.Question != "" {
			go run.Refuse("not what I meant")
		}
	})
	if err := run.Start(context.Background()); err == nil {
		t.Fatal("a refused specification went on")
	}
	v := run.View()
	if v.Areas[1] != Failed || v.Areas[2] != Idle {
		t.Errorf("areas after refusal: %v", v.Areas)
	}
	if b, _ := os.ReadFile(filepath.Join(v.Dir, "record.md")); !strings.Contains(string(b), "not what I meant") {
		t.Error("the refusal's ground was not recorded")
	}
}

func TestASlugNothingSurvivesOf(t *testing.T) {
	repo := estateFixture(t)
	est := estate.NewLocal()
	cyrillic := strings.Replace(spec, `"slug":"shout"`, `"slug":"страницы-каталога"`, 1)
	cfg := Config{Model: stubModel{answer: cyrillic}, Executor: stubExecutor{}, Estate: est, Gate: gate.New(est), Person: "a test"}
	var run *Run
	run = New(cfg, repo, "shout", func() {
		if v := run.View(); v.Question != "" {
			go run.Refuse("only the folder is under test")
		}
	})
	_ = run.Start(context.Background())
	want := filepath.Join(repo, "changes", "change-001")
	if got := run.View().Dir; got != want {
		t.Fatalf("change folder = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(want, "00-intent.md")); err != nil {
		t.Error("the intent is not in the change's folder")
	}
}

// A specification sent back is written again from the person's notes, and
// the person is asked again; the second time they accept it and the path
// goes on.
func TestSendingBackRewritesTheSpecification(t *testing.T) {
	repo := estateFixture(t)
	est := estate.NewLocal()
	var prompts []string
	cfg := Config{Model: stubModel{answer: spec, handed: &prompts}, Executor: stubExecutor{}, Estate: est, Gate: gate.New(est), Person: "a test"}
	var mu sync.Mutex
	asked := 0
	var run *Run
	run = New(cfg, repo, "make it possible to shout", func() {
		if v := run.View(); v.Question != "" {
			mu.Lock()
			asked++
			n := asked
			mu.Unlock()
			if n == 1 {
				go run.Refuse("an empty name should shout HELLO, WORLD")
			} else {
				go run.Refuse("stop here") // the second answer only proves it was asked again
			}
		}
	})
	_ = run.Start(context.Background())
	again := ""
	for _, p := range prompts {
		if strings.Contains(p, "sent it back, saying") && strings.Contains(p, "HELLO, WORLD") {
			again = p
		}
	}
	if again == "" {
		t.Error("the author was never handed the person's notes")
	}
	mu.Lock()
	defer mu.Unlock()
	if asked < 2 {
		t.Errorf("the person was asked %d time(s), not again after sending it back", asked)
	}
}

// Two changes in flight on the same ground: when another lands on a file
// while this one is carried out, this one's landing stops rather than undo
// it, and says why.
func TestALandingDoesNotUndoAnotherChange(t *testing.T) {
	repo := estateFixture(t)
	est := estate.NewLocal()
	other := "package greet\n\n// changed by another change\nfunc Greet(n string) string { return \"Hi, \" + n }\n"
	cfg := Config{
		Model: stubModel{answer: spec},
		Executor: stubExecutor{files: map[string]string{
			"shout_test.go": "package greet\n\nimport \"testing\"\n\nfunc TestShout(t *testing.T) {\n\tif Shout(\"a\") != \"HELLO, A\" {\n\t\tt.Fatal()\n\t}\n}\n",
			"greet.go":      "package greet\n\nimport \"strings\"\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n\nfunc Shout(n string) string { return strings.ToUpper(Greet(n)) }\n",
		}, meanwhile: func() { _ = os.WriteFile(filepath.Join(repo, "greet.go"), []byte(other), 0o644) }},
		Estate: est, Gate: gate.New(est), Person: "a test",
	}
	var run *Run
	run = New(cfg, repo, "make it possible to shout the greeting", func() {
		if v := run.View(); v.Question != "" {
			go run.Accept()
		}
	})
	err := run.Start(context.Background())
	if err == nil {
		t.Fatal("the change landed on ground that moved under it")
	}
	if v := run.View(); v.Areas[6] != Failed {
		t.Errorf("areas: %v", v.Areas)
	}
	if b, _ := os.ReadFile(filepath.Join(repo, "greet.go")); string(b) != other {
		t.Errorf("the other change was undone:\n%s", b)
	}
	if !strings.Contains(err.Error(), "greet.go was changed") && !strings.Contains(fmt.Sprint(run.View().Steps), "greet.go was changed") {
		t.Errorf("the landing does not say what moved: %v", err)
	}
}

// Admission names every other change in flight whose scope meets this one's.
func TestAdmissionNamesChangesInFlight(t *testing.T) {
	repo := estateFixture(t)
	in := filepath.Join(repo, "changes", "louder")
	if err := os.MkdirAll(in, 0o755); err != nil {
		t.Fatal(err)
	}
	other := artifact.SpecDoc{Name: "louder", ID: "SPEC-009", Intent: "INT-009", Class: "C3", Included: []string{"greet.go"},
		Criteria: []artifact.CriterionRow{{ID: "CR-009-01", Criterion: "c", When: "w", Then: "t", Plan: "not-required", Procedure: "machine", Evidence: "test-run"}}}
	if err := artifact.Write(in, artifact.Specification, other.Markdown()); err != nil {
		t.Fatal(err)
	}
	est := estate.NewLocal()
	cfg := Config{Model: stubModel{answer: spec}, Executor: stubExecutor{files: map[string]string{
		"shout_test.go": "package greet\n\nimport \"testing\"\n\nfunc TestShout(t *testing.T) {\n\tif Shout(\"a\") != \"HELLO, A\" {\n\t\tt.Fatal()\n\t}\n}\n",
		"greet.go":      "package greet\n\nimport \"strings\"\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n\nfunc Shout(n string) string { return strings.ToUpper(Greet(n)) }\n",
	}}, Estate: est, Gate: gate.New(est), Person: "a test"}
	var run *Run
	run = New(cfg, repo, "shout", func() {
		if v := run.View(); v.Question != "" {
			go run.Accept()
		}
	})
	_ = run.Start(context.Background())
	b, _ := os.ReadFile(filepath.Join(run.View().Dir, "03-admission.md"))
	if !strings.Contains(string(b), "louder") || !strings.Contains(string(b), "greet.go") {
		t.Errorf("admission does not name the change in flight on the same ground:\n%s", b)
	}
}

// Runs started side by side each take a number of their own.
func TestNumbersAreTakenOnce(t *testing.T) {
	repo := t.TempDir()
	got := make(chan int, 20)
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() { got <- nextNumber(repo) })
	}
	wg.Wait()
	close(got)
	seen := map[int]bool{}
	for n := range got {
		if seen[n] {
			t.Fatalf("number %d was taken twice", n)
		}
		seen[n] = true
	}
}

// Tests written first may fail to build only for what the contract
// promises; anything else they got wrong goes back to their agent.
func TestTestsWrittenFirstAreJudgedByTheContract(t *testing.T) {
	r := &Run{units: []unit{{id: "WU-001-03", scope: []string{"internal/notify/notify.go", "internal/flow/flow.go"}, provides: []string{"Step.Doing string"}}, {id: "WU-001-02", provides: []string{"func Shout(n string) string", "func (p *project) closeTop() bool", "key.NameN"}}}}
	judge := r.judgeTests()
	if ok, why := judge(false, "vet: ./shout_test.go:5:9: undefined: Shout\n./x_test.go:9:4: p.closeTop undefined (type *project has no field or method closeTop)\n"); !ok {
		t.Errorf("a missing promised name was held against the tests: %s", why)
	}
	if ok, why := judge(false, "vet: internal/ui/graph_test.go:32:24: r.Semantic undefined (type *input.Router has no field or method Semantic)\n"); ok || !strings.Contains(why, "Semantic") {
		t.Errorf("an invented method of a library passed: %v %s", ok, why)
	}
	if ok, why := judge(false, "internal/ui/graph_test.go:33:53: unknown field Doing in struct literal of type flow.Step\n"); !ok {
		t.Errorf("a field the contract promises was held against the tests: %s", why)
	}
	if ok, why := judge(false, "vet: ./shortcuts_test.go:53:40: undefined: key.NameN\n"); ok || !strings.Contains(why, "key.NameN") {
		t.Errorf("a name a library does not have passed: %v %s", ok, why)
	}
	if ok, why := judge(false, "internal/ui/notify_test.go:6:2: no required module provides package ex.com/m/internal/notify; to add it:\n"); !ok {
		t.Errorf("a package a code unit creates was held against the tests: %s", why)
	}
	if ok, _ := judge(false, "internal/ui/notify_test.go:6:2: no required module provides package ex.com/m/internal/elsewhere; to add it:\n"); ok {
		t.Error("a package no unit creates passed")
	}
	if ok, _ := judge(false, "vet: x_test.go:11:15: not enough arguments in call to testUI\n"); ok {
		t.Error("the wrong arguments to a function that exists passed")
	}
	if ok, why := judge(false, "# ./internal/notify\nstat /tmp/ws/internal/notify: directory not found\nFAIL\t./internal/notify [setup failed]\n"); ok || !strings.Contains(why, "not all written") {
		t.Errorf("tests not written yet passed: %v %s", ok, why)
	}
}

// secondTry is an executor whose tests are wrong the first time: the code
// cannot pass them, says so, and passes the tests written again.
type secondTry struct {
	goals *[]string
	calls map[string]int
}

func (s secondTry) Name() string    { return "second try" }
func (s secondTry) Version() string { return "1" }
func (s secondTry) Run(_ context.Context, t harness.Task, _ func(harness.Event)) (harness.Result, error) {
	*s.goals = append(*s.goals, t.Goal)
	scope := t.Scope[0]
	s.calls[scope]++
	files := map[string]string{
		"shout_test.go": "package greet\n\nimport \"testing\"\n\nfunc TestShout(t *testing.T) {\n\tif Shout(\"a\") != \"HELLO, A\" {\n\t\tt.Fatal()\n\t}\n}\n",
		"greet.go":      "package greet\n\nimport \"strings\"\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n\nfunc Shout(n string) string { return strings.ToUpper(Greet(n)) }\n",
	}
	if scope == "greet.go" && s.calls["shout_test.go"] == 1 {
		return harness.Result{Terminal: harness.FailedTask, Summary: "the check fails in shout_test.go, which this unit may not change: undefined: strings.Shouty"}, nil
	}
	if err := os.WriteFile(filepath.Join(t.Workspace, scope), []byte(files[scope]), 0o644); err != nil {
		return harness.Result{}, err
	}
	return harness.Result{Terminal: harness.Candidate, Summary: "wrote " + scope}, nil
}

// Tests the code cannot pass, because they are wrong, are carried again
// with what the code found, and the code is carried on what comes back.
func TestWrongTestsAreCarriedAgain(t *testing.T) {
	repo := estateFixture(t)
	est := estate.NewLocal()
	var goals []string
	ex := secondTry{goals: &goals, calls: map[string]int{}}
	cfg := Config{Model: stubModel{answer: spec}, Executor: ex, Estate: est, Gate: gate.New(est), Person: "a test"}
	var run *Run
	run = New(cfg, repo, "make it possible to shout the greeting", func() {
		if v := run.View(); v.Question != "" {
			go run.Accept()
		}
	})
	if err := run.Start(context.Background()); err != nil {
		for _, s := range run.View().Steps {
			t.Logf("%d %s", s.Area, s.Text)
		}
		t.Fatalf("the task stopped: %v", err)
	}
	if ex.calls["shout_test.go"] != 2 || ex.calls["greet.go"] != 2 {
		t.Errorf("carried %v", ex.calls)
	}
	again := ""
	for _, g := range goals {
		if strings.Contains(g, "Why you are asked again") {
			again = g
		}
	}
	if !strings.Contains(again, "strings.Shouty") {
		t.Errorf("the tests were not handed what the code found:\n%s", strings.Join(goals, "\n---\n"))
	}
	if b, _ := os.ReadFile(filepath.Join(repo, "greet.go")); !strings.Contains(string(b), "Shout") {
		t.Error("the code did not land")
	}
}

// A change whose ground moved, where the other change's edits are apart
// from its own, lands merged with them: neither undoes the other, and the
// record says the suite passed on the merged ground.
func TestAMovedChangeLandsMerged(t *testing.T) {
	repo := estateFixture(t)
	est := estate.NewLocal()
	other := "// Package greet greets.\npackage greet\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n"
	cfg := Config{
		Model: stubModel{answer: spec},
		Executor: stubExecutor{files: map[string]string{
			"shout_test.go": "package greet\n\nimport \"testing\"\n\nfunc TestShout(t *testing.T) {\n\tif Shout(\"a\") != \"HELLO, A\" {\n\t\tt.Fatal()\n\t}\n}\n",
			"greet.go":      "package greet\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n\nfunc Shout(n string) string { return \"HELLO, \" + strings.ToUpper(n) }\n\nvar _ = strings.ToUpper\n",
		}, meanwhile: func() { _ = os.WriteFile(filepath.Join(repo, "greet.go"), []byte(other), 0o644) }},
		Estate: est, Gate: gate.New(est), Person: "a test",
	}
	// The stub's greet.go needs strings imported; the import goes where
	// the other change does not reach.
	files := cfg.Executor.(stubExecutor).files
	files["greet.go"] = strings.Replace(files["greet.go"], "package greet\n\nfunc Greet", "package greet\n\nimport \"strings\"\n\nfunc Greet", 1)
	var run *Run
	run = New(cfg, repo, "make it possible to shout the greeting", func() {
		if v := run.View(); v.Question != "" {
			go run.Accept()
		}
	})
	if err := run.Start(context.Background()); err != nil {
		for _, s := range run.View().Steps {
			t.Logf("%d %s", s.Area, s.Text)
		}
		t.Fatalf("the change did not land: %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(repo, "greet.go"))
	if s := string(b); !strings.Contains(s, "// Package greet greets.") || !strings.Contains(s, "func Shout") {
		t.Errorf("one change undid the other:\n%s", s)
	}
	if rec, _ := os.ReadFile(filepath.Join(run.View().Dir, "record.md")); !strings.Contains(string(rec), "merged onto the estate as it is now") {
		t.Errorf("the record does not say the change was merged:\n%s", rec)
	}
}

func TestARequirementIsNumberedOnce(t *testing.T) {
	for _, q := range []string{"FR-2 : Shout shouts.", "FR-2 — Shout shouts.", "FR-2. Shout shouts.", "Shout shouts."} {
		if got := requirementText(q, 2); got != "Shout shouts." {
			t.Errorf("%q gave %q", q, got)
		}
	}
}

// A change is carried again in its own folder, from its plan or from its
// specification: what it wrote from there on is kept under superseded/, the
// record says it was reopened, and it lands again passing the gate.
func TestAReopenedChangeIsCarriedAgain(t *testing.T) {
	repo := estateFixture(t)
	est := estate.NewLocal()
	files := map[string]string{
		"shout_test.go": "package greet\n\nimport \"testing\"\n\nfunc TestShout(t *testing.T) {\n\tif Shout(\"a\") != \"HELLO, A\" {\n\t\tt.Fatal()\n\t}\n}\n",
		"greet.go":      "package greet\n\nimport \"strings\"\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n\nfunc Shout(n string) string { return strings.ToUpper(Greet(n)) }\n",
	}
	var prompts []string
	cfg := Config{Model: stubModel{spec, &prompts}, Executor: stubExecutor{files: files}, Estate: est, Gate: gate.New(est), Person: "a test"}
	carry := func(run *Run) {
		t.Helper()
		run.changed = func() {
			if v := run.View(); v.Question != "" {
				go run.Accept()
			}
		}
		if err := run.Start(context.Background()); err != nil {
			for _, s := range run.View().Steps {
				t.Logf("%d %s", s.Area, s.Text)
			}
			t.Fatalf("the change did not land: %v", err)
		}
	}
	first := New(cfg, repo, "make it possible to shout the greeting", nil)
	carry(first)
	dir := first.View().Dir
	for i, from := range []int{2, 1} {
		// What the person asks for is new behaviour: tests that fail on the
		// estate as the first landing left it, and code that passes them.
		bang := strings.Repeat("!", i+1)
		files["shout_test.go"] = "package greet\n\nimport \"testing\"\n\nfunc TestShout(t *testing.T) {\n\tif Shout(\"a\") != \"HELLO, A" + bang + "\" {\n\t\tt.Fatal()\n\t}\n}\n"
		files["greet.go"] = "package greet\n\nimport \"strings\"\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n\nfunc Shout(n string) string { return strings.ToUpper(Greet(n)) + \"" + bang + "\" }\n"
		prompts = nil
		run, err := Reopen(cfg, repo, dir, from, "shout with an exclamation mark", nil)
		if err != nil {
			t.Fatal(err)
		}
		carry(run)
		if run.View().Dir != dir {
			t.Errorf("reopened into %s, not its own folder", run.View().Dir)
		}
		handed := strings.Join(prompts, "\n")
		if !strings.Contains(handed, "shout with an exclamation mark") {
			t.Errorf("from %d: the person's words were not handed on", from)
		}
		c, err := artifact.Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		if rep := cfg.Gate.Check(context.Background(), c); !rep.Passed() {
			t.Errorf("from %d: the change does not pass the gate again: %v", from, rep.Problems())
		}
		reopened := 0
		for _, cr := range c.Meta.Record {
			if cr.Stage == "reopened" {
				reopened++
			}
		}
		if reopened == 0 || c.Meta.Intent.ID != "INT-001" {
			t.Errorf("from %d: the record does not say it was reopened, or the number moved: %d, %s", from, reopened, c.Meta.Intent.ID)
		}
	}
	for _, kept := range []string{"superseded/1/02-plan.json", "superseded/2/01-specification.json", "superseded/2/change.md"} {
		if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
			t.Errorf("%s was not kept", kept)
		}
	}
	if _, err := Reopen(cfg, repo, dir, 3, "x", nil); err == nil {
		t.Error("reopened from area 3")
	}
}

// failing fails the code unit once, as a process ending would leave it.
type failing struct {
	stubExecutor
	fail *bool
}

func (f failing) Run(ctx context.Context, t harness.Task, emit func(harness.Event)) (harness.Result, error) {
	if *f.fail && !strings.HasSuffix(t.Scope[0], "_test.go") {
		*f.fail = false
		return harness.Result{Terminal: harness.FailedSubstrate, Summary: "the process ended"}, nil
	}
	return f.stubExecutor.Run(ctx, t, emit)
}

// A change that stopped is carried on from where it stopped, in its own
// folder, without doing again what it finished, and lands passing the gate.
func TestAStoppedChangeIsResumed(t *testing.T) {
	files := map[string]string{
		"shout_test.go": "package greet\n\nimport \"testing\"\n\nfunc TestShout(t *testing.T) {\n\tif Shout(\"a\") != \"HELLO, A\" {\n\t\tt.Fatal()\n\t}\n}\n",
		"greet.go":      "package greet\n\nimport \"strings\"\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n\nfunc Shout(n string) string { return strings.ToUpper(Greet(n)) }\n",
	}
	land := func(t *testing.T, cfg Config, run *Run) {
		t.Helper()
		run.changed = func() {
			if v := run.View(); v.Question != "" {
				go run.Accept()
			}
		}
		if err := run.Start(context.Background()); err != nil {
			for _, s := range run.View().Steps {
				t.Logf("%d %s", s.Area, s.Text)
			}
			t.Fatalf("it did not land: %v", err)
		}
		c, err := artifact.Load(run.View().Dir)
		if err != nil {
			t.Fatal(err)
		}
		if rep := cfg.Gate.Check(context.Background(), c); !rep.Passed() {
			t.Errorf("it does not pass the gate: %v", rep.Problems())
		}
	}
	t.Run("at the specification", func(t *testing.T) {
		repo := estateFixture(t)
		est := estate.NewLocal()
		var prompts []string
		cfg := Config{Model: stubModel{spec, &prompts}, Executor: stubExecutor{files: files}, Estate: est, Gate: gate.New(est), Person: "a test"}
		ctx, stop := context.WithCancel(context.Background())
		var first *Run
		first = New(cfg, repo, "make it possible to shout the greeting", func() {
			if first.View().Question != "" {
				stop() // the person never answers: the window is closed
			}
		})
		if err := first.Start(ctx); err == nil {
			t.Fatal("the run did not stop")
		}
		written := len(prompts)
		run, err := Resume(cfg, repo, first.View().Dir, nil)
		if err != nil {
			t.Fatal(err)
		}
		if run.from != 1 || !run.resumed {
			t.Fatalf("resumed from %d", run.from)
		}
		land(t, cfg, run)
		for _, p := range prompts[written:] {
			if strings.HasPrefix(p, "The task, as a person wrote it") {
				t.Error("the specification was written again")
			}
		}
	})
	t.Run("stopped by request", func(t *testing.T) {
		repo := estateFixture(t)
		est := estate.NewLocal()
		cfg := Config{Model: stubModel{answer: spec}, Executor: stubExecutor{files: files}, Estate: est, Gate: gate.New(est), Person: "a test"}
		var first *Run
		asked := false
		first = New(cfg, repo, "make it possible to shout the greeting", func() {
			if v := first.View(); v.Question != "" && !asked {
				asked = true
				go func() {
					if err := Stop(v.Dir, "someone in another window"); err != nil {
						t.Error(err)
					}
				}()
			}
		})
		if err := first.Start(context.Background()); err == nil {
			t.Fatal("the run did not stop when asked")
		}
		c, err := artifact.Load(first.View().Dir)
		if err != nil {
			t.Fatal(err)
		}
		if last := c.Meta.Record[len(c.Meta.Record)-1]; last.Stage != "stopped" || !strings.Contains(last.Note, "someone in another window") {
			t.Errorf("the record does not say who stopped it: %+v", last)
		}
		run, err := Resume(cfg, repo, first.View().Dir, nil)
		if err != nil {
			t.Fatal(err)
		}
		land(t, cfg, run)
	})
	t.Run("in execution", func(t *testing.T) {
		repo := estateFixture(t)
		est := estate.NewLocal()
		fail := true
		cfg := Config{Model: stubModel{answer: spec}, Executor: failing{stubExecutor{files: files}, &fail}, Estate: est, Gate: gate.New(est), Person: "a test"}
		first := New(cfg, repo, "make it possible to shout the greeting", nil)
		first.changed = func() {
			if first.View().Question != "" {
				go first.Accept()
			}
		}
		if err := first.Start(context.Background()); err == nil {
			t.Fatal("the run did not stop")
		}
		if c, _ := artifact.Load(first.View().Dir); c.Meta.Record[len(c.Meta.Record)-1].Stage != "stopped" {
			t.Errorf("the record does not say the run stopped: %+v", c.Meta.Record[len(c.Meta.Record)-1])
		}
		run, err := Resume(cfg, repo, first.View().Dir, nil)
		if err != nil {
			t.Fatal(err)
		}
		if run.from != 3 {
			t.Fatalf("resumed from %d", run.from)
		}
		land(t, cfg, run)
	})
	t.Run("after execution", func(t *testing.T) {
		repo := estateFixture(t)
		est := estate.NewLocal()
		cfg := Config{Model: stubModel{answer: spec}, Executor: stubExecutor{files: files}, Estate: est, Gate: gate.New(est), Person: "a test"}
		first := New(cfg, repo, "make it possible to shout the greeting", nil)
		land(t, cfg, first)
		dir := first.View().Dir
		// As if it had stopped before it was judged: the code back as it
		// was, no assurance, no landing.
		for _, name := range []string{"05-assurance.json", "05-assurance.md", "06-landing.json", "06-landing.md"} {
			os.Remove(filepath.Join(dir, name))
		}
		b, _ := os.ReadFile(filepath.Join(dir, "baseline", "greet.go.before"))
		os.WriteFile(filepath.Join(repo, "greet.go"), b, 0o644)
		os.Remove(filepath.Join(repo, "shout_test.go"))
		run, err := Resume(cfg, repo, dir, nil)
		if err != nil {
			t.Fatal(err)
		}
		if run.from != 5 {
			t.Fatalf("resumed from %d", run.from)
		}
		land(t, cfg, run)
		if b, _ := os.ReadFile(filepath.Join(repo, "greet.go")); !strings.Contains(string(b), "Shout") {
			t.Error("the candidate did not land")
		}
	})
}

// A specification written before the model said which tests are written
// first says it through its arbiters: those it keeps out of its scope that
// the estate does not hold yet.
func TestTheTestsWrittenFirstAreInferred(t *testing.T) {
	repo := estateFixture(t)
	s := &meta.Specification{Name: "x", Scope: meta.Scope{Included: []string{"greet.go"}, Excluded: []string{"shout_test.go", "greet_test.go"}},
		Criteria: []meta.Criterion{{ID: "CR-001-01"}, {ID: "CR-001-02"}},
		Arbiters: []meta.Arbiter{{Path: "shout_test.go", Arbitrates: "CR-001-01"}, {Path: "greet_test.go", Arbitrates: "CR-001-02"}}}
	d := draftOf(&meta.Intent{Statement: "s"}, s, repo)
	if !slices.Equal(d.NewTests, []string{"shout_test.go"}) || !slices.Equal(d.SuiteTests, []string{"greet_test.go"}) || !d.Criteria[0].New || d.Criteria[1].New {
		t.Errorf("inferred %v, %v, %+v", d.NewTests, d.SuiteTests, d.Criteria)
	}
}

// saysCannot says, the first time it is handed the code unit, that it cannot
// be done as planned.
type saysCannot struct {
	stubExecutor
	said *bool
}

func (s saysCannot) Run(ctx context.Context, t harness.Task, emit func(harness.Event)) (harness.Result, error) {
	if !*s.said && !strings.HasSuffix(t.Scope[0], "_test.go") {
		*s.said = true
		return harness.Result{Terminal: harness.FailedTask, Summary: harness.BlockedPrefix + "the helper Shoutify it relies on does not exist"}, nil
	}
	return s.stubExecutor.Run(ctx, t, emit)
}

// A unit its agent says cannot be done as planned sends the change back to
// its planner, once, with why; the plan made again is carried out and lands.
func TestAUnitThatCannotBeDoneIsPlannedAgain(t *testing.T) {
	repo := estateFixture(t)
	est := estate.NewLocal()
	said := false
	var prompts []string
	cfg := Config{Model: stubModel{spec, &prompts}, Estate: est, Gate: gate.New(est), Person: "a test",
		Executor: saysCannot{stubExecutor{files: map[string]string{
			"shout_test.go": "package greet\n\nimport \"testing\"\n\nfunc TestShout(t *testing.T) {\n\tif Shout(\"a\") != \"HELLO, A\" {\n\t\tt.Fatal()\n\t}\n}\n",
			"greet.go":      "package greet\n\nimport \"strings\"\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n\nfunc Shout(n string) string { return strings.ToUpper(Greet(n)) }\n",
		}}, &said}}
	var run *Run
	run = New(cfg, repo, "make it possible to shout the greeting", func() {
		if v := run.View(); v.Question != "" {
			go run.Accept()
		}
	})
	if err := run.Start(context.Background()); err != nil {
		for _, s := range run.View().Steps {
			t.Logf("%d %s", s.Area, s.Text)
		}
		t.Fatalf("it did not land: %v", err)
	}
	replanned := false
	for _, p := range prompts {
		if strings.Contains(p, "Shoutify it relies on does not exist") {
			replanned = true
		}
	}
	if !replanned {
		t.Error("the planner was not handed why the unit could not be done")
	}
	dir := run.View().Dir
	if _, err := os.Stat(filepath.Join(dir, "superseded", "1", "02-plan.json")); err != nil {
		t.Error("the plan that could not be carried out was not kept")
	}
	c, _ := artifact.Load(dir)
	if rep := cfg.Gate.Check(context.Background(), c); !rep.Passed() {
		t.Errorf("it does not pass the gate: %v", rep.Problems())
	}
}

// What only a person can see is affirmed by the person reading the finished
// change: no test is written for it, and the assurance asks. An affirmation
// lands the change; a refusal carries it again with what the person said,
// twice at most, and a change never affirmed does not land.
func TestAPersonAffirmsWhatOnlyAPersonCanSee(t *testing.T) {
	human := strings.Replace(spec, `{"criterion":"Nothing else moves"`, `{"criterion":"The greeting reads well","when":"a person reads a shouted greeting","then":"it reads as a greeting, not as an error","plan":"not-required","threshold":"the person's word","judged_by_new_tests":false,"procedure":"human"},
 {"criterion":"Nothing else moves"`, 1)
	files := map[string]string{
		"shout_test.go": "package greet\n\nimport \"testing\"\n\nfunc TestShout(t *testing.T) {\n\tif Shout(\"a\") != \"HELLO, A\" {\n\t\tt.Fatal()\n\t}\n}\n",
		"greet.go":      "package greet\n\nimport \"strings\"\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n\nfunc Shout(n string) string { return strings.ToUpper(Greet(n)) }\n",
	}
	for _, tc := range []struct {
		name    string
		refuse  int // how many times the person refuses before affirming
		lands   bool
		judging int // how many times the person is asked
	}{
		{"affirmed", 0, true, 1},
		{"refused once, then affirmed", 1, true, 2},
		{"never affirmed", 99, false, 1 + maxRefusals},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := estateFixture(t)
			est := estate.NewLocal()
			var goals []string
			cfg := Config{Model: stubModel{answer: human}, Executor: stubExecutor{files: files, goals: &goals}, Estate: est, Gate: gate.New(est), Person: "a test"}
			var run *Run
			judged := 0
			run = New(cfg, repo, "make it possible to shout the greeting", func() {
				v := run.View()
				if v.Question == "" {
					return
				}
				if v.Areas[5] == Waiting {
					judged++
					if !strings.Contains(v.Question, "it reads as a greeting") {
						t.Errorf("the person is not asked what they judge: %s", v.Question)
					}
					if judged > tc.refuse {
						go run.Accept()
					} else {
						go run.Refuse("it reads like an error")
					}
					return
				}
				go run.Accept()
			})
			err := run.Start(context.Background())
			if judged != tc.judging {
				t.Fatalf("the person was asked to judge the finished change %d times, want %d", judged, tc.judging)
			}
			if tc.lands != (err == nil) {
				t.Fatalf("the run ended %v", err)
			}
			if tc.refuse > 0 && !slices.ContainsFunc(goals, func(g string) bool { return strings.Contains(g, "it reads like an error") }) {
				t.Error("the unit carried again was not told what the person said")
			}
			c, _ := artifact.Load(run.View().Dir)
			if tc.lands {
				if rep := cfg.Gate.Check(context.Background(), c); !rep.Passed() {
					t.Errorf("it does not pass the gate: %v", rep.Problems())
				}
				if !slices.ContainsFunc(c.Meta.Assurance.Evidence, func(e meta.Evidence) bool { return e.Kind == meta.HumanAffirmation }) {
					t.Error("the affirmation is not evidence")
				}
			} else if c.Meta.Landing != nil {
				t.Error("a change the person did not affirm landed")
			}
		})
	}
}

// A change stopped because the person did not affirm it, resumed and
// refused again, is carried again from admission all the same.
func TestARefusedChangeResumedIsMended(t *testing.T) {
	human := strings.Replace(spec, `{"criterion":"Nothing else moves"`, `{"criterion":"The greeting reads well","when":"a person reads a shouted greeting","then":"it reads as a greeting, not as an error","plan":"not-required","threshold":"the person's word","judged_by_new_tests":false,"procedure":"human"},
 {"criterion":"Nothing else moves"`, 1)
	files := map[string]string{
		"shout_test.go": "package greet\n\nimport \"testing\"\n\nfunc TestShout(t *testing.T) {\n\tif Shout(\"a\") != \"HELLO, A\" {\n\t\tt.Fatal()\n\t}\n}\n",
		"greet.go":      "package greet\n\nimport \"strings\"\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n\nfunc Shout(n string) string { return strings.ToUpper(Greet(n)) }\n",
	}
	repo := estateFixture(t)
	est := estate.NewLocal()
	cfg := Config{Model: stubModel{answer: human}, Executor: stubExecutor{files: files}, Estate: est, Gate: gate.New(est), Person: "a test"}
	answer := func(run **Run, refuse *int) func() {
		return func() {
			v := (*run).View()
			if v.Question == "" {
				return
			}
			if v.Areas[5] == Waiting && *refuse > 0 {
				*refuse--
				go (*run).Refuse("it reads like an error")
				return
			}
			go (*run).Accept()
		}
	}
	var first *Run
	refuse := 1 + maxRefusals
	first = New(cfg, repo, "make it possible to shout the greeting", answer(&first, &refuse))
	if err := first.Start(context.Background()); err == nil {
		t.Fatal("a change never affirmed landed")
	}
	var run *Run
	refuse = 1
	run, err := Resume(cfg, repo, first.View().Dir, answer(&run, &refuse))
	if err != nil {
		t.Fatal(err)
	}
	if err := run.Start(context.Background()); err != nil {
		t.Fatalf("refused once on resuming, then affirmed, it did not land: %v", err)
	}
}

// A change whose tests, written first, pass on the estate as it stands asks
// for what holds already: it is closed saying so, nothing is changed, and it
// passes the gate.
func TestWhatHoldsAlreadyIsAnAnswer(t *testing.T) {
	repo := estateFixture(t)
	est := estate.NewLocal()
	cfg := Config{Model: stubModel{answer: spec}, Estate: est, Gate: gate.New(est), Person: "a test",
		Executor: stubExecutor{files: map[string]string{
			"shout_test.go": "package greet\n\nimport \"testing\"\n\nfunc TestShout(t *testing.T) {\n\tif Greet(\"a\") != \"Hello, a\" {\n\t\tt.Fatal()\n\t}\n}\n",
			"greet.go":      "package greet\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n",
		}}}
	before, _ := os.ReadFile(filepath.Join(repo, "greet.go"))
	var run *Run
	run = New(cfg, repo, "make sure the greeting names the person", func() {
		if v := run.View(); v.Question != "" {
			go run.Accept()
		}
	})
	if err := run.Start(context.Background()); !errors.Is(err, ErrHolds) {
		t.Fatalf("ended %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(repo, "greet.go")); string(b) != string(before) {
		t.Error("something was changed")
	}
	c, _ := artifact.Load(run.View().Dir)
	if why, closed := gate.Closed(c); !closed || !strings.Contains(why, "holds already") {
		t.Errorf("not closed as holding already: %q", why)
	}
	if rep := cfg.Gate.Check(context.Background(), c); !rep.Passed() {
		t.Errorf("it does not pass the gate: %v", rep.Problems())
	}
}

// A run stopped while it carried a change again with what the person said
// is resumed with it; once the change is admitted again, it is spent.
func TestAMendStoppedIsResumedWithThePersonsWords(t *testing.T) {
	dir := t.TempDir()
	add := func(stage, note string) {
		if err := artifact.AppendRecord(dir, "a-change", meta.Crossing{At: today(), Stage: stage, Object: "SPEC-001", Note: note}); err != nil {
			t.Fatal(err)
		}
	}
	add("admission", "admitted")
	add("execution", mendNote+"the border is missing")
	add("stopped", "stopped in area 4")
	if got := mendOf(dir); got != "the border is missing" {
		t.Errorf("resumed with %q", got)
	}
	add("admission", "admitted again")
	if got := mendOf(dir); got != "" {
		t.Errorf("an admitted mend is resumed with again: %q", got)
	}
}

// What an agent writes is put in the estate's one form before it is kept:
// form is not the agent's to spend turns on.
func TestACandidateIsKeptInTheEstatesForm(t *testing.T) {
	files := map[string]string{
		"shout_test.go": "package greet\n\nimport \"testing\"\n\nfunc TestShout(t *testing.T) {\n\tif Shout(\"a\") != \"HELLO, A\" {\n\t\tt.Fatal()\n\t}\n}\n",
		"greet.go":      "package greet\n\nimport \"strings\"\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n\nfunc Shout(n string) string {\nreturn strings.ToUpper(Greet(n))\n}\n",
	}
	repo := estateFixture(t)
	est := estate.NewLocal()
	cfg := Config{Model: stubModel{answer: spec}, Executor: stubExecutor{files: files}, Estate: est, Gate: gate.New(est), Person: "a test"}
	var run *Run
	run = New(cfg, repo, "make it possible to shout the greeting", func() {
		if run.View().Question != "" {
			go run.Accept()
		}
	})
	if err := run.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(repo, "greet.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "\n\treturn strings.ToUpper") {
		t.Errorf("what entered is not in gofmt's form:\n%s", b)
	}
}

// stoppedAtPlanning stops the run when the planner is asked, the way a
// person's stop lands while the planner works.
type stoppedAtPlanning struct {
	stubModel
	stop context.CancelFunc
}

func (s stoppedAtPlanning) Ask(ctx context.Context, system, prompt string) (string, model.Usage, error) {
	if system == plannerSystem {
		s.stop()
		return "", model.Usage{}, ctx.Err()
	}
	return s.stubModel.Ask(ctx, system, prompt)
}

// A run stopped while its plan is asked for stops: no plan is made in the
// planner's place, and nothing is admitted on one.
func TestARunStoppedWhilePlanningStops(t *testing.T) {
	repo := estateFixture(t)
	est := estate.NewLocal()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := Config{Model: stoppedAtPlanning{stubModel{answer: spec}, cancel}, Executor: stubExecutor{}, Estate: est, Gate: gate.New(est), Person: "a test"}
	var run *Run
	run = New(cfg, repo, "make it possible to shout the greeting", func() {
		if run.View().Question != "" {
			go run.Accept()
		}
	})
	if err := run.Start(ctx); err == nil {
		t.Fatal("a run stopped while planning went on")
	}
	c, _ := artifact.Load(run.View().Dir)
	if c.Meta.Plan != nil || c.Meta.Admission != nil {
		t.Error("a plan was made in the planner's place, though the run was stopped")
	}
}

// Tests written before the package they import exists are vetted against
// a stub of it made from the contract, so that what they get wrong beyond
// the missing package is found then — a value miscounted, a name misspelt
// — and not by the code that has to pass them.
func TestTestsWrittenFirstAreVettedAgainstTheContract(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain")
	}
	ws := t.TempDir()
	write := func(p, s string) {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(ws, p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws, p), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module ex.com/m\n\ngo 1.22\n")
	write("ui/ui.go", "package ui\n")
	write("ui/ui_test.go", "package ui\n\nimport (\n\t\"testing\"\n\n\t\"ex.com/m/notify\"\n)\n\nfunc TestIt(t *testing.T) {\n\tname := notify.Choose(\"darwin\")\n\t_ = name\n\tvar s notify.Sender = notify.System{}\n\ts.Send(\"a\", \"b\")\n}\n")
	r := &Run{units: []unit{
		{id: "WU-001-01", tests: true, scope: []string{"ui/ui_test.go"}},
		{id: "WU-001-02", scope: []string{"notify/notify.go"}, provides: []string{
			"type Sender interface { Send(title, message string) }",
			"type System struct{}",
			"func (System) Send(title, message string)",
			"func Choose(goos string) (name string, args []string, ok bool) — what runs on goos",
			"the package is stateless",
		}},
	}}
	overlay, done := r.contractOverlay(ws)
	defer done()
	if overlay == "" {
		t.Fatal("no stub was made for the package the code unit creates")
	}
	cmd := append([]string{"go", "test", overlay}, goVet([]string{"ui/ui_test.go"})[2:]...)
	c := exec.Command(cmd[0], cmd[1:]...)
	c.Dir = ws
	out, err := c.CombinedOutput()
	ok, why := r.judgeTests()(err == nil, string(out))
	if ok || !strings.Contains(why, "assignment mismatch") {
		t.Errorf("the miscounted values passed: %v\n%s\n%s", ok, why, out)
	}
	if _, err := os.Stat(filepath.Join(ws, "notify")); err == nil {
		t.Error("the stub was written in the workspace")
	}
}

// A configuration that leaves out the estate and the gate carries a task
// all the same, on the estate on disk and the gate over it.
func TestAConfigWithoutEstateOrGateWorks(t *testing.T) {
	repo := estateFixture(t)
	cfg := Config{Model: stubModel{answer: spec}, Executor: stubExecutor{files: map[string]string{
		"shout_test.go": "package greet\n\nimport \"testing\"\n\nfunc TestShout(t *testing.T) {\n\tif Shout(\"a\") != \"HELLO, A\" {\n\t\tt.Fatal()\n\t}\n}\n",
		"greet.go":      "package greet\n\nimport \"strings\"\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n\nfunc Shout(n string) string { return strings.ToUpper(Greet(n)) }\n",
	}}, Person: "a test"}
	var run *Run
	run = New(cfg, repo, "make it possible to shout the greeting", func() {
		if run.View().Question != "" {
			go run.Accept()
		}
	})
	if err := run.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// A file a unit's ticket names, by path or by name in a folder of its
// scope, is in what the unit's agent is handed.
func TestAFileTheTicketNamesIsHanded(t *testing.T) {
	repo := t.TempDir()
	for p, s := range map[string]string{"ui/render_test.go": "package ui\n\nfunc testUI() {}\n", "lib/x.go": "package lib\n"} {
		if err := os.MkdirAll(filepath.Join(repo, filepath.Dir(p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, p), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := &Run{repo: repo}
	u := unit{scope: []string{"ui/notify_test.go"}, brief: "Relies on:\n- testUI, as render_test.go builds it, and lib/x.go; not missing.go"}
	got := r.named(u)
	if !slices.Equal(got, []string{"ui/render_test.go", "lib/x.go"}) {
		t.Errorf("named %v", got)
	}
}

// The names a contract says a unit provides are read from it as the code
// declares them; what it says in prose names nothing.
func TestTheNamesAContractProvides(t *testing.T) {
	got := providedNames([]string{
		"package notify",
		"type Sender interface { Send(title, message string) }",
		"func (System) Send(title, message string)",
		"func commandFor(goos string) (name string, ok bool)",
		"var lookPath = exec.LookPath  // replaced by tests",
		"field Notify notify.Sender on type Options (internal/ui/app.go)",
		"func (u *UI) quiet() bool",
		"(no new names; the frame now calls u.notifyStates(p))",
		"def shout(name):",
	})
	want := []string{"Sender", "Send", "commandFor", "lookPath", "Notify", "quiet", "shout"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// A plan whose code unit relies on a name only a later unit provides, and
// the estate does not have, is sent back; one ordered right is not.
func TestAPlanUsesOnlyWhatComesBefore(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "app.go"), []byte("package ui\n\nfunc (u *UI) frame() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &Run{repo: repo}
	helper := unit{title: "the helper", scope: []string{"project.go"}, brief: "Relies on:\n- u.notify, set by the app unit; frame, as it is\n\nProvides:\n- func (u *UI) notifyStates()\n",
		provides: []string{"func (u *UI) notifyStates()"}}
	fields := unit{title: "the fields", scope: []string{"app.go"}, brief: "Tasks:\n1. add notify\n", provides: []string{"field notify notify.Sender on type UI", "func (u *UI) frame()"}}
	if got := r.orderOf([]unit{helper, fields}); !strings.Contains(got, "relies on notify") {
		t.Errorf("a plan out of order passed: %q", got)
	}
	if got := r.orderOf([]unit{fields, helper}); got != "" {
		t.Errorf("a plan in order was sent back: %q", got)
	}
}

// tiring is an executor whose code unit spends its steps while the tests
// written first are ones no code can pass, until those tests are written
// again; then it carries the unit.
type tiring struct {
	goals *[]string
	calls map[string]int
}

func (s tiring) Name() string    { return "tiring" }
func (s tiring) Version() string { return "1" }
func (s tiring) Run(_ context.Context, t harness.Task, _ func(harness.Event)) (harness.Result, error) {
	*s.goals = append(*s.goals, t.Goal)
	scope := t.Scope[0]
	s.calls[scope]++
	body := "package greet\n\nimport \"strings\"\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n\nfunc Shout(n string) string { return strings.ToUpper(Greet(n)) }\n"
	if scope == "shout_test.go" {
		body = "package greet\n\nimport \"testing\"\n\nfunc TestShout(t *testing.T) {\n\tif Shout(\"a\") != \"HELLO, A\" || Shout(\"a\") == \"HELLO, A\" {\n\t\tt.Fatal(\"asks for two things at once\")\n\t}\n}\n"
		if s.calls[scope] > 1 {
			body = "package greet\n\nimport \"testing\"\n\nfunc TestShout(t *testing.T) {\n\tif Shout(\"a\") != \"HELLO, A\" {\n\t\tt.Fatal()\n\t}\n}\n"
		}
	}
	if err := os.WriteFile(filepath.Join(t.Workspace, scope), []byte(body), 0o644); err != nil {
		return harness.Result{}, err
	}
	if scope != "shout_test.go" && s.calls["shout_test.go"] == 1 {
		return harness.Result{Terminal: harness.Exhausted, Summary: "stopped after 30 steps without finishing"}, nil
	}
	return harness.Result{Terminal: harness.Candidate, Summary: "wrote " + scope}, nil
}

// A code unit that spends its steps on tests no code can pass sends the
// tests back to be written again, with where they fail, and is carried
// again after them.
func TestTestsNoCodeCanPassAreWrittenAgain(t *testing.T) {
	repo := estateFixture(t)
	est := estate.NewLocal()
	var goals []string
	cfg := Config{Model: stubModel{answer: spec}, Executor: tiring{goals: &goals, calls: map[string]int{}}, Estate: est, Gate: gate.New(est), Person: "a test"}
	var run *Run
	run = New(cfg, repo, "make it possible to shout the greeting", func() {
		if run.View().Question != "" {
			go run.Accept()
		}
	})
	if err := run.Start(context.Background()); err != nil {
		t.Fatalf("the run ended: %v", err)
	}
	if !slices.ContainsFunc(goals, func(g string) bool {
		return strings.Contains(g, "Why you are asked again") && strings.Contains(g, "shout_test.go:")
	}) {
		t.Error("the tests were not written again with where they failed")
	}
}

// Every unit's agent builds from the specification's own decisions — its
// requirements and interfaces, and for the tests its test strategy — so
// that what the tests check and what the code does are decided once.
func TestEveryAgentIsHandedTheSpecificationsDecisions(t *testing.T) {
	repo := estateFixture(t)
	est := estate.NewLocal()
	var goals []string
	files := map[string]string{
		"shout_test.go": "package greet\n\nimport \"testing\"\n\nfunc TestShout(t *testing.T) {\n\tif Shout(\"a\") != \"HELLO, A\" {\n\t\tt.Fatal()\n\t}\n}\n",
		"greet.go":      "package greet\n\nimport \"strings\"\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n\nfunc Shout(n string) string { return strings.ToUpper(Greet(n)) }\n",
	}
	cfg := Config{Model: stubModel{answer: spec}, Executor: stubExecutor{files: files, goals: &goals}, Estate: est, Gate: gate.New(est), Person: "a test"}
	var run *Run
	run = New(cfg, repo, "make it possible to shout the greeting", func() {
		if run.View().Question != "" {
			go run.Accept()
		}
	})
	if err := run.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(goals) < 2 {
		t.Fatalf("%d units were carried", len(goals))
	}
	for i, g := range goals {
		if !strings.Contains(g, "What the specification decides") || !strings.Contains(g, "func Shout(n string) string") || !strings.Contains(g, "A shouted greeting is the greeting in capitals") {
			t.Errorf("unit %d was not handed the specification's decisions:\n%s", i, g)
		}
	}
	if !strings.Contains(goals[0], "shout_test.go calls Shout") {
		t.Error("the tests were not handed the test strategy")
	}
}

// planned answers the planner with a plan of its own.
type planned struct {
	stubModel
	plan string
}

func (s planned) Ask(ctx context.Context, system, prompt string) (string, model.Usage, error) {
	if system == plannerSystem {
		return s.plan, model.Usage{Model: "stub"}, nil
	}
	return s.stubModel.Ask(ctx, system, prompt)
}

// Two code units that give each other names are preceded by a contract
// that declares them: it is carried first, the tests are handed it, and
// the code units come after.
func TestTwoCodeUnitsBuildOnAContract(t *testing.T) {
	repo := estateFixture(t)
	est := estate.NewLocal()
	twoFiles := strings.Replace(spec, `"scope":["greet.go"]`, `"scope":["greet.go","shout.go"]`, 1)
	twoUnits := `{"approach":{"summary":"Shout, then Loud.","steps":["test","shout","loud"],"risks":["none"]},
"units":[
 {"title":"the tests","kind":"tests","scope":["shout_test.go"],"criteria":["CR-001-01"],"objective":"Judge from outside.","tasks":["Test Loud."],"provides":["TestShout"],"done_when":["it fails first"]},
 {"title":"Shout","kind":"code","scope":["shout.go"],"criteria":["CR-001-01"],"objective":"Shout.","tasks":["Add Shout."],"provides":["func Shout(n string) string"],"done_when":["builds"]},
 {"title":"Loud","kind":"code","scope":["greet.go"],"criteria":["CR-001-01","CR-001-02"],"objective":"Loud greets.","relies_on":["Shout, from the unit before"],"tasks":["Add Loud using Shout."],"provides":["func Loud(n string) string"],"done_when":["the suite passes"]}]}`
	files := map[string]string{
		"shout_test.go":     "package greet\n\nimport \"testing\"\n\nfunc TestShout(t *testing.T) {\n\tif Loud(\"a\") != \"HELLO, A\" {\n\t\tt.Fatal()\n\t}\n}\n",
		"shout.go":          "package greet\n\nimport \"strings\"\n\nfunc Shout(n string) string { return strings.ToUpper(n) }\n",
		"greet.go":          "package greet\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n\nfunc Loud(n string) string { return Shout(Greet(n)) }\n",
		"declared shout.go": "package greet\n\nfunc Shout(n string) string { return \"\" }\n",
		"declared greet.go": "package greet\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n\nfunc Loud(n string) string { return \"\" }\n",
	}
	var goals, packs []string
	cfg := Config{Model: planned{stubModel{answer: twoFiles}, twoUnits}, Executor: stubExecutor{files: files, goals: &goals, packs: &packs}, Estate: est, Gate: gate.New(est), Person: "a test"}
	var run *Run
	run = New(cfg, repo, "make it possible to shout the greeting", func() {
		if run.View().Question != "" {
			go run.Accept()
		}
	})
	if err := run.Start(context.Background()); err != nil {
		c, _ := artifact.Load(run.View().Dir)
		t.Fatalf("%v: %v", err, cfg.Gate.Check(context.Background(), c).Problems())
	}
	if len(goals) != 4 || !strings.Contains(goals[0], "Declare exactly these names") || !strings.Contains(goals[0], "func Loud(n string) string") {
		t.Fatalf("the contract was not carried first: %d units, the first asked\n%.400s", len(goals), goals[0])
	}
	if !strings.Contains(goals[1], "Write only the tests") || !strings.Contains(packs[1], "Written by WU-001-01") {
		t.Error("the tests were not handed the contract")
	}
	c, _ := artifact.Load(run.View().Dir)
	if rep := cfg.Gate.Check(context.Background(), c); !rep.Passed() {
		t.Errorf("it does not pass the gate: %v", rep.Problems())
	}
}

// Code units that share no file are put in chains of their own; units that
// change a file in common share one, in the plan's order.
func TestChainsOfCodeUnits(t *testing.T) {
	r := &Run{units: []unit{
		{id: "c", contract: true},
		{id: "t", tests: true},
		{id: "a", scope: []string{"notify.go"}},
		{id: "b", scope: []string{"settings.go"}},
		{id: "c1", scope: []string{"app.go"}},
		{id: "c2", scope: []string{"project.go", "app.go"}},
		{id: "last", scope: []string{"view.go"}},
	}}
	got := r.chains(2, 6)
	want := [][]int{{2}, {3}, {4, 5}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("chains %v, want %v", got, want)
	}
	r.units[0].contract = false
	r.units[3].brief = "Relies on:\n- Send, from notify\n"
	r.units[2].provides = []string{"func Send()"}
	if got := r.chains(2, 6); fmt.Sprint(got) != fmt.Sprint([][]int{{2, 3}, {4, 5}}) {
		t.Errorf("without a contract, a unit using another's name was not chained to it: %v", got)
	}
}

// Code units in chains of their own are carried at once, and the change
// lands as if they had been carried one by one.
func TestChainsAreCarriedAtOnce(t *testing.T) {
	repo := estateFixture(t)
	est := estate.NewLocal()
	threeFiles := strings.Replace(spec, `"scope":["greet.go"]`, `"scope":["greet.go","shout.go","whisper.go"]`, 1)
	plan := `{"approach":{"summary":"Shout and whisper, then loud.","steps":["test","shout","whisper","loud"],"risks":["none"]},
"units":[
 {"title":"the tests","kind":"tests","scope":["shout_test.go"],"criteria":["CR-001-01"],"objective":"Judge from outside.","tasks":["Test Loud."],"provides":["TestShout"],"done_when":["it fails first"]},
 {"title":"Shout","kind":"code","scope":["shout.go"],"criteria":["CR-001-01"],"objective":"Shout.","tasks":["Add Shout."],"provides":["func Shout(n string) string"],"done_when":["builds"]},
 {"title":"Whisper","kind":"code","scope":["whisper.go"],"criteria":["CR-001-01"],"objective":"Whisper.","tasks":["Add Whisper."],"provides":["func Whisper(n string) string"],"done_when":["builds"]},
 {"title":"Loud","kind":"code","scope":["greet.go"],"criteria":["CR-001-01","CR-001-02"],"objective":"Loud greets.","tasks":["Add Loud using Shout."],"provides":["func Loud(n string) string"],"done_when":["the suite passes"]}]}`
	files := map[string]string{
		"shout_test.go":       "package greet\n\nimport \"testing\"\n\nfunc TestShout(t *testing.T) {\n\tif Loud(\"a\") != \"HELLO, A\" || Whisper(\"A\") != \"a\" {\n\t\tt.Fatal()\n\t}\n}\n",
		"shout.go":            "package greet\n\nimport \"strings\"\n\nfunc Shout(n string) string { return strings.ToUpper(n) }\n",
		"whisper.go":          "package greet\n\nimport \"strings\"\n\nfunc Whisper(n string) string { return strings.ToLower(n) }\n",
		"greet.go":            "package greet\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n\nfunc Loud(n string) string { return Shout(Greet(n)) }\n",
		"declared shout.go":   "package greet\n\nfunc Shout(n string) string { return \"\" }\n",
		"declared whisper.go": "package greet\n\nfunc Whisper(n string) string { return \"\" }\n",
		"declared greet.go":   "package greet\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n\nfunc Loud(n string) string { return \"\" }\n",
	}
	var goals []string
	cfg := Config{Model: planned{stubModel{answer: threeFiles}, plan}, Executor: stubExecutor{files: files, goals: &goals}, Estate: est, Gate: gate.New(est), Person: "a test"}
	var run *Run
	run = New(cfg, repo, "make it possible to shout and whisper", func() {
		if run.View().Question != "" {
			go run.Accept()
		}
	})
	if err := run.Start(context.Background()); err != nil {
		c, _ := artifact.Load(run.View().Dir)
		t.Fatalf("%v: %v", err, cfg.Gate.Check(context.Background(), c).Problems())
	}
	if len(goals) != 5 {
		t.Errorf("%d units were carried, want the contract, the tests and three code units", len(goals))
	}
	b, _ := os.ReadFile(filepath.Join(repo, "whisper.go"))
	if !strings.Contains(string(b), "ToLower") {
		t.Errorf("what a chain carried did not land:\n%s", b)
	}
}

// A test named by its runner's identifier is taken as the file it is in.
func TestTestsAreNamedByTheirFiles(t *testing.T) {
	d := draft{Scope: []string{"greeting.py"}, NewTests: []string{"test_greeting.py::FarewellTest.test_a", "test_greeting.py::FarewellTest.test_b", "greeting_test.go:TestX", "web/a.spec.ts#says hello", "plain_test.go"}}
	d.files()
	if want := []string{"test_greeting.py", "greeting_test.go", "web/a.spec.ts", "plain_test.go"}; !slices.Equal(d.NewTests, want) {
		t.Errorf("new tests %v, want %v", d.NewTests, want)
	}
}

// Tests the estate's runner did not run have not passed: that they ran is
// read from what the runner said.
func TestTestsThatDidNotRunDidNotPass(t *testing.T) {
	for out, none := range map[string]bool{
		"ok  \texample.com/greeting\t0.257s [no tests to run]\n": true,
		"Ran 0 tests in 0.000s\n\nOK\n":                          true,
		"ℹ tests 0\nℹ pass 0\n":                                  true,
		"ok  \texample.com/greeting\t0.2s\n":                     false,
		"--- PASS: TestTitle (0.00s)\nok\n[no tests to run]\n":   false,
	} {
		if got := nothingRan(out, []string{"TestTitle"}); got != none {
			t.Errorf("nothingRan(%q) = %v, want %v", out, got, none)
		}
	}
}

// The tests a file declares are read as its language declares them.
func TestTestsAreReadInTheirLanguage(t *testing.T) {
	for file, c := range map[string]struct {
		content string
		want    []string
	}{
		"a_test.go":  {"func TestA(t *testing.T) {}\nfunc helper() {}\nfunc TestB(t *testing.T) {}\n", []string{"TestA", "TestB"}},
		"test_a.py":  {"class T(unittest.TestCase):\n    def test_a(self):\n        pass\n    async def test_b(self):\n        pass\n    def helper(self): pass\n", []string{"test_a", "test_b"}},
		"a.test.js":  {"test(\"greet names the person\", () => {});\nit('says goodbye', () => {});\ndescribe(\"x\", () => {});\n", []string{"greet names the person", "says goodbye"}},
		"lib.rs":     {"#[test]\nfn adds() {}\n#[test]\n#[should_panic]\npub fn panics() {}\nfn helper() {}\n", []string{"adds", "panics"}},
		"ATest.java": {"@Test\nvoid greets() {}\n@Test public void bids() {}\n", []string{"greets", "bids"}},
		"README.md":  {"func TestNothing", nil},
	} {
		if got := testsIn(file, c.content); !slices.Equal(got, c.want) {
			t.Errorf("%s: %q, want %q", file, got, c.want)
		}
	}
}

// The estate's own runner, where it keeps one, runs the tests; its language
// still gives an agent its build and its check of one file.
func TestARunnerTheEstateKeepsAndItsLanguage(t *testing.T) {
	repo := t.TempDir()
	for f, s := range map[string]string{"go.mod": "module m\n", "test.sh": "#!/bin/sh\n"} {
		if err := os.WriteFile(filepath.Join(repo, f), []byte(s), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	p, ok := detect(repo)
	if !ok || p.name != "test.sh" || p.vet == nil || p.format == nil || len(p.build) == 0 {
		t.Fatalf("a Go estate with its own runner lost its language: %+v", p)
	}
	if got := p.only([]string{"TestA", "greet (names)"}); !slices.Equal(got, []string{"test.sh", "TestA", `greet \(names\)`}) {
		t.Errorf("the runner was asked %q", got)
	}
	if got := quickCheck([]string{"a.py", "b.py"}); !slices.Equal(got, []string{"python3", "-m", "py_compile", "a.py", "b.py"}) {
		t.Errorf("python's check: %q", got)
	}
	if got := quickCheck([]string{"a.js"}); !slices.Equal(got, []string{"node", "--check", "a.js"}) {
		t.Errorf("node's check: %q", got)
	}
}

// An agent asks the estate's model where a name is defined and who uses
// it, and is answered with places and their lines.
func TestTheEstateAnswersAnAgentByName(t *testing.T) {
	repo := estateFixture(t)
	g := estateGraph{repo: estate.NewLocal().Repo(repo), root: repo}
	def, err := g.Defined("Greet")
	if err != nil || !strings.Contains(def, "greet.go:") || !strings.Contains(def, "func Greet") {
		t.Errorf("Greet is defined at %q (%v)", def, err)
	}
	if none, _ := g.Defined("Nowhere"); !strings.Contains(none, "defined nowhere") {
		t.Errorf("a name defined nowhere: %q", none)
	}
	callers, err := g.Callers("Greet")
	if err != nil || !strings.Contains(callers, "_test.go:") {
		t.Errorf("Greet is called from %q (%v)", callers, err)
	}
}
