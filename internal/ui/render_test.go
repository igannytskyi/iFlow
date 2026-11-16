package ui

import (
	"context"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/igannytskyi/iflow"
	"github.com/igannytskyi/iflow/internal/estate"
	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/gate"
	"github.com/igannytskyi/iflow/internal/harness"
	"github.com/igannytskyi/iflow/internal/method"
	"github.com/igannytskyi/iflow/internal/model"
)

// painter draws frames without a window and, when IFLOW_SHOTS names a
// directory, keeps each as a PNG there.
type painter struct {
	t   *testing.T
	win *headless.Window
	dir string
}

const shotW, shotH, shotScale = 1440, 900, 2

func newPainter(t *testing.T) *painter {
	win, err := headless.NewWindow(shotW*shotScale, shotH*shotScale)
	if err != nil {
		t.Skipf("no headless GPU here: %v", err)
	}
	t.Cleanup(win.Release)
	return &painter{t: t, win: win, dir: os.Getenv("IFLOW_SHOTS")}
}

func (p *painter) paint(u *UI, name string) {
	p.t.Helper()
	gtx := func(ops *op.Ops) C {
		return layout.Context{Ops: ops, Metric: unit.Metric{PxPerDp: shotScale, PxPerSp: shotScale},
			Constraints: layout.Exact(image.Pt(shotW*shotScale, shotH*shotScale)), Now: time.Now()}
	}
	// The first frame takes in clicks and what a run wrote; the second is
	// what a person would see.
	var ops op.Ops
	u.frame(gtx(&ops))
	ops.Reset()
	u.frame(gtx(&ops))
	if err := p.win.Frame(&ops); err != nil {
		p.t.Fatalf("%s: %v", name, err)
	}
	if p.dir == "" {
		return
	}
	img := image.NewRGBA(image.Rectangle{Max: p.win.Size()})
	if err := p.win.Screenshot(img); err != nil {
		p.t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(p.dir, name+".png"))
	if err != nil {
		p.t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		p.t.Fatal(err)
	}
}

func testUI(t *testing.T, cfg *flow.Config, projects ...string) *UI {
	t.Helper()
	m, err := method.Load(iflow.Texts)
	if err != nil {
		t.Fatal(err)
	}
	est := estate.NewLocal()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s := &settings{Widths: map[string]float32{}, path: filepath.Join(t.TempDir(), "desk.json")}
	u := newUI(ctx, Options{Model: m, Gate: gate.New(est), Parallel: 2, Projects: projects, Flow: cfg}, s)
	u.setTheme(os.Getenv("IFLOW_DARK") != "")
	return u
}

func waitFor(t *testing.T, what string, done func() bool) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Minute); !done(); time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("gave up waiting for %s", what)
		}
	}
}

// TestHistoryRenders draws a project as it opens, with its newest task on
// view and numbers in every place, then an older task, an area and an
// object of it, and the settings.
func TestHistoryRenders(t *testing.T) {
	greeting := filepath.Join(gate.Home(), "examples", "greeting")
	u := testUI(t, nil, greeting)
	p := u.cur
	if p == nil || len(p.ws.Changes) == 0 {
		t.Fatal("the example has no changes")
	}
	waitFor(t, "the gate", func() bool { return int(p.checked.Load()) == len(p.ws.Changes) })
	paint := newPainter(t)
	if _, ok := p.shownOf(); ok {
		t.Fatal("a project opens on a task rather than on its overview")
	}
	newest, old := p.order[0], p.order[len(p.order)-1]
	for _, s := range []struct {
		name string
		set  func()
	}{
		{"project", func() {}},
		{"project-landed", func() { p.showDone = true }},
		{"task", func() {
			p.show(newest.Change.Dir, newest)
			if s, ok := p.shownOf(); !ok || s.c != newest || len(s.costs) == 0 {
				t.Fatal("the task chosen is not on view, with what it cost")
			}
		}},
		{"task-older", func() { p.show(old.Change.Dir, old) }},
		{"task-area", func() { p.area = 5 }},
		{"task-object", func() { p.item = old.Item("VE-001-01") }},
		{"new-intent", func() { p.selected = newKey }},
		{"foundation-economy", func() { p.selected, p.foundation = "", 9 }},
		{"foundation-estate", func() { p.foundation = 7 }},
		{"settings", func() { u.preferring = true }},
	} {
		s.set()
		paint.paint(u, s.name)
	}
}

type cannedModel struct{}

