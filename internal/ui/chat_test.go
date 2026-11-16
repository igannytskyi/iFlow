package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/igannytskyi/iflow/internal/chat"
	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/gate"
)

// Each task has its own conversation, kept in its folder and handed what
// the task wrote; the project has one of its own; and a new intent's box
// starts a task rather than asks.
func TestEachTaskHasItsOwnConversation(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "greeting")
	if err := exec.Command("cp", "-R", filepath.Join(gate.Home(), "examples", "greeting"), dir).Run(); err != nil {
		t.Fatal(err)
	}
	u := testUI(t, &flow.Config{Model: cannedModel{}}, dir)
	p := u.cur
	if !u.asking(p, "") {
		t.Error("the project's box does not ask")
	}
	p.selected = newKey
	if u.asking(p, "") {
		t.Error("a new intent's box asks rather than starts a task")
	}

	ask := func(c string) *chat.Thread {
		t.Helper()
		key, where := u.subject(p)
		th := u.thread(p, key, where)
		u.ask(p, key, where, "what is "+c+"?")
		waitFor(t, "the answer", func() bool { return !th.Waiting() })
		return th
	}
	old, newest := p.order[len(p.order)-1], p.order[0]
	p.show(old.Change.Dir, old)
	first := ask("old")
	p.show(newest.Change.Dir, newest)
	second := ask("newest")
	if first == second || len(first.Messages()) != 2 || len(second.Messages()) != 2 {
		t.Fatalf("the two tasks share a conversation: %d and %d turns", len(first.Messages()), len(second.Messages()))
	}
	if _, err := os.Stat(filepath.Join(old.Change.Dir, chat.File)); err != nil {
		t.Errorf("the conversation is not kept beside its task: %v", err)
	}
	if about := chat.AboutTask(old.Change.Dir); !strings.Contains(about, "record") {
		t.Errorf("the model is not handed the task's record:\n%s", about)
	}
	p.selected = ""
	project := ask("the project")
	if project == first || project == second {
		t.Error("the project's conversation is a task's")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "changes", chat.File)); !strings.Contains(string(b), "what is the project?") {
		t.Error("the project's conversation is not kept under changes")
	}
	p.show(newest.Change.Dir, newest)
	newPainter(t).paint(u, "chat")
}
