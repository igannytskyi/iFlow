package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/flow"
)

// Describe writes, after the fact, the whole specification of each change
// named that has none: four at a time, each by the author's model, each
// recorded as written after the fact.
func (app *App) Describe(ctx context.Context, dirs []string) int {
	if len(dirs) == 0 {
		fmt.Fprintln(app.Out, "usage: iflow describe <change>...")
		return Misused
	}
	cfg, problems := app.FlowConfig()
	if problems != "" {
		fmt.Fprintln(app.Err, problems)
	}
	var mu sync.Mutex
	failed := 0
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for _, dir := range dirs {
		abs, _ := filepath.Abs(dir)
		if _, err := os.Stat(filepath.Join(abs, string(artifact.Specification)+".md")); err != nil {
			continue // not a change
		}
		if _, err := os.Stat(filepath.Join(abs, artifact.ChangeFile)); err == nil {
			fmt.Fprintf(app.Out, "  %s already has its specification\n", filepath.Base(abs))
			continue
		}
		repo := filepath.Dir(filepath.Dir(abs))
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			err := flow.Describe(ctx, cfg, repo, abs)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failed++
				fmt.Fprintf(app.Out, "! %s: %v\n", filepath.Base(abs), err)
				return
			}
			fmt.Fprintf(app.Out, "  %s: written\n", filepath.Base(abs))
		}()
	}
	wg.Wait()
	if failed > 0 {
		return Failed
	}
	return OK
}
