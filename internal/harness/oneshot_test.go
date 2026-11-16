package harness

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/igannytskyi/iflow/internal/model"
)

// scripted answers each question with the next of its answers, and keeps
// every prompt it was handed.
type scripted struct {
	answers []string
	asked   *[]string
}

func (s scripted) Name() string { return "scripted" }
func (s scripted) Ask(_ context.Context, _, prompt string) (string, model.Usage, error) {
	*s.asked = append(*s.asked, prompt)
	a := s.answers[len(*s.asked)-1]
	return a, model.Usage{Input: 100, Output: 10, CostUSD: 0.01}, nil
}

// Each agent is a new question: the second is handed what the first wrote
// and what the check said, not a conversation, and nothing outside the
// scope is written.
func TestOneshotAsksAFreshAgentWhenTheCheckFails(t *testing.T) {
	ws := t.TempDir()
	var asked []string
	m := scripted{asked: &asked, answers: []string{
		"=== FILE: a.txt\nwrong\n=== END\n=== FILE: ../escape.txt\nx\n=== END\n=== SUMMARY: first try",
		"Here it is:\n=== FILE: a.txt\n```\nright\n```\n=== END\n=== SUMMARY: second try",
	}}
	check := []string{"sh", "-c", "grep -q right a.txt || { echo 'a.txt is not right'; exit 1; }"}
	var events []Event
	res, err := Oneshot{Model: m}.Run(context.Background(),
		Task{Workspace: ws, Goal: "make a.txt right", Scope: []string{"a.txt"}, Context: "# Pack\n\nsmall", Check: check},
		func(e Event) { events = append(events, e) })
	if err != nil {
		t.Fatal(err)
	}
	if res.Terminal != Candidate || res.Turns != 2 || res.Summary != "second try" {
		t.Fatalf("the run ended %+v", res)
	}
	if b, _ := os.ReadFile(filepath.Join(ws, "a.txt")); string(b) != "right\n" {
		t.Errorf("a.txt holds %q", b)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(ws), "escape.txt")); err == nil {
		t.Error("a file outside the scope was written")
	}
	second := asked[1]
	for _, want := range []string{"make a.txt right", "# Pack", "wrong", "a.txt is not right"} {
		if !strings.Contains(second, want) {
			t.Errorf("the second agent was not handed %q:\n%s", want, second)
		}
	}
	if strings.Contains(second, "first try") {
		t.Error("the second agent was handed the first one's words, a conversation")
	}
	refused := 0
	for _, e := range events {
		if e.Kind == Refused {
			refused++
		}
	}
	if refused != 1 || res.Usage.CostUSD != 0.02 {
		t.Errorf("refused %d, spent %v", refused, res.Usage.CostUSD)
	}
}

// named is a scripted model with a name of its own.
type named struct {
	scripted
	name string
}

func (n named) Name() string { return n.name }

// The last attempt goes to the stronger model when the cheap one's failed.
func TestTheLastAttemptGoesToTheStrongerModel(t *testing.T) {
	ws := t.TempDir()
	var cheapAsked, strongAsked []string
	wrong := "=== FILE: a.txt\nwrong\n=== END\n=== SUMMARY: tried"
	cheap := named{scripted{answers: []string{wrong, wrong}, asked: &cheapAsked}, "cheap"}
	strong := named{scripted{answers: []string{"=== FILE: a.txt\nright\n=== END\n=== SUMMARY: done"}, asked: &strongAsked}, "strong"}
	check := []string{"sh", "-c", "grep -q right a.txt"}
	res, err := Oneshot{Model: cheap, Stronger: strong}.Run(context.Background(),
		Task{Workspace: ws, Goal: "make a.txt right", Scope: []string{"a.txt"}, Check: check}, func(Event) {})
	if err != nil || res.Terminal != Candidate {
		t.Fatalf("%+v %v", res, err)
	}
	if len(cheapAsked) != 2 || len(strongAsked) != 1 {
		t.Errorf("the cheap model was asked %d times and the stronger %d", len(cheapAsked), len(strongAsked))
	}
}

// An existing file is changed by edits; a long one written whole and cut
// short is refused; and a check failing only outside the scope ends the run
// at once instead of spending the attempts.
func TestEditsTruncationAndFaultsOutside(t *testing.T) {
	ws := t.TempDir()
	long := strings.Repeat("line\n", 200) + "func Old() {}\n"
	if err := os.WriteFile(filepath.Join(ws, "big.go"), []byte(long), 0o644); err != nil {
		t.Fatal(err)
	}
	var asked []string
	m := scripted{asked: &asked, answers: []string{
		"=== FILE: big.go\nfunc New() {}\n=== END\n=== SUMMARY: rewrote it",
		"=== EDIT: big.go\n<<<<<<< SEARCH\nfunc Old() {}\n=======\nfunc New() {}\n>>>>>>> REPLACE\n=== END\n=== SUMMARY: edited",
	}}
	res, err := Oneshot{Model: m}.Run(context.Background(),
		Task{Workspace: ws, Goal: "rename Old to New", Scope: []string{"big.go"}, Check: []string{"sh", "-c", "grep -q 'func New' big.go && grep -c line big.go | grep -q 200"}},
		func(Event) {})
	if err != nil || res.Terminal != Candidate || len(asked) != 2 {
		t.Fatalf("%+v %v after %d answers", res, err, len(asked))
	}
	if !strings.Contains(asked[1], "lost more than half") {
		t.Errorf("the second agent was not told why the whole file was refused:\n%s", asked[1])
	}

	asked = nil
	m = scripted{asked: &asked, answers: []string{"=== FILE: a.go\nx\n=== END\n=== SUMMARY: s", "unused", "unused"}}
	res, _ = Oneshot{Model: m}.Run(context.Background(),
		Task{Workspace: ws, Goal: "g", Scope: []string{"a.go"}, Check: []string{"sh", "-c", "echo 'tests/a_test.go:3:5: syntax error: unexpected }'; exit 1"}},
		func(Event) {})
	if res.Terminal != FailedTask || len(asked) != 1 || !strings.Contains(res.Summary, "tests/a_test.go") {
		t.Errorf("a fault outside the scope did not end the run at once: %+v after %d answers", res, len(asked))
	}

	// A test finding a name missing that this unit was to define is this
	// unit's to mend: the run goes on to its next agent.
	asked = nil
	m = scripted{asked: &asked, answers: []string{"=== FILE: a.go\nx\n=== END\n=== SUMMARY: s", "=== FILE: a.go\ny\n=== END\n=== SUMMARY: s"}}
	res, _ = Oneshot{Model: m, Attempts: 2}.Run(context.Background(),
		Task{Workspace: ws, Goal: "g", Scope: []string{"a.go"}, Check: []string{"sh", "-c", "grep -q y a.go || { echo 'tests/a_test.go:3:5: undefined: Thing'; exit 1; }"}},
		func(Event) {})
	if res.Terminal != Candidate || len(asked) != 2 {
		t.Errorf("a missing name this unit was to define ended the run: %+v after %d answers", res, len(asked))
	}
}
