// Command iflow is the method as one program.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/igannytskyi/iflow/internal/cli"
	"github.com/igannytskyi/iflow/internal/ui"
)

func main() {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "desk" {
		// A window needs the main thread on some platforms, so the program
		// runs beside it and ends the process when it is done.
		go func() { os.Exit(run(args)) }()
		ui.Main()
	}
	os.Exit(run(args))
}

func run(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	app, closeApp := cli.New(os.Stdout, os.Stderr)
	defer closeApp()
	return app.Run(ctx, args)
}
