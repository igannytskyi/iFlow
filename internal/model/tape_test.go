package model

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
)

// counting answers each question with how many it has been asked.
type counting struct{ n *int }

func (c counting) Name() string { return "counting" }
func (c counting) Ask(context.Context, string, string) (string, Usage, error) {
	*c.n++
	return fmt.Sprint(*c.n), Usage{Output: 1, CostUSD: 0.01}, nil
}
func (c counting) Chat(_ context.Context, msgs []Message, _ []Tool) (Message, Usage, error) {
	*c.n++
	return Message{Role: "assistant", Content: fmt.Sprint(*c.n), ToolCalls: []ToolCall{{ID: "1"}}}, Usage{}, nil
}

// A tape replays every answer to who asked it, the how-manieth time they
// asked, whatever order the askers come in; a question it has no answer
// for is said to be off the tape.
func TestATapeReplaysWhatWasRecorded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.jsonl")
	rec, err := Record(path)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	m := rec.Taped(counting{&n}, "counting")
	chat := rec.TapedChat(counting{&n})
	author, unit := Within(context.Background(), "author"), Within(context.Background(), "unit WU-1")
	a1, _, _ := m.Ask(author, "s", "p")
	u1, _, _ := chat.Chat(unit, nil, nil)
	a2, _, _ := m.Ask(author, "s", "p")
	if err := rec.Close(); err != nil {
		t.Fatal(err)
	}

	play, err := Replay(path)
	if err != nil {
		t.Fatal(err)
	}
	m, chat = play.Taped(nil, "counting"), play.TapedChat(counting{&n})
	before := n
	// Asked in another order: each answer still goes to who asked it.
	if got, _, _ := chat.Chat(unit, nil, nil); got.Content != u1.Content || len(got.ToolCalls) != 1 {
		t.Errorf("the unit was answered %+v, not %+v", got, u1)
	}
	if got, u, _ := m.Ask(author, "other", "prompt"); got != a1 || u.CostUSD != 0.01 {
		t.Errorf("the author's first question was answered %q, not %q", got, a1)
	}
	if got, _, _ := m.Ask(author, "s", "p"); got != a2 {
		t.Errorf("the author's second question was answered %q, not %q", got, a2)
	}
	if n != before {
		t.Error("a model was asked during a replay")
	}
	if _, _, err := m.Ask(author, "s", "p"); !errors.Is(err, ErrOffTape) {
		t.Errorf("a question the tape has no answer for was answered: %v", err)
	}
}
