package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/estate"
	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/models"
)

// FlowConfig is what a run is carried out with: the models chosen for each
// role (see models.Config.Flow), and from the environment
//
//	IFLOW_BUDGET_USD    the most one unit may spend (2 when unset)
//	IFLOW_PACK_TOKENS   the most one agent is handed (16000 when unset)
//
// The second value says what is wrong with the choice of models, if
// anything.
func (app *App) FlowConfig() (flow.Config, string) {
	cfg := flow.Config{Estate: app.Estate.(*estate.Local), Gate: app.Gate, Person: person(), BudgetUSD: 2}
	if v := os.Getenv("IFLOW_BUDGET_USD"); v != "" {
		fmt.Sscan(v, &cfg.BudgetUSD)
	}
	if v := os.Getenv("IFLOW_PACK_TOKENS"); v != "" {
		fmt.Sscan(v, &cfg.PackTokens)
	}
	m, err := models.Load()
	if err != nil {
		cfg, problems := models.Default().Flow(cfg)
		return cfg, strings.TrimSpace(err.Error() + "\n" + problems)
	}
	return m.Flow(cfg)
}

func person() string {
	if u, err := user.Current(); err == nil && u.Name != "" {
		return u.Name
	}
	return "the person at the workbench"
}

// Run carries a task through the path without a window, printing each step.
// With --accept the criteria are accepted as written, which the record says.
func (app *App) RunTask(ctx context.Context, args []string) int {
	accept, ask := false, false
	var rest []string
	for _, a := range args {
		switch a {
		case "--accept":
			accept = true
		case "--ask":
			ask = true
		default:
			rest = append(rest, a)
		}
	}
	if len(rest) < 2 {
		fmt.Fprintln(app.Out, "usage: iflow run [--accept | --ask] <estate> <task…>")
		return Misused
	}
	return app.carry(ctx, accept, ask, func(cfg flow.Config, changed func()) (*flow.Run, error) {
		return flow.New(cfg, rest[0], strings.Join(rest[1:], " "), changed), nil
	})
}

// carry runs what start makes, printing each step as it comes. With --accept
// the criteria are accepted as written, which the record says; with --ask
// the person answers on standard input, one line a question: "accept", or
// the notes the specification is sent back with.
func (app *App) carry(ctx context.Context, accept, ask bool, start func(flow.Config, func()) (*flow.Run, error)) int {
	answers := bufio.NewScanner(os.Stdin)
	asking := false
	cfg, problems := app.FlowConfig()
	if problems != "" {
		fmt.Fprintln(app.Err, problems)
	}
	if accept {
		cfg.Person += " (accepted with --accept, without reading)"
	}
	var mu sync.Mutex
	shown := 0
	var run *flow.Run
	print := func() {
		mu.Lock()
		defer mu.Unlock()
		v := run.View()
		for _, s := range v.Steps[shown:] {
			mark := " "
			if s.Bad {
				mark = "!"
			}
			fmt.Fprintf(app.Out, "%s %d  %s\n", mark, s.Area, s.Text)
		}
		shown = len(v.Steps)
		switch {
		case v.Question != "" && accept:
			go run.Accept()
		case v.Question != "" && ask && !asking:
			asking = true
			fmt.Fprintf(app.Out, "? %s\n? read %s and answer: accept, or the notes to send it back with\n", v.Question, filepath.Join(v.Dir, artifact.ChangeFile))
			go func() {
				line := "sent back: no answer was given"
				if answers.Scan() {
					line = strings.TrimSpace(answers.Text())
				}
				mu.Lock()
				asking = false
				mu.Unlock()
				if line == "accept" {
					run.Accept()
				} else {
					run.Refuse(line)
				}
			}()
		}
	}
	run, err := start(cfg, func() {
		if run != nil {
			print()
		}
	})
	if err != nil {
		return app.fatal(err)
	}
	err = run.Start(ctx)
	print()
	v := run.View()
	fmt.Fprintf(app.Out, "  spent $%.2f, %d tokens in (%d from cache), %d out\n", v.Spent.CostUSD, v.Spent.In(), v.Spent.CacheRead, v.Spent.Output)
	if err != nil {
		return Failed
	}
	fmt.Fprintf(app.Out, "  landed: %s\n", v.Dir)
	return OK
}

// Reopen carries a change again in its own folder from its specification (1)
// or its plan (2), with what the person says is to change:
// iflow reopen [--accept|--ask] <change folder> <1|2> <what is to change>.
func (app *App) Reopen(ctx context.Context, args []string) int {
	accept, ask := false, false
	var rest []string
	for _, a := range args {
		switch a {
		case "--accept":
			accept = true
		case "--ask":
			ask = true
		default:
			rest = append(rest, a)
		}
	}
	if len(rest) < 3 || (rest[1] != "1" && rest[1] != "2") {
		fmt.Fprintln(app.Out, "usage: iflow reopen [--accept | --ask] <change folder> <1 | 2> <what is to change…>")
		return Misused
	}
	dir := absolute(rest[0])
	repo := filepath.Dir(filepath.Dir(dir))
	from := 1
	if rest[1] == "2" {
		from = 2
	}
	return app.carry(ctx, accept, ask, func(cfg flow.Config, changed func()) (*flow.Run, error) {
		return flow.Reopen(cfg, repo, dir, from, strings.Join(rest[2:], " "), changed)
	})
}

// Resume carries a stopped change on from where it stopped:
// iflow resume [--accept|--ask] <change folder>.
func (app *App) Resume(ctx context.Context, args []string) int {
	accept, ask := false, false
	var rest []string
	for _, a := range args {
		switch a {
		case "--accept":
			accept = true
		case "--ask":
			ask = true
		default:
			rest = append(rest, a)
		}
	}
	if len(rest) != 1 {
		fmt.Fprintln(app.Out, "usage: iflow resume [--accept | --ask] <change folder>")
		return Misused
	}
	dir := absolute(rest[0])
	return app.carry(ctx, accept, ask, func(cfg flow.Config, changed func()) (*flow.Run, error) {
		return flow.Resume(cfg, filepath.Dir(filepath.Dir(dir)), dir, changed)
	})
}

// Stop asks the run carrying a change to stop, wherever it was started:
// iflow stop <change folder>.
func (app *App) Stop(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(app.Out, "usage: iflow stop <change folder>")
		return Misused
	}
	if err := flow.Stop(args[0], person()); err != nil {
		return app.fatal(err)
	}
	fmt.Fprintf(app.Out, "  asked %s to stop; it stops within a second, and iflow resume carries it on\n", args[0])
	return OK
}

// Close closes a change that will not be finished:
// iflow close <change folder> <why>.
func (app *App) Close(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(app.Out, "usage: iflow close <change folder> <why>")
		return Misused
	}
	if err := flow.Close(args[0], person(), strings.Join(args[1:], " ")); err != nil {
		return app.fatal(err)
	}
	fmt.Fprintf(app.Out, "  closed %s\n", args[0])
	return OK
}
