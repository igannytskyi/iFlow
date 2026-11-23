package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/harness"
	"github.com/igannytskyi/iflow/internal/model"
	"github.com/igannytskyi/iflow/internal/protocol"
)

// fakeRun is a run that says it began, asks the person one question, and
// lands or stops on the answer; an agent of one unit acts on the way.
type fakeRun struct {
	mu      sync.Mutex
	v       flow.View
	answer  chan bool
	why     string
	changed func()
}

func (r *fakeRun) step(area int, text string) {
	r.mu.Lock()
	r.v.Steps = append(r.v.Steps, flow.Step{At: time.Now(), Area: area, Text: text})
	r.v.Areas[area] = flow.Working
	r.mu.Unlock()
	r.changed()
}

func (r *fakeRun) Start(ctx context.Context) error {
	r.step(1, "asking the author")
	r.mu.Lock()
	r.v.Agent = map[string][]harness.Event{"WU-1": {{Kind: "called", Tool: "Write", Detail: "greet.go"}}}
	r.v.Question = "Accept the specification?"
	r.v.Areas[1] = flow.Waiting
	r.mu.Unlock()
	r.changed()
	select {
	case ok := <-r.answer:
		r.mu.Lock()
		r.v.Question = ""
		r.mu.Unlock()
		if !ok {
			r.step(1, "sent back: "+r.why)
			return errors.New("refused")
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	r.step(6, "landed")
	r.mu.Lock()
	r.v.Finished, r.v.Areas[6] = true, flow.Done
	r.mu.Unlock()
	return nil
}

func (r *fakeRun) View() flow.View {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := r.v
	v.Steps = append([]flow.Step(nil), r.v.Steps...)
	return v
}

func (r *fakeRun) Accept()           { r.answer <- true }
func (r *fakeRun) Refuse(why string) { r.why = why; r.answer <- false }

type fakeMaker struct{}

func (fakeMaker) New(project, statement string, _ []string, changed func()) (Run, error) {
	return &fakeRun{v: flow.View{Repo: project, Statement: statement}, answer: make(chan bool), changed: changed}, nil
}
func (fakeMaker) Resume(string, []string, func()) (Run, error) { return nil, errors.New("not here") }
func (fakeMaker) Reopen(string, int, string, []string, func()) (Run, error) {
	return nil, errors.New("not here")
}

// serve starts a service on a fresh socket and answers a screen connected
// to it, told whenever a run moves.
func serve(t *testing.T) (*Client, chan string, string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	socket := filepath.Join(shortTemp(t), "s.sock")
	l, err := Listen(ctx, socket)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = New(ctx, Config{Maker: fakeMaker{}, Person: "a test"}).Serve(ctx, l) }()
	moved := make(chan string, 64)
	c, err := Dial(ctx, socket, "a test", func(run string) { moved <- run })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c, moved, socket
}

// shortTemp is a folder whose path a Unix socket can be made in: macOS
// bounds a socket's path to about a hundred bytes.
func shortTemp(t *testing.T) string {
	dir, err := os.MkdirTemp("/tmp", "iflow-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// until waits for the screen to hold a run of which ok holds.
func until(t *testing.T, c *Client, moved chan string, run string, ok func(State) bool) State {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		if s, told := c.Run(run); told && ok(s) {
			return s
		}
		select {
		case <-moved:
		case <-deadline:
			s, _ := c.Run(run)
			t.Fatalf("the screen was never told it: %+v", s)
		}
	}
}

// A screen starts a task, is told what moves — steps and an agent's
// events, each once and in order — answers the question it is asked, and
// is told the run landed.
func TestAScreenCarriesARunThroughTheService(t *testing.T) {
	c, moved, _ := serve(t)
	ctx := context.Background()
	run, err := c.Start(ctx, protocol.StartParams{Project: "/estate", Statement: "shout"})
	if err != nil {
		t.Fatal(err)
	}
	s := until(t, c, moved, run, func(s State) bool { return s.Question != "" })
	if s.Statement != "shout" || len(s.Steps) != 1 || len(s.Agent["WU-1"]) != 1 || s.Areas[1] != protocol.Waiting {
		t.Fatalf("not told the run as it stands: %+v", s)
	}
	if err := c.Accept(ctx, run); err != nil {
		t.Fatal(err)
	}
	s = until(t, c, moved, run, func(s State) bool { return s.Finished })
	if len(s.Steps) != 2 || s.Steps[1].Text != "landed" || s.Error != "" || len(s.Agent["WU-1"]) != 1 {
		t.Errorf("not told the run landed, each step once: %+v", s)
	}
	runs, err := c.Runs(ctx)
	if err != nil || len(runs) != 1 || !runs[0].Finished {
		t.Errorf("the service does not list the run as it ended: %+v %v", runs, err)
	}
}

// A refusal goes to the run with its ground, and the run's end is told
// with why; a run not waiting is not answered.
func TestARefusalIsCarriedWithItsGround(t *testing.T) {
	c, moved, _ := serve(t)
	ctx := context.Background()
	run, err := c.Start(ctx, protocol.StartParams{Project: "/estate", Statement: "shout"})
	if err != nil {
		t.Fatal(err)
	}
	until(t, c, moved, run, func(s State) bool { return s.Question != "" })
	if err := c.Refuse(ctx, run, "too loud"); err != nil {
		t.Fatal(err)
	}
	s := until(t, c, moved, run, func(s State) bool { return s.Finished })
	if s.Error != "refused" || !strings.Contains(s.Steps[len(s.Steps)-1].Text, "too loud") {
		t.Errorf("the refusal did not reach the run: %+v", s)
	}
	if err := c.Accept(ctx, run); err == nil {
		t.Error("a run that waits for nothing was answered")
	}
}

// A second screen sees the same run from its start, and a screen of any
// version is told who the service is.
func TestScreensShareRunsAndAgreeOnAVersion(t *testing.T) {
	c, moved, socket := serve(t)
	ctx := context.Background()
	run, err := c.Start(ctx, protocol.StartParams{Project: "/estate", Statement: "shout"})
	if err != nil {
		t.Fatal(err)
	}
	until(t, c, moved, run, func(s State) bool { return s.Question != "" })
	other, err := Dial(ctx, socket, "a second screen", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	runs, err := other.Runs(ctx)
	if err != nil || len(runs) != 1 || runs[0].Question == "" {
		t.Fatalf("the second screen is not shown the run waiting: %+v %v", runs, err)
	}
	if err := other.Watch(ctx, run); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for s, ok := other.Run(run); !ok || len(s.Steps) != 1; s, ok = other.Run(run) {
		if time.Now().After(deadline) {
			t.Fatalf("the second screen was not told the run from its start: %+v", s)
		}
		time.Sleep(10 * time.Millisecond)
	}
	// A screen of another version is told who the service is, so that it
	// can replace it: it is never refused that.
	var who protocol.InitializeResult
	if err := c.conn.Call(ctx, protocol.Initialize, protocol.InitializeParams{Version: protocol.Version + 1, Client: "a later screen"}, &who); err != nil {
		t.Fatalf("a screen of another version was not told who the service is: %v", err)
	}
	if who.Version != protocol.Version || who.PID != os.Getpid() || who.Build == "" || who.Running != 1 {
		t.Errorf("the service did not say who it is: %+v", who)
	}
	if err := c.Watch(ctx, "run-99"); err == nil {
		t.Error("a run not carried here was watched")
	}
}

// A service will not listen where another listens; it takes the place of a
// socket one that ended left behind.
func TestOneServiceListensAtASocket(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	socket := filepath.Join(shortTemp(t), "s.sock")
	if err := os.WriteFile(socket, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := Listen(ctx, socket)
	if err != nil {
		t.Fatalf("a socket left behind was not taken: %v", err)
	}
	go func() { _ = New(ctx, Config{Maker: fakeMaker{}, Person: "a test"}).Serve(ctx, l) }()
	if _, err := Listen(ctx, socket); !errors.Is(err, ErrRunning) {
		t.Errorf("a second service listened where one listens: %v", err)
	}
}

// A change is read whole for a screen: its documents in the data model, its
// record and its costs; a candidate's steps are asked for by its id alone.
func TestAChangeIsReadWhole(t *testing.T) {
	c, _, _ := serve(t)
	ctx := context.Background()
	dir, err := filepath.Abs("../../examples/greeting/changes/greet-the-world")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := c.Change(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Change.State != protocol.Landed || doc.Specification == nil || doc.Plan == nil || len(doc.Plan.Units) == 0 ||
		doc.Assurance == nil || doc.Landing == nil || len(doc.Record) == 0 || doc.Document == "" {
		t.Errorf("the change was not read whole: state %s, %d record lines, plan %v", doc.Change.State, len(doc.Record), doc.Plan != nil)
	}
	if _, err := c.Trace(ctx, dir, "../greet-the-world"); err == nil {
		t.Error("a candidate named by a path was read")
	}
	if _, err := c.Change(ctx, filepath.Join(dir, "nothing")); err == nil {
		t.Error("a folder that holds no change was read as one")
	}
}

// saying is a model that answers what it is told to.
type saying string

func (saying) Name() string { return "a test model" }
func (s saying) Ask(context.Context, string, string) (string, model.Usage, error) {
	return string(s), model.Usage{CostUSD: 0.001}, nil
}

// A conversation about a task is had through the service: the screen that
// watches it is told the question and the answer, and it is kept beside the
// task.
func TestAConversationIsHadThroughTheService(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	socket := filepath.Join(shortTemp(t), "s.sock")
	l, err := Listen(ctx, socket)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_ = New(ctx, Config{Maker: fakeMaker{}, Person: "a test", Model: func() model.Model { return saying("it landed") }}).Serve(ctx, l)
	}()
	moved := make(chan string, 16)
	c, err := Dial(ctx, socket, "a test", func(k string) { moved <- k })
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	task := t.TempDir()
	ref := protocol.ChatRef{Project: filepath.Dir(task), Dir: task}
	if err := c.Talk(ctx, ref); err != nil {
		t.Fatal(err)
	}
	if err := c.Ask(ctx, ref, "did it land?"); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(5 * time.Second)
	for {
		if s, ok := c.Chat(ref); ok && len(s.Messages) == 2 && s.Messages[1].Text == "it landed" && !s.Waiting {
			break
		}
		select {
		case <-moved:
		case <-deadline:
			s, _ := c.Chat(ref)
			t.Fatalf("the conversation was not told: %+v", s)
		}
	}
	if _, err := os.Stat(filepath.Join(task, "chat.jsonl")); err != nil {
		t.Error("the conversation is not kept beside the task")
	}
}

// A service asked to end, keeping its runs, stops them and leaves them for
// the next, which carries them on; a screen of a later build replaces it.
func TestAServiceIsReplacedAndItsRunsCarriedOn(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dir := shortTemp(t)
	socket, keep := filepath.Join(dir, "s.sock"), filepath.Join(dir, "kept.json")
	begin := func() chan error {
		l, err := Listen(ctx, socket)
		if err != nil {
			t.Fatal(err)
		}
		ended := make(chan error, 1)
		go func() { ended <- New(ctx, Config{Maker: fakeMaker{}, Person: "a test", KeepFile: keep}).Serve(ctx, l) }()
		return ended
	}
	ended := begin()
	c, err := Dial(ctx, socket, "a test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Start(ctx, protocol.StartParams{Project: "/estate", Statement: "shout"}); err != nil {
		t.Fatal(err)
	}
	if err := Replace(ctx, c, socket); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-ended:
		if err != nil {
			t.Fatalf("the service ended with %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the service did not end")
	}
	begin()
	next, err := Dial(ctx, socket, "a test", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	runs, err := next.Runs(ctx)
	if err != nil || len(runs) != 1 || runs[0].Statement != "shout" || runs[0].Project != "/estate" {
		t.Fatalf("the next service did not carry the run on: %+v %v", runs, err)
	}
	if _, err := os.Stat(keep); err == nil {
		t.Error("the runs kept are kept still, to be carried on twice")
	}
}

// A screen replaces only a service older than itself: another version of
// the protocol, or another build written before its own.
func TestOnlyAnOlderServiceIsReplaced(t *testing.T) {
	id, at := Build()
	if id == "" {
		t.Skip("this program's executable cannot be read")
	}
	for _, c := range []struct {
		res  protocol.InitializeResult
		want bool
	}{
		{protocol.InitializeResult{Version: protocol.Version, Build: id, BuiltAt: at}, false},
		{protocol.InitializeResult{Version: protocol.Version - 1, Build: id, BuiltAt: at}, true},
		{protocol.InitializeResult{Version: protocol.Version, Build: "other", BuiltAt: at.Add(-time.Hour)}, true},
		{protocol.InitializeResult{Version: protocol.Version, Build: "other", BuiltAt: at.Add(time.Hour)}, false},
	} {
		if got := Older(c.res); got != c.want {
			t.Errorf("%+v is older: %v, want %v", c.res, got, c.want)
		}
	}
}
