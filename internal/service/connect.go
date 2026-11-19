package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Connect speaks to the service at socket as client, starting one apart
// from the screen where none listens: the runs it carries go on when the
// screen closes.
func Connect(ctx context.Context, socket, client string, changed func(run string)) (*Client, error) {
	if c, err := Dial(ctx, socket, client, changed); err == nil {
		return c, nil
	}
	if err := spawn(socket); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		c, err := Dial(ctx, socket, client, changed)
		if err == nil {
			return c, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("the service did not come up at %s: %w", socket, err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// spawn starts the service apart from the screen, writing what it says to a
// log beside its socket.
func spawn(socket string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(socket), 0o700); err != nil {
		return err
	}
	log, err := os.OpenFile(filepath.Join(filepath.Dir(socket), "serve.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer log.Close()
	cmd := exec.Command(self, "serve")
	cmd.Env = append(os.Environ(), "IFLOW_SOCKET="+socket)
	cmd.Stdout, cmd.Stderr = log, log
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
