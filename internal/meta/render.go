package meta

import (
	"fmt"
	"strings"
)

// Each document renders as Markdown for a person to read. The Markdown is
// generated from the document and never read back: the JSON is the record.

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

func list(ids []string) string { return strings.Join(ids, ", ") }

func number(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprint(n)
}

func sections(ss []Section) string {
	var b strings.Builder
	for _, s := range ss {
		b.WriteString("\n## " + s.Title + "\n")
		if t := strings.TrimSpace(s.Text); t != "" {
			b.WriteString("\n" + t + "\n")
		}
		for _, t := range s.Tables {
			b.WriteString("\n" + table(t.Headers, t.Rows))
		}
	}
	return b.String()
}

// Markdown renders the intent.
func (d Intent) Markdown() string {
	return "# Intent — " + d.Name + "\n\n" + fields(
		"Id", d.ID, "Stated by", d.StatedBy, "Deciding authority", d.Authority,
		"Priority", d.Priority, "Deadline", d.Deadline, "Statement", d.Statement, "Status", string(d.Status))
}

// Markdown renders the specification.
func (d Specification) Markdown() string {
	var b strings.Builder
	b.WriteString("# Specification — " + d.Name + "\n\n")
	b.WriteString(fields("Id", d.ID, "Intent", d.Intent, "Default class", string(d.Class)))
	if d.Prose != "" {
		b.WriteString("\n" + strings.TrimSpace(d.Prose) + "\n")
	}
	var crit [][]string
	for _, c := range d.Criteria {
		crit = append(crit, []string{c.ID, c.Criterion, c.When, c.Then, string(c.Plan), string(c.Procedure), string(c.Evidence), c.Threshold})
	}
	b.WriteString("\n## Acceptance criteria\n\n" + table([]string{"Id", "Criterion", "When", "Then", "Plan", "Procedure", "Required evidence", "Threshold"}, crit))
	var term [][]string
	for _, t := range d.Termination {
		term = append(term, []string{t.Condition, t.Action})
	}
	b.WriteString("\n## Termination\n\n" + table([]string{"Condition", "Action"}, term))
	var scope [][]string
	for i := range max(len(d.Scope.Included), len(d.Scope.Excluded)) {
		row := []string{"", ""}
		if i < len(d.Scope.Included) {
			row[0] = d.Scope.Included[i]
		}
		if i < len(d.Scope.Excluded) {
			row[1] = d.Scope.Excluded[i]
		}
		scope = append(scope, row)
	}
	b.WriteString("\n## Scope\n\n" + table([]string{"Included", "Excluded"}, scope))
	var arb [][]string
	for _, a := range d.Arbiters {
		arb = append(arb, []string{a.Path, a.Arbitrates})
	}
	b.WriteString("\n## Arbiter\n\n" + table([]string{"Path", "What it arbitrates"}, arb))
	if len(d.Inputs) > 0 {
		var in [][]string
		for _, i := range d.Inputs {
			in = append(in, []string{i.Input, i.InScope})
		}
		b.WriteString("\n## Arbiter inputs\n\n" + table([]string{"Input", "In scope?"}, in))
	}
	b.WriteString(sections(d.Sections))
	return b.String()
}

// Markdown renders the plan's record.
func (d Plan) Markdown() string {
	var units [][]string
	for _, u := range d.Units {
		units = append(units, []string{u.ID, number(u.Phase), string(u.Class), list(u.Scope), u.AreaOfEffect, list(u.Criteria), list(u.DependsOn), list(u.PreparatoryFor)})
	}
	var b strings.Builder
	b.WriteString("# Plan — " + d.Name + "\n\n" +
		fields("Specification", d.Specification, "Phases", number(d.Phases), "Intermediate states valid", d.IntermediateValid, "Residue on abandonment", d.Residue))
	if d.Planner != "" {
		b.WriteString("\nPlanned by " + d.Planner + ".\n")
	}
	b.WriteString("\n## Work units\n\n" + table([]string{"Id", "Phase", "Class", "Scope", "Area of effect", "Criteria", "Depends on", "Preparatory for"}, units))
	b.WriteString("\n## Coverage\n")
	if len(d.Coverage) > 0 {
		var qa [][]string
		for _, a := range d.Coverage {
			qa = append(qa, []string{a.Question, a.Answer})
		}
		b.WriteString("\n" + table([]string{"Question", "Answer"}, qa))
	}
	if t := strings.TrimSpace(d.CoverageText); t != "" {
		b.WriteString("\n" + t + "\n")
	}
	b.WriteString(sections(d.Notes))
	return b.String()
}

