// Package flow carries one task through the path: from a sentence a person
// typed to a change entered into the estate, area by area, each area's
// output written as the artefact the method defines and checked by the gate
// at its boundary before the next area receives it.
//
// The order is fixed here and nowhere else; agents work inside an area and
// never decide what happens next. A person is asked once — to accept the
// criteria — and may refuse at any boundary.
package flow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/estate"
	"github.com/igannytskyi/iflow/internal/gate"
	"github.com/igannytskyi/iflow/internal/harness"
	"github.com/igannytskyi/iflow/internal/model"
)

// Status is where an area stands in a run.
type Status string

// The states an area passes through.
const (
	Idle    Status = "idle"
	Working Status = "working"
	Waiting Status = "waiting for you"
	Done    Status = "done"
	Failed  Status = "stopped"
)

// Step is one thing that happened in a run, as the person watching sees it.
type Step struct {
	At     time.Time
	Area   int
	Text   string
	Object string // an identifier the step produced or concerns, if any
	Bad    bool
}

// Config is what a run is carried out with.
type Config struct {
	// Model states the intent and its specification; Reviewer and Planner,
	// where given, review documents and divide the work, and are Model
	// where not.
	Model    model.Model
	Reviewer model.Model
	Planner  model.Model
	Executor harness.Executor
	Estate   *estate.Local
	Gate     *gate.Gate
	// Person is who accepts criteria and is named where a person acted.
	Person string
	// BudgetUSD bounds each unit's execution.
	BudgetUSD float64
	// PackTokens bounds what one agent is handed of the estate; 8000 when
	// zero.
	PackTokens int
}

func (c Config) reviewer() model.Model { return orModel(c.Reviewer, c.Model) }
func (c Config) planner() model.Model  { return orModel(c.Planner, c.Model) }

func orModel(m, otherwise model.Model) model.Model {
	if m != nil {
		return m
	}
	return otherwise
}

func (c Config) packBudget() int {
	if c.PackTokens > 0 {
		return c.PackTokens
	}
	return 8000
}

// Run is one task on its way along the path.
type Run struct {
	cfg  Config
	repo string

	mu        sync.Mutex
	statement string
	slug, dir string
	n         int
	areas     [7]Status
	steps     []Step
	agent     map[string][]harness.Event // per unit
	question  string
	answer    chan decision
	spent     model.Usage
	costs     []artifact.Cost
	unkept    []artifact.Cost // recorded before the change had a folder
	finished  bool
	changed   func()

	spec draft
	// reviewLeft is what the last review of the specification still found,
	// which the person is shown beside it.
	reviewLeft review
	units      []unit
	ev         []artifact.EvidenceRow
	cands      map[string]string // unit → candidate folder
}

type decision struct {
	ok     bool
	ground string
}

// New prepares a run of a task against the estate at repo. changed is called
// whenever there is something new to show.
func New(cfg Config, repo, statement string, changed func()) *Run {
	if abs, err := filepath.Abs(repo); err == nil {
		repo = abs
	}
	r := &Run{cfg: cfg, repo: repo, statement: statement, agent: map[string][]harness.Event{},
		answer: make(chan decision, 1), changed: changed, cands: map[string]string{}}
	for i := range r.areas {
		r.areas[i] = Idle
	}
	return r
}

// View is a run as it stands, copied so it can be read without holding it.
type View struct {
	Repo, Statement, Slug, Dir string
	Areas                      [7]Status
	Steps                      []Step
	Agent                      map[string][]harness.Event
	Question                   string
	// Reviewed is what the reviewing agent still found in the document the
	// person is asked about, if anything.
	Reviewed string
	Spent    model.Usage
	Costs    []artifact.Cost
	Finished bool
}

// View returns the run as it stands.
func (r *Run) View() View {
	r.mu.Lock()
	defer r.mu.Unlock()
	agent := map[string][]harness.Event{}
	for k, v := range r.agent {
		agent[k] = slices.Clone(v)
	}
	return View{Repo: r.repo, Statement: r.statement, Slug: r.slug, Dir: r.dir, Areas: r.areas,
		Steps: slices.Clone(r.steps), Agent: agent, Question: r.question, Reviewed: r.reviewLeft.forAuthor(), Spent: r.spent,
		Costs: slices.Clone(r.costs), Finished: r.finished}
}

// Accept answers the question the run is waiting on, yes.
func (r *Run) Accept() { r.reply(decision{ok: true}) }

// Refuse answers it no, with the ground. A refusal is the producer's
// failure, not a request to try again.
func (r *Run) Refuse(ground string) { r.reply(decision{ground: ground}) }

func (r *Run) reply(d decision) {
	r.mu.Lock()
	waiting := r.question != ""
	r.mu.Unlock()
	if waiting {
		select {
		case r.answer <- d:
		default:
		}
	}
}

func (r *Run) say(area int, object, format string, args ...any) {
	r.mu.Lock()
	r.steps = append(r.steps, Step{At: time.Now(), Area: area, Object: object, Text: fmt.Sprintf(format, args...)})
	r.mu.Unlock()
	r.changed()
}

