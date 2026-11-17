// Package cli is the program's surface: it parses what was asked, asks the
// packages that know, and renders their answers. Nothing here decides
// anything about a change.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/estate"
	"github.com/igannytskyi/iflow/internal/gate"
	"github.com/igannytskyi/iflow/internal/ledger"
	"github.com/igannytskyi/iflow/internal/models"
)

const usage = `usage: iflow desk [project...]            the workbench: projects side by side, a task carried through each
       iflow run [--accept|--ask] <estate> <task> carry a task along the path; --ask reads
                                         each answer on stdin: accept, or notes to send it back
       iflow describe <change>...        write, after the fact, the whole specification of changes
                                         carried before specifications were whole
       iflow settle <change> <n> <decision>  close an escalation the change raised by your decision
       iflow migrate [--dry] [changes...]  write every change in the data model, as JSON;
                                         its Markdown is kept under legacy/
       iflow models [--try]              which model does each piece of work, and whether its key is found;
                                         --try asks each one a one-word question
       iflow plan <changes/slug> [--draft]  what the estate says about a specification
       iflow baseline <estate>... [--method] the present way of working, and the method beside it
       iflow adopt openspec|speckit <folder> read another method's change as intent and criteria
       iflow measure <estate>...         what the estate model is worth: reach and quotation
       iflow estate <question>           what the estate says; iflow estate for the questions
       iflow check <changes/slug>        one change
       iflow check --status [dir]         every change, at a glance
       iflow check --repeat <slug>        run what claims to repeat, twice
       iflow check --escalations          what the method owes itself
       iflow check --arbiters             run every arbiter of the framework
       iflow check --mutate               break each rule, see who notices
       iflow check --unused               fields nobody reads, codes nobody checks
Exit status is 0 when nothing failed.`

// Exit statuses.
const (
	OK      = 0
	Failed  = 1
	Misused = 2
)

// App is one invocation's dependencies.
type App struct {
	Out    io.Writer
	Err    io.Writer
	Estate estate.Estate
	Gate   *gate.Gate
}

// New returns an app writing to out and err, asking the estate on disk.
func New(out, errw io.Writer) (*App, func()) {
	est := estate.NewLocal()
	return &App{Out: out, Err: errw, Estate: est, Gate: gate.New(est)}, func() {}
}

// Run carries out one invocation and returns its exit status.
func (app *App) Run(ctx context.Context, args []string) int {
	switch {
	case len(args) > 0 && args[0] == "check":
		return app.check(ctx, args[1:])
	case len(args) > 0 && args[0] == "desk":
		return app.Desk(ctx, args[1:])
	case len(args) > 0 && args[0] == "describe":
		return app.Describe(ctx, args[1:])
	case len(args) > 0 && args[0] == "settle":
		return app.Settle(args[1:])
	case len(args) > 0 && args[0] == "migrate":
		return app.Migrate(args[1:])
	case len(args) > 0 && args[0] == "models":
		m, err := models.Load()
		if err != nil {
			return app.fatal(err)
		}
		m.Describe(app.Out)
		if len(args) > 1 && args[1] == "--try" {
			return m.Try(ctx, app.Out)
		}
		return OK
	case len(args) > 0 && args[0] == "run":
		return app.RunTask(ctx, args[1:])
	case len(args) > 0 && args[0] == "adopt":
		return app.Adopt(ctx, args[1:])
	case len(args) > 0 && args[0] == "baseline":
		return app.Baseline(ctx, args[1:])
	case len(args) > 0 && args[0] == "measure":
		return app.Measure(ctx, args[1:])
	case len(args) > 0 && args[0] == "plan":
		return app.Plan(ctx, args[1:])
	case len(args) > 0 && args[0] == "estate":
		return app.EstateCommand(ctx, args[1:])
	}
	fmt.Fprintln(app.Out, usage)
	return Misused
}

