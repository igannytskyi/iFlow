package gate

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/igannytskyi/iflow/internal/meta"
)

// The framework's own arbiters are its Go tests. Running them is one action,
// so that it can be done where a change lands and not only where a fixture
// may still agree with the world; passing notes it against exactly this
// framework, which R22 asks for before anything enters.

// ArbiterRun is what running the arbiters said.
type ArbiterRun struct {
	Passed bool
	Output string
}

// Arbiters runs every test of the framework at home.
func Arbiters(ctx context.Context, home string) (ArbiterRun, error) {
	// While they run, the gate says so rather than that they were never run.
	key := ArbitersKey(home)
	marker := arbitersRunning(home)
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err == nil {
		_ = os.WriteFile(marker, []byte(key), 0o644)
		defer os.Remove(marker)
	}
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "./...")
	cmd.Dir = home
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if _, ok := err.(*exec.ExitError); err != nil && !ok {
		return ArbiterRun{}, err
	}
	run := ArbiterRun{Passed: err == nil, Output: out.String()}
	if run.Passed {
		if err := NoteVerified(home, key, "every arbiter passed against this framework"); err != nil {
			return run, err
		}
	}
	return run, nil
}

// arbitersRunning is the file that says the arbiters are being run now,
// and against which state of the framework.
func arbitersRunning(home string) string { return filepath.Join(home, ".estate", "arbiters.running") }

// ArbitersRunning reports whether the arbiters are being run now against
// the framework at home as it is: started within the last quarter hour,
// against this very state of it.
func ArbitersRunning(home string) bool {
	p := arbitersRunning(home)
	st, err := os.Stat(p)
	if err != nil || time.Since(st.ModTime()) > 15*time.Minute {
		return false
	}
	b, err := os.ReadFile(p)
	return err == nil && string(b) == ArbitersKey(home)
}

// Mutation is one rule broken on purpose, and whether an arbiter noticed.
type Mutation struct {
	Rule    string
	Noticed bool
}

// Mutate builds the gate's arbiters once per rule with that rule suppressed
// and runs them: a rule whose loss nothing notices is not being tested,
// whatever the arbiters claim. The demonstration is repeated now rather than
// believed about a run that has ended.
func Mutate(ctx context.Context, home string) ([]Mutation, error) {
	var out []Mutation
	for _, rule := range Rules() {
		cmd := exec.CommandContext(ctx, "go", "test", "-count=1",
			"-ldflags=-X github.com/igannytskyi/iflow/internal/gate.suppressed="+rule, "./internal/gate/")
		cmd.Dir = home
		err := cmd.Run()
		if _, ok := err.(*exec.ExitError); err != nil && !ok {
			return out, err
		}
		out = append(out, Mutation{Rule: rule, Noticed: err != nil})
	}
	return out, nil
}

var (
	quotedString = regexp.MustCompile("\"((?:[^\"\\\\]|\\\\.)*)\"")
	headerLine   = regexp.MustCompile(`(?m)^\|(.*)\|\s*$\n^\|[-:| ]+\|\s*$`)
	codeCell     = regexp.MustCompile("`([A-Za-z0-9-]+)`")
)

// Unused is every template column the gate reads nowhere and every code the
// conventions define and it validates nowhere. It does not call them
// ceremony — most exist for a person — but it makes the question askable.
func Unused(home string) (columns map[string][]string, codes []string, err error) {
	read := map[string]bool{}
	sources, _ := filepath.Glob(filepath.Join(home, "internal", "gate", "*.go"))
	ledger, _ := filepath.Glob(filepath.Join(home, "internal", "ledger", "*.go"))
	sources = append(sources, ledger...)
	for _, f := range sources {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, err
		}
		for _, m := range quotedString.FindAllStringSubmatch(string(b), -1) {
			read[m[1]] = true
		}
	}
	columns = map[string][]string{}
	templates, _ := filepath.Glob(filepath.Join(home, "docs", "templates", "*.md"))
	for _, f := range templates {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, err
		}
		for _, m := range headerLine.FindAllStringSubmatch(string(b), -1) {
			for c := range strings.SplitSeq(m[1], "|") {
				if c = strings.TrimSpace(c); c != "" && c != "#" && !read[c] {
					columns[c] = append(columns[c], filepath.Base(f))
				}
			}
		}
	}
	checked := map[string]bool{}
	for _, values := range meta.Codes {
		for _, v := range values {
			checked[v] = true
		}
	}
	b, err := os.ReadFile(filepath.Join(home, "docs", "method", "conventions.md"))
	if err != nil {
		return nil, nil, err
	}
	seen := map[string]bool{}
	for line := range strings.SplitSeq(string(b), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		for _, m := range codeCell.FindAllStringSubmatch(line, -1) {
			c := m[1]
			lowerHyphen := c == strings.ToLower(c) && strings.Contains(c, "-")
			if (lowerHyphen || c == "yes" || c == "no" || c == "unknown") && !checked[c] && !seen[c] {
				seen[c] = true
				codes = append(codes, c)
			}
		}
	}
	slices.Sort(codes)
	return columns, codes, nil
}
