package harness

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/igannytskyi/iflow/internal/model"
)

// A study is an agent that decides, not one that changes: the author of a
// specification, the planner. It is handed what the framework knows — the
// estate's graph and map — and reads on with the tools that only read, as
// far as its decision needs, before it answers. What it decides is decided
// on the code, not on a guess at it.

// Study is one decision taken with the estate at hand.
type Study struct {
	Dir    string // the estate, read and never written
	Graph  Graph  // its model of its code
	System string // what the decision is and how it is answered
	Prompt string // what the framework hands it
	// BudgetUSD bounds the reading: spent, the agent is asked to answer
	// with what it has. Where there is none, MaxSteps does — 25 when zero.
	MaxSteps  int
	BudgetUSD float64
}

// Study has m read what it needs and answer: the answer, what it cost, and
// how many tools it called.
func (s Study) Run(ctx context.Context, m Chatter, emit func(Event)) (string, model.Usage, int, error) {
	reader := NewReader(ctx, s.Dir).WithGraph(s.Graph)
	system := s.System + "\n\nBefore you answer, study the code your decision is about: ask find_definition and find_callers — the estate's graph — where things are and what calls what, then read_file what you must see whole. Read what the decision needs and no more; when you know, answer without calling a tool, in the form asked, and nothing else."
	msgs := []model.Message{{Role: "system", Content: system}, {Role: "user", Content: s.Prompt}}
	tools := reader.Tools()
	steps := s.MaxSteps
	if steps <= 0 {
		steps = 25
	}
	var used model.Usage
	calls := 0
	for turn := 0; ; turn++ {
		offered := tools
		if (s.BudgetUSD > 0 && used.CostUSD >= s.BudgetUSD) || (s.BudgetUSD <= 0 && calls >= steps) {
			// Enough has been read: the answer is asked for, with no tool.
			offered = nil
			msgs = append(msgs, model.Message{Role: "user", Content: "You have read enough. Answer now, in the form asked, with what you know."})
		}
		msg, u, err := m.Chat(ctx, compact(msgs), offered)
		used = used.Add(u)
		emit(Event{At: time.Now(), Kind: Spent, Usage: u})
		if err != nil {
			return "", used, calls, err
		}
		msg.Role = "assistant"
		msgs = append(msgs, msg)
		if len(msg.ToolCalls) == 0 {
			return strings.TrimSpace(msg.Content), used, calls, nil
		}
		if offered == nil {
			return "", used, calls, fmt.Errorf("it called tools after it was asked to answer")
		}
		for _, call := range msg.ToolCalls {
			calls++
			emit(Event{At: time.Now(), Kind: Called, Tool: call.Function.Name, Detail: clip(call.Function.Arguments, 240)})
			answer := reader.Call(call)
			emit(Event{At: time.Now(), Kind: Got, Tool: call.Function.Name, Detail: clip(answer, 2000)})
			msgs = append(msgs, model.Message{Role: "tool", ToolCallID: call.ID, Name: call.Function.Name, Content: answer})
		}
	}
}
