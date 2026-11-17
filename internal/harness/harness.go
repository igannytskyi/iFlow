// Package harness carries out one unit of work with an agent, in a
// workspace apart from the live tree, and reports every step it took.
//
// The harness is what stands between a model and the estate: it decides
// what the agent may touch, how much it may spend, when it has gone round in
// circles, and what it leaves behind. The agent proposes; nothing here
// applies anything to the live tree — a candidate is the files that changed
// in the workspace, and landing is someone else's work.
package harness

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/igannytskyi/iflow/internal/model"
)

// Task is one unit of work, as the executor receives it.
type Task struct {
	// Workspace is the directory the agent works in: a copy of the estate,
	// never the estate itself.
	Workspace string
	// Goal is what the unit must achieve, in the vocabulary of the intent.
	Goal string
	// Scope lists the paths the unit may change. Anything else changed is
	// reported, and refused at the boundary.
	Scope []string
	// Context is what the estate says about the scope: the code and what it
	// touches.
	Context string
	// Check is a command the agent may run to see where it stands — the
	// estate's own tests. It is not what judges the unit.
	Check []string
	// Judge, where given, decides whether the check's outcome is acceptable
	// in place of its exit status, and says why not: tests written before
	// their code fail to build, and only some of those failures are theirs.
	Judge func(passed bool, output string) (ok bool, why string)
	// BudgetUSD and MaxSteps bound the run; a run that reaches either stops
	// with what it has and says which it reached.
	BudgetUSD float64
	MaxSteps  int
	// MaxRead bounds the tokens the agent reads, all its turns together;
	// 250000 when zero. A unit that needs more is too large, or cannot be
	// done as it is asked.
	MaxRead int
}

// Kind says what a step was.
type Kind string

// The kinds of step a run reports.
const (
	Said    Kind = "said"    // the agent wrote text
	Called  Kind = "called"  // the agent called a tool
	Got     Kind = "got"     // a tool answered
	Refused Kind = "refused" // the harness refused a call
	Spent   Kind = "spent"   // a running account of cost
	Ended   Kind = "ended"   // the run ended
)

// Event is one step of a run. Usage is what the model turn that produced the
// step cost, carried on the first step of that turn, or on a Spent event
// where the turn produced no step; an Ended event carries the run's total.
type Event struct {
	At     time.Time   `json:"at"`
	Kind   Kind        `json:"kind"`
	Tool   string      `json:"tool,omitempty"`
	Detail string      `json:"detail,omitempty"`
	Usage  model.Usage `json:"usage,omitzero"`
}

// Terminal states, as execution records them.
const (
	Candidate       = "candidate"
	Terminated      = "terminated"
	Exhausted       = "exhausted"
	FailedSubstrate = "failed-substrate"
	FailedTask      = "failed-task"
	Cancelled       = "cancelled"
)

// Result is how a run ended, and what it took.
type Result struct {
	Terminal string
	Summary  string
	Steps    int
	Turns    int
	Usage    model.Usage
	Elapsed  time.Duration
}

// Executor carries out a task. Run returns when the task has ended, and
// calls emit for every step on the way, in order.
type Executor interface {
	Name() string
	Version() string
	Run(ctx context.Context, t Task, emit func(Event)) (Result, error)
}

// TraceFile is where a run's steps are kept beside its candidate.
const TraceFile = "trace.jsonl"

// WriteTrace keeps a run's steps, one JSON object a line.
func WriteTrace(path string, events []Event) error {
	var b []byte
	for _, e := range events {
		line, err := json.Marshal(e)
		if err != nil {
			return err
		}
		b = append(append(b, line...), '\n')
	}
	return os.WriteFile(path, b, 0o644)
}

// ReadTrace reads the steps a run kept.
func ReadTrace(path string) ([]Event, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []Event
	for line := range strings.SplitSeq(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		var e Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return out, err
		}
		out = append(out, e)
	}
	return out, nil
}
