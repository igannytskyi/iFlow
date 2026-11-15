package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// What is written here is written in the columns the templates declare and
// no others: the templates are the schema, and a column invented by a writer
// is one the gate does not read.

// table renders one markdown table.
func table(headers []string, rows [][]string) string {
	var b strings.Builder
	b.WriteString("| " + strings.Join(headers, " | ") + " |\n|")
	for range headers {
		b.WriteString("---|")
	}
	b.WriteString("\n")
	for _, r := range rows {
		cells := make([]string, len(headers))
		for i := range headers {
			if i < len(r) {
				cells[i] = cell(r[i])
			}
		}
		b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
	}
	return b.String()
}

// cell makes a value safe inside a table: one line, no bare separators.
func cell(s string) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n", " ")
	return strings.ReplaceAll(s, "|", "\\|")
}

func fields(pairs ...string) string {
	var rows [][]string
	for i := 0; i+1 < len(pairs); i += 2 {
		rows = append(rows, []string{pairs[i], pairs[i+1]})
	}
	return table([]string{"Field", "Value"}, rows)
}

// IntentDoc is 00-intent.md.
type IntentDoc struct {
	Name, ID, StatedBy, Authority, Priority, Deadline, Statement, Status string
}

// Markdown renders the intent.
func (d IntentDoc) Markdown() string {
	return "# Intent — " + d.Name + "\n\n" + fields(
		"Id", d.ID, "Stated by", d.StatedBy, "Deciding authority", d.Authority,
		"Priority", d.Priority, "Deadline", d.Deadline, "Statement", d.Statement, "Status", d.Status)
}

// CriterionRow is one acceptance criterion.
type CriterionRow struct {
	ID, Criterion, When, Then, Plan, Procedure, Evidence, Threshold string
}

// ArbiterRow is one path holding what judges the change.
type ArbiterRow struct{ Path, Arbitrates string }

// SpecDoc is 01-specification.md.
type SpecDoc struct {
	Name, ID, Intent, Class, Prose string
	// Change is the change described for a reader: what it is for, what a
	// person sees it do, how it is built, what it leaves out, what could go
	// wrong. The tables below say how it is judged; this says what it is.
	Change             []Part
	Criteria           []CriterionRow
	Termination        [][2]string // condition, action
	Included, Excluded []string
	Arbiter            []ArbiterRow
	Inputs             [][2]string // input, in scope?
}

// Markdown renders the specification.
func (d SpecDoc) Markdown() string {
	var b strings.Builder
	b.WriteString("# Specification — " + d.Name + "\n\n")
	b.WriteString(fields("Id", d.ID, "Intent", d.Intent, "Default class", d.Class))
	if d.Prose != "" {
		b.WriteString("\n" + strings.TrimSpace(d.Prose) + "\n")
	}
	if len(d.Change) > 0 {
		b.WriteString("\n## The change\n")
		for _, p := range d.Change {
			b.WriteString(p.markdown())
		}
	}
	var crit [][]string
	for _, c := range d.Criteria {
		crit = append(crit, []string{c.ID, c.Criterion, c.When, c.Then, c.Plan, c.Procedure, c.Evidence, c.Threshold})
	}
	b.WriteString("\n## Acceptance criteria\n\n" + table([]string{"Id", "Criterion", "When", "Then", "Plan", "Procedure", "Required evidence", "Threshold"}, crit))
	var term [][]string
	for _, t := range d.Termination {
		term = append(term, []string{t[0], t[1]})
	}
	b.WriteString("\n## Termination\n\n" + table([]string{"Condition", "Action"}, term))
	var scope [][]string
	for i := range max(len(d.Included), len(d.Excluded)) {
		row := []string{"", ""}
		if i < len(d.Included) {
			row[0] = d.Included[i]
		}
		if i < len(d.Excluded) {
			row[1] = d.Excluded[i]
		}
		scope = append(scope, row)
	}
	b.WriteString("\n## Scope\n\n" + table([]string{"Included", "Excluded"}, scope))
	var arb [][]string
	for _, a := range d.Arbiter {
		arb = append(arb, []string{a.Path, a.Arbitrates})
	}
	b.WriteString("\n## Arbiter\n\n" + table([]string{"Path", "What it arbitrates"}, arb))
	if len(d.Inputs) > 0 {
		var in [][]string
		for _, i := range d.Inputs {
			in = append(in, []string{i[0], i[1]})
		}
		b.WriteString("\n## Arbiter inputs\n\n" + table([]string{"Input", "In scope?"}, in))
	}
	return b.String()
}

