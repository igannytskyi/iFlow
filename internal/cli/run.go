package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/estate"
	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/models"
	"github.com/igannytskyi/iflow/internal/protocol"
	"github.com/igannytskyi/iflow/internal/service"
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

// Every run is carried by the service, whichever screen starts it: a run
// started here is seen in iflow tui and the window as it goes, goes on if
// this command is interrupted, and is carried on by the next service if
// the service is replaced. This command is one more screen: it prints the
// run's steps and answers its questions as it is told to.

// flags reads --accept and --ask off args.
func flags(args []string) (accept, ask bool, rest []string) {
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
	return accept, ask, rest
}

// RunTask carries a task through the path, printing each step:
// iflow run [--accept|--ask] <estate> <task>.
func (app *App) RunTask(ctx context.Context, args []string) int {
	accept, ask, rest := flags(args)
	if len(rest) < 2 {
		fmt.Fprintln(app.Out, "usage: iflow run [--accept | --ask] <estate> <task…>")
		return Misused
	}
	project, statement := absolute(rest[0]), strings.Join(rest[1:], " ")
	return app.carry(ctx, accept, ask, func(c *service.Client) (string, error) {
		return c.Start(ctx, protocol.StartParams{Project: project, Statement: statement})
	})
}

// Reopen carries a change again in its own folder from its specification (1)
// or its plan (2), with what the person says is to change:
// iflow reopen [--accept|--ask] <change folder> <1|2> <what is to change>.
func (app *App) Reopen(ctx context.Context, args []string) int {
	accept, ask, rest := flags(args)
	if len(rest) < 3 || (rest[1] != "1" && rest[1] != "2") {
		fmt.Fprintln(app.Out, "usage: iflow reopen [--accept | --ask] <change folder> <1 | 2> <what is to change…>")
		return Misused
	}
	dir, from, notes := absolute(rest[0]), 1, strings.Join(rest[2:], " ")
	if rest[1] == "2" {
		from = 2
	}
	return app.carry(ctx, accept, ask, func(c *service.Client) (string, error) {
		return c.Reopen(ctx, protocol.ReopenParams{Dir: dir, From: from, Notes: notes})
	})
}

// Resume carries a stopped change on from where it stopped:
// iflow resume [--accept|--ask] <change folder>.
func (app *App) Resume(ctx context.Context, args []string) int {
	accept, ask, rest := flags(args)
	if len(rest) != 1 {
		fmt.Fprintln(app.Out, "usage: iflow resume [--accept | --ask] <change folder>")
		return Misused
	}
	dir := absolute(rest[0])
	return app.carry(ctx, accept, ask, func(c *service.Client) (string, error) {
		return c.Resume(ctx, protocol.ResumeParams{Dir: dir})
	})
}

// carry has the service carry the run begin starts, and prints its steps as
// they come. With --accept every question is answered yes, and it says so;
// with --ask the person answers on standard input, one line a question:
// "accept", or the notes it is sent back with; with neither, the question
// waits for a screen to answer it. Interrupted, it stops watching; the run
// goes on.
func (app *App) carry(ctx context.Context, accept, ask bool, begin func(*service.Client) (string, error)) int {
	moved := make(chan struct{}, 1)
	c, err := service.Connect(ctx, protocol.Socket(), "iflow run", func(string) {
		select {
		case moved <- struct{}{}:
		default:
		}
	})
	if err != nil {
		return app.fatal(err)
	}
	defer c.Close()
	id, err := begin(c)
	if err != nil {
		return app.fatal(err)
	}
	if err := c.Watch(ctx, id); err != nil {
		return app.fatal(err)
	}
	answers := bufio.NewScanner(os.Stdin)
	shown, asked := 0, ""
	for {
		select {
		case <-ctx.Done():
			fmt.Fprintf(app.Out, "  no longer watching %s; it goes on in the service — iflow tui shows it, iflow stop stops it\n", id)
			return OK
		case <-c.Done():
			fmt.Fprintln(app.Out, "  the service ended; the runs it kept are carried on when it starts again")
			return Failed
		case <-moved:
		}
		s, ok := c.Run(id)
		if !ok {
			continue
		}
		for _, st := range s.Steps[min(shown, len(s.Steps)):] {
			mark := " "
			if st.Bad {
				mark = "!"
			}
			fmt.Fprintf(app.Out, "%s %d  %s\n", mark, st.Area, st.Text)
		}
		shown = len(s.Steps)
		if s.Question != "" && s.Question != asked {
			asked = s.Question
			switch {
			case accept:
				fmt.Fprintln(app.Out, "? "+s.Question+" — accepted with --accept, without reading")
				if err := c.AcceptAs(ctx, id, person()+" (accepted with --accept, without reading)"); err != nil {
					return app.fatal(err)
				}
			case ask:
				fmt.Fprintf(app.Out, "? %s\n? read %s and answer: accept, or the notes to send it back with\n", s.Question, filepath.Join(s.Dir, artifact.ChangeFile))
				line := "sent back: no answer was given"
				if answers.Scan() {
					line = strings.TrimSpace(answers.Text())
				}
				if line == "accept" {
					err = c.Accept(ctx, id)
				} else {
					err = c.Refuse(ctx, id, line)
				}
				if err != nil {
					return app.fatal(err)
				}
			default:
				fmt.Fprintln(app.Out, "? "+s.Question+" — waiting for you: answer in iflow tui or the window")
			}
		}
		if !s.Finished {
			continue
		}
		fmt.Fprintf(app.Out, "  spent $%.2f, %d tokens in (%d from cache), %d out\n", s.Spent.CostUSD, s.Spent.Input+s.Spent.CacheRead, s.Spent.CacheRead, s.Spent.Output)
		switch s.Outcome {
		case protocol.EndLanded:
			fmt.Fprintf(app.Out, "  landed: %s\n", s.Dir)
			return OK
		case protocol.EndHolds:
			fmt.Fprintf(app.Out, "  nothing to change: what was asked holds already; closed: %s\n", s.Dir)
			return OK
		case protocol.EndStopped:
			fmt.Fprintf(app.Out, "  stopped: %s — iflow resume carries it on\n", s.Dir)
			return Failed
		}
		fmt.Fprintf(app.Out, "  failed: %s\n", s.Error)
		return Failed
	}
}

// Stop asks the run carrying a change to stop, wherever it was started:
// iflow stop <change folder>.
func (app *App) Stop(ctx context.Context, args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(app.Out, "usage: iflow stop <change folder>")
		return Misused
	}
	dir := absolute(args[0])
	if c, err := service.Dial(ctx, protocol.Socket(), "iflow stop", nil); err == nil {
		defer c.Close()
		runs, err := c.Runs(ctx)
		if err != nil {
			return app.fatal(err)
		}
		for _, r := range runs {
			if r.Dir == dir && !r.Finished {
				if err := c.Stop(ctx, r.Run); err != nil {
					return app.fatal(err)
				}
				fmt.Fprintf(app.Out, "  asked %s to stop; iflow resume carries it on\n", dir)
				return OK
			}
		}
	}
	// Not the service's: a run of another process — the bench — watches the
	// change's own file.
	if err := flow.Stop(dir, person()); err != nil {
		return app.fatal(err)
	}
	fmt.Fprintf(app.Out, "  asked %s to stop; it stops within a second, and iflow resume carries it on\n", dir)
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
