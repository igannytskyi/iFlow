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

// extraction says which rows of which stage are which object.
type extraction struct {
	stage   artifact.Stage
	headers []string
	kind    string
	id      string // the column naming the item; "" for a Field|Value table
}

var extractions = []extraction{
	{artifact.Intent, []string{"Field", "Value"}, "Intent", ""},
	{artifact.Specification, []string{"Field", "Value"}, "Specification", ""},
	{artifact.Specification, []string{"Criterion", "Procedure"}, "Criterion", "Id"},
	{artifact.Specification, []string{"Condition", "Action"}, "TerminationCondition", "Condition"},
	{artifact.Specification, []string{"Included"}, "Scope", "Included"},
	{artifact.Specification, []string{"Path", "What it arbitrates"}, "Arbiter", "Path"},
	{artifact.Plan, []string{"Field", "Value"}, "ChangePlan", ""},
	{artifact.Plan, []string{"Class", "Preparatory for"}, "WorkUnit", "Id"},
	{artifact.Admission, []string{"Reviewed before admission"}, "Review", "By"},
	{artifact.Admission, []string{"Unit", "Outcome"}, "AdmissionDecision", "Unit"},
	{artifact.Admission, []string{"Units", "Intersecting region"}, "Conflict", "Units"},
	{artifact.Admission, []string{"Observed", "Derived from"}, "EvidencePlan", "Criterion"},
	{artifact.Admission, []string{"Artefact", "Captured to"}, "PriorState", "Artefact"},
	{artifact.Execution, []string{"Terminal state"}, "Trace", "Unit"},
	{artifact.Execution, []string{"Stored at", "Artefacts"}, "Candidate", "Id"},
	{artifact.Assurance, []string{"Independence"}, "Evidence", "Id"},
	{artifact.Assurance, []string{"Outcome", "Evidence"}, "Verdict", "Id"},
	{artifact.Assurance, []string{"Standing", "Ground"}, "Escalation", "#"},
	{artifact.Assurance, []string{"Escalation", "Resolved by"}, "Settlement", "Escalation"},
	{artifact.Assurance, []string{"Refused by", "Ground"}, "Refusal", "Id"},
	{artifact.Landing, []string{"Unit", "Entered at"}, "Entry", "Unit"},
}

func fieldsOf(r artifact.Row, headers []string) []Field {
	var out []Field
	for _, h := range headers {
		if v, ok := r.Get(h); ok {
			out = append(out, Field{h, v})
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

// Build extracts every item a change carries.
func Build(c *artifact.Change) *Change {
	wc := &Change{Change: c}
	for _, ex := range extractions {
		doc := c.Doc(ex.stage)
		for _, t := range doc.All() {
			if !t.Has(ex.headers...) {
				continue
			}
			if ex.id == "" {
				it := &Item{Kind: ex.kind, Stage: ex.stage, Area: method.StageArea[ex.stage]}
				for _, r := range t.Rows {
					if !r.Placeholder() && r.Cell("Field") != "" {
						it.Fields = append(it.Fields, Field{r.Cell("Field"), r.Cell("Value")})
					}
				}
				it.ID = it.Field("Id")
				if it.ID == "" {
					it.ID = ex.kind
				}
				it.Refs = refsOf(it.Fields, it.ID)
				wc.Items = append(wc.Items, it)
				continue
			}
			for i, r := range t.Rows {
				if r.Placeholder() {
					continue
				}
				fields := fieldsOf(r, t.Headers)
				id := method.Plain(r.Cell(ex.id))
				if id == "" && (ex.kind == "Scope" || ex.kind == "Arbiter") {
					continue // a row filled only on the other side of the table
				}
				if id == "" {
					id = ex.kind + " " + strconv.Itoa(i+1)
				}
				if ex.kind == "Escalation" {
					id = c.Name() + "#" + id
				}
				it := &Item{Kind: ex.kind, ID: id, Stage: ex.stage, Area: method.StageArea[ex.stage], Fields: fields}
				it.Refs = refsOf(fields, artifactID(id))
				wc.Items = append(wc.Items, it)
			}
		}
	}
	wc.Costs, _ = artifact.Costs(c.Dir)
	wc.Spent = artifact.Total(wc.Costs)
	for i, k := range wc.Costs {
		it := &Item{Kind: "CostRecord", ID: fmt.Sprintf("cost %d", i+1), Area: 9, Fields: costFields(k)}
		wc.Items = append(wc.Items, it)
	}
	for _, r := range c.Record.Rows("#", "Stage") {
		it := &Item{Kind: "Crossing", ID: "record #" + r.Cell("#"), Area: 8, Fields: fieldsOf(r, []string{"#", "At", "Stage", "Object", "Digest", "Note"})}
		it.Refs = refsOf(it.Fields, "")
		wc.Items = append(wc.Items, it)
	}
	return wc
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
	for _, it := range c.Items {
		if it.Kind == "Crossing" && it.Field("Stage") == ClosedStage {
			return true, it.Field("Note")
		}
	}
	return false, ""
}