// ChangeFile is where the change described is kept beside its
// specification: the document a person reads and changes, while
// 01-specification.md is the method's record of how the change is judged.
const ChangeFile = "change.md"

// ChangeDoc is the change described.
type ChangeDoc struct {
	Name, Statement, Summary string
	Parts                    []Part
}

// Markdown renders the change described.
func (d ChangeDoc) Markdown() string {
	var b strings.Builder
	b.WriteString("# Specification — " + d.Name + "\n\n## Intent\n\n" + strings.TrimSpace(d.Statement) + "\n")
	if s := strings.TrimSpace(d.Summary); s != "" && s != strings.TrimSpace(d.Statement) {
		b.WriteString("\n" + s + "\n")
	}
	for _, p := range d.Parts {
		// A document's sections are its second level; what a section with no
		// words of its own heads is the third.
		if p.Level == 0 {
			p.Level = 2
		}
		b.WriteString(p.markdown())
	}
	return b.String()
}

// Part is one titled part of a document's prose: a paragraph, points, or
// both.
type Part struct {
	Title  string
	Text   string
	Points []string
	// Level is the heading's depth: 2 for a section, 3 (the default) for a
	// part of one. A level-2 part with nothing in it heads those after it.
	Level int
}

func (p Part) markdown() string {
	level := p.Level
	if level == 0 {
		level = 3
	}
	if level == 2 && strings.TrimSpace(p.Text) == "" && len(p.Points) == 0 {
		return "\n## " + p.Title + "\n"
	}
	if strings.TrimSpace(p.Text) == "" && len(p.Points) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n" + strings.Repeat("#", level) + " " + p.Title + "\n\n")
	if t := strings.TrimSpace(p.Text); t != "" {
		b.WriteString(t + "\n")
		if len(p.Points) > 0 {
			b.WriteString("\n")
		}
	}
	for _, pt := range p.Points {
		b.WriteString("- " + strings.ReplaceAll(strings.TrimSpace(pt), "\n", " ") + "\n")
	}
	return b.String()
}

// UnitRow is one work unit.
type UnitRow struct {
	ID, Phase, Class, Scope, AreaOfEffect, Criteria, DependsOn, PreparatoryFor string
}

// PlanDoc is 02-plan.md.
type PlanDoc struct {
	Name, Spec, Phases, Intermediate, Residue, Coverage string
	Units                                               []UnitRow
	// Approach is how the change is built, as a whole, before it is divided.
	Approach []Part
	// Briefs are the units described, each whole enough for an agent that
	// sees nothing else of the change to carry it out.
	Briefs []Part
	// Planner is who divided the work, and how.
	Planner string
}

// Markdown renders the plan.
func (d PlanDoc) Markdown() string {
	var units [][]string
	for _, u := range d.Units {
		units = append(units, []string{u.ID, u.Phase, u.Class, u.Scope, u.AreaOfEffect, u.Criteria, u.DependsOn, u.PreparatoryFor})
	}
	var b strings.Builder
	b.WriteString("# Plan — " + d.Name + "\n\n" +
		fields("Specification", d.Spec, "Phases", d.Phases, "Intermediate states valid", d.Intermediate, "Residue on abandonment", d.Residue))
	if d.Planner != "" {
		b.WriteString("\nPlanned by " + d.Planner + ".\n")
	}
	b.WriteString("\n## Work units\n\n" + table([]string{"Id", "Phase", "Class", "Scope", "Area of effect", "Criteria", "Depends on", "Preparatory for"}, units))
	b.WriteString("\n## Coverage\n\n" + strings.TrimSpace(d.Coverage) + "\n")
	return b.String()
}

// PlanFile is where the plan described is kept beside the plan's record: how
// the change is built and every unit as a ticket, while 02-plan.md records
// the units the method checks.
const PlanFile = "plan.md"

// UnitsDir holds one ticket a unit: all its agent is told of the change.
const UnitsDir = "units"

// Document is the described half of the plan, for PlanFile.
func (d PlanDoc) Document() string {
	var b strings.Builder
	b.WriteString("# The plan — " + d.Name + "\n")
	if d.Planner != "" {
		b.WriteString("\nPlanned by " + d.Planner + ".\n")
	}
	if len(d.Approach) > 0 {
		b.WriteString("\n## Approach\n")
		for _, p := range d.Approach {
			b.WriteString(p.markdown())
		}
	}
	if len(d.Briefs) > 0 {
		b.WriteString("\n## The units\n")
		for _, p := range d.Briefs {
			b.WriteString(p.markdown())
		}
	}
	return b.String()
}

