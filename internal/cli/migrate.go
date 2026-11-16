package cli

import (
	"fmt"
	"strings"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/workspace"
)

// Migrate writes every change under the folders named in the data model:
// its documents as JSON, its Markdown kept under legacy/. With --dry it says
// what it would write and what stands in the way, and writes nothing. A
// change any of whose documents cannot be read whole is left as it is.
func (app *App) Migrate(args []string) int {
	dry := false
	var roots []string
	for _, a := range args {
		if a == "--dry" {
			dry = true
			continue
		}
		roots = append(roots, a)
	}
	if len(roots) == 0 {
		roots = []string{"changes"}
	}
	stuck, moved := 0, 0
	for _, dir := range workspace.Folders(roots...) {
		m, err := artifact.Migrate(dir, dry)
		if err != nil {
			return app.fatal(err)
		}
		switch {
		case len(m.Problems) > 0:
			stuck++
			fmt.Fprintf(app.Out, "  FAIL    %s\n", dir)
			for _, p := range m.Problems {
				fmt.Fprintf(app.Out, "          %s\n", p)
			}
		case len(m.Written) > 0:
			moved++
			verb := "written"
			if dry {
				verb = "would write"
			}
			fmt.Fprintf(app.Out, "  ok      %s  %s %s\n", dir, verb, strings.Join(m.Written, ", "))
		}
	}
	fmt.Fprintf(app.Out, "  %d change(s) migrated, %d held back\n", moved, stuck)
	if stuck > 0 {
		return Failed
	}
	return OK
}
