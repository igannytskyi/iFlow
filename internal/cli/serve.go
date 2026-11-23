package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/home"
	"github.com/igannytskyi/iflow/internal/model"
	"github.com/igannytskyi/iflow/internal/protocol"
	"github.com/igannytskyi/iflow/internal/service"
)

const serviceUsage = `usage: iflow service status     whether it runs, which build, since when, how many runs it carries
       iflow service start      start it, if none runs (a screen starts it as well)
       iflow service stop       end it; the runs it carries are kept, and the next one carries them on
       iflow service restart    end it and start this build, carrying its runs on
       iflow service log        the last of what it said
       iflow service run        carry runs in this process, until it is stopped (what start runs)`

// Service is the service's life: iflow service <status|start|stop|restart|log|run>.
func (app *App) Service(ctx context.Context, args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(app.Out, serviceUsage)
		return Misused
	}
	socket := protocol.Socket()
	switch args[0] {
	case "run":
		return app.Serve(ctx)
	case "status":
		c, err := service.Dial(ctx, socket, "iflow service", nil)
		if err != nil {
			fmt.Fprintf(app.Out, "no service runs at %s\n", socket)
			return OK
		}
		defer c.Close()
		app.describeService(c.Server(), socket)
		return OK
	case "start":
		c, err := service.Connect(ctx, socket, "iflow service", nil)
		if err != nil {
			return app.fatal(err)
		}
		defer c.Close()
		app.describeService(c.Server(), socket)
		return OK
	case "stop", "restart":
		c, err := service.Dial(ctx, socket, "iflow service", nil)
		if err == nil {
			kept := c.Server().Running
			if err := service.Replace(ctx, c, socket); err != nil {
				return app.fatal(err)
			}
			fmt.Fprintf(app.Out, "the service ended; %d run(s) kept, to be carried on by the next\n", kept)
		} else {
			fmt.Fprintf(app.Out, "no service ran at %s\n", socket)
		}
		if args[0] == "stop" {
			return OK
		}
		c, err = service.Start(ctx, socket, "iflow service", nil)
		if err != nil {
			return app.fatal(err)
		}
		defer c.Close()
		app.describeService(c.Server(), socket)
		return OK
	case "log":
		b, err := os.ReadFile(service.LogFile(socket))
		if err != nil {
			return app.fatal(err)
		}
		fmt.Fprint(app.Out, lastLinesOf(string(b), 40))
		return OK
	}
	fmt.Fprintln(app.Out, serviceUsage)
	return Misused
}

// describeService says who the service is, and whether this program is a
// later build of it.
func (app *App) describeService(s protocol.InitializeResult, socket string) {
	fmt.Fprintf(app.Out, "the service runs: process %d, build %s of %s, since %s, carrying %d run(s)\n  socket %s\n  log    %s\n",
		s.PID, s.Build, s.BuiltAt.Format(time.DateTime), s.Since.Format(time.DateTime), s.Running, socket, service.LogFile(socket))
	if service.Older(s) {
		id, at := service.Build()
		fmt.Fprintf(app.Out, "  this program is a later build (%s of %s): iflow service restart, or any screen opened, replaces it\n", id, at.Format(time.DateTime))
	}
}

func lastLinesOf(s string, n int) string {
	lines := strings.SplitAfter(s, "\n")
	return strings.Join(lines[max(0, len(lines)-n):], "")
}

// Serve carries runs for every screen that connects, until it is stopped:
// iflow service run. The socket is IFLOW_SOCKET, or beside the iflow
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
	id, at := service.Build()
	fmt.Fprintf(app.Out, "%s carrying runs for the screens at %s, build %s of %s, process %d\n",
		time.Now().Format(time.DateTime), socket, id, at.Format(time.DateTime), os.Getpid())
	err = service.New(ctx, service.Config{Maker: service.FlowMaker{Config: app.flowConfigLogged}, Person: person(),
		Projects: home.Projects, Model: func() model.Model { return app.flowConfigLogged().Model }, KeepFile: service.KeepFile(socket)}).Serve(ctx, l)
	fmt.Fprintf(app.Out, "%s the service ended\n", time.Now().Format(time.DateTime))
	if err != nil {
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
