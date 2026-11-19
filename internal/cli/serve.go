package cli

import (
	"context"
	"errors"
	"fmt"

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
	if err := service.New(ctx, service.FlowMaker{Config: app.flowConfigLogged}, person()).Serve(ctx, l); err != nil {
		return app.fatal(err)
	}
	return OK
}

// flowConfigLogged is the configuration a run is carried with, saying on
// standard error what is wrong with the choice of models, if anything.
func (app *App) flowConfigLogged() flow.Config {
	cfg, problems := app.FlowConfig()
	if problems != "" {
		fmt.Fprintln(app.Err, problems)
	}
	return cfg
}
