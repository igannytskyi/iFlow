package ui

import (
	"github.com/igannytskyi/iflow/internal/workspace"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/igannytskyi/iflow/internal/artifact"
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
	held(p).show(old.Change.Dir, old)
	first := ask("old")
	held(p).show(newest.Change.Dir, newest)
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
	held(p).show(newest.Change.Dir, newest)
	newPainter(t).paint(u, "chat")
}

// A task whose executor went on to a stronger model is marked with it; one
// carried by the first model throughout is not.
func TestTheStrongerModelIsNamed(t *testing.T) {
	cheap := []artifact.Cost{{Area: 1, Model: "a/x, b/y"}, {Area: 4, Model: "or/z-ai/glm-5.3-flash"}}
	if got := strongerModel(cheap); got != "" {
		t.Errorf("a task carried by the first model was marked %q", got)
	}
	strong := append(cheap, artifact.Cost{Area: 4, Model: "or/z-ai/glm-5.3-flash, or/z-ai/glm-5.3"})
	if got := strongerModel(strong); got != "glm-5.3" {
		t.Errorf("marked %q, want glm-5.3", got)
	}
}

// held is p, with its workspace held while what is done with it is done, as
// a frame holds it: what the workspace's check writes is not read halfway.
func held(p *project) heldProject { return heldProject{p} }

type heldProject struct{ p *project }

func (h heldProject) show(sel string, c *workspace.Change) {
	if h.p.ws != nil {
		h.p.ws.RLock()
		defer h.p.ws.RUnlock()
	}
	h.p.show(sel, c)
}
