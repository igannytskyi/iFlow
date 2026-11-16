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
	// With --ask the person answers on standard input, one line a question:
	// "accept", or the notes the specification is sent back with.
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
	run = flow.New(cfg, rest[0], strings.Join(rest[1:], " "), func() {
		if run != nil {
			print()
		}
	})
	err := run.Start(ctx)
	print()
	v := run.View()
	fmt.Fprintf(app.Out, "  spent $%.2f, %d tokens in (%d from cache), %d out\n", v.Spent.CostUSD, v.Spent.In(), v.Spent.CacheRead, v.Spent.Output)
	if err != nil {
		return Failed
	}
	fmt.Fprintf(app.Out, "  landed: %s\n", v.Dir)
	return OK
}
