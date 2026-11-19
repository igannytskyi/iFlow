package cli

import (
	"context"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/igannytskyi/iflow/internal/protocol"
	"github.com/igannytskyi/iflow/internal/tui"
)

// TUI is the path in the terminal, through the service — started beside it
// where none listens: iflow tui [project], the project being the folder it
// is started in where none is named.
func (app *App) TUI(ctx context.Context, args []string) int {
	project, err := os.Getwd()
	if len(args) > 0 {
		project, err = filepath.Abs(args[0])
	}
	if err != nil {
		return app.fatal(err)
	}
	moved := make(chan string, 256)
	client, err := tui.Connect(ctx, protocol.Socket(), moved)
	if err != nil {
		return app.fatal(err)
	}
	defer client.Close()
	p := tea.NewProgram(tui.New(ctx, client, moved, project), tea.WithAltScreen(), tea.WithContext(ctx))
	if _, err := p.Run(); err != nil && ctx.Err() == nil {
		return app.fatal(err)
	}
	return OK
}