func (cannedModel) Name() string { return "canned" }
func (cannedModel) Ask(_ context.Context, system, _ string) (string, model.Usage, error) {
	use := model.Usage{Model: "canned", Input: 400, CacheRead: 2000, Output: 600, CostUSD: 0.02}
	switch {
	case strings.HasPrefix(system, "You review"):
		return `{"verdict":"sufficient","findings":[{"severity":"minor","where":"Risks","finding":"say what an empty name shouts","suggestion":"add it to the edge cases"}],"questions_for_the_person":[]}`, use, nil
	case strings.HasPrefix(system, "You divide"):
		return `{"approach":{"summary":"Add Shout beside Greet, after a test that fails without it.","steps":["write the test","add Shout"],"risks":["none"]},
"units":[{"title":"the shout test","kind":"tests","scope":["shout_test.go"],"criteria":["CR-001-01"],"requirements":["FR-1"],"objective":"Judge shouting from outside.","tasks":["Test that Shout(\"a\") is HELLO, A."],"provides":["TestShout"],"done_when":["it fails before Shout exists"]},
{"title":"Shout","kind":"code","scope":["greet.go"],"criteria":["CR-001-01","CR-001-02"],"requirements":["FR-1"],"objective":"Shout greets in capitals.","tasks":["Add func Shout(n string) string returning Greet(n) in capitals."],"provides":["func Shout(n string) string"],"constraints":["leave Greet as it is"],"done_when":["the whole suite passes"]}]}`, use, nil
	}
	return `{"slug":"shout","name":"shout","statement":"The greeting can be shouted.","class":"C3","prose":"A shouted greeting is in capitals.",
"document":{"summary":"A greeting can be shouted: the same words, in capitals.","context":"Greetings are only spoken today.","goals":["a greeting can be shouted"],"non_goals":["whispering"],"users":["a caller of the package"],"user_stories":["As a caller, I want to shout a greeting, so that it stands out."],"flows":["A caller shouts a name and gets the greeting in capitals."],"requirements":["Shout returns the greeting of the name in capitals."],"architecture":["Shout wraps Greet."],"interfaces":["func Shout(n string) string"],"components":["greet.go holds Shout beside Greet."],"alternatives":["a flag on Greet — rejected: it changes Greet's signature"],"rollout":["ships with the package; nothing to migrate"],"success_metrics":["callers use Shout"],"edge_cases":["an empty name shouts HELLO, "],"risks":["none"],"test_strategy":["shout_test.go calls Shout"]},
"criteria":[{"criterion":"A shouted greeting is in capitals","when":"a greeting is shouted","then":"every letter is a capital","plan":"required","threshold":"exact text","judged_by_new_tests":true},
{"criterion":"Nothing else moves","when":"the whole suite is run","then":"it stands as before","plan":"not-required","threshold":"identical","judged_by_new_tests":false}],
"scope":["greet.go"],"new_tests":["shout_test.go"],"existing_tests":["greet_test.go"],"termination":[{"condition":"shouting needs a dependency","action":"stop"}]}`, use, nil
}

type cannedExecutor struct{}

func (cannedExecutor) Name() string    { return "canned" }
func (cannedExecutor) Version() string { return "1" }
func (cannedExecutor) Run(_ context.Context, t harness.Task, emit func(harness.Event)) (harness.Result, error) {
	files := map[string]string{
		"shout_test.go": "package greet\n\nimport \"testing\"\n\nfunc TestShout(t *testing.T) {\n\tif Shout(\"a\") != \"HELLO, A\" {\n\t\tt.Fatal()\n\t}\n}\n",
		"greet.go":      "package greet\n\nimport \"strings\"\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n\nfunc Shout(n string) string { return strings.ToUpper(Greet(n)) }\n",
	}
	var total model.Usage
	for _, p := range t.Scope {
		turn := model.Usage{Model: "canned-1", Input: 3, CacheRead: 18200, CacheWrite: 900, Output: 240, CostUSD: 0.0121}
		total = total.Add(turn)
		emit(harness.Event{At: time.Now(), Kind: harness.Called, Tool: "Read", Detail: "greet.go", Usage: turn})
		emit(harness.Event{At: time.Now(), Kind: harness.Got, Detail: "package greet …"})
		emit(harness.Event{At: time.Now(), Kind: harness.Called, Tool: "Write", Detail: p, Usage: turn})
		total = total.Add(turn)
		emit(harness.Event{At: time.Now(), Kind: harness.Got, Detail: "written"})
		_ = os.WriteFile(filepath.Join(t.Workspace, p), []byte(files[p]), 0o644)
	}
	emit(harness.Event{At: time.Now(), Kind: harness.Ended, Detail: harness.Candidate, Usage: total})
	return harness.Result{Terminal: harness.Candidate, Summary: "wrote what the unit asked", Steps: 2, Turns: 2,
		Usage: total, Elapsed: 41 * time.Second}, nil
}