func (app *App) check(ctx context.Context, args []string) int {
	switch {
	case len(args) == 1 && args[0] == "--escalations":
		wd, err := os.Getwd()
		if err != nil {
			return app.fatal(err)
		}
		return app.Escalations(wd)
	case len(args) == 2 && args[0] == "--repeat":
		return app.Repeat(ctx, args[1])
	case len(args) >= 1 && len(args) <= 2 && args[0] == "--status":
		where := "changes"
		if len(args) == 2 {
			where = args[1]
		}
		return app.Status(ctx, where)
	case len(args) == 1 && args[0] == "--arbiters":
		return app.Arbiters(ctx)
	case len(args) == 1 && args[0] == "--mutate":
		return app.Mutate(ctx)
	case len(args) == 1 && args[0] == "--unused":
		return app.Unused()
	case len(args) == 1 && !strings.HasPrefix(args[0], "-"):
		return app.Check(ctx, args[0])
	}
	fmt.Fprintln(app.Out, usage)
	return Misused
}

func (app *App) fatal(err error) int {
	fmt.Fprintln(app.Err, "iflow:", err)
	return Misused
}

func (app *App) tally(problems int) int {
	if problems == 0 {
		fmt.Fprintln(app.Out, "  passed")
		return OK
	}
	fmt.Fprintf(app.Out, "  %d problem(s)\n", problems)
	return Failed
}

// Check checks one change folder.
func (app *App) Check(ctx context.Context, dir string) int {
	c, err := artifact.Load(dir)
	if err != nil {
		fmt.Fprintf(app.Out, "not a directory: %s\n", dir)
		return Misused
	}
	rep := app.Gate.Check(ctx, c)
	var stages []string
	for _, s := range c.Present() {
		stages = append(stages, string(s))
	}
	shown := strings.Join(stages, ", ")
	if shown == "" {
		shown = "none"
	}
	fmt.Fprintf(app.Out, "%s  stages: %s\n", c.Dir, shown)
	for _, n := range rep.Notes() {
		fmt.Fprintf(app.Out, "  note    %s\n", n)
	}
	for _, p := range rep.Problems() {
		fmt.Fprintf(app.Out, "  FAIL    %s\n", p)
	}
	return app.tally(len(rep.Problems()))
}

// Status writes every change under where at a glance.
func (app *App) Status(ctx context.Context, where string) int {
	rows, err := app.Gate.Status(ctx, where)
	if err != nil {
		return app.fatal(err)
	}
	if len(rows) == 0 {
		fmt.Fprintf(app.Out, "  no changes under %s\n", where)
		return OK
	}
	const line = "  %-28s %-16s %5s %5v %5v %6v %5v\n"
	fmt.Fprintf(app.Out, line, "change", "reached", "gate", "held", "wait", "undec", "owed")
	for _, r := range rows {
		gateCol := "ok"
		if r.Problems > 0 {
			gateCol = fmt.Sprint(r.Problems)
		}
		fmt.Fprintf(app.Out, line, r.Name, r.Reached, gateCol, r.Held, r.Waiting, r.Undecided, r.Owed)
	}
	fmt.Fprintf(app.Out, "  %d change(s)\n", len(rows))
	return OK
}

// Repeat runs what a change claims to repeat, twice.
func (app *App) Repeat(ctx context.Context, dir string) int {
	c, err := artifact.Load(dir)
	if err != nil {
		return app.fatal(err)
	}
	reps, err := app.Gate.Repeat(ctx, c)
	if err != nil && !errors.Is(err, context.Canceled) {
		return app.fatal(err)
	}
	if len(reps) == 0 {
		fmt.Fprintln(app.Out, "  nothing here claims to repeat")
		return OK
	}
	bad := 0
	for _, r := range reps {
		switch {
		case r.Command == nil:
			fmt.Fprintf(app.Out, "  FAIL    R13: %s claims to repeat and names nothing to re-run\n", r.Evidence)
			bad++
		case r.Same:
			fmt.Fprintf(app.Out, "  ok      %s  %s gave the same result twice\n", r.Evidence, r.Shown())
		default:
			fmt.Fprintf(app.Out, "  FAIL    R13: %s claims to repeat and %s gave two different results\n", r.Evidence, r.Shown())
			bad++
		}
	}
	return app.tally(bad)
}

