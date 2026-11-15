package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/igannytskyi/iflow/internal/adopt"
)

// Adopt reads a change written for another spec-driven method as intent and
// criteria, says what did not translate, and writes it as a change here when
// asked to.
func (app *App) Adopt(_ context.Context, args []string) int {
	if len(args) < 2 || (args[0] != "openspec" && args[0] != "speckit") {
		fmt.Fprintln(app.Out, "usage: iflow adopt openspec|speckit <folder> [--out <changes/slug>] [--number=N]")
		return Misused
	}
	read := adopt.OpenSpec
	if args[0] == "speckit" {
		read = adopt.SpecKit
	}
	t, err := read(args[1])
	if err != nil {
		return app.fatal(err)
	}
	fmt.Fprintf(app.Out, "%s %s: %d criterion(s), %d not translated\n", t.Method, t.Name, len(t.Criteria), len(t.Untranslated))
	for _, u := range t.Untranslated {
		fmt.Fprintf(app.Out, "    not translated  %s  %s — %s\n", u.From, clip(u.What, 70), u.Why)
	}
	out, number := "", 1
	for i, a := range args {
		switch {
		case a == "--out" && i+1 < len(args):
			out = args[i+1]
		case strings.HasPrefix(a, "--number="):
			fmt.Sscan(strings.TrimPrefix(a, "--number="), &number)
		}
	}
	if out != "" {
		if err := adopt.Write(t, out, number); err != nil {
			return app.fatal(err)
		}
		fmt.Fprintf(app.Out, "  written to %s\n", out)
	}
	return OK
}
