package tui

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// terminal is what the terminal the screen runs in can do beyond text, as
// far as it says who it is. What it cannot do is left out, never faked: a
// path is still a path without a link, a question still a question without
// a notification.
type terminal struct {
	name string
	// links says it opens OSC 8 hyperlinks: a file's path is a link to it.
	links bool
	// notify is the escape sequence of a desktop notification, or "" where
	// the terminal has none the screen knows.
	notify func(title, body string) string
}

// osc9 is the notification iTerm2 began and Ghostty and WezTerm read.
func osc9(title, body string) string { return "\x1b]9;" + clean(title+": "+body) + "\x07" }

// osc777 is the notification rxvt began, with a title of its own.
func osc777(title, body string) string {
	return "\x1b]777;notify;" + clean(title) + ";" + clean(body) + "\x07"
}

// osc99 is kitty's notification.
func osc99(title, body string) string {
	return "\x1b]99;i=1:d=0;" + clean(title) + "\x1b\\" + "\x1b]99;i=1:d=1:p=body;" + clean(body) + "\x1b\\"
}

// clean keeps a notification's words from ending its escape sequence early.
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == ';' {
			return ' '
		}
		return r
	}, s)
}

// detect is the terminal the screen runs in, by what it says of itself.
func detect(env func(string) string) terminal {
	switch {
	case env("TERM") == "xterm-kitty" || env("KITTY_WINDOW_ID") != "":
		return terminal{name: "kitty", links: true, notify: osc99}
	case env("TERM_PROGRAM") == "ghostty" || env("GHOSTTY_RESOURCES_DIR") != "":
		return terminal{name: "Ghostty", links: true, notify: osc777}
	case env("TERM_PROGRAM") == "WezTerm":
		return terminal{name: "WezTerm", links: true, notify: osc777}
	case env("TERM_PROGRAM") == "iTerm.app":
		return terminal{name: "iTerm2", links: true, notify: osc9}
	case env("TERM_PROGRAM") == "vscode":
		return terminal{name: "VS Code", links: true}
	}
	return terminal{name: "a terminal"}
}

// link is a file's path as the terminal shows it: a link to the file where
// it opens links, the path alone where not.
func (t terminal) link(path, text string) string {
	if !t.links || path == "" {
		return text
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return text
	}
	host, _ := os.Hostname()
	u := url.URL{Scheme: "file", Host: host, Path: abs}
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", u.String(), text)
}
