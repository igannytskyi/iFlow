package flow

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/estate"
	"github.com/igannytskyi/iflow/internal/gate"
	"github.com/igannytskyi/iflow/internal/harness"
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
		if err := os.WriteFile(filepath.Join(t.Workspace, p), []byte(s.files[p]), 0o644); err != nil {
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
	r := &Run{units: []unit{{id: "WU-001-02", provides: []string{"func Shout(n string) string", "func (p *project) closeTop() bool"}}}}
	judge := r.judgeTests()
	if ok, why := judge(false, "vet: ./shout_test.go:5:9: undefined: Shout\n./x_test.go:9:4: p.closeTop undefined (type *project has no field or method closeTop)\n"); !ok {
		t.Errorf("a missing promised name was held against the tests: %s", why)
	}
	if ok, why := judge(false, "vet: internal/ui/graph_test.go:32:24: r.Semantic undefined (type *input.Router has no field or method Semantic)\n"); ok || !strings.Contains(why, "Semantic") {
		t.Errorf("an invented method of a library passed: %v %s", ok, why)
	}
	if ok, _ := judge(false, "vet: x_test.go:11:15: not enough arguments in call to testUI\n"); ok {
		t.Error("the wrong arguments to a function that exists passed")
	}
}
