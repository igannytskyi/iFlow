package harness

import (
	"context"
	"time"
)

// Ladder carries out a unit with the cheapest executor that can: First —
// one question, nothing more — and, where it could not reach the check,
// Then, an agent that works in the workspace as First left it until the
// check passes. Most units need only the first rung; the ones that need
// reading, running and trying again are the ones that get it.
type Ladder struct {
	First, Then Executor
}

// Name implements Executor.
func (l Ladder) Name() string { return l.First.Name() + ", then " + l.Then.Name() }

// Version implements Executor.
func (l Ladder) Version() string { return "ladder/1" }

// Run implements Executor.
func (l Ladder) Run(ctx context.Context, t Task, emit func(Event)) (Result, error) {
	first, err := l.First.Run(ctx, t, emit)
	switch first.Terminal {
	case Candidate, Cancelled, FailedTask:
		// Done; stopped; failing where no agent of this unit may mend, or
		// said to be impossible as asked — a stronger agent cannot help.
		return first, err
	}
	if ctx.Err() != nil {
		return first, err
	}
	next := t
	next.Goal = t.Goal + "\n\n# Where the first attempt left it\n\nAn agent before you tried this unit without tools; whatever it wrote is in the workspace as it left it. It ended: " + first.Summary + "\nStart by running the check, then read what fails."
	if t.BudgetUSD > 0 {
		next.BudgetUSD = max(t.BudgetUSD-first.Usage.CostUSD, t.BudgetUSD/2)
	}
	emit(Event{At: time.Now(), Kind: Said, Detail: "handed on to " + l.Then.Name() + ": " + first.Summary})
	then, err := l.Then.Run(ctx, next, emit)
	then.Usage = first.Usage.Add(then.Usage)
	then.Steps += first.Steps
	then.Turns += first.Turns
	then.Elapsed += first.Elapsed
	return then, err
}
