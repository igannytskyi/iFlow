package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/igannytskyi/iflow/internal/protocol"
)

// Connect speaks to the service at socket as client, starting one apart
// from the screen where none listens: the runs it carries go on when the
// screen closes. A service running an older program than the screen's is
// replaced first, its runs kept and carried on by the new one.
func Connect(ctx context.Context, socket, client string, changed func(run string)) (*Client, error) {
	if c, err := Dial(ctx, socket, client, changed); err == nil {
		if !Older(c.Server()) {
			return c, nil
		}
		if err := Replace(ctx, c, socket); err != nil {
			return nil, err
		}
	}
	return Start(ctx, socket, client, changed)
}

// Start starts a service at socket, apart from the screen, and connects to
// it as client.
func Start(ctx context.Context, socket, client string, changed func(run string)) (*Client, error) {
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

// Replace asks the service c speaks to to end, keeping its runs, and waits
// until it no longer listens at socket.
func Replace(ctx context.Context, c *Client, socket string) error {
	_, err := c.Shutdown(ctx, true)
	_ = c.Close()
	if err != nil {
		// A service that ends at once may close before its answer is read:
		// that it is gone is the answer.
		if Gone(ctx, socket, 5*time.Second) == nil {
			return nil
		}
		return fmt.Errorf("the service at %s would not end: %w", socket, err)
	}
	return Gone(ctx, socket, 90*time.Second)
}

// Gone waits until no service listens at socket, for at most within.
func Gone(ctx context.Context, socket string, within time.Duration) error {
	deadline := time.Now().Add(within)
	for {
		c, err := protocol.Dial(ctx, socket, nil)
		if err != nil {
			return nil
		}
		_ = c.Close()
		if time.Now().After(deadline) {
			return fmt.Errorf("the service at %s still listens after %s", socket, within)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
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
	log, err := os.OpenFile(LogFile(socket), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer log.Close()
	cmd := exec.Command(self, "service", "run")
	cmd.Env = append(os.Environ(), "IFLOW_SOCKET="+socket)
	cmd.Stdout, cmd.Stderr = log, log
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// LogFile is where the service at socket writes what it says.
func LogFile(socket string) string { return filepath.Join(filepath.Dir(socket), "serve.log") }

// KeepFile is where the service at socket keeps the runs it was asked to
// keep when it ended, for the next one.
func KeepFile(socket string) string { return filepath.Join(filepath.Dir(socket), "kept.json") }
