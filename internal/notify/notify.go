// Package notify posts a notification to the person's desktop: what a
// window tells them when it is not the window they are looking at.
//
// On macOS it is posted by osascript — the one way a program not signed
// for the notification centre can post one — and on Linux by notify-send,
// where the desktop has it. Elsewhere, and where neither is there, nothing
// is posted, and Send says so.
package notify

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// ErrNone is why nothing was posted: this system has no way here to post.
var ErrNone = errors.New("this system offers no notification to post")

// command is how a notification of title and body is posted here, or nil.
// The words are passed as arguments, never written into a script: a
// question with a quote in it is a question, not a command.
func command(goos string, have func(string) bool, title, body string) []string {
	switch {
	case goos == "darwin" && have("osascript"):
		return []string{"osascript",
			"-e", "on run argv",
			"-e", "display notification (item 2 of argv) with title (item 1 of argv)",
			"-e", "end run",
			"--", title, body}
	case goos == "linux" && have("notify-send"):
		return []string{"notify-send", "--app-name=iflow", "--", title, body}
	}
	return nil
}

func found(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// Send posts a notification of title and body, waiting at most a few
// seconds for the system to take it.
func Send(ctx context.Context, title, body string) error {
	args := command(runtime.GOOS, found, oneLine(title), oneLine(body))
	if args == nil {
		return ErrNone
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
	if err != nil {
		return errors.New(args[0] + ": " + strings.TrimSpace(string(out)) + " " + err.Error())
	}
	return nil
}

// oneLine is text as a notification shows it: on one line, not too long
// to be read in one.
func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 240 {
		s = string(r[:239]) + "…"
	}
	return s
}