// Escalations writes what the method still owes itself.
func (app *App) Escalations(root string) int {
	l, err := ledger.Read(root)
	if err != nil {
		return app.fatal(err)
	}
	for _, p := range l.Problems {
		fmt.Fprintf(app.Out, "  FAIL    %s\n", p)
	}
	for _, ref := range sortedKeys(l.Settled) {
		fmt.Fprintf(app.Out, "  settled %s  by %s\n", ref, l.Settled[ref])
	}
	fmt.Fprintf(app.Out, "  still owed: %d\n", len(l.Owed))
	for _, o := range l.Owed {
		fmt.Fprintf(app.Out, "    %s  %s\n", o.Ref, firstRunes(o.Ground, 100))
	}
	if shared := l.Shared(); len(shared) > 0 {
		fmt.Fprintln(app.Out, "  named by more than one, which is how one defect looks like several:")
		for _, s := range shared {
			fmt.Fprintf(app.Out, "    %s  %s\n", s.Rule, strings.Join(s.Refs, ", "))
		}
	}
	if len(l.Problems) > 0 {
		return Failed
	}
	return OK
}

// Arbiters runs every arbiter of the framework and notes it when they pass.
func (app *App) Arbiters(ctx context.Context) int {
	run, err := gate.Arbiters(ctx, gate.Home())
	if err != nil {
		return app.fatal(err)
	}
	for line := range strings.SplitSeq(strings.TrimSpace(run.Output), "\n") {
		if strings.HasPrefix(line, "ok") || strings.HasPrefix(line, "FAIL") || strings.HasPrefix(line, "---") {
			fmt.Fprintf(app.Out, "  %s\n", line)
		}
	}
	if !run.Passed {
		fmt.Fprintln(app.Out, "  an arbiter failed; nothing is noted, and R22 holds every landing until they pass")
		return Failed
	}
	fmt.Fprintln(app.Out, "  passed — noted against this framework; any edit to it voids the note")
	return OK
}

// Mutate breaks each rule in turn and runs the gate's arbiters against it.
func (app *App) Mutate(ctx context.Context) int {
	ms, err := gate.Mutate(ctx, gate.Home())
	if err != nil {
		return app.fatal(err)
	}
	missed := 0
	for _, m := range ms {
		if m.Noticed {
			fmt.Fprintf(app.Out, "  ok      %s\n", m.Rule)
		} else {
			fmt.Fprintf(app.Out, "  FAIL    R12: nothing notices the loss of %s, so nothing is testing it\n", m.Rule)
			missed++
		}
	}
	fmt.Fprintf(app.Out, "  %d of %d rules are actually tested\n", len(ms)-missed, len(ms))
	if missed > 0 {
		return Failed
	}
	return OK
}

// Unused lists the columns the gate reads nowhere and the codes it checks
// nowhere.
func (app *App) Unused() int {
	cols, codes, err := gate.Unused(gate.Home())
	if err != nil {
		return app.fatal(err)
	}
	for _, c := range sortedKeys(cols) {
		fmt.Fprintf(app.Out, "  column   '%s' — filled in %s, read by nothing\n", c, strings.Join(cols[c], ", "))
	}
	for _, c := range codes {
		fmt.Fprintf(app.Out, "  code     '%s' — defined in conventions, validated by nothing\n", c)
	}
	if n := len(cols) + len(codes); n > 0 {
		fmt.Fprintf(app.Out, "  %d field(s) nobody reads\n", n)
	} else {
		fmt.Fprintln(app.Out, "  nothing is filled in for its own sake")
	}
	return OK
}