// AdmissionDoc is 03-admission.md.
type AdmissionDoc struct {
	Name, Digest string
	Review       [4]string   // reviewed, by, at, note
	Decisions    [][4]string // unit, outcome, deciding condition, confidence
	Conflicts    [][4]string // units, region, evidence invalidated, resolution
	EvidencePlan [][6]string // criterion, observed, method, unchanged means, derived from, fixed at
	Captured     [][3]string // criterion, artefact, captured to
}

func rows[T ~[2]string | ~[3]string | ~[4]string | ~[6]string](in []T) [][]string {
	out := make([][]string, len(in))
	for i, r := range in {
		switch v := any(r).(type) {
		case [2]string:
			out[i] = v[:]
		case [3]string:
			out[i] = v[:]
		case [4]string:
			out[i] = v[:]
		case [6]string:
			out[i] = v[:]
		}
	}
	return out
}

// Markdown renders the admission.
func (d AdmissionDoc) Markdown() string {
	return "# Admission — " + d.Name + "\n\n" + fields("Specification digest", d.Digest) +
		"\n## Criteria review\n\n" + table([]string{"Reviewed before admission", "By", "At", "Note"}, [][]string{d.Review[:]}) +
		"\n## Decisions\n\n" + table([]string{"Unit", "Outcome", "Deciding condition", "Confidence of the edges relied on"}, rows(d.Decisions)) +
		"\n## Conflicts\n\n" + table([]string{"Units", "Intersecting region", "Evidence it would invalidate", "Resolution"}, rows(d.Conflicts)) +
		"\n## Evidence plan\n\n" + table([]string{"Criterion", "Observed", "Method", "Unchanged means", "Derived from", "Fixed at"}, rows(d.EvidencePlan)) +
		"\n## Prior state captured\n\n" + table([]string{"Criterion", "Artefact", "Captured to"}, rows(d.Captured))
}

// RunRow is one unit's execution; CandidateRow one candidate.
type (
	RunRow       struct{ Unit, Executor, Version, Terminal, Candidate string }
	CandidateRow struct{ ID, Unit, StoredAt, Artefacts, ProducedAt string }
)

// ExecutionDoc is 04-execution.md.
type ExecutionDoc struct {
	Name       string
	Runs       []RunRow
	Candidates []CandidateRow
	Prose      string
}

// Markdown renders the execution.
func (d ExecutionDoc) Markdown() string {
	var runs, cands [][]string
	for _, r := range d.Runs {
		runs = append(runs, []string{r.Unit, r.Executor, r.Version, r.Terminal, r.Candidate})
	}
	for _, c := range d.Candidates {
		cands = append(cands, []string{c.ID, c.Unit, c.StoredAt, c.Artefacts, c.ProducedAt})
	}
	out := "# Execution — " + d.Name + "\n\n" + table([]string{"Unit", "Executor", "Version", "Terminal state", "Candidate"}, runs) +
		"\n## Candidates\n\n" + table([]string{"Id", "Unit", "Stored at", "Artefacts", "Produced at"}, cands)
	if d.Prose != "" {
		out += "\n" + strings.TrimSpace(d.Prose) + "\n"
	}
	return out
}

// EvidenceRow and VerdictRow are assurance's two tables.
type (
	EvidenceRow struct {
		ID, Subject, Kind, Producer, Independence, How, Repeatable, ObtainedAt, ValidUntil string
	}
	VerdictRow struct {
		ID, Unit, Criterion, Outcome, Evidence, State, Window, Baseline string
	}
	EscalationRow struct{ N, Standing, Ground, Against, To string }
)

// AssuranceDoc is 05-assurance.md.
type AssuranceDoc struct {
	Name        string
	Evidence    []EvidenceRow
	Verdicts    []VerdictRow
	Escalations []EscalationRow
	Refusals    [][6]string // id, object, boundary, refused by, ground, at
	// Settled are escalations other changes raised that this one closes:
	// escalation, resolved by, note.
	Settled [][3]string
}

