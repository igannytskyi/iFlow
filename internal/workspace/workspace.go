// Package workspace is what a viewer of the method needs to know about a set
// of changes: every object each change carries, in which area it was
// produced, what it refers to, and what the gate found about it.
package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/gate"
	"github.com/igannytskyi/iflow/internal/method"
)

// Field is one named value of an item.
type Field struct {
	Name  string
	Value string
}

// Item is one object a change carries: an intent, a criterion, a unit, a
// candidate, a piece of evidence, a verdict, an entry, a crossing.
type Item struct {
	// Kind is the method's name for the object, e.g. "Criterion".
	Kind string
	// ID is the object's identifier, or a name the item is known by.
	ID     string
	Stage  artifact.Stage
	Area   int
	Fields []Field
	// Refs are the identifiers this item cites.
	Refs     []string
	Findings []gate.Finding
}

// Field returns the value of a field, or "".
func (it *Item) Field(name string) string {
	for _, f := range it.Fields {
		if f.Name == name {
			return f.Value
		}
	}
	return ""
}

// Title is the line an item is shown by.
func (it *Item) Title() string {
	for _, k := range []string{"Criterion", "Statement", "Rule", "Condition", "Note", "Ground", "Object"} {
		if v := it.Field(k); v != "" {
			return method.Plain(v)
		}
	}
	return ""
}

// State is how far a change's gate check has got.
type State int

const (
	// Pending means the gate has not answered yet.
	Pending State = iota
	// Checked means the gate has answered.
	Checked
)

// Change is one change folder and everything it carries.
type Change struct {
	Change  *artifact.Change
	Items   []*Item
	Report  gate.Report
	State   State
	General []gate.Finding // findings that name no item
	// Costs are the change's cost records, and Spent their sum.
	Costs []artifact.Cost
	Spent artifact.Cost
}

// Name is the change's slug.
func (c *Change) Name() string { return c.Change.Name() }

// Item returns the item with the given id, or nil.
func (c *Change) Item(id string) *Item {
	for _, it := range c.Items {
		if it.ID == id {
			return it
		}
	}
	return nil
}

// Reached is the furthest path area the change has a stage in.
func (c *Change) Reached() int {
	n := 0
	for _, s := range c.Change.Present() {
		n = max(n, method.StageArea[s])
	}
	return n
}

// InArea returns the items produced in an area.
func (c *Change) InArea(area int) []*Item {
	var out []*Item
	for _, it := range c.Items {
		if it.Area == area {
			out = append(out, it)
		}
	}
	return out
}

