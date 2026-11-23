package ui

// How a step reaches an open view more than once, confirmed against the
// estate as it stands, using only existing names (render_test.go, macro.go,
// project.go, remote.go):
//
// A run is carried by the service, not by the window. u.start calls
// u.begin (remote.go), which hangs a remoteRun on the project; every frame
// the view of that run is re-read through remoteRun.View(), which asks the
// service for the run's state again. What the service tells is turned into
// flow.View by viewOf, and shownOf (project.go) hands it to the frame as a
// shown. So a second delivery wave to the same open view flows through the
// same path as the first: the executor emits again, the service's state
// moves, the window's `changed` callback (wired in testUI through
// service.Dial) lands in u.moved, which calls u.invalidate — the same
// repaint by which cost and status already move — and the next frame's
// shownOf re-reads the run and the feed is painted from the held sequence,
// with no view reopened. The same path is what a person's leave-and-return
// re-delivery flows through, and u.reload (project.go) re-reading a change
// while its view stays open is the landed counterpart of it.
//
// The tests below deliver the second wave with a canned executor that emits,
// lets the view paint, and emits again (twoWaveExecutor): the only extension
// beyond the estate's machinery, local to this file.

import (
	"context"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/igannytskyi/iflow/internal/estate"
	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/gate"
	"github.com/igannytskyi/iflow/internal/harness"
	"github.com/igannytskyi/iflow/internal/model"
)

// waitShort never waits longer than five seconds, so a failing test fails
// in seconds.
func waitShort(t *testing.T, what string, done func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !done(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("gave up waiting for %s", what)
		}
	}
}

// stepEvent is one written step: words the agent put down.
func stepEvent(words string) harness.Event {
	return harness.Event{At: time.Now(), Kind: harness.Got, Detail: words}
}

// agentEvents is every event the run's view holds, unit by unit in a
// deterministic order.
func agentEvents(s shown) []harness.Event {
	var evs []harness.Event
	for _, unit := range sortedStrings(s.v.Agent) {
		evs = append(evs, s.v.Agent[unit]...)
	}
	return evs
}

// lastStep is the last event whose words are shown at all.
func lastStep(evs []harness.Event) harness.Event {
	for i := len(evs) - 1; i >= 0; i-- {
		if strings.TrimSpace(stepWords(evs[i])) != "" {
			return evs[i]
		}
	}
	return harness.Event{}
}

// lineAt is a painted line and where it stands.
type lineAt struct {
	words string
	y     int
}

// panelAt is what a painted feed panel holds: the cost line's place and
// every feed line's place, captured with recordOp.
type panelAt struct {
	costY, costBottom int
	lines             []lineAt
}

// capturePanel lays the right panel as the task view will: the cost line of
// the area on view, then the feed section under it, capturing each painted
// line's position with recordOp. The words are what the code under test
// returns; the positions are what the painter recorded.
func capturePanel(u *UI, p *project, s shown, max, height int) panelAt {
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 2, PxPerSp: 2},
		Constraints: layout.Exact(image.Pt(600, height)), Now: time.Now()}
	cs := inArea(s.costs, p.area)
	if len(cs) == 0 {
		cs = s.costs
	}
	var out panelAt
	y := 0
	cost := recordOp(gtx, u.oneLine(sizeSmall, u.p.Line, costLine(cs)))
	cost.at(gtx, 0, y)
	out.costY = y
	y += cost.dims.Size.Y
	out.costBottom = y
	for _, ln := range u.feedSection(p, s, max) {
		rec := recordOp(gtx, u.oneLine(sizeSmall, u.p.Line, ln))
		rec.at(gtx, 0, y)
		out.lines = append(out.lines, lineAt{ln, y})
		y += rec.dims.Size.Y
	}
	return out
}

func wordsOf(lines []lineAt) []string {
	out := make([]string, len(lines))
	for i, ln := range lines {
		out[i] = ln.words
	}
	return out
}

// startRun types a task into a scratch repository and carries it with the
// test's own model and executor, as TestRunRenders does. The task view of
// the run is open from the moment the run begins.
func startRun(t *testing.T, cfg flow.Config) (*UI, *project) {
	t.Helper()
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
	cfg.Estate, cfg.Gate, cfg.Person = est, gate.New(est), "a test"
	u := testUI(t, &cfg, repo, filepath.Join(gate.Home(), "examples", "greeting"))
	p := u.cur
	u.start(p, "make it possible to shout the greeting")
	waitShort(t, "the criteria", func() bool { v, _ := p.view(); return v.Question != "" })
	p.run.Accept()
	return u, p
}

