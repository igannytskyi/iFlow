package harness

import (
	"context"
	"encoding/json"
	"fmt"
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

// Reading a file again after reading others is work; the same call three
// times in a row is a loop.
func TestALoopIsTheSameCallInARow(t *testing.T) {
	ws := t.TempDir()
	for _, f := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(ws, f), []byte(f), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	read := func(f string) model.Message { return calling("read_file", map[string]string{"path": f}) }
	var sent [][]model.Message
	m := chatting{sent: &sent, turns: []model.Message{read("a.txt"), read("b.txt"), read("a.txt"), read("b.txt"), read("a.txt"), {Content: "read"}}}
	res, _ := Loop{Model: m}.Run(context.Background(), Task{Workspace: ws, Goal: "read", Scope: []string{"a.txt"}}, func(Event) {})
	if res.Terminal != Candidate {
		t.Errorf("reading files in turn was taken for a loop: %+v", res)
	}
	sent = nil
	m = chatting{sent: &sent, turns: []model.Message{read("a.txt"), read("a.txt"), read("a.txt"), {Content: "read"}}}
	res, _ = Loop{Model: m}.Run(context.Background(), Task{Workspace: ws, Goal: "read", Scope: []string{"a.txt"}}, func(Event) {})
	if res.Terminal != Terminated {
		t.Errorf("the same call three times in a row was not stopped: %+v", res)
	}
}

// An agent that only reads is told it has read enough, then stopped.
func TestAnAgentThatOnlyReadsIsStopped(t *testing.T) {
	ws := t.TempDir()
	for _, f := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(ws, f), []byte(f), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var turns []model.Message
	for i := range idleStop + 5 {
		turns = append(turns, calling("read_file", map[string]string{"path": []string{"a.txt", "b.txt"}[i%2]}))
	}
	var sent [][]model.Message
	res, _ := Loop{Model: chatting{sent: &sent, turns: turns}}.Run(context.Background(), Task{Workspace: ws, Goal: "g", Scope: []string{"a.txt"}}, func(Event) {})
	if res.Terminal != Exhausted || !strings.Contains(res.Summary, "without changing anything") {
		t.Fatalf("ended %+v", res)
	}
	told := false
	for _, m := range sent[len(sent)-1] {
		if m.Role == "user" && strings.Contains(m.Content, "read enough") {
			told = true
		}
	}
	if !told {
		t.Error("the agent was never told it had read enough")
	}
}

// A long file answers with its outline; a range answers its lines; the
// same lines read again a step later, unchanged, are not handed over again.
func TestReadingIsSparing(t *testing.T) {
	var b strings.Builder
	b.WriteString("package a\n\n")
	for i := range 200 {
		fmt.Fprintf(&b, "func F%d() {}\n", i)
	}
	w := &workspace{dir: t.TempDir()}
	outline := w.read("a.go", b.String(), 0, 0)
	if !strings.Contains(outline, "outline") || !strings.Contains(outline, "func F150()") || strings.Count(outline, "\n") > 210 {
		t.Errorf("no outline:\n%.300s", outline)
	}
	w.calls = 1
	part := w.read("a.go", b.String(), 10, 12)
	if !strings.Contains(part, "   10  func F7()") || strings.Count(part, "\n") > 4 {
		t.Errorf("the range read %q", part)
	}
	w.calls = 2
	if again := w.read("a.go", b.String(), 10, 12); !strings.Contains(again, "unchanged") {
		t.Errorf("read again: %q", again)
	}
	w.calls = 20
	if later := w.read("a.go", b.String(), 10, 12); strings.Contains(later, "unchanged") {
		t.Error("lines read long ago, cut from the conversation since, were withheld")
	}
}

// An agent stops when it has read the most a unit may.
func TestAnAgentReadsNoMoreThanItMay(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	var turns []model.Message
	for range 10 {
		turns = append(turns, calling("list_dir", map[string]string{"path": "."}), calling("read_file", map[string]string{"path": "a.txt"}))
	}
	var sent [][]model.Message
	res, _ := Loop{Model: chatting{sent: &sent, turns: turns}}.Run(context.Background(), Task{Workspace: ws, Goal: "g", Scope: []string{"a.txt"}, MaxRead: 350}, func(Event) {})
	if res.Terminal != Exhausted || !strings.Contains(res.Summary, "tokens") || res.Turns > 4 {
		t.Errorf("ended %+v", res)
	}
}

// What the estate says is sent whole for the agent's first turns, then as
// an index of what it held: no turn after carries every file again.
func TestTheEstateIsReadOnceThenIndexed(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	big := "# May change: a.go\n\n" + strings.Repeat("// a line of the file\n", 200) + "# Used from elsewhere\n\nA()\n"
	var sent [][]model.Message
	m := chatting{sent: &sent, turns: []model.Message{
		calling("read_file", map[string]string{"path": "a.go"}),
		calling("list_dir", map[string]string{"path": "."}),
		calling("search", map[string]string{"pattern": "package"}),
		{Content: "nothing to change"},
	}}
	if _, err := (Loop{Model: m}).Run(context.Background(), Task{Workspace: ws, Goal: "the unit", Scope: []string{"a.go"}, Context: big}, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sent[0][1].Content, "a line of the file") || !strings.Contains(sent[1][1].Content, "a line of the file") {
		t.Error("the first turns were not sent the estate whole")
	}
	later := sent[2][1].Content
	if strings.Contains(later, "a line of the file") {
		t.Error("a later turn was sent the estate whole again")
	}
	for _, want := range []string{"# May change: a.go", "read_file", "# Used from elsewhere\n\nA()", "the unit"} {
		if !strings.Contains(later, want) {
			t.Errorf("the index lost %q:\n%s", want, later)
		}
	}
}

// The first rung sees only what it was handed, so its "cannot" is put to
// the agent, which reads the estate and decides.
func TestTheLadderPutsTheFirstRungsCannotToTheAgent(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "a.txt"), []byte("half"), 0o644); err != nil {
		t.Fatal(err)
	}
	var asked []string
	first := Oneshot{Attempts: 1, Model: scripted{asked: &asked, answers: []string{"=== CANNOT: the helper it relies on was not shown"}}}
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
	if res.Terminal != Candidate {
		t.Fatalf("the first rung's cannot ended the unit: %+v", res)
	}
	if !strings.Contains(sent[0][1].Content, "the helper it relies on was not shown") {
		t.Error("the agent was not told what the first rung said")
	}
}
