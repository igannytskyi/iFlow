package gate

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/igannytskyi/iflow/internal/artifact"
)

// Repetition is one claim to repeat, and what running it twice showed.
type Repetition struct {
	Evidence string
	// Command is nil when the claim names nothing that can be run.
	Command []string
	Same    bool
}

// Shown is the command as it was run.
func (r Repetition) Shown() string { return strings.Join(r.Command, " ") }

type outcome struct {
	code   int
	output string
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
	return outcome{code, SameResult(out.String() + errb.String())}
}

// Repeat runs every claim to repeat twice, compares what each run said, and
// notes the commands that agreed with themselves. A claim is a claim about a
// command, so it is the command that is noted, keyed by its content.
func (g *Gate) Repeat(ctx context.Context, c *artifact.Change) ([]Repetition, error) {
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
			first, second := runOnce(ctx, c.Root, argv), runOnce(ctx, c.Root, argv)
			rep.Same = first == second
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
