package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/igannytskyi/iflow/internal/protocol"
	"github.com/igannytskyi/iflow/internal/service"
	"github.com/igannytskyi/iflow/internal/tui"
)

// TUI is the path in the terminal, through the service — started beside it
// where none listens: iflow tui [--theme light|terminal] [project], the
// project being the folder it is started in where none is named. The light
// theme, the default, sets the terminal to black on white while the screen
// is open; IFLOW_THEME chooses where --theme does not.
func (app *App) TUI(ctx context.Context, args []string) int {
	theme := tui.Light
	if t := os.Getenv("IFLOW_THEME"); t != "" {
		theme = tui.Theme(t)
	}
	var rest []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--theme" && i+1 < len(args) {
			theme = tui.Theme(args[i+1])
			i++
			continue
		}
		rest = append(rest, args[i])
	}
	if theme != tui.Light && theme != tui.Terminal {
		fmt.Fprintf(app.Out, "usage: iflow tui [--theme light|terminal] [project]; %q is no theme\n", theme)
		return Misused
	}
	project, err := os.Getwd()
	if len(rest) > 0 {
		project, err = filepath.Abs(rest[0])
	}
	if err != nil {
		return app.fatal(err)
	}
	moved := make(chan string, 256)
	client, err := service.Connect(ctx, protocol.Socket(), "iflow tui", func(run string) {
		select {
		case moved <- run:
		default: // the screen reads every run again when it next draws
		}
	})
	if err != nil {
		return app.fatal(err)
	}
	defer client.Close()
	fmt.Fprint(os.Stdout, tui.Paint(theme))
	defer fmt.Fprint(os.Stdout, tui.Unpaint(theme))
	p := tea.NewProgram(tui.New(ctx, client, moved, project, theme), tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithContext(ctx))
	if _, err := p.Run(); err != nil && ctx.Err() == nil {
		return app.fatal(err)
	}
	return OK
}
