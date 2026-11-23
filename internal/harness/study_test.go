package harness

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/igannytskyi/iflow/internal/model"
)

// studying answers each turn with the next of its moves, and keeps what it
// was told.
type studying struct {
	moves []model.Message
	heard *[]model.Message
	tools *[]int
}

func (s *studying) Name() string { return "scripted" }
func (s *studying) Chat(_ context.Context, msgs []model.Message, tools []model.Tool) (model.Message, model.Usage, error) {
	*s.heard = msgs
	*s.tools = append(*s.tools, len(tools))
	m := s.moves[0]
	s.moves = s.moves[1:]
	return m, model.Usage{CostUSD: 0.001}, nil
}

type oneGraph struct{}

func (oneGraph) Defined(name string) (string, error) { return name + " is defined at greet.go:3", nil }
func (oneGraph) Callers(name string) (string, error) { return "main calls " + name, nil }

func call(name, args string) model.ToolCall {
	c := model.ToolCall{ID: name}
	c.Function.Name, c.Function.Arguments = name, args
	return c
}

// A decision is taken on the code: the agent asks the graph, reads a file,
// is refused a write, and answers in the form asked.
func TestAStudyReadsTheCodeBeforeItDecides(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "greet.go"), []byte("package greet\n\nfunc Greet() string { return \"hi\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var heard []model.Message
	var offered []int
	m := &studying{heard: &heard, tools: &offered, moves: []model.Message{
		{ToolCalls: []model.ToolCall{call("find_definition", `{"name":"Greet"}`), call("find_callers", `{"name":"Greet"}`)}},
		{ToolCalls: []model.ToolCall{call("read_file", `{"path":"greet.go"}`), call("write_file", `{"path":"greet.go","content":"x"}`)}},
		{Content: `{"units": []}`},
	}}
	answer, u, calls, err := Study{Dir: dir, Graph: oneGraph{}, System: "Plan.", Prompt: "Plan the change."}.Run(context.Background(), m, func(Event) {})
	if err != nil || answer != `{"units": []}` || calls != 4 || u.CostUSD != 0.003 {
		t.Fatalf("the study did not answer as asked: %q %v %d %v", answer, u, calls, err)
	}
	var said []string
	for _, msg := range heard {
		if msg.Role == "tool" {
			said = append(said, msg.Content)
		}
	}
	all := strings.Join(said, "\n")
	for _, want := range []string{"defined at greet.go:3", "main calls Greet", "func Greet()", "refused"} {
		if !strings.Contains(all, want) {
			t.Errorf("the study was not answered %q:\n%s", want, all)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "greet.go")); !strings.Contains(string(b), "func Greet") {
		t.Error("a study wrote to the estate")
	}
	if offered[0] != 5 {
		t.Errorf("a study was offered %d tools, not the five that read", offered[0])
	}
}

// Past its steps, a study is asked to answer with what it has, and is
// offered no tool.
func TestAStudyAnswersWhenItHasReadEnough(t *testing.T) {
	var heard []model.Message
	var offered []int
	m := &studying{heard: &heard, tools: &offered, moves: []model.Message{
		{ToolCalls: []model.ToolCall{call("list_dir", `{"path":"."}`)}},
		{Content: "the answer"},
	}}
	answer, _, _, err := Study{Dir: t.TempDir(), System: "s", Prompt: "p", MaxSteps: 1}.Run(context.Background(), m, func(Event) {})
	if err != nil || answer != "the answer" || offered[1] != 0 {
		t.Errorf("a study past its steps was not asked to answer: %q %v %v", answer, offered, err)
	}
}