// twoWaveExecutor is the canned executor delivering its steps in two waves
// through the run's delivery path: it emits the first wave, waits for the
// test to open the way, then emits the second wave to the same open view.
type twoWaveExecutor struct{ wave2 chan struct{} }

func (twoWaveExecutor) Name() string    { return "canned" }
func (twoWaveExecutor) Version() string { return "1" }

func (e twoWaveExecutor) Run(ctx context.Context, t harness.Task, emit func(harness.Event)) (harness.Result, error) {
	files := map[string]string{
		"shout_test.go": "package greet\n\nimport \"testing\"\n\nfunc TestShout(t *testing.T) {\n\tif Shout(\"a\") != \"HELLO, A\" {\n\t\tt.Fatal()\n\t}\n}\n",
		"greet.go":      "package greet\n\nimport \"strings\"\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n\nfunc Shout(n string) string { return strings.ToUpper(Greet(n)) }\n",
	}
	turn := model.Usage{Model: "canned-1", Input: 3, CacheRead: 18200, CacheWrite: 900, Output: 240, CostUSD: 0.0121}
	// Wave one: the test unit writes its file.
	emit(harness.Event{At: time.Now(), Kind: harness.Called, Tool: "Write", Detail: "shout_test.go", Usage: turn})
	emit(harness.Event{At: time.Now(), Kind: harness.Got, Detail: "wave one wrote the shout test"})
	_ = os.WriteFile(filepath.Join(t.Workspace, "shout_test.go"), []byte(files["shout_test.go"]), 0o644)
	// The view paints the first wave before the second is written.
	select {
	case <-e.wave2:
	case <-ctx.Done():
		return harness.Result{}, ctx.Err()
	}
	// Wave two: the code unit writes its file, delivered to the view that
	// is already open.
	emit(harness.Event{At: time.Now(), Kind: harness.Called, Tool: "Write", Detail: "greet.go", Usage: turn})
	emit(harness.Event{At: time.Now(), Kind: harness.Got, Detail: "wave two added Shout"})
	_ = os.WriteFile(filepath.Join(t.Workspace, "greet.go"), []byte(files["greet.go"]), 0o644)
	emit(harness.Event{At: time.Now(), Kind: harness.Ended, Detail: harness.Candidate, Usage: turn})
	return harness.Result{Terminal: harness.Candidate, Summary: "wrote in two waves", Steps: 2, Turns: 2,
		Usage: turn, Elapsed: 41 * time.Second}, nil
}

// TestFeedPaintsStepsUnderTheCostLine: the painted words of each step stand
// in the panel after the words of the cost line, in the order written,
// newest last, and no step stands above the cost line (CR-001-01).
func TestFeedPaintsStepsUnderTheCostLine(t *testing.T) {
	u, p := startRun(t, flow.Config{Model: cannedModel{}, Executor: cannedExecutor{}})
	paint := newPainter(t)
	waitShort(t, "the run", func() bool { return !p.running() })
	p.area = 4
	paint.paint(u, "feed-steps")
	s, ok := p.shownOf()
	if !ok {
		t.Fatal("the task view of the run is not open")
	}
	panel := capturePanel(u, p, s, 1000, 900)
	if panel.costBottom <= panel.costY {
		t.Fatal("the cost line is not painted")
	}
	if len(panel.lines) == 0 {
		t.Fatal("the feed paints no step")
	}
	for i, ln := range panel.lines {
		if ln.y <= panel.costY {
			t.Errorf("%q is painted at y=%d, not below the cost line's words at y=%d", ln.words, ln.y, panel.costY)
		}
		if i > 0 && ln.y <= panel.lines[i-1].y {
			t.Errorf("%q is not painted after %q", ln.words, panel.lines[i-1].words)
		}
	}
	// The painted words are the feed's own words for the steps the run
	// holds, in the order written, newest last.
	want := visibleFeed(agentEvents(s), 1000)
	if got := wordsOf(panel.lines); !slices.Equal(got, want) {
		t.Errorf("the panel paints %q, want %q", got, want)
	}
	if last := stepWords(lastStep(agentEvents(s))); panel.lines[len(panel.lines)-1].words != last {
		t.Errorf("the newest step %q is not painted last: %q", last, panel.lines[len(panel.lines)-1].words)
	}
}