func (r *Run) fail(area int, format string, args ...any) error {
	r.mu.Lock()
	r.steps = append(r.steps, Step{At: time.Now(), Area: area, Text: fmt.Sprintf(format, args...), Bad: true})
	r.areas[area] = Failed
	r.mu.Unlock()
	r.changed()
	return fmt.Errorf(format, args...)
}

func (r *Run) set(area int, s Status) {
	r.mu.Lock()
	r.areas[area] = s
	r.mu.Unlock()
	r.changed()
}

func (r *Run) spend(u model.Usage) {
	r.mu.Lock()
	r.spent = r.spent.Add(u)
	r.mu.Unlock()
}

// cost records what one piece of work consumed, against the area and unit it
// was for: in the change's folder where it has one, and kept until it does.
func (r *Run) cost(c artifact.Cost, u model.Usage, elapsed time.Duration) {
	c.At = time.Now().Format(time.RFC3339)
	c.Provenance = "derived"
	c.Model = u.Model
	c.Input, c.CacheRead, c.CacheWrite, c.Output, c.CostUSD = u.Input, u.CacheRead, u.CacheWrite, u.Output, u.CostUSD
	c.ElapsedMS = elapsed.Milliseconds()
	r.mu.Lock()
	r.costs = append(r.costs, c)
	dir := r.dir
	pending := append(r.unkept, c)
	r.unkept = nil
	if dir == "" {
		r.unkept = pending
	}
	r.mu.Unlock()
	if dir != "" {
		for _, p := range pending {
			_ = artifact.AppendCost(dir, p)
		}
	}
	r.changed()
}

// keepCosts writes what was recorded before the change had a folder.
func (r *Run) keepCosts() {
	r.mu.Lock()
	pending, dir := r.unkept, r.dir
	r.unkept = nil
	r.mu.Unlock()
	for _, p := range pending {
		_ = artifact.AppendCost(dir, p)
	}
}

// ask waits for the person.
func (r *Run) ask(ctx context.Context, area int, question string) (decision, error) {
	r.mu.Lock()
	r.question = question
	r.areas[area] = Waiting
	r.mu.Unlock()
	r.changed()
	defer func() {
		r.mu.Lock()
		r.question = ""
		r.mu.Unlock()
		r.changed()
	}()
	select {
	case d := <-r.answer:
		return d, nil
	case <-ctx.Done():
		return decision{}, ctx.Err()
	}
}

// Start carries the task along the path, and returns when it has landed or
// stopped. What stopped it is the last step.
func (r *Run) Start(ctx context.Context) error {
	defer func() {
		r.mu.Lock()
		r.finished = true
		r.mu.Unlock()
		r.changed()
	}()
	for _, area := range []func(context.Context) error{r.intent, r.plan, r.admit, r.execute, r.assure, r.land} {
		if err := area(ctx); err != nil {
			return err
		}
	}
	return nil
}

func today() string { return time.Now().Format("2006-01-02") }

var intentID = regexp.MustCompile(`\bINT-(\d{3})\b`)

// nextNumber is one more than the highest intent number any change of this
// estate carries.
func nextNumber(repo string) int {
	n := 0
	entries, _ := os.ReadDir(filepath.Join(repo, "changes"))
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(repo, "changes", e.Name(), "00-intent.md"))
		if err != nil {
			continue
		}
		for _, m := range intentID.FindAllStringSubmatch(string(b), -1) {
			if v, _ := strconv.Atoi(m[1]); v > n {
				n = v
			}
		}
	}
	return n + 1
}

func (r *Run) id(prefix string, parts ...int) string {
	s := fmt.Sprintf("%s-%03d", prefix, r.n)
	for _, p := range parts {
		s += fmt.Sprintf("-%02d", p)
	}
	return s
}

// check runs the gate over the change as it stands, at the boundary of an
// area. A rule judging a later area's output is not asked yet: an evidence
// plan is owed at admission, and its absence before then is not a failure.
func (r *Run) check(ctx context.Context, area int) (gate.Report, error) {
	c, err := artifact.Load(r.dir)
	if err != nil {
		r.mu.Lock()
		r.steps = append(r.steps, Step{At: time.Now(), Area: area, Text: "gate: the change could not be read: " + err.Error(), Bad: true})
		r.mu.Unlock()
		r.changed()
		return gate.Report{}, err
	}
	full := r.cfg.Gate.Check(ctx, c)
	rep := gate.Report{Change: full.Change}
	for _, f := range full.Findings {
		if owner, ok := gate.AreaOf(f.Rule); ok && owner > area && f.Severity == gate.Fail {
			continue
		}
		rep.Findings = append(rep.Findings, f)
	}
	for _, f := range rep.Findings {
		r.mu.Lock()
		r.steps = append(r.steps, Step{At: time.Now(), Area: area, Text: "gate: " + f.String(), Bad: f.Severity == gate.Fail})
		r.mu.Unlock()
	}
	if rep.Passed() {
		r.say(area, "", "gate: the boundary holds")
	}
	r.changed()
	return rep, nil
}

func (r *Run) record(stage, object, note string) error {
	return artifact.Record{Name: r.slug, Dir: r.dir}.Append(artifact.Crossing{At: today(), Stage: stage, Object: object, Note: note})
}

func joinIDs(ids []string) string { return strings.Join(ids, ", ") }
