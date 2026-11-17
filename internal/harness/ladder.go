package harness

import (
	"context"
	"strings"
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
	why, said := strings.CutPrefix(first.Summary, BlockedPrefix)
	switch {
	case first.Terminal == Candidate, first.Terminal == Cancelled:
		return first, err // done, or stopped
	case first.Terminal == FailedTask && !said:
		// Failing where no agent of this unit may mend: a stronger agent
		// cannot help.
		return first, err
	}
	if ctx.Err() != nil {
		return first, err
	}
	next := t
	if said {
		// The first rung sees only what it was handed: its "cannot" may be
		// only what it was not shown. The agent can read the estate, and
		// decides.
		next.Goal = t.Goal + "\n\n# What the first attempt said\n\nAn agent before you, without tools, said this unit cannot be done as asked: " + why +
			"\nIt saw only what it was handed. Read the estate and see whether that is so: if what it missed is there, do the unit; if it truly cannot be done, call cannot with why."
	} else {
		next.Goal = t.Goal + "\n\n# Where the first attempt left it\n\nAn agent before you tried this unit without tools; whatever it wrote is in the workspace as it left it. It ended: " + first.Summary + "\nStart by running the check, then read what fails."
	}
	if t.BudgetUSD > 0 {
		next.BudgetUSD = max(t.BudgetUSD-first.Usage.CostUSD, t.BudgetUSD/2)
	}
	emit(Event{At: time.Now(), Kind: Said, Detail: "handed on to " + l.Then.Name() + ": " + first.Summary})
	then, err := l.Then.Run(ctx, next, emit)
	if said && err == nil && then.Terminal != Candidate && then.Terminal != Cancelled && then.Terminal != FailedTask {
		// The agent neither did the unit nor showed the first rung wrong:
		// what the first rung said stands, with what both spent.
		then.Terminal, then.Summary = first.Terminal, first.Summary
	}
	then.Usage = first.Usage.Add(then.Usage)
	then.Steps += first.Steps
	then.Turns += first.Turns
	then.Elapsed += first.Elapsed
	return then, err
}
