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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
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
	// Estate and Gate read and judge the estate; where left out, the
	// estate on disk and the gate over it.
	Estate *estate.Local
	Gate   *gate.Gate
	// Person is who accepts criteria and is named where a person acted.
	Person string
	// BudgetUSD bounds each unit's execution.
	BudgetUSD float64
	// PackTokens bounds what one agent is handed — its goal, the tests it
	// must pass, its files and what surrounds them — and so how large a
	// unit the planner may make; 16000 when zero.
	PackTokens int
	// ReviewRounds, Attempts and RepeatOnce are described at reviewRounds.
	ReviewRounds int
	Attempts     int
	RepeatOnce   bool
}

func (c Config) reviewer() model.Model { return orModel(c.Reviewer, c.Model) }
func (c Config) planner() model.Model  { return orModel(c.Planner, c.Model) }

func orModel(m, otherwise model.Model) model.Model {
	if m != nil {
		return m
	}
	return otherwise
}

// Rounds of review, attempts of the first rung, and how a claim to repeat
// is tested at landing: what the bench varies to learn what each is worth.
// Zero is the default.
//
//	ReviewRounds  how often a specification or a plan is reviewed and
//	              written again before the person or the executor sees it; 2
//	Attempts      how many one-shot answers a unit gets before its agent; 2
//	RepeatOnce    a claim to repeat is run once more at landing and set
//	              against the output its evidence kept, not run twice

func (c Config) reviewRounds() int {
	if c.ReviewRounds > 0 {
		return c.ReviewRounds
	}
	return reviewRounds
}