// TestRunRenders types a task into one of two projects, draws the criteria
// waiting to be accepted, accepts them, and draws the run landed: the graph,
// an object opened, and what the executor did.
func TestRunRenders(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain")
	}
	repo := filepath.Join(t.TempDir(), "greet")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"go.mod":        "module example.com/greet\n\ngo 1.22\n",
		"greet.go":      "package greet\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n",
		"greet_test.go": "package greet\n\nimport \"testing\"\n\nfunc TestGreet(t *testing.T) {\n\tif Greet(\"a\") != \"Hello, a\" {\n\t\tt.Fatal()\n\t}\n}\n",
	} {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	est := estate.NewLocal()
	cfg := flow.Config{Model: cannedModel{}, Executor: cannedExecutor{}, Estate: est, Gate: gate.New(est), Person: "a test"}
	u := testUI(t, &cfg, repo, filepath.Join(gate.Home(), "examples", "greeting"))
	p := u.cur
	if p.path != repo || len(u.projects) != 2 {
		t.Fatalf("the workbench opened %d projects, showing %s", len(u.projects), p.path)
	}
	paint := newPainter(t)
	u.start(p, "make it possible to shout the greeting")
	waitFor(t, "the criteria", func() bool { v, _ := p.view(); return v.Question != "" })
	paint.paint(u, "run-review")
	p.run.Accept()
	waitFor(t, "the run", func() bool { return !p.running() })
	p.area = 6
	paint.paint(u, "run-landed")
	if p.live == nil || len(p.live.Items) == 0 {
		t.Fatal("the workbench shows nothing the run carried")
	}
	p.area = 1
	paint.paint(u, "run-spec")
	p.area = 2
	paint.paint(u, "run-plan")
	p.area = 4
	p.open["diff/"+filepath.Join(p.live.Change.Dir, "candidates", "CA-001-02", "files", "greet.go")] = true
	paint.paint(u, "run-diff")
	p.area, p.item = 4, p.live.Item("EV-001-01-01")
	paint.paint(u, "run-execution")
	// What the run wrote is taken in between frames; here no frame is
	// drawn while waiting, so the wait takes it in itself.
	waitFor(t, "the gate on the landed change", func() bool {
		u.intake()
		return len(p.ws.Changes) == 1 && int(p.checked.Load()) == len(p.ws.Changes)
	})
	if len(p.ws.Changes) != 1 {
		t.Fatalf("the landed change is not among the project's changes: %d", len(p.ws.Changes))
	}
	p.item = nil
	paint.paint(u, "run-reloaded")
	if s, _ := p.shownOf(); !s.live {
		t.Error("once the project is read again, its run is no longer on view")
	}
	u.select_(u.projects[1])
	paint.paint(u, "other-project")
}

// Closing a task that will not be finished records it, keeps everything it
// wrote, and moves it out of the active ones.
func TestClosingATask(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "greeting")
	if err := exec.Command("cp", "-R", filepath.Join(gate.Home(), "examples", "greeting"), dir).Run(); err != nil {
		t.Fatal(err)
	}
	u := testUI(t, nil, dir)
	p := u.cur
	c := p.order[len(p.order)-1]
	before, _ := os.ReadDir(c.Change.Dir)
	if err := u.close(p, c, "a later task does it"); err != nil {
		t.Fatal(err)
	}
	u.load(p)
	for _, again := range p.order {
		if again.Change.Dir != c.Change.Dir {
			continue
		}
		closed, note := again.Closed()
		if !closed || !strings.Contains(note, "a later task does it") {
			t.Errorf("closed %v, %q", closed, note)
		}
		if s := (shown{c: again}); s.state(p) != "closed" {
			t.Errorf("the task stands %q", s.state(p))
		}
	}
	after, _ := os.ReadDir(c.Change.Dir)
	if len(after) != len(before) {
		t.Errorf("closing changed what the task kept: %d files, then %d", len(before), len(after))
	}
}

// TestFramesAreInstant holds every view of a project to the workbench's
// first goal: once a view has been drawn, drawing it again takes well under
// a millisecond. The budget here is loose for slow machines; a frame that
// reads disk or recomputes what it showed last time blows through it.
func TestFramesAreInstant(t *testing.T) {
	u := testUI(t, nil, filepath.Join(gate.Home(), "examples", "greeting"))
	p := u.cur
	waitFor(t, "the gate", func() bool { return int(p.checked.Load()) == len(p.ws.Changes) })
	frame := func() time.Duration {
		var ops op.Ops
		gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 2, PxPerSp: 2},
			Constraints: layout.Exact(image.Pt(2880, 1800)), Now: time.Now()}
		began := time.Now()
		u.frame(gtx)
		return time.Since(began)
	}
	c := p.order[0]
	views := []struct {
		name string
		set  func()
	}{
		{"overview", func() { p.selected = "" }},
		{"task", func() { p.show(c.Change.Dir, c) }},
		{"plan", func() { p.area = 2 }},
		{"execution", func() { p.area = 4 }},
		{"assurance", func() { p.area = 5 }},
		{"settings", func() { u.preferring = true }},
	}
	for _, v := range views {
		v.set()
		frame() // the first frame of a view reads and shapes what it shows
		best := time.Hour
		for range 5 {
			best = min(best, frame())
		}
		if best > 8*time.Millisecond {
			t.Errorf("%s: a frame takes %v", v.name, best)
		}
	}
}