// Markdown renders the admission.
func (d Admission) Markdown() string {
	var b strings.Builder
	b.WriteString("# Admission — " + d.Name + "\n\n" + fields("Specification digest", d.Digest))
	if len(d.Reviews) > 0 {
		var r [][]string
		for _, v := range d.Reviews {
			r = append(r, []string{string(v.Reviewed), v.By, v.At, v.Note})
		}
		b.WriteString("\n## Criteria review\n\n" + table([]string{"Reviewed before admission", "By", "At", "Note"}, r))
	}
	var dec [][]string
	escalates := false
	for _, v := range d.Decisions {
		dec = append(dec, []string{v.Unit, string(v.Outcome), v.Condition, v.Confidence, v.Escalates})
		escalates = escalates || v.Escalates != ""
	}
	heads := []string{"Unit", "Outcome", "Deciding condition", "Confidence of the edges relied on"}
	if escalates {
		heads = append(heads, "Escalates at")
	}
	b.WriteString("\n## Decisions\n\n" + table(heads, dec))
	var con [][]string
	for _, v := range d.Conflicts {
		con = append(con, []string{v.Units, v.Region, v.Invalidates, v.Resolution})
	}
	b.WriteString("\n## Conflicts\n\n" + table([]string{"Units", "Intersecting region", "Evidence it would invalidate", "Resolution"}, con))
	if len(d.Plans) > 0 {
		var ep [][]string
		for _, v := range d.Plans {
			ep = append(ep, []string{v.Criterion, v.Observed, v.Method, v.Unchanged, v.DerivedFrom, v.FixedAt})
		}
		b.WriteString("\n## Evidence plan\n\n" + table([]string{"Criterion", "Observed", "Method", "Unchanged means", "Derived from", "Fixed at"}, ep))
	}
	var cap [][]string
	for _, v := range d.Prior {
		cap = append(cap, []string{v.Criterion, v.Artefact, v.CapturedTo})
	}
	b.WriteString("\n## Prior state captured\n\n" + table([]string{"Criterion", "Artefact", "Captured to"}, cap))
	if len(d.Grants) > 0 {
		var gr [][]string
		for _, v := range d.Grants {
			gr = append(gr, []string{v.ID, v.Unit, v.Operations, v.Targets, v.Expires})
		}
		b.WriteString("\n## Grants\n\n" + table([]string{"Id", "Unit", "Permitted operations", "Targets", "Expires"}, gr))
	}
	b.WriteString(sections(d.Notes))
	return b.String()
}

// Markdown renders the execution.
func (d Execution) Markdown() string {
	var runs, cands [][]string
	traced := false
	for _, r := range d.Runs {
		runs = append(runs, []string{r.Unit, r.Executor, r.Version, string(r.Terminal), r.Candidate, r.Trace})
		traced = traced || r.Trace != ""
	}
	heads := []string{"Unit", "Executor", "Version", "Terminal state", "Candidate"}
	if traced {
		heads = append(heads, "Trace")
	}
	for _, c := range d.Candidates {
		cands = append(cands, []string{c.ID, c.Unit, c.StoredAt, c.Artefacts, c.ProducedAt})
	}
	out := "# Execution — " + d.Name + "\n\n" + table(heads, runs) +
		"\n## Candidates\n\n" + table([]string{"Id", "Unit", "Stored at", "Artefacts", "Produced at"}, cands)
	if len(d.Reproductions) > 0 {
		var rp [][]string
		for _, r := range d.Reproductions {
			rp = append(rp, []string{r.Unit, r.Criterion, r.BeforeRepair})
		}
		out += "\n## Reproductions\n\n" + table([]string{"Unit", "Criterion", "Before repair"}, rp)
	}
	if d.Prose != "" {
		out += "\n" + strings.TrimSpace(d.Prose) + "\n"
	}
	return out
}

