package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/igannytskyi/iflow/internal/model"
)

// chatting answers each turn with the next of its turns, and keeps every
// conversation it was sent.
type chatting struct {
	turns []model.Message
	sent  *[][]model.Message
}

func (c chatting) Name() string { return "chatting" }
func (c chatting) Chat(_ context.Context, msgs []model.Message, _ []model.Tool) (model.Message, model.Usage, error) {
	*c.sent = append(*c.sent, msgs)
	return c.turns[len(*c.sent)-1], model.Usage{Input: 100, Output: 10, CostUSD: 0.01}, nil
}

func calling(name string, args map[string]string) model.Message {
	b, _ := json.Marshal(args)
	var tc model.ToolCall
	tc.ID, tc.Type = name+"-1", "function"
	tc.Function.Name, tc.Function.Arguments = name, string(b)
	return model.Message{ToolCalls: []model.ToolCall{tc}}
}

// An agent that says it is done while the check fails is told so and goes
// on; an edit copied with its spacing changed still lands; nothing outside
// the scope is written.
func TestTheLoopIsDoneWhenTheCheckPasses(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "a.go"), []byte("package a\n\nfunc A() int {\n\treturn 1\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var sent [][]model.Message
	m := chatting{sent: &sent, turns: []model.Message{
		calling("write_file", map[string]string{"path": "b.go", "content": "x"}),
		{Content: "done"},
		calling("edit_file", map[string]string{"path": "a.go", "old": "  return 1", "new": "\treturn 2"}),
		{Content: "A returns 2 now."},
	}}
	check := []string{"sh", "-c", "grep -q 'return 2' a.go || { echo 'a.go:4: A must return 2'; exit 1; }"}
	res, err := Loop{Model: m}.Run(context.Background(), Task{Workspace: ws, Goal: "A returns 2", Scope: []string{"a.go"}, Check: check}, func(Event) {})
	if err != nil {
		t.Fatal(err)
	}
	if res.Terminal != Candidate || res.Summary != "A returns 2 now." || res.Turns != 4 {
		t.Fatalf("the run ended %+v", res)
	}
	if _, err := os.Stat(filepath.Join(ws, "b.go")); err == nil {
		t.Error("a file outside the scope was written")
	}
	told := sent[2][len(sent[2])-1]
	if told.Role != "user" || !strings.Contains(told.Content, "A must return 2") {
		t.Errorf("the agent was not told the check fails: %+v", told)
	}
}

// What the first rung cannot carry, the second carries from where the
// first left it, and the cost is the two together.
func TestTheLadderHandsOnWhatTheFirstCouldNotCarry(t *testing.T) {
	ws := t.TempDir()
	var asked []string
	first := Oneshot{Attempts: 1, Model: scripted{asked: &asked, answers: []string{"=== FILE: a.txt\nhalf\n=== END\n=== SUMMARY: tried"}}}
	var sent [][]model.Message
	then := Loop{Model: chatting{sent: &sent, turns: []model.Message{
		calling("edit_file", map[string]string{"path": "a.txt", "old": "half", "new": "whole"}),
		{Content: "finished"},
	}}}
	check := []string{"sh", "-c", "grep -q whole a.txt"}
	res, err := Ladder{First: first, Then: then}.Run(context.Background(), Task{Workspace: ws, Goal: "a.txt whole", Scope: []string{"a.txt"}, Check: check}, func(Event) {})
	if err != nil {
		t.Fatal(err)
	}
	if res.Terminal != Candidate || res.Turns != 3 || res.Usage.CostUSD < 0.029 {
		t.Fatalf("the run ended %+v", res)
	}
	if !strings.Contains(sent[0][1].Content, "Where the first attempt left it") {
		t.Error("the agent was not told where the first attempt left the unit")
	}
}

func TestReplaceOnce(t *testing.T) {
	for _, c := range []struct{ s, old, new, want, fails string }{
		{s: "a\nb\nc", old: "b", new: "B", want: "a\nB\nc"},
		{s: "a\n\tb\nc", old: "  b\n", new: "\tB", want: "a\n\tB\nc"},
		{s: "b\nb", old: "b", fails: "2 times"},
		{s: "a", old: "z", fails: "not found"},
	} {
		got, _, err := replaceOnce(c.s, c.old, c.new)
		switch {
		case c.fails != "" && (err == nil || !strings.Contains(err.Error(), c.fails)):
			t.Errorf("%q in %q: %v, want an error saying %q", c.old, c.s, err, c.fails)
		case c.fails == "" && (err != nil || got != c.want):
			t.Errorf("%q in %q gave %q, %v; want %q", c.old, c.s, got, err, c.want)
		}
	}
}