func (c Config) packBudget() int {
	if c.PackTokens > 0 {
		return c.PackTokens
	}
	return 16000
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
	// settles are escalations this task is meant to close.
	settles []string
	// handovers counts what agents were handed; unkeptHanded is what was
	// handed before the change had a folder, as path and text.
	handovers    int
	unkeptHanded [][2]string
	// reviewLeft is what the last review of the specification still found,
	// which the person is shown beside it.
	reviewLeft review
	units      []unit
	cands      map[string]string // unit → candidate folder
	// again is, for a tests unit carried again, what the code that could
	// not pass its tests found wrong with them.
	again map[string]string
	// from is the area a reopened change is carried again from, 0 for a new
	// one; reopened is what its author or planner is handed of why.
	from     int
	reopened string
	// resumed says the specification already written is put to the person
	// as it is, not written again; replanned that the plan was made again
	// once because a unit could not be done as planned.
	resumed, replanned bool
	// refusals is how many times the person did not affirm the finished
	// change, and mend what they said the last time.
	refusals int
	mend     string
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
	// A configuration that leaves the estate or the gate out reads the
	// estate on disk and judges with the gate over it: the zero value works.
	if cfg.Estate == nil {
		cfg.Estate = estate.NewLocal()
	}
	if cfg.Gate == nil {
		cfg.Gate = gate.New(cfg.Estate)
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

// Settles names escalations this task is meant to close: where every
// criterion is met, its assurance records them closed, and the ledger counts
// them settled. Called before Start.
func (r *Run) Settles(refs ...string) { r.settles = append(r.settles, refs...) }

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

// keepCosts writes what was recorded before the change had a folder, and
// what was handed before it had one.
func (r *Run) keepCosts() {
	r.mu.Lock()
	pending, dir, texts := r.unkept, r.dir, r.unkeptHanded
	r.unkept, r.unkeptHanded = nil, nil
	r.mu.Unlock()
	for _, p := range pending {
		_ = artifact.AppendCost(dir, p)
	}
	for _, t := range texts {
		writeHanded(dir, t[0], t[1])
	}
}

// HandedDir is where what each agent was handed is kept, one file a
// handover, beside the change.
const HandedDir = "handed"

// handed keeps what one agent was handed — its instructions and its prompt,
// exactly — and says where, for the cost record to cite. Before the change
// has a folder the text waits for one.
func (r *Run) handed(what, system, prompt string) (rel string, tokens int) {
	text := "# Instructions\n\n" + system + "\n\n# Handed\n\n" + prompt
	r.mu.Lock()
	r.handovers++
	rel = fmt.Sprintf("%s/%02d-%s.md", HandedDir, r.handovers, slugRE.ReplaceAllString(strings.ToLower(what), "-"))
	dir := r.dir
	if dir == "" {
		r.unkeptHanded = append(r.unkeptHanded, [2]string{rel, text})
	}
	r.mu.Unlock()
	if dir != "" {
		writeHanded(dir, rel, text)
	}
	return rel, estate.Tokens(system + prompt)
}

func writeHanded(dir, rel, text string) {
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if os.MkdirAll(filepath.Dir(p), 0o755) == nil {
		_ = os.WriteFile(p, []byte(text), 0o644)
	}
}

// ask waits for the person.
func (r *Run) ask(ctx context.Context, area int, question string) (decision, error) {
	// An answer given twice to the question before — a second click — is
	// not an answer to this one.
	select {
	case <-r.answer:
	default:
	}
	r.mu.Lock()
	r.question = question
	r.areas[area] = Waiting
	r.mu.Unlock()
	r.changed()
	defer func() {
		r.mu.Lock()
		r.question = ""
		if r.areas[area] == Waiting {
			r.areas[area] = Working // answered: the area goes on
		}
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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go r.watchStop(ctx, cancel)
	defer func() {
		r.mu.Lock()
		r.finished = true
		dir := r.dir
		r.mu.Unlock()
		if dir != "" {
			_ = os.Remove(filepath.Join(dir, RunningFile))
		}
		r.changed()
	}()
	if r.from > 0 {
		_ = os.WriteFile(filepath.Join(r.dir, RunningFile), []byte(fmt.Sprint(os.Getpid())), 0o644)
	}
	areas := []func(context.Context) error{r.intent, r.plan, r.admit, r.execute, r.assure, r.land}
	for i := 0; i < len(areas); i++ {
		if i+1 < r.from {
			r.set(i+1, Done) // carried before, and kept: the change is reopened after it
			continue
		}
		err := areas[i](ctx)
		var rp *replan
		if errors.As(err, &rp) && !r.replanned {
			// A unit cannot be done as planned: the plan is made again, once,
			// with why, and what the areas from the plan on wrote is kept
			// under superseded/.
			if err = r.replanFrom(rp); err == nil {
				i = 0 // the plan, next
				continue
			}
		}
		var rf *refused
		if errors.As(err, &rf) {
			// The person did not affirm the finished change: it is carried
			// again from admission with what they said, and what the areas
			// from admission on wrote is kept under superseded/.
			if err = r.mendFrom(rf); err == nil {
				i = 1 // the admission, next
				continue
			}
		}
		var h *holds
		if errors.As(err, &h) {
			// Nothing to change: the change is closed saying why, and that
			// is an answer, not a failure — unless the tests miss what was
			// meant, which reopening it says.
			_ = r.record("closed", r.id("INT"), "nothing to change: what was asked holds already — "+h.Error()+"; if it does not, reopen it saying what the tests miss")
			r.say(i+1, r.id("INT"), "what was asked holds already: %s", h.Error())
			r.set(i+1, Done)
			return ErrHolds
		}
		if err != nil {
			r.stopped(i+1, err)
			return err
		}
	}
	return nil
}

// ErrHolds is a run that found what was asked true already: its tests,
// written first, pass on the estate as it stands. The change is closed
// saying so; nothing was changed.
var ErrHolds = errors.New("what was asked holds already; nothing was changed")

// StopFile asks the run carrying a change to stop, from wherever the run
// was started: it says who asked.
const StopFile = ".stop"

// Stop asks the run carrying the change in dir to stop; the run stops within
// a second, between steps, and its record says who stopped it. Resume
// carries it on from there.
func Stop(dir, person string) error {
	if !Running(dir) {
		return errors.New("nothing is carrying the change now")
	}
	return os.WriteFile(filepath.Join(dir, StopFile), []byte(person), 0o644)
}

// mendNote begins the crossing that carries a change again with what the
// person said; a run resumed before it was admitted again reads it back.
const mendNote = "carried again: the person did not affirm the change: "

// mendFrom carries the change again from admission, with what the person
// said when they did not affirm it.
func (r *Run) mendFrom(rf *refused) error {
	r.refusals++
	if err := supersedeFrom(r.dir, 3); err != nil {
		return err
	}
	_ = r.record("execution", r.id("SPEC"), mendNote+rf.why)
	r.say(5, r.id("SPEC"), "carried again from admission, with what %s said", r.cfg.Person)
	r.from, r.mend, r.cands, r.again = 3, rf.why, map[string]string{}, nil
	for n := 3; n <= 6; n++ {
		r.set(n, Idle)
	}
	return nil
}

// replanFrom sends the change back to its planner with why a unit could
// not be done.
func (r *Run) replanFrom(rp *replan) error {
	r.replanned = true
	if err := supersedeFrom(r.dir, 2); err != nil {
		return err
	}
	_ = r.record("plan", r.id("SPEC"), "sent back to the planner: "+rp.Error())
	r.from, r.units, r.cands = 2, nil, map[string]string{}
	r.reopened = "\n\n# The plan before could not be carried out\n\nThe agent of " + rp.unit + " said it cannot be done as it was planned:\n\n" + rp.why +
		"\n\nPlan again, whole, so that no unit is asked for what cannot be done: what a unit relies on must exist or be provided by an earlier unit, and what it must change must be in its scope."
	for n := 3; n <= 6; n++ {
		r.set(n, Idle)
	}
	return nil
}

// stopped records that the run stopped in an area, and why — unless a
// person stopping it said so already — so that the record of a change that
// stopped says where and why, and not only how far it got.
func (r *Run) stopped(area int, err error) {
	r.mu.Lock()
	dir := r.dir
	r.mu.Unlock()
	if dir == "" {
		return
	}
	if rec, _ := artifact.ReadRecord(dir); len(rec) > 0 && rec[len(rec)-1].Stage == "stopped" {
		return
	}
	_ = r.record("stopped", r.id("INT"), fmt.Sprintf("stopped in area %d: %v; it can be resumed from where it stopped", area, err))
}

// watchStop stops the run when asked to through StopFile.
func (r *Run) watchStop(ctx context.Context, cancel context.CancelFunc) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		r.mu.Lock()
		dir := r.dir
		r.mu.Unlock()
		if dir == "" {
			continue
		}
		who, err := os.ReadFile(filepath.Join(dir, StopFile))
		if err != nil {
			continue
		}
		_ = os.Remove(filepath.Join(dir, StopFile))
		_ = r.record("stopped", r.id("INT"), "stopped by "+strings.TrimSpace(string(who))+"; it can be resumed from where it stopped")
		cancel()
		return
	}
}

// RunningFile marks a change a run is carrying now, wherever that run was
// started — the workbench, a terminal, another window — with its process:
// what is in it is not a stopped task but one at work.
const RunningFile = ".running"

// Running reports whether a live process is carrying the change in dir.
func Running(dir string) bool {
	b, err := os.ReadFile(filepath.Join(dir, RunningFile))
	if err != nil {
		return false
	}
	var pid int
	if _, err := fmt.Sscan(string(b), &pid); err != nil || pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	return err == nil && p.Signal(syscall.Signal(0)) == nil
}

func today() string { return time.Now().Format("2006-01-02") }

var intentID = regexp.MustCompile(`\bINT-(\d{3})\b`)

// nextNumber is one more than the highest intent number any change of this
// estate carries.
func nextNumber(repo string) int {
	// Runs started side by side would each see the same highest number; a
	// number is taken by creating its reservation, which only one can do.
	dir := filepath.Join(repo, "changes")
	_ = os.MkdirAll(dir, 0o755)
	for n := highestNumber(repo) + 1; ; n++ {
		f, err := os.OpenFile(filepath.Join(dir, fmt.Sprintf(".taken-%03d", n)), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			f.Close()
			return n
		}
		if !os.IsExist(err) {
			return n
		}
	}
}

// highestNumber is the largest intent number any change here has used or
// reserved.
func highestNumber(repo string) int {
	n := 0
	taken, _ := filepath.Glob(filepath.Join(repo, "changes", ".taken-*"))
	for _, t := range taken {
		if v, err := strconv.Atoi(strings.TrimPrefix(filepath.Base(t), ".taken-")); err == nil && v > n {
			n = v
		}
	}
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
	return n
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