// Markdown renders the assurance.
func (d Assurance) Markdown() string {
	var ev, ve [][]string
	// Evidence written before the method asked whether a run repeats says
	// nothing of it, and its table has no such column.
	stated := false
	for _, e := range d.Evidence {
		stated = stated || e.Repeatable != ""
	}
	evHeads := []string{"Id", "Subject", "Kind", "Producer", "Independence", "How established", "Repeatable", "Obtained at", "Valid until"}
	for _, e := range d.Evidence {
		row := []string{e.ID, string(e.Subject), string(e.Kind), e.Producer, string(e.Independence), e.How, string(e.Repeatable), e.ObtainedAt, e.ValidUntil}
		if !stated {
			row = append(row[:6:6], row[7:]...)
		}
		ev = append(ev, row)
	}
	if !stated {
		evHeads = append(evHeads[:6:6], evHeads[7:]...)
	}
	for _, v := range d.Verdicts {
		ve = append(ve, []string{v.ID, v.Unit, v.Criterion, string(v.Outcome), list(v.Evidence), string(v.State), v.Window, v.Baseline})
	}
	out := "# Assurance — " + d.Name + "\n\n## Evidence\n\n" +
		table(evHeads, ev) +
		"\n## Verdicts\n\n" + table([]string{"Id", "Unit", "Criterion", "Outcome", "Evidence", "State", "Window", "Baseline"}, ve)
	if len(d.Raised) > 0 {
		var es [][]string
		for _, e := range d.Raised {
			es = append(es, []string{number(e.N), e.Standing, e.Ground, e.Against, e.To})
		}
		out += "\n## Escalations\n\n" + table([]string{"#", "Standing", "Ground", "Raised against", "To"}, es)
	}
	if len(d.Refusals) > 0 {
		var rf [][]string
		for _, r := range d.Refusals {
			rf = append(rf, []string{r.ID, r.Object, r.Boundary, string(r.By), r.Ground, r.At})
		}
		out += "\n## Refusals\n\n" + table([]string{"Id", "Object", "Boundary", "Refused by", "Ground", "At"}, rf)
	}
	if len(d.Settled) > 0 {
		var st [][]string
		for _, s := range d.Settled {
			st = append(st, []string{s.Escalation, s.ResolvedBy, s.Note})
		}
		out += "\n## Escalations closed\n\n" + table([]string{"Escalation", "Resolved by", "Note"}, st)
	}
	return out + sections(d.Sections)
}

// Markdown renders the landing.
func (d Landing) Markdown() string {
	var en [][]string
	for _, e := range d.Entries {
		en = append(en, []string{number(e.Order), e.Unit, e.VerdictState, e.StillValid, e.EnteredAt, e.Invalidated, e.Reestablished})
	}
	var un [][]string
	for _, u := range d.Unplanned {
		un = append(un, []string{u.Occurred, u.Halted, u.ResolvedBy})
	}
	if len(un) == 0 {
		un = [][]string{{"no", "", ""}}
	}
	return "# Landing — " + d.Name + "\n\n" +
		table([]string{"Order", "Unit", "Verdict state", "Evidence still valid", "Entered at", "Invalidated by this entry", "Re-established before next"}, en) +
		"\n## Unplanned state\n\n" + table([]string{"Occurred", "Area of effect halted", "Resolved by"}, un) + sections(d.Notes)
}

// RecordMarkdown renders the record.
func RecordMarkdown(name string, cs []Crossing) string {
	digest := false
	for _, c := range cs {
		digest = digest || c.Digest != ""
	}
	heads := []string{"#", "At", "Stage", "Object", "Note"}
	if digest {
		heads = []string{"#", "At", "Stage", "Object", "Digest", "Note"}
	}
	var rows [][]string
	for _, c := range cs {
		r := []string{fmt.Sprint(c.N), c.At, c.Stage, c.Object, c.Note}
		if digest {
			r = []string{fmt.Sprint(c.N), c.At, c.Stage, c.Object, c.Digest, c.Note}
		}
		rows = append(rows, r)
	}
	return "# Record — " + name + "\n\nAppend-only. Each crossing of a stage boundary is one line.\n\n" + table(heads, rows)
}
