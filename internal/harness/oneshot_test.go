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
