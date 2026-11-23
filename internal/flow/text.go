package flow

import (
	"strings"
)

// head is the first n bytes of s, marked where cut.
func head(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// tail is the last n bytes of s, marked where cut.
func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}

// lastLines is the last n lines of out.
func lastLines(out string, n int) string {
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	return strings.Join(lines[max(0, len(lines)-n):], "\n")
}