// Markdown renders the assurance.
func (d AssuranceDoc) Markdown() string {
	var ev, ve, es [][]string
	for _, e := range d.Evidence {
		ev = append(ev, []string{e.ID, e.Subject, e.Kind, e.Producer, e.Independence, e.How, e.Repeatable, e.ObtainedAt, e.ValidUntil})
	}
	for _, v := range d.Verdicts {
		ve = append(ve, []string{v.ID, v.Unit, v.Criterion, v.Outcome, v.Evidence, v.State, v.Window, v.Baseline})
	}
	for _, e := range d.Escalations {
		es = append(es, []string{e.N, e.Standing, e.Ground, e.Against, e.To})
	}
	out := "# Assurance — " + d.Name + "\n\n## Evidence\n\n" +
		table([]string{"Id", "Subject", "Kind", "Producer", "Independence", "How established", "Repeatable", "Obtained at", "Valid until"}, ev) +
		"\n## Verdicts\n\n" + table([]string{"Id", "Unit", "Criterion", "Outcome", "Evidence", "State", "Window", "Baseline"}, ve)
	if len(es) > 0 {
		out += "\n## Escalations\n\n" + table([]string{"#", "Standing", "Ground", "Raised against", "To"}, es)
	}
	if len(d.Refusals) > 0 {
		out += "\n## Refusals\n\n" + table([]string{"Id", "Object", "Boundary", "Refused by", "Ground", "At"}, rows(d.Refusals))
	}
	if len(d.Settled) > 0 {
		out += "\n## Escalations closed\n\n" + table([]string{"Escalation", "Resolved by", "Note"}, rows(d.Settled))
	}
	return out
}

// EntryRow is one landing entry.
type EntryRow struct {
	Order, Unit, VerdictState, StillValid, EnteredAt, Invalidated, Reestablished string
}

// LandingDoc is 06-landing.md.
type LandingDoc struct {
	Name    string
	Entries []EntryRow
}

// Markdown renders the landing.
func (d LandingDoc) Markdown() string {
	var en [][]string
	for _, e := range d.Entries {
		en = append(en, []string{e.Order, e.Unit, e.VerdictState, e.StillValid, e.EnteredAt, e.Invalidated, e.Reestablished})
	}
	return "# Landing — " + d.Name + "\n\n" +
		table([]string{"Order", "Unit", "Verdict state", "Evidence still valid", "Entered at", "Invalidated by this entry", "Re-established before next"}, en) +
		"\n## Unplanned state\n\n" + table([]string{"Occurred", "Area of effect halted", "Resolved by"}, [][]string{{"no", "", ""}})
}

// Crossing is one line of record.md.
type Crossing struct{ At, Stage, Object, Note string }

// Record is record.md, appended to and never rewritten.
type Record struct {
	Name string
	Dir  string
}

// Append adds crossings to the record, writing its header the first time.
// Ordinals continue from the last line, whatever is already there.
func (r Record) Append(lines ...Crossing) error {
	p := filepath.Join(r.Dir, "record.md")
	n := 0
	// A line is written in the columns the record already has: an older
	// record keeps a Digest column a newer one does not, and a line written
	// in the wrong shape puts its note under the wrong heading.
	columns := []string{"#", "At", "Stage", "Object", "Note"}
	if b, err := os.ReadFile(p); err == nil {
		doc := ParseDoc(string(b))
		n = len(doc.Rows("#", "Stage"))
		for _, t := range doc.All() {
			if slices.Contains(t.Headers, "Stage") && slices.Contains(t.Headers, "#") {
				columns = t.Headers
			}
		}
	} else {
		head := "# Record — " + r.Name + "\n\nAppend-only. Each crossing of a stage boundary is one line.\n\n| # | At | Stage | Object | Note |\n|---|---|---|---|---|\n"
		if err := os.WriteFile(p, []byte(head), 0o644); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, l := range lines {
		n++
		value := map[string]string{"#": fmt.Sprint(n), "At": l.At, "Stage": l.Stage, "Object": l.Object, "Note": l.Note}
		cells := make([]string, len(columns))
		for i, c := range columns {
			cells[i] = cell(value[c])
		}
		if _, err := fmt.Fprintf(f, "| %s |\n", strings.Join(cells, " | ")); err != nil {
			return err
		}
	}
	return nil
}

// Write writes one stage of a change.
func Write(dir string, s Stage, markdown string) error {
	return os.WriteFile(filepath.Join(dir, string(s)+".md"), []byte(markdown), 0o644)
}

// DigestFile is the digest of a file's content, as admission records it.
func DigestFile(p string) (string, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])[:16], nil
}
