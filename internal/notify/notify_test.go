package notify

import (
	"slices"
	"testing"
)

// The words of a notification are the arguments of what posts it, never a
// part of its script, on every system that posts one; where none can,
// nothing is run.
func TestANotificationsWordsAreArgumentsOnly(t *testing.T) {
	title, body := `iflow — waiting for you: "accept?"`, `it says "end run" -e 'do shell script "x"'`
	all := func(string) bool { return true }
	for _, goos := range []string{"darwin", "linux"} {
		args := command(goos, all, title, body)
		if len(args) < 3 || args[len(args)-2] != title || args[len(args)-1] != body || args[len(args)-3] != "--" {
			t.Errorf("%s: the words are not the last arguments, after --: %q", goos, args)
		}
		for _, a := range args[:len(args)-2] {
			if a == title || a == body {
				t.Errorf("%s: the words are in the command itself: %q", goos, args)
			}
		}
	}
	if args := command("darwin", func(string) bool { return false }, title, body); args != nil {
		t.Errorf("a notification was to be posted with no tool to post it: %q", args)
	}
	if args := command("windows", all, title, body); args != nil {
		t.Errorf("a notification was to be posted where none is known: %q", args)
	}
}

func TestANotificationIsOneLine(t *testing.T) {
	long := string(slices.Repeat([]rune("a "), 300))
	if got := oneLine("a\n\tb  c"); got != "a b c" {
		t.Errorf("%q", got)
	}
	if got := []rune(oneLine(long)); len(got) != 240 || got[239] != '…' {
		t.Errorf("a long body is %d runes", len(got))
	}
}
