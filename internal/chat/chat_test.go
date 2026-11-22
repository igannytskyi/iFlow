package chat

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
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
	changed := make(chan struct{}, 64)
	if err := th.Ask(context.Background(), m, text, func() { changed <- struct{}{} }); err != nil {
		t.Fatal(err)
	}
	for th.Waiting() {
		<-changed
	}
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

// reading calls read_file once, then answers with what it read.
type reading struct{ turns *int }

func (r reading) Name() string { return "reading" }
func (r reading) Ask(context.Context, string, string) (string, model.Usage, error) {
	panic("a model that chats is asked in turns")
}
func (r reading) Chat(_ context.Context, msgs []model.Message, tools []model.Tool) (model.Message, model.Usage, error) {
	*r.turns++
	if last := msgs[len(msgs)-1]; last.Role == "tool" {
		return model.Message{Content: "it says: " + last.Content}, model.Usage{CostUSD: 0.01}, nil
	}
	for _, t := range tools {
		if t.Function.Name == "write_file" {
			panic("a conversation was offered a tool that writes")
		}
	}
	var call model.ToolCall
	call.ID, call.Function.Name, call.Function.Arguments = "1", "read_file", `{"path":"notes.txt"}`
	return model.Message{ToolCalls: []model.ToolCall{call}}, model.Usage{CostUSD: 0.01}, nil
}

// The model reads the repository before it answers, and may only read.
func TestTheModelReadsBeforeItAnswers(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("the secret is 42"), 0o644); err != nil {
		t.Fatal(err)
	}
	turns := 0
	th := Open("", nil)
	th.Root = root
	askAndWait(t, th, reading{&turns}, "what is the secret?")
	got := th.Messages()[1]
	if !strings.Contains(got.Text, "the secret is 42") || turns != 2 || got.Usage.CostUSD < 0.019 {
		t.Errorf("answered %+v after %d turns", got, turns)
	}
}

// streaming hands its answer over in two pieces, and holds the second until
// the test has seen the first.
type streaming struct{ hold chan struct{} }

func (s streaming) Name() string { return "streaming" }
func (s streaming) Ask(context.Context, string, string) (string, model.Usage, error) {
	panic("a model that streams is asked in turns")
}
func (s streaming) Chat(context.Context, []model.Message, []model.Tool) (model.Message, model.Usage, error) {
	panic("a model that streams is streamed")
}
func (s streaming) Stream(_ context.Context, _ []model.Message, _ []model.Tool, said func(string)) (model.Message, model.Usage, error) {
	said("Hel")
	<-s.hold
	said("lo")
	return model.Message{Content: "Hello"}, model.Usage{}, nil
}

// An answer is shown as it is written, and kept whole once written.
func TestAnAnswerIsShownAsItIsWritten(t *testing.T) {
	th := Open("", nil)
	m := streaming{hold: make(chan struct{})}
	if err := th.Ask(context.Background(), m, "hi", nil); err != nil {
		t.Fatal(err)
	}
	for th.Partial() != "Hel" {
		runtime.Gosched()
	}
	close(m.hold)
	for th.Waiting() {
		runtime.Gosched()
	}
	if got := th.Messages(); len(got) != 2 || got[1].Text != "Hello" || th.Partial() != "" {
		t.Errorf("kept %+v, partial %q", got, th.Partial())
	}
}

// searching is a model whose server searches the web: it says what it was
// offered, and cites a page.
type searching struct{ offered *[]model.Tool }

func (s searching) Name() string   { return "searching" }
func (s searching) Searches() bool { return true }
func (s searching) Ask(context.Context, string, string) (string, model.Usage, error) {
	panic("asked in turns")
}
func (s searching) Chat(_ context.Context, _ []model.Message, tools []model.Tool) (model.Message, model.Usage, error) {
	*s.offered = tools
	return model.Message{Content: "Go 1.27 is the latest [1].", Annotations: []model.Annotation{
		{Type: "url_citation", URLCitation: &model.Citation{URL: "https://go.dev/doc/go1.27", Title: "Go 1.27"}}}}, model.Usage{}, nil
}

// A model whose server searches the web is offered the search, and the
// pages its answer cites are kept with the answer.
func TestAnAnswerKeepsThePagesItCites(t *testing.T) {
	var offered []model.Tool
	th := Open(filepath.Join(t.TempDir(), File), nil)
	askAndWait(t, th, searching{&offered}, "what is the latest Go?")
	if len(offered) != 1 || offered[0].Type != "openrouter:web_search" {
		t.Errorf("the search was not offered: %+v", offered)
	}
	msgs := th.Messages()
	if a := msgs[len(msgs)-1]; len(a.Sources) != 1 || a.Sources[0].URL != "https://go.dev/doc/go1.27" {
		t.Errorf("the page cited was not kept: %+v", a)
	}
	again := Open(th.Path, nil)
	if a := again.Messages()[1]; len(a.Sources) != 1 {
		t.Error("the pages cited were not kept on disk")
	}
	if searches(chatting{new([]model.Message)}) {
		t.Error("a model that cannot search would be offered the search")
	}
}
