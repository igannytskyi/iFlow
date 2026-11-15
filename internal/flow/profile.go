package flow

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// profile is how an estate's tests are run: the whole suite, and the named
// tests alone. It is read off the estate, not configured — a test.sh the
// estate keeps wins, then what its language's own runner is.
type profile struct {
	name string
	// suite is the command running every test, as a record names it.
	suite []string
	// only is the command running the named tests.
	only func(names []string) []string
	// build passes when the code builds, running nothing; empty where the
	// estate's language has no such step.
	build []string
	// names finds the tests a file declares.
	names *regexp.Regexp
}

var (
	goTests     = regexp.MustCompile(`(?m)^func (Test\w+)\(`)
	pythonTests = regexp.MustCompile(`(?m)^\s*def (test_\w+)\(`)
)

func detect(repo string) (profile, bool) {
	has := func(name string) bool { _, err := os.Stat(filepath.Join(repo, name)); return err == nil }
	alternation := func(names []string) string { return "^(" + strings.Join(names, "|") + ")$" }
	switch {
	case has("test.sh"):
		return profile{"test.sh", []string{"test.sh"},
			func(n []string) []string { return []string{"test.sh", alternation(n)} }, nil, goTests}, true
	case has("go.mod"):
		return profile{"go test", []string{"go", "test", "-count=1", "./..."},
			func(n []string) []string { return []string{"go", "test", "-count=1", "-run", alternation(n), "./..."} },
			[]string{"go", "build", "./..."}, goTests}, true
	case has("pyproject.toml") || has("pytest.ini") || has("setup.py"):
		return profile{"pytest", []string{"python3", "-m", "pytest", "-q"},
			func(n []string) []string {
				return []string{"python3", "-m", "pytest", "-q", "-k", strings.Join(n, " or ")}
			}, nil, pythonTests}, true
	}
	return profile{}, false
}

// resolve turns a command as a record names it into one that can be run in
// dir: a file the estate keeps by its path there, anything else from PATH.
func resolve(dir string, cmd []string) []string {
	out := append([]string(nil), cmd...)
	if p := filepath.Join(dir, cmd[0]); fileExists(p) {
		out[0] = p
	} else if p, err := exec.LookPath(cmd[0]); err == nil {
		out[0] = p
	}
	return out
}

func fileExists(p string) bool { st, err := os.Stat(p); return err == nil && st.Mode().IsRegular() }

// shown is a command as a record writes it: in backticks, whole.
func shown(cmd []string) string {
	parts := make([]string, len(cmd))
	for i, c := range cmd {
		parts[i] = c
		if strings.ContainsAny(c, " ") {
			parts[i] = "'" + c + "'"
		}
	}
	return "`" + strings.Join(parts, " ") + "`"
}

// runIn runs a command in dir and returns whether it passed, what it said,
// and how long it took.
func runIn(ctx context.Context, dir string, cmd []string) (bool, string, time.Duration) {
	c := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	c.Dir = dir
	began := time.Now()
	out, err := c.CombinedOutput()
	return err == nil, string(out), time.Since(began)
}

// shelter keeps the estate's own test runner out of the change folders. They
// hold copies of every candidate, tests among them, which a runner walking
// the estate would build and run as if they were the estate's — failing on
// tests written to fail before their change existed. Each runner is told in
// its own way; what is written is written once and never overwritten.
func shelter(changes string) error {
	for name, body := range map[string]string{
		// A module of its own is outside the estate's ./... in Go.
		"go.mod": "// The records of changes carried through iFlow, with copies of their\n" +
			"// candidates. A module of its own, so that the estate's go test ./...\n" +
			"// does not build what is kept here.\nmodule iflow.changes\n",
		// pytest collects nothing below a conftest that ignores everything.
		"conftest.py": "# The records of changes carried through iFlow, with copies of their\n" +
			"# candidates: nothing here is the estate's to collect.\ncollect_ignore_glob = [\"*\"]\n",
	} {
		p := filepath.Join(changes, name)
		if fileExists(p) {
			continue
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// DecisionsFile is where a project keeps what its person decided — answers to
// what reviewers asked, reasons a specification was sent back — so that no
// later author asks again what was settled once. Area 10: knowledge kept
// because something could have shown it wrong, and nothing has yet.
const DecisionsFile = "decisions.md"

// decide keeps one decision of the person's, appended, never rewritten.
func decide(repo, change, decision string) error {
	dir := filepath.Join(repo, "changes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	p := filepath.Join(dir, DecisionsFile)
	if !fileExists(p) {
		head := "# Decisions\n\nWhat the person who asks for changes here has decided, newest last. Every author of a specification is handed it.\n\n"
		if err := os.WriteFile(p, []byte(head), 0o644); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "- %s, in %s: %s\n", time.Now().Format("2006-01-02"), change, strings.ReplaceAll(strings.TrimSpace(decision), "\n", " "))
	return err
}

// decisions is what the person has decided before, the newest that fit in
// budget tokens.
func decisions(repo string, budget int) string {
	b, err := os.ReadFile(filepath.Join(repo, "changes", DecisionsFile))
	if err != nil {
		return ""
	}
	var lines []string
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "- ") {
			lines = append(lines, l)
		}
	}
	out, used := []string{}, 0
	for i := len(lines) - 1; i >= 0 && used+len(lines[i])/4 <= budget; i-- {
		out = append([]string{lines[i]}, out...)
		used += len(lines[i]) / 4
	}
	return strings.Join(out, "\n")
}
