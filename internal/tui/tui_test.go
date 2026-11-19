package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/harness"
	"github.com/igannytskyi/iflow/internal/service"
)

func TestTheTerminalIsKnownByWhatItSays(t *testing.T) {
	for _, c := range []struct {
		env    map[string]string
		name   string
		notify bool
	}{
		{map[string]string{"TERM_PROGRAM": "ghostty"}, "Ghostty", true},
		{map[string]string{"TERM": "xterm-kitty"}, "kitty", true},
		{map[string]string{"TERM_PROGRAM": "WezTerm"}, "WezTerm", true},
		{map[string]string{"TERM_PROGRAM": "iTerm.app"}, "iTerm2", true},
		{map[string]string{"TERM": "xterm-256color"}, "a terminal", false},
	} {
		got := detect(func(k string) string { return c.env[k] })
		if got.name != c.name || (got.notify != nil) != c.notify {
			t.Errorf("%v is %q (notifies %v), want %q (%v)", c.env, got.name, got.notify != nil, c.name, c.notify)
		}
	}
	plain := detect(func(string) string { return "" })
	if plain.link("/a/change.md", "the task") != "the task" {
		t.Error("a terminal that opens no links was given one")
	}
	if l := detect(func(k string) string { return map[string]string{"TERM_PROGRAM": "ghostty"}[k] }).link("/a/change.md", "the task"); !strings.Contains(l, "\x1b]8;;file://") || !strings.Contains(l, "/a/change.md") {
		t.Errorf("no link to the file: %q", l)
	}
	if n := osc777("waiting; now", "a\x07b"); strings.Count(n, "\x07") != 1 {
		t.Errorf("a notification's words ended its sequence early: %q", n)
	}
}

// fakeRun asks one question and lands on the answer; one unit's agent acts.
type fakeRun struct {
	mu      sync.Mutex
	v       flow.View
	answer  chan bool
	changed func()
}

func (r *fakeRun) Start(ctx context.Context) error {
	r.mu.Lock()
	r.v.Steps = append(r.v.Steps, flow.Step{Area: 1, Text: "asking the author"})
	r.v.Areas[1], r.v.Question = flow.Waiting, "Accept the specification?"
	r.v.Agent = map[string][]harness.Event{"WU-001-02": {{Kind: "called", Tool: "Write", Detail: "greet.go"}}}
	r.mu.Unlock()
	r.changed()
	select {
	case <-r.answer:
	case <-ctx.Done():
		return ctx.Err()
	}
	r.mu.Lock()
	r.v.Question, r.v.Areas[1], r.v.Areas[6] = "", flow.Done, flow.Done
	r.v.Steps = append(r.v.Steps, flow.Step{Area: 6, Text: "landed"})
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
func (r *fakeRun) Accept()       { r.answer <- true }
func (r *fakeRun) Refuse(string) { r.answer <- false }

type fakeMaker struct{}

func (fakeMaker) New(project, statement string, _ []string, changed func()) (service.Run, error) {
	return &fakeRun{v: flow.View{Repo: project, Statement: statement}, answer: make(chan bool), changed: changed}, nil
}
func (fakeMaker) Resume(string, []string, func()) (service.Run, error) { return nil, errors.New("no") }
func (fakeMaker) Reopen(string, int, string, []string, func()) (service.Run, error) {
	return nil, errors.New("no")
}

// drive applies msg and, for a key, what its command answers, as the
// program would. What waits on the service is the test's to feed, from
// moved: a command that waits is never run here.
func drive(m Model, msg tea.Msg) Model {
	next, cmd := m.Update(msg)
	m = next.(Model)
	if _, key := msg.(tea.KeyMsg); key && cmd != nil {
		if out := cmd(); out != nil {
			switch out.(type) {
			case tea.BatchMsg, tea.KeyMsg:
			default:
				m = drive(m, out)
			}
		}
	}
	return m
}

// A task typed on the screen is started in the service; the screen shows
// it, its unit's agent and the question; a is the answer, and the screen
// shows the run landed.
func TestTheScreenCarriesATask(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dir, err := os.MkdirTemp("/tmp", "iflow-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	socket := filepath.Join(dir, "s.sock")
	l, err := service.Listen(ctx, socket)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = service.New(ctx, fakeMaker{}, "a test").Serve(ctx, l) }()
	moved := make(chan string, 64)
	client, err := service.Dial(ctx, socket, "a test", func(run string) { moved <- run })
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	m := New(ctx, client, moved, "/estate")
	m.term = terminal{name: "a terminal"}
	m = drive(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	if !strings.Contains(m.View(), "press n") {
		t.Fatalf("an empty project does not say how to begin:\n%s", m.View())
	}
	m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	for _, r := range "shout the greeting" {
		m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = drive(m, tea.KeyMsg{Type: tea.KeyEnter})
	waitFor(t, &m, moved, func(v string) bool { return strings.Contains(v, "waiting for you") })
	v := m.View()
	for _, want := range []string{"shout the greeting", "1 spec ?", "WU-001-02", "› write greet.go", "[a] accept"} {
		if !strings.Contains(v, want) {
			t.Errorf("the screen does not show %q:\n%s", want, v)
		}
	}
	m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	waitFor(t, &m, moved, func(v string) bool { return strings.Contains(v, "6 land ✓") })
}

// waitFor applies what the service tells until the screen shows what ok
// asks.
func waitFor(t *testing.T, m *Model, moved chan string, ok func(string) bool) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for !ok(m.View()) {
		select {
		case run := <-moved:
			*m = drive(*m, movedMsg(run))
		case <-deadline:
			t.Fatalf("the screen never showed it:\n%s", m.View())
		}
	}
}
