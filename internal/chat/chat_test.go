package chat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/igannytskyi/iflow/internal/model"
)

type asking struct{ system, prompt *string }

func (a asking) Name() string { return "asking" }
func (a asking) Ask(_ context.Context, system, prompt string) (string, model.Usage, error) {
	*a.system, *a.prompt = system, prompt
	return "answer to: " + prompt[strings.LastIndex(prompt, "\n")+1:], model.Usage{CostUSD: 0.01}, nil
}

type chatting struct{ sent *[]model.Message }

func (c chatting) Name() string { return "chatting" }
func (c chatting) Ask(context.Context, string, string) (string, model.Usage, error) {
	panic("a model that chats is asked in turns")
}
func (c chatting) Chat(_ context.Context, msgs []model.Message, _ []model.Tool) (model.Message, model.Usage, error) {
	*c.sent = msgs
	return model.Message{Content: "turned"}, model.Usage{}, nil
}

func askAndWait(t *testing.T, th *Thread, m model.Model, text string) {
	t.Helper()
	done := make(chan struct{})
	if err := th.Ask(context.Background(), m, text, func() { close(done) }); err != nil {
		t.Fatal(err)
	}
	<-done
}

// A task's conversation is kept beside the task and goes on where it was
// left; the model is handed what the task wrote and the conversation so far.
func TestAThreadIsKeptAndGoesOn(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "change.md"), []byte("# Specification — x\n\nmake x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var sys, prompt string
	m := asking{&sys, &prompt}
	th := Open(filepath.Join(dir, File), func() string { return AboutTask(dir) })
	askAndWait(t, th, m, "first")
	if !strings.Contains(sys, "make x") {
		t.Errorf("the model was not handed the task:\n%s", sys)
	}
	again := Open(filepath.Join(dir, File), th.About)
	if n := len(again.Messages()); n != 2 {
		t.Fatalf("the thread kept %d messages, want 2", n)
	}
	askAndWait(t, again, m, "second")
	if !strings.Contains(prompt, "first") || !strings.Contains(prompt, "answer to: first") {
		t.Errorf("the conversation so far was not handed:\n%s", prompt)
	}
	if c := again.Cost(); c < 0.019 {
		t.Errorf("the thread cost %v", c)
	}
}

func TestAModelThatChatsIsAskedInTurns(t *testing.T) {
	var sent []model.Message
	th := Open("", nil)
	askAndWait(t, th, chatting{&sent}, "one")
	askAndWait(t, th, chatting{&sent}, "two")
	roles := ""
	for _, m := range sent {
		roles += m.Role[:1]
	}
	if roles != "suau" {
		t.Errorf("turns sent as %q, want system, user, assistant, user", roles)
	}
}
