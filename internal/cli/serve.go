package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/protocol"
	"github.com/igannytskyi/iflow/internal/service"
)

// Serve carries runs for every screen that connects, until it is stopped:
// iflow serve. The socket is IFLOW_SOCKET, or beside the iflow
// configuration.
func (app *App) Serve(ctx context.Context) int {
	socket := protocol.Socket()
	l, err := service.Listen(ctx, socket)
	if errors.Is(err, service.ErrRunning) {
		fmt.Fprintf(app.Out, "a service is listening at %s already\n", socket)
		return OK
	}
	if err != nil {
		return app.fatal(err)
	}
	fmt.Fprintf(app.Out, "carrying runs for the screens at %s\n", socket)
	if err := service.New(ctx, flowMaker{app}, person()).Serve(ctx, l); err != nil {
		return app.fatal(err)
	}
	return OK
}

// flowMaker makes the service's runs as the path carries any: with the
// models chosen now, read again for every run.
type flowMaker struct{ app *App }

func (m flowMaker) config() flow.Config {
	cfg, problems := m.app.FlowConfig()
	if problems != "" {
		fmt.Fprintln(m.app.Err, problems)
	}
	return cfg
}

func (m flowMaker) New(project, statement string, settles []string, changed func()) (service.Run, error) {
	r := flow.New(m.config(), project, statement, changed)
	r.Settles(settles...)
	return r, nil
}

func (m flowMaker) Resume(dir string, changed func()) (service.Run, error) {
	return flow.Resume(m.config(), filepath.Dir(filepath.Dir(dir)), dir, changed)
}

func (m flowMaker) Reopen(dir string, from int, notes string, changed func()) (service.Run, error) {
	return flow.Reopen(m.config(), filepath.Dir(filepath.Dir(dir)), dir, from, notes, changed)
}
