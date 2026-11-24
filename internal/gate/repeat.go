package gate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/testrun"
)

// Repetition is one claim to repeat, and what running it twice showed.
type Repetition struct {
	Evidence string
	// Command is nil when the claim names nothing that can be run.
	Command []string
	Same    bool
	// Passed says the command, as it ran here, exited without failing.
	Passed bool
	// Differs is the first line where the two runs said different things,
	// each as it said it, where they were not the same.
	Differs string
}

// Shown is the command as it was run.
func (r Repetition) Shown() string { return strings.Join(r.Command, " ") }

type outcome struct {
	code   int
	output string // what it said, with what is not a result masked
	raw    string // what it said
}

func keptOutput(kept func(string) (string, bool), id string) (string, bool) {
	if kept == nil {
		return "", false
	}
	return kept(id)
}

func runOnce(ctx context.Context, dir string, argv []string) outcome {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	code := 0
	if err := cmd.Run(); err != nil {
		if ee, ok := errors.AsType[*exec.ExitError](err); ok {
			code = ee.ExitCode()
		} else {
			code = -1
		}
	}
	return outcome{code, SameResult(out.String() + errb.String()), out.String() + errb.String()}
}

// Repeat runs every claim to repeat twice, compares what each run said, and
// notes the commands that agreed with themselves. A claim is a claim about a
// command, so it is the command that is noted, keyed by its content.
func (g *Gate) Repeat(ctx context.Context, c *artifact.Change) ([]Repetition, error) {
	return g.RepeatAgainst(ctx, c, nil)
}

// RepeatAgainst is Repeat with what a claim's first run said already kept:
// where kept gives the output its evidence kept, the command is run once
// more and set against it, rather than twice — the repeat is the run made
// for the evidence and the one made now.
func (g *Gate) RepeatAgainst(ctx context.Context, c *artifact.Change, kept func(evidence string) (string, bool)) ([]Repetition, error) {
	ids, _ := Producers(c)
	ids = slices.Sorted(slices.Values(ids))
	var out []Repetition
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		cmd := ProducerCommand(c, id)
		rep := Repetition{Evidence: id, Command: cmd}
		if cmd != nil {
			argv := cmd
			if filepath.Ext(cmd[0]) == ".py" {
				argv = append([]string{"python3"}, cmd...)
			}
			if out, ok := keptOutput(kept, id); ok {
				run := runOnce(ctx, c.Root, argv)
				rep.Passed = run.code == 0
				if was, err := testrun.ReadGo([]byte(out), "", ""); err == nil && len(was.Results) > 0 {
					// A claim kept as the runner's report repeats if every
					// test ends as it did: its result is its tests' outcomes.
					now, _ := testrun.ReadGo([]byte(run.raw), "", "")
					rep.Differs = differentOutcomes(was, now)
					rep.Same = rep.Differs == ""
				} else {
					rep.Same = SameResult(out) == SameResult(run.raw)
					if !rep.Same {
						rep.Differs = firstDifference(SameResult(out), SameResult(run.raw))
					}
				}
			} else {
				first, second := runOnce(ctx, c.Root, argv), runOnce(ctx, c.Root, argv)
				rep.Same = first.code == second.code && first.output == second.output
				rep.Passed = first.code == 0 && second.code == 0
				if !rep.Same {
					rep.Differs = firstDifference(first.output, second.output)
				}
			}
			if rep.Same {
				if err := NoteVerified(g.home, RepeatKey(cmd), rep.Shown()+" gave the same result twice"); err != nil {
					return out, err
				}
			}
		}
		out = append(out, rep)
	}
	return out, nil
}

// IsArbiterRun says whether cmd is the run of every arbiter — the
// framework's whole suite, as Arbiters runs it — so that a claim to repeat
// made of it, run here, is that run too.
func IsArbiterRun(cmd []string) bool {
	return len(cmd) == 4 && filepath.Base(cmd[0]) == "go" && cmd[1] == "test" && cmd[2] == "-count=1" && cmd[3] == "./..."
}

// firstDifference is the first line where a and b differ, each as it reads.
func firstDifference(a, b string) string {
	la, lb := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := range max(len(la), len(lb)) {
		var x, y string
		if i < len(la) {
			x = la[i]
		}
		if i < len(lb) {
			y = lb[i]
		}
		if x != y {
			return fmt.Sprintf("line %d: %q, then %q", i+1, x, y)
		}
	}
	return "the same lines, ended differently"
}

// differentOutcomes is the first test that ended otherwise in now than in
// was, or "" where every one ended as it did.
func differentOutcomes(was, now testrun.Report) string {
	word := func(o testrun.Outcome, ok bool) string {
		switch {
		case !ok:
			return "did not run"
		case o.Skipped:
			return "was skipped"
		case o.Passed:
			return "passed"
		}
		return "failed"
	}
	if (was.Unbuilt == "") != (now.Unbuilt == "") {
		return "the tests built once and not the other time"
	}
	names := map[string]bool{}
	for n := range was.Results {
		names[n] = true
	}
	for n := range now.Results {
		names[n] = true
	}
	for _, n := range slices.Sorted(maps.Keys(names)) {
		a, inA := was.Results[n]
		b, inB := now.Results[n]
		if word(a, inA) != word(b, inB) {
			return fmt.Sprintf("%s %s, then %s", n, word(a, inA), word(b, inB))
		}
	}
	return ""
}