func refsOf(fields []Field, self string) []string {
	var out []string
	for _, f := range fields {
		for _, id := range artifact.IDs(f.Value) {
			if id != self {
				out = append(out, id)
			}
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// fields is the named values of an item, the empty ones left out; each is
// named as the method names it, which is what a reader is shown.
func fields(pairs ...string) []Field {
	var out []Field
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i+1] != "" {
			out = append(out, Field{pairs[i], pairs[i+1]})
		}
	}
	return out
}

func list(ids []string) string { return strings.Join(ids, ", ") }

func num(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

// Build makes every item a change carries from its documents in the data
// model.
func Build(c *artifact.Change) *Change {
	wc := &Change{Change: c}
	add := func(kind, id string, stage artifact.Stage, fs []Field) {
		it := &Item{Kind: kind, ID: id, Stage: stage, Area: method.StageArea[stage], Fields: fs}
		it.Refs = refsOf(fs, artifactID(id))
		wc.Items = append(wc.Items, it)
	}
	m := c.Meta
	if d := m.Intent; d != nil {
		add("Intent", or(d.ID, "Intent"), artifact.Intent, fields("Id", d.ID, "Stated by", d.StatedBy, "Deciding authority", d.Authority,
			"Priority", d.Priority, "Deadline", d.Deadline, "Statement", d.Statement, "Status", string(d.Status)))
	}
	if d := m.Specification; d != nil {
		st := artifact.Specification
		add("Specification", or(d.ID, "Specification"), st, fields("Id", d.ID, "Intent", d.Intent, "Default class", string(d.Class)))
		for i, cr := range d.Criteria {
			add("Criterion", or(cr.ID, fmt.Sprintf("Criterion %d", i+1)), st, fields("Id", cr.ID, "Criterion", cr.Criterion, "When", cr.When, "Then", cr.Then,
				"Plan", string(cr.Plan), "Procedure", string(cr.Procedure), "Required evidence", string(cr.Evidence), "Threshold", cr.Threshold))
		}
		for i, t := range d.Termination {
			add("TerminationCondition", or(method.Plain(t.Condition), fmt.Sprintf("TerminationCondition %d", i+1)), st, fields("Condition", t.Condition, "Action", t.Action))
		}
		for _, p := range d.Scope.Included {
			add("Scope", method.Plain(p), st, fields("Included", p))
		}
		for _, a := range d.Arbiters {
			if a.Path != "" {
				add("Arbiter", method.Plain(a.Path), st, fields("Path", a.Path, "What it arbitrates", a.Arbitrates))
			}
		}
	}
	if d := m.Plan; d != nil {
		st := artifact.Plan
		add("ChangePlan", "ChangePlan", st, fields("Specification", d.Specification, "Phases", num(d.Phases),
			"Intermediate states valid", d.IntermediateValid, "Residue on abandonment", d.Residue))
		for i, u := range d.Units {
			add("WorkUnit", or(u.ID, fmt.Sprintf("WorkUnit %d", i+1)), st, fields("Id", u.ID, "Phase", num(u.Phase), "Class", string(u.Class),
				"Scope", list(u.Scope), "Area of effect", u.AreaOfEffect, "Criteria", list(u.Criteria), "Depends on", list(u.DependsOn), "Preparatory for", list(u.PreparatoryFor)))
		}
	}
	if d := m.Admission; d != nil {
		st := artifact.Admission
		for i, v := range d.Reviews {
			add("Review", or(v.By, fmt.Sprintf("Review %d", i+1)), st, fields("Reviewed before admission", string(v.Reviewed), "By", v.By, "At", v.At, "Note", v.Note))
		}
		for i, v := range d.Decisions {
			add("AdmissionDecision", or(v.Unit, fmt.Sprintf("AdmissionDecision %d", i+1)), st, fields("Unit", v.Unit, "Outcome", string(v.Outcome),
				"Deciding condition", v.Condition, "Confidence of the edges relied on", v.Confidence, "Escalates at", v.Escalates))
		}
		for i, v := range d.Conflicts {
			add("Conflict", or(v.Units, fmt.Sprintf("Conflict %d", i+1)), st, fields("Units", v.Units, "Intersecting region", v.Region,
				"Evidence it would invalidate", v.Invalidates, "Resolution", v.Resolution))
		}
		for i, v := range d.Plans {
			add("EvidencePlan", or(v.Criterion, fmt.Sprintf("EvidencePlan %d", i+1)), st, fields("Criterion", v.Criterion, "Observed", v.Observed,
				"Method", v.Method, "Unchanged means", v.Unchanged, "Derived from", v.DerivedFrom, "Fixed at", v.FixedAt))
		}
		for i, v := range d.Prior {
			add("PriorState", or(method.Plain(v.Artefact), fmt.Sprintf("PriorState %d", i+1)), st, fields("Criterion", v.Criterion, "Artefact", v.Artefact, "Captured to", v.CapturedTo))
		}
	}
	if d := m.Execution; d != nil {
		st := artifact.Execution
		for i, v := range d.Runs {
			add("Trace", or(v.Unit, fmt.Sprintf("Trace %d", i+1)), st, fields("Unit", v.Unit, "Executor", v.Executor, "Version", v.Version,
				"Terminal state", string(v.Terminal), "Candidate", v.Candidate, "Trace", v.Trace))
		}
		for i, v := range d.Candidates {
			add("Candidate", or(v.ID, fmt.Sprintf("Candidate %d", i+1)), st, fields("Id", v.ID, "Unit", v.Unit, "Stored at", v.StoredAt,
				"Artefacts", v.Artefacts, "Produced at", v.ProducedAt))
		}
	}
	if d := m.Assurance; d != nil {
		st := artifact.Assurance
		for i, v := range d.Evidence {
			add("Evidence", or(v.ID, fmt.Sprintf("Evidence %d", i+1)), st, fields("Id", v.ID, "Subject", string(v.Subject), "Kind", string(v.Kind),
				"Producer", v.Producer, "Independence", string(v.Independence), "How established", v.How, "Repeatable", string(v.Repeatable),
				"Obtained at", v.ObtainedAt, "Valid until", v.ValidUntil))
		}
		for i, v := range d.Verdicts {
			add("Verdict", or(v.ID, fmt.Sprintf("Verdict %d", i+1)), st, fields("Id", v.ID, "Unit", v.Unit, "Criterion", v.Criterion,
				"Outcome", string(v.Outcome), "Evidence", list(v.Evidence), "State", string(v.State), "Window", v.Window, "Baseline", v.Baseline))
		}
		for _, v := range d.Raised {
			add("Escalation", c.Name()+"#"+num(v.N), st, fields("#", num(v.N), "Standing", v.Standing, "Ground", v.Ground, "Raised against", v.Against, "To", v.To))
		}
		for i, v := range d.Settled {
			add("Settlement", or(v.Escalation, fmt.Sprintf("Settlement %d", i+1)), st, fields("Escalation", v.Escalation, "Resolved by", v.ResolvedBy, "Note", v.Note))
		}
		for i, v := range d.Refusals {
			add("Refusal", or(v.ID, fmt.Sprintf("Refusal %d", i+1)), st, fields("Id", v.ID, "Object", v.Object, "Boundary", v.Boundary,
				"Refused by", string(v.By), "Ground", v.Ground, "At", v.At))
		}
	}
	if d := m.Landing; d != nil {
		for i, v := range d.Entries {
			add("Entry", or(v.Unit, fmt.Sprintf("Entry %d", i+1)), artifact.Landing, fields("Order", num(v.Order), "Unit", v.Unit, "Verdict state", v.VerdictState,
				"Evidence still valid", v.StillValid, "Entered at", v.EnteredAt, "Invalidated by this entry", v.Invalidated, "Re-established before next", v.Reestablished))
		}
	}
	wc.Costs, _ = artifact.Costs(c.Dir)
	wc.Spent = artifact.Total(wc.Costs)
	for i, k := range wc.Costs {
		it := &Item{Kind: "CostRecord", ID: fmt.Sprintf("cost %d", i+1), Area: 9, Fields: costFields(k)}
		wc.Items = append(wc.Items, it)
	}
	for _, r := range m.Record {
		it := &Item{Kind: "Crossing", ID: "record #" + strconv.Itoa(r.N), Area: 8,
			Fields: fields("#", strconv.Itoa(r.N), "At", r.At, "Stage", r.Stage, "Object", r.Object, "Digest", r.Digest, "Note", r.Note)}
		it.Refs = refsOf(it.Fields, "")
		wc.Items = append(wc.Items, it)
	}
	return wc
}

func or(v, otherwise string) string {
	if v == "" {
		return otherwise
	}
	return v
}

func costFields(k artifact.Cost) []Field {
	out := []Field{{"What", k.What}, {"Area", fmt.Sprint(k.Area)}, {"Unit", k.Unit}, {"Actor", k.Actor}, {"Model", k.Model}}
	if k.Handed > 0 {
		out = append(out, Field{"Context handed", fmt.Sprintf("%d tokens", k.Handed)})
	}
	if k.Turns > 0 || k.Steps > 0 {
		out = append(out, Field{"Turns", fmt.Sprint(k.Turns)}, Field{"Steps", fmt.Sprint(k.Steps)})
	}
	if k.In() > 0 || k.Output > 0 {
		out = append(out, Field{"Tokens read", fmt.Sprint(k.Input)}, Field{"Tokens read from cache", fmt.Sprint(k.CacheRead)},
			Field{"Tokens written to cache", fmt.Sprint(k.CacheWrite)}, Field{"Tokens written", fmt.Sprint(k.Output)})
	}
	out = append(out, Field{"Money", fmt.Sprintf("$%.4f", k.CostUSD)}, Field{"Elapsed", fmt.Sprintf("%.1fs", float64(k.ElapsedMS)/1000)},
		Field{"Provenance", k.Provenance}, Field{"Note", k.Note}, Field{"At", k.At})
	return out
}

func artifactID(id string) string {
	if ids := artifact.IDs(id); len(ids) == 1 && ids[0] == id {
		return id
	}
	return ""
}

// attach hangs each finding on the items its message names, or on the change
// when it names none.
func (wc *Change) attach(rep gate.Report) {
	wc.Report = rep
	wc.State = Checked
	wc.General = nil
	for _, it := range wc.Items {
		it.Findings = nil
	}
	for _, f := range rep.Findings {
		named := false
		for _, id := range artifact.IDs(f.Message) {
			// A finding belongs to the object it names where that object is
			// defined, not to every row that mentions it.
			if it := wc.Item(id); it != nil && it.Kind != "Crossing" {
				it.Findings = append(it.Findings, f)
				named = true
			}
		}
		if !named {
			wc.General = append(wc.General, f)
		}
	}
}

// Links returns every pair of items where the first cites the second.
func (wc *Change) Links() [][2]*Item {
	var out [][2]*Item
	for _, it := range wc.Items {
		if it.Kind == "Crossing" {
			continue
		}
		for _, ref := range it.Refs {
			if to := wc.Item(ref); to != nil && to != it {
				out = append(out, [2]*Item{it, to})
			}
		}
	}
	return out
}

// Workspace is every change under a set of directories.
type Workspace struct {
	Model   *method.Model
	Changes []*Change

	mu sync.RWMutex
}

// Folders lists the change folders under each root: a root that is itself a
// change is taken as one, a directory of changes as many.
func Folders(roots ...string) []string {
	var out []string
	for _, root := range roots {
		if _, err := os.Stat(filepath.Join(root, string(artifact.Intent)+".md")); err == nil {
			out = append(out, root)
			continue
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if _, err := os.Stat(filepath.Join(root, e.Name(), string(artifact.Intent)+".md")); e.IsDir() && err == nil {
				out = append(out, filepath.Join(root, e.Name()))
			}
		}
	}
	return out
}

// Open reads every change under the roots. Reading is cheap and immediate;
// the gate is asked separately, by Check, because it may not be.
func Open(model *method.Model, roots ...string) (*Workspace, error) {
	ws := &Workspace{Model: model}
	for _, dir := range Folders(roots...) {
		c, err := artifact.Load(dir)
		if err != nil {
			return nil, err
		}
		ws.Changes = append(ws.Changes, Build(c))
	}
	slices.SortStableFunc(ws.Changes, func(a, b *Change) int { return strings.Compare(a.Name(), b.Name()) })
	return ws, nil
}

// Check asks the gate about every change, a few at a time, and calls done
// after each answer arrives. It returns when every change has been answered
// or ctx ends.
func (ws *Workspace) Check(ctx context.Context, g *gate.Gate, parallel int, done func(*Change)) {
	limit := make(chan struct{}, max(parallel, 1))
	var wg sync.WaitGroup
	for _, c := range ws.Changes {
		wg.Go(func() {
			select {
			case limit <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-limit }()
			rep := g.Check(ctx, c.Change)
			if ctx.Err() != nil {
				return
			}
			ws.mu.Lock()
			c.attach(rep)
			ws.mu.Unlock()
			done(c)
		})
	}
	wg.Wait()
}

// RLock holds the workspace still while it is read.
func (ws *Workspace) RLock() { ws.mu.RLock() }

// RUnlock releases RLock.
func (ws *Workspace) RUnlock() { ws.mu.RUnlock() }

// RuleCount is how many changes each rule currently fails in, and how many
// times it was noted.
type RuleCount struct {
	Failed map[string]int
	Noted  map[string]int
}

// Rules counts the gate's answers across every change checked so far.
func (ws *Workspace) Rules() RuleCount {
	rc := RuleCount{Failed: map[string]int{}, Noted: map[string]int{}}
	for _, c := range ws.Changes {
		if c.State != Checked {
			continue
		}
		type key struct {
			rule string
			sev  gate.Severity
		}
		seen := map[key]bool{}
		for _, f := range c.Report.Findings {
			k := key{f.Rule, f.Severity}
			if seen[k] {
				continue
			}
			seen[k] = true
			if f.Severity == gate.Fail {
				rc.Failed[f.Rule]++
			} else {
				rc.Noted[f.Rule]++
			}
		}
	}
	return rc
}

// ClosedStage is the stage a record names when a person closes a change
// that will not be finished: nothing it produced entered the estate, and
// nothing more will be done under it.
const ClosedStage = "closed"

// Closed reports whether a person closed the change, and what they said.
func (c *Change) Closed() (bool, string) {
	why, ok := gate.Closed(c.Change)
	return ok, why
}
