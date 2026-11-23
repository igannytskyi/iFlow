package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/igannytskyi/iflow/internal/flow"
)

// Settle closes an escalation by the person's decision:
// iflow settle <change folder> <n> <decision>.
func (app *App) Settle(args []string) int {
	if len(args) < 3 {
		fmt.Fprintln(app.Out, "usage: iflow settle <change folder> <n> <decision>")
		return Misused
	}
	dir := filepath.Clean(args[0])
	ref := filepath.Base(dir) + "#" + args[1]
	repo := filepath.Dir(filepath.Dir(absolute(dir)))
	if err := flow.Settle(repo, dir, ref, person(), strings.Join(args[2:], " ")); err != nil {
		return app.fatal(err)
	}
	fmt.Fprintf(app.Out, "  settled %s by %s\n", ref, person())
	return OK
}

func absolute(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}
