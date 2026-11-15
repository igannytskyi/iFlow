package cli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"

	"github.com/igannytskyi/iflow"
	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/gate"
	"github.com/igannytskyi/iflow/internal/method"
	"github.com/igannytskyi/iflow/internal/ui"
)

// Desk opens the workbench on the projects named beside those it remembers,
// or on the examples that come with the framework when there are none.
func (app *App) Desk(ctx context.Context, projects []string) int {
	model, err := method.Load(iflow.Texts)
	if err != nil {
		return app.fatal(err)
	}
	var examples []string
	entries, _ := os.ReadDir(filepath.Join(gate.Home(), "examples"))
	for _, e := range entries {
		if e.IsDir() {
			examples = append(examples, filepath.Join(gate.Home(), "examples", e.Name()))
		}
	}
	cfg, problems := app.FlowConfig()
	err = ui.Run(ctx, ui.Options{Model: model, Gate: app.Gate, Parallel: runtime.GOMAXPROCS(0),
		Projects: projects, Examples: examples, Flow: &cfg, ModelsProblem: problems,
		Rebuild: func() (flow.Config, string) { return app.FlowConfig() }})
	if err != nil {
		return app.fatal(err)
	}
	return OK
}