// TestFeedArrivesLiveWithoutReopening: a step written while the view is
// already open is painted in the same panel, after the earlier steps, with
// the view opened once and never reopened (CR-001-02).
func TestFeedArrivesLiveWithoutReopening(t *testing.T) {
	wave2 := make(chan struct{})
	u, p := startRun(t, flow.Config{Model: cannedModel{}, Executor: twoWaveExecutor{wave2: wave2}})
	paint := newPainter(t)
	p.area = 4
	// The view is open from the run's start (p.selected is runKey) and is
	// never shown again; every delivery flows through the frame's intake
	// and shownOf's re-read of the run.
	var first panelAt
	waitShort(t, "the first wave painted", func() bool {
		p.area = 4
		paint.paint(u, "feed-wave-one")
		s, ok := p.shownOf()
		if !ok {
			return false
		}
		first = capturePanel(u, p, s, 1000, 900)
		return len(first.lines) > 0
	})
	// A paint between waves, with no new events, shows what it showed.
	paint.paint(u, "feed-quiet")
	if s, ok := p.shownOf(); !ok {
		t.Fatal("the task view of the run is not open")
	} else if quiet := capturePanel(u, p, s, 1000, 900); !slices.Equal(wordsOf(quiet.lines), wordsOf(first.lines)) {
		t.Errorf("a paint with no new events changed the feed: %q, was %q", wordsOf(quiet.lines), wordsOf(first.lines))
	}
	// The second wave arrives through the same delivery path, on the
	// repaint the existing invalidation provides.
	close(wave2)
	var second panelAt
	waitShort(t, "the second wave painted", func() bool {
		p.area = 4
		paint.paint(u, "feed-wave-two")
		s, ok := p.shownOf()
		if !ok {
			return false
		}
		second = capturePanel(u, p, s, 1000, 900)
		return len(second.lines) > len(first.lines)
	})
	if p.selected != runKey {
		t.Errorf("the view was reopened: selected is %q", p.selected)
	}
	if second.lines[len(second.lines)-1].y <= first.lines[len(first.lines)-1].y {
		t.Errorf("the second wave's step is not painted after the first wave's: y=%d, first wave ended at y=%d",
			second.lines[len(second.lines)-1].y, first.lines[len(first.lines)-1].y)
	}
}

// TestFeedShowsItsNothingSentenceWhenNoStepIsShown: an area with no shown
// steps — none written, or all empty or whitespace — renders exactly the
// one sentence, painted, and no step text (CR-001-03).
func TestFeedShowsItsNothingSentenceWhenNoStepIsShown(t *testing.T) {
	u := testUI(t, nil, filepath.Join(gate.Home(), "examples", "greeting"))
	p := u.cur
	p.area = 4
	// Nothing written at all.
	if lines := u.feedSection(p, shown{}, 20); len(lines) != 1 || lines[0] != noStepsSentence {
		t.Errorf("an area with no events renders %q, want exactly the one sentence", lines)
	}
	// Only empty or whitespace steps written.
	s := shown{v: flow.View{Agent: map[string][]harness.Event{"canned": {stepEvent("   "), stepEvent("\t")}}}, live: true}
	if lines := u.feedSection(p, s, 20); len(lines) != 1 || lines[0] != noStepsSentence {
		t.Errorf("an area whose only steps are blank renders %q, want exactly the one sentence", lines)
	}
	// Painted: the sentence stands, and no step text beside it.
	panel := capturePanel(u, p, s, 20, 900)
	if len(panel.lines) != 1 || panel.lines[0].words != noStepsSentence {
		t.Errorf("the painted feed holds %q, want exactly the one sentence", wordsOf(panel.lines))
	}
}

