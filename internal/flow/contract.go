package flow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/igannytskyi/iflow/internal/estate"
)

// estateGraph answers an agent's questions of the estate's model of its
// code, each place with its line as the estate holds it.
type estateGraph struct {
	repo *estate.Repo
	root string
}

// Defined is where name is defined, each with its first line.
func (g estateGraph) Defined(name string) (string, error) {
	places, err := g.repo.WhereDefined(name)
	if err != nil {
		return "", err
	}
	if len(places) == 0 {
		return name + " is defined nowhere in the estate as it stood; an earlier unit may provide it — see what you were handed", nil
	}
	var b strings.Builder
	for _, p := range places[:min(len(places), 12)] {
		fmt.Fprintf(&b, "%s:%d-%d  %s\n", p.File, p.Start, p.End, g.line(p.File, p.Start))
	}
	return b.String(), nil
}

// Callers is every place that uses name, firmest first, each with its line.
func (g estateGraph) Callers(name string) (string, error) {
	sites, err := g.repo.CallSites(name)
	if err != nil {
		return "", err
	}
	if len(sites) == 0 {
		return "nothing in the estate uses " + name, nil
	}
	var b strings.Builder
	for _, s := range sites[:min(len(sites), 40)] {
		fmt.Fprintf(&b, "%s:%d  %s\n", s.File, s.Line, g.line(s.File, s.Line))
	}
	if len(sites) > 40 {
		fmt.Fprintf(&b, "[%d more]\n", len(sites)-40)
	}
	return b.String(), nil
}

func (g estateGraph) line(file string, n int) string {
	b, err := os.ReadFile(filepath.Join(g.root, filepath.FromSlash(file)))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	if n < 1 || n > len(lines) {
		return ""
	}
	return strings.TrimSpace(lines[n-1])
}
