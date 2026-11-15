// Package gate is the mechanical check over one change: what its artefacts
// can decide. What they cannot decide is listed in docs/method/rules.md under
// the rules marked H, and is not attempted here.
//
// Each check is a value in a registry, declaring the rules it can find
// broken. That declaration is what lets the gate be mutated: suppress a rule,
// run the arbiters, and see whether anything notices.
package gate

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/estate"
)

// Severity says whether a finding stops the change.
type Severity int

const (
	// Fail is a rule broken: the change does not pass.
	Fail Severity = iota
	// Note is something the gate could not ask, or a change that predates an
	// obligation rather than violating it. Said, never silently skipped.
	Note
)

// Finding is one thing the gate found.
type Finding struct {
	Rule     string
	Severity Severity
	Message  string
}

// String renders a finding as the gate prints it.
func (f Finding) String() string { return f.Rule + ": " + f.Message }

// Report is everything the gate found in one change.
type Report struct {
	Change   *artifact.Change
	Findings []Finding
}

// Problems returns the failures, in the order they were found.
func (r Report) Problems() []Finding { return r.only(Fail) }

// Notes returns the notes, in the order they were found.
func (r Report) Notes() []Finding { return r.only(Note) }

// Passed reports whether nothing failed.
func (r Report) Passed() bool { return len(r.Problems()) == 0 }

func (r Report) only(s Severity) []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if f.Severity == s {
			out = append(out, f)
		}
	}
	return out
}

// Gate checks changes against one framework and one estate.
type Gate struct {
	estate   estate.Estate
	home     string
	suppress map[string]bool
}

// Option configures a Gate.
type Option func(*Gate)

// WithHome names the framework that judges, when it is not Home().
func WithHome(dir string) Option { return func(g *Gate) { g.home = dir } }

// Suppress makes the gate blind to the named rules. It exists for one
// purpose: to break a rule on purpose and see whether the arbiters notice.
func Suppress(rules ...string) Option {
	return func(g *Gate) {
		for _, r := range rules {
			g.suppress[r] = true
		}
	}
}

// suppressed is set only in a build made to break a rule, with
// -ldflags "-X github.com/igannytskyi/iflow/internal/gate.suppressed=R7".
// A gate built normally suppresses nothing.
var suppressed string

// New returns a gate asking the given estate.
func New(est estate.Estate, opts ...Option) *Gate {
	g := &Gate{estate: est, home: Home(), suppress: map[string]bool{}}
	for r := range strings.SplitSeq(suppressed, ",") {
		if r != "" {
			g.suppress[r] = true
		}
	}
	for _, o := range opts {
		o(g)
	}
	return g
}

// Home is the framework this gate judges by.
func (g *Gate) Home() string { return g.home }

// Estate is what this gate asks.
func (g *Gate) Estate() estate.Estate { return g.estate }

// check is one entry in the registry.
type check struct {
	name string
	// area is the area of responsibility whose output the check judges; 0
	// for a check that holds at every boundary.
	area  int
	rules []string
	run   func(*run)
}

// run is one check of one change in progress.
type run struct {
	ctx      context.Context
	gate     *Gate
	c        *artifact.Change
	findings []Finding
}

func (r *run) fail(rule, format string, args ...any) { r.add(rule, Fail, format, args...) }
func (r *run) note(rule, format string, args ...any) { r.add(rule, Note, format, args...) }

func (r *run) add(rule string, s Severity, format string, args ...any) {
	if r.gate.suppress[rule] {
		return
	}
	r.findings = append(r.findings, Finding{Rule: rule, Severity: s, Message: fmt.Sprintf(format, args...)})
}

func (r *run) doc(s artifact.Stage) *artifact.Doc { return r.c.Doc(s) }
func (r *run) has(s artifact.Stage) bool          { return r.c.Has(s) }

// Check runs every check against one change, in the order the rules were
// written.
func (g *Gate) Check(ctx context.Context, c *artifact.Change) Report {
	r := &run{ctx: ctx, gate: g, c: c}
	for _, ch := range registry {
		if ctx.Err() != nil {
			break
		}
		ch.run(r)
	}
	return Report{Change: c, Findings: r.findings}
}

// AreaOf is the area of responsibility a rule's check judges the output of,
// and whether the gate checks the rule at all.
func AreaOf(rule string) (int, bool) {
	for _, ch := range registry {
		if slices.Contains(ch.rules, rule) {
			return ch.area, true
		}
	}
	return 0, false
}

// Rules lists every rule a check can find broken, sorted.
func Rules() []string {
	var out []string
	for _, ch := range registry {
		out = append(out, ch.rules...)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// registry is every check, in the order the rules were written.
var registry = []check{
	{"stage order", 0, []string{"B1"}, stageOrder},
	{"headers present", 0, []string{"B1"}, headersPresent},
	{"criteria vocabulary", 1, []string{"R19"}, criteriaVocabulary},
	{"evidence plan", 3, []string{"R16", "CONVENTIONS"}, evidencePlan},
	{"arbiter inputs", 3, []string{"R7b"}, arbiterInputs},
	{"every criterion arbitrated", 1, []string{"R11"}, everyCriterionArbitrated},
	{"vocabulary", 0, []string{"CONVENTIONS"}, vocabulary},
	{"specification immutable", 1, []string{"R1"}, specImmutable},
	{"candidates stored", 4, []string{"R2"}, candidatesStored},
	{"scope and arbiter disjoint", 1, []string{"R7"}, scopeArbiterDisjoint},
	{"evidence independence", 5, []string{"R3", "R3b", "B2"}, evidenceIndependence},
	{"refusals", 5, []string{"R17", "CONVENTIONS"}, refusals},
	{"criteria review", 3, []string{"R18"}, criteriaReview},
	{"repeatability", 5, []string{"R13", "R14"}, repeatability},
	{"deferred complete", 5, []string{"R6"}, deferredComplete},
	{"verdict coverage", 5, []string{"A5"}, verdictCoverage},
	{"landing backed", 6, []string{"A6", "R10"}, landingBacked},
	{"record append-only", 8, []string{"R4"}, recordAppendOnly},
	{"arbiter changes separate", 4, []string{"R8"}, arbiterChangesSeparate},
	{"record complete", 8, []string{"B4"}, recordComplete},
	{"landing taught", 7, []string{"R21"}, landingTaught},
	{"verified before entry", 6, []string{"R22"}, verifiedBeforeEntry},
}