// TestFeedShowsAStepOnceWhateverTheDelivery: a re-delivered step is painted
// once, two distinct steps with the same wording are painted twice in
// order, and at the string level an empty step between two same-worded
// steps disturbs neither the count nor the order (CR-001-04).
func TestFeedShowsAStepOnceWhateverTheDelivery(t *testing.T) {
	u, p := startRun(t, flow.Config{Model: cannedModel{}, Executor: cannedExecutor{}})
	paint := newPainter(t)
	waitShort(t, "the run", func() bool { return !p.running() })
	p.area = 4
	// The same events delivered again, as when a person leaves and
	// returns: the view is re-read through the run's delivery path and
	// painted again, showing every step once.
	paint.paint(u, "feed-first-look")
	s, ok := p.shownOf()
	if !ok {
		t.Fatal("the task view of the run is not open")
	}
	first := capturePanel(u, p, s, 1000, 900)
	paint.paint(u, "feed-returned")
	s, ok = p.shownOf()
	if !ok {
		t.Fatal("the task view of the run is not open")
	}
	second := capturePanel(u, p, s, 1000, 900)
	if !slices.Equal(wordsOf(first.lines), wordsOf(second.lines)) {
		t.Errorf("the re-delivered feed changed: %q, was %q", wordsOf(second.lines), wordsOf(first.lines))
	}
	// Two distinct steps with the same wording: two lines, in order.
	same := shown{v: flow.View{Agent: map[string][]harness.Event{"canned": {stepEvent("wrote it"), stepEvent("wrote it")}}}, live: true}
	panel := capturePanel(u, p, same, 100, 900)
	if len(panel.lines) != 2 || panel.lines[0].words != panel.lines[1].words {
		t.Errorf("two distinct same-worded steps render %q, want two lines", wordsOf(panel.lines))
	}
	// Directly: the same at the string level, with an empty step between
	// two same-worded steps disturbing neither count nor order, and
	// filtering never renumbering the steps it keeps.
	two := []harness.Event{stepEvent("wrote it"), stepEvent("wrote it")}
	if got := visibleFeed(two, 10); len(got) != 2 {
		t.Errorf("two distinct same-worded steps give %d lines, want 2", len(got))
	}
	mixed := []harness.Event{stepEvent("first"), stepEvent("   "), stepEvent("first")}
	want := []string{stepWords(mixed[0]), stepWords(mixed[2])}
	if got := visibleFeed(mixed, 10); !slices.Equal(got, want) {
		t.Errorf("an empty step between two same-worded steps gives %q, want %q", got, want)
	}
}

// TestFeedKeepsTheNewestStepsUnderTheCostLine: when more steps are written
// than the panel has height for, the cost line is painted, every painted
// step stands below it, and the newest step written is among those painted
// (CR-001-05).
func TestFeedKeepsTheNewestStepsUnderTheCostLine(t *testing.T) {
	u := testUI(t, nil, filepath.Join(gate.Home(), "examples", "greeting"))
	p := u.cur
	p.area = 4
	var evs []harness.Event
	for i := 1; i <= 8; i++ {
		evs = append(evs, stepEvent(fmt.Sprintf("step %d wrote its piece", i)))
	}
	s := shown{v: flow.View{Agent: map[string][]harness.Event{"canned": evs}}, live: true}
	// A first paint measures the lines, so the second can fix the height
	// to hold fewer steps than were written.
	wide := capturePanel(u, p, s, 100, 900)
	if len(wide.lines) < 2 {
		t.Fatalf("the wide panel paints %d steps, want more", len(wide.lines))
	}
	lineH := wide.lines[1].y - wide.lines[0].y
	height := wide.costBottom + 2*lineH + lineH/2
	fit := (height - wide.costBottom) / lineH
	if fit >= len(evs) {
		t.Fatalf("the tight panel would hold %d of %d steps", fit, len(evs))
	}
	tight := capturePanel(u, p, s, fit, height)
	if tight.costBottom <= tight.costY {
		t.Fatal("the cost line is not painted")
	}
	for _, ln := range tight.lines {
		if ln.y <= tight.costY {
			t.Errorf("%q is painted at y=%d, not below the cost line's words at y=%d", ln.words, ln.y, tight.costY)
		}
	}
	newest := stepWords(evs[len(evs)-1])
	if tight.lines[len(tight.lines)-1].words != newest {
		t.Errorf("the newest step %q is not among those painted: the last is %q", newest, tight.lines[len(tight.lines)-1].words)
	}
}
