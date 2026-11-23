package artifact

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/igannytskyi/iflow/internal/meta"
)

// A change written before the data model kept its stage documents as
// Markdown only. Decode reads such a document into its type, once, for
// Migrate: every table and section it holds lands in a field, or in the
// document's Sections when the model has no field for it, and what cannot be
// read is said rather than dropped.

// section is a part of a Markdown document under one "## " heading; the
// part before the first heading has an empty title.
type section struct {
	title  string
	prose  []string
	tables []Table
}

func sectionsOf(text string) []section {
	out := []section{{}}
	var block []string
	flush := func() {
		if len(block) == 0 {
			return
		}
		cur := &out[len(out)-1]
		// A table broken by a blank line goes on under its own header: rows
		// with no header and separator of their own, as wide as the table
		// before them, are that table's.
		if n := len(cur.tables); n > 0 && (len(block) < 2 || !separator(cellsOf(strings.TrimSpace(block[1])))) &&
			len(cellsOf(strings.TrimSpace(block[0]))) == len(cur.tables[n-1].Headers) {
			prev := &cur.tables[n-1]
			for _, l := range block {
				prev.Rows = append(prev.Rows, newRow(prev.Headers, cellsOf(strings.TrimSpace(l))))
			}
		} else {
			cur.tables = append(cur.tables, ParseTables(strings.Join(block, "\n"))...)
		}
		block = nil
	}
	for _, l := range lines(text) {
		s := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(s, "## "):
			flush()
			out = append(out, section{title: strings.TrimSpace(strings.TrimPrefix(s, "## "))})
		case strings.HasPrefix(s, "|"):
			block = append(block, l)
		default:
			flush()
			if s != "" && !strings.HasPrefix(s, "# ") {
				cur := &out[len(out)-1]
				cur.prose = append(cur.prose, s)
			}
		}
	}
	flush()
	return out
}

// decoder reads one document and keeps what it could not.
type decoder struct {
	stage    Stage
	problems []string
}

func (d *decoder) fail(format string, args ...any) {
	d.problems = append(d.problems, fmt.Sprintf(string(d.stage)+": "+format, args...))
}

// fieldsOf is a Field | Value table as a map.
func fieldsOf(ts []Table) map[string]string {
	out := map[string]string{}
	for _, t := range ts {
		if t.Has("Field", "Value") {
			for _, r := range t.Rows {
				out[r.Cell("Field")] = r.Cell("Value")
			}
		}
	}
	return out
}

// rowsOf is the rows of the first table with these headers, placeholders
// left out.
func rowsOf(ts []Table, headers ...string) ([]Row, bool) {
	for _, t := range ts {
		if t.Has(headers...) {
			var out []Row
			for _, r := range t.Rows {
				if !r.Placeholder() {
					out = append(out, r)
				}
			}
			return out, true
		}
	}
	return nil, false
}

func ids(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" && p != "—" && p != "-" {
			out = append(out, p)
		}
	}
	return out
}

var leadingNumber = regexp.MustCompile(`^\d+`)

func (d *decoder) int(what, s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(leadingNumber.FindString(s))
	if err != nil {
		d.fail("%s %q is not a number", what, s)
	}
	return n
}

// asSection keeps a section the model has no field for, whole.
func asSection(s section) meta.Section {
	out := meta.Section{Title: s.title, Text: strings.Join(s.prose, "\n\n")}
	for _, t := range s.tables {
		td := meta.TableData{Headers: t.Headers}
		for _, r := range t.Rows {
			td.Rows = append(td.Rows, r.Values())
		}
		out.Tables = append(out.Tables, td)
	}
	return out
}

// DecodeIntent reads a Markdown 00-intent.
func DecodeIntent(name, text string) (meta.Intent, []string) {
	f := fieldsOf(sectionsOf(text)[0].tables)
	return meta.Intent{Schema: meta.SchemaVersion, Name: name, ID: f["Id"], StatedBy: f["Stated by"], Authority: f["Deciding authority"],
		Priority: f["Priority"], Deadline: f["Deadline"], Statement: f["Statement"], Status: meta.IntentStatus(f["Status"])}, nil
}

// DecodeSpecification reads a Markdown 01-specification. A table is known by
// its columns wherever it stands; a section none of whose tables the model
// has a field for is kept whole.
func DecodeSpecification(name, text string) (meta.Specification, []string) {
	d := &decoder{stage: Specification}
	secs := sectionsOf(text)
	f := map[string]string{}
	for _, s := range secs {
		for k, v := range fieldsOf(s.tables) {
			f[k] = v
		}
	}
	out := meta.Specification{Schema: meta.SchemaVersion, Name: name, ID: f["Id"], Intent: f["Intent"], Class: meta.Class(f["Default class"]),
		Prose: strings.Join(secs[0].prose, "\n\n")}
	for i, s := range secs {
		var other []Table
		for _, t := range s.tables {
			rows := placed(t)
			switch {
			case t.Has("Field", "Value"):
			case t.Has("Id", "Criterion"):
				for _, r := range rows {
					out.Criteria = append(out.Criteria, meta.Criterion{ID: r.Cell("Id"), Criterion: r.Cell("Criterion"), When: r.Cell("When"), Then: r.Cell("Then"),
						Plan: meta.PlanNeed(r.Cell("Plan")), Procedure: meta.Procedure(r.Cell("Procedure")), Evidence: meta.EvidenceKind(r.Cell("Required evidence")), Threshold: r.Cell("Threshold")})
				}
			case t.Has("Condition", "Action"):
				for _, r := range rows {
					out.Termination = append(out.Termination, meta.Termination{Condition: r.Cell("Condition"), Action: r.Cell("Action")})
				}
			case t.Has("Included"):
				for _, r := range rows {
					if v := r.Cell("Included"); v != "" {
						out.Scope.Included = append(out.Scope.Included, v)
					}
					if v := r.Cell("Excluded"); v != "" {
						out.Scope.Excluded = append(out.Scope.Excluded, v)
					}
				}
			case t.Has("Path", "What it arbitrates"):
				for _, r := range rows {
					out.Arbiters = append(out.Arbiters, meta.Arbiter{Path: r.Cell("Path"), Arbitrates: r.Cell("What it arbitrates")})
				}
			case t.Has("Input", "In scope?"):
				for _, r := range rows {
					out.Inputs = append(out.Inputs, meta.ArbiterInput{Input: r.Cell("Input"), InScope: r.Cell("In scope?")})
				}
			default:
				other = append(other, t)
			}
		}
		if i == 0 {
			continue // its prose is the specification's Prose
		}
		if len(other) > 0 || len(s.prose) > 0 {
			out.Sections = append(out.Sections, asSection(section{title: s.title, prose: s.prose, tables: other}))
		}
	}
	return out, d.problems
}

// placed is a table's rows, placeholders left out.
func placed(t Table) []Row {
	var out []Row
	for _, r := range t.Rows {
		if !r.Placeholder() {
			out = append(out, r)
		}
	}
	return out
}

var plannedBy = regexp.MustCompile(`^Planned by (.+)\.$`)

// DecodePlan reads a Markdown 02-plan.
func DecodePlan(name, text string) (meta.Plan, []string) {
	d := &decoder{stage: Plan}
	secs := sectionsOf(text)
	out := meta.Plan{Schema: meta.PlanVersion, Name: name}
	for _, s := range secs {
		for _, t := range s.tables {
			rows := placed(t)
			switch {
			case t.Has("Field", "Value"):
				f := fieldsOf([]Table{t})
				out.Specification, out.Phases = f["Specification"], d.int("Phases", f["Phases"])
				out.IntermediateValid, out.Residue = f["Intermediate states valid"], f["Residue on abandonment"]
			case t.Has("Id", "Scope"):
				for _, r := range rows {
					out.Units = append(out.Units, meta.WorkUnit{ID: r.Cell("Id"), Phase: d.int("Phase of "+r.Cell("Id"), r.Cell("Phase")), Class: meta.Class(r.Cell("Class")),
						Scope: ids(r.Cell("Scope")), AreaOfEffect: r.Cell("Area of effect"), Criteria: ids(r.Cell("Criteria")),
						DependsOn: ids(r.Cell("Depends on")), PreparatoryFor: ids(r.Cell("Preparatory for")),
						Kind: kindOf(ids(r.Cell("Preparatory for")))})
				}
			case t.Has("Question", "Answer"):
				for _, r := range rows {
					out.Coverage = append(out.Coverage, meta.Answer{Question: r.Cell("Question"), Answer: r.Cell("Answer")})
				}
			default:
				d.fail("a table %v the plan has no place for", t.Headers)
			}
		}
		var prose []string
		for _, p := range s.prose {
			if m := plannedBy.FindStringSubmatch(p); m != nil && s.title == "" {
				out.Planner = m[1]
				continue
			}
			prose = append(prose, p)
		}
		switch {
		case len(prose) == 0:
		case s.title == "Coverage":
			out.CoverageText = strings.Join(prose, "\n\n")
		default:
			out.Notes = append(out.Notes, meta.Section{Title: or(s.title, "Notes"), Text: strings.Join(prose, "\n\n")})
		}
	}
	return out, d.problems
}

func or(v, otherwise string) string {
	if v == "" {
		return otherwise
	}
	return v
}

// DecodeAdmission reads a Markdown 03-admission.
func DecodeAdmission(name, text string) (meta.Admission, []string) {
	d := &decoder{stage: Admission}
	out := meta.Admission{Schema: meta.SchemaVersion, Name: name}
	for _, s := range sectionsOf(text) {
		for _, t := range s.tables {
			rows := placed(t)
			switch {
			case t.Has("Field", "Value"):
				out.Digest = fieldsOf([]Table{t})["Specification digest"]
			case t.Has("Reviewed before admission"):
				for _, r := range rows {
					out.Reviews = append(out.Reviews, meta.Review{Reviewed: meta.Reviewed(r.Cell("Reviewed before admission")), By: r.Cell("By"), At: r.Cell("At"), Note: r.Cell("Note")})
				}
			case t.Has("Unit", "Outcome"):
				for _, r := range rows {
					out.Decisions = append(out.Decisions, meta.Decision{Unit: r.Cell("Unit"), Outcome: meta.AdmissionOutcome(r.Cell("Outcome")),
						Condition: r.Cell("Deciding condition"), Confidence: r.Cell("Confidence of the edges relied on"), Escalates: r.Cell("Escalates at")})
				}
			case t.Has("Units", "Intersecting region"):
				for _, r := range rows {
					out.Conflicts = append(out.Conflicts, meta.Conflict{Units: r.Cell("Units"), Region: r.Cell("Intersecting region"),
						Invalidates: r.Cell("Evidence it would invalidate"), Resolution: r.Cell("Resolution")})
				}
			case t.Has("Criterion", "Observed"):
				for _, r := range rows {
					out.Plans = append(out.Plans, meta.EvidencePlan{Criterion: r.Cell("Criterion"), Observed: r.Cell("Observed"), Method: r.Cell("Method"),
						Unchanged: r.Cell("Unchanged means"), DerivedFrom: r.Cell("Derived from"), FixedAt: r.Cell("Fixed at")})
				}
			case t.Has("Artefact", "Captured to"):
				for _, r := range rows {
					out.Prior = append(out.Prior, meta.Capture{Criterion: r.Cell("Criterion"), Artefact: r.Cell("Artefact"), CapturedTo: r.Cell("Captured to")})
				}
			case t.Has("Id", "Permitted operations"):
				for _, r := range rows {
					out.Grants = append(out.Grants, meta.Grant{ID: r.Cell("Id"), Unit: r.Cell("Unit"), Operations: r.Cell("Permitted operations"), Targets: r.Cell("Targets"), Expires: r.Cell("Expires")})
				}
			default:
				d.fail("a table %v admission has no place for", t.Headers)
			}
		}
		if len(s.prose) > 0 {
			out.Notes = append(out.Notes, meta.Section{Title: or(s.title, "Notes"), Text: strings.Join(s.prose, "\n\n")})
		}
	}
	return out, d.problems
}

// DecodeExecution reads a Markdown 04-execution.
func DecodeExecution(name, text string) (meta.Execution, []string) {
	d := &decoder{stage: Execution}
	out := meta.Execution{Schema: meta.SchemaVersion, Name: name}
	var prose []string
	for _, s := range sectionsOf(text) {
		for _, t := range s.tables {
			switch {
			case t.Has("Unit", "Executor"):
				for _, r := range t.Rows {
					if !r.Placeholder() {
						out.Runs = append(out.Runs, meta.Run{Unit: r.Cell("Unit"), Executor: r.Cell("Executor"), Version: r.Cell("Version"),
							Terminal: meta.Terminal(r.Cell("Terminal state")), Candidate: r.Cell("Candidate"), Trace: r.Cell("Trace")})
					}
				}
			case t.Has("Id", "Stored at"):
				for _, r := range t.Rows {
					if !r.Placeholder() {
						out.Candidates = append(out.Candidates, meta.Candidate{ID: r.Cell("Id"), Unit: r.Cell("Unit"), StoredAt: r.Cell("Stored at"),
							Artefacts: r.Cell("Artefacts"), ProducedAt: r.Cell("Produced at")})
					}
				}
			case t.Has("Unit", "Criterion", "Before repair"):
				for _, r := range t.Rows {
					if !r.Placeholder() {
						out.Reproductions = append(out.Reproductions, meta.Reproduction{Unit: r.Cell("Unit"), Criterion: r.Cell("Criterion"), BeforeRepair: r.Cell("Before repair")})
					}
				}
			default:
				d.fail("a table %v execution has no place for", t.Headers)
			}
		}
		prose = append(prose, s.prose...)
	}
	out.Prose = strings.Join(prose, "\n\n")
	return out, d.problems
}

// DecodeAssurance reads a Markdown 05-assurance.
func DecodeAssurance(name, text string) (meta.Assurance, []string) {
	d := &decoder{stage: Assurance}
	out := meta.Assurance{Schema: meta.SchemaVersion, Name: name}
	for _, s := range sectionsOf(text) {
		var other section
		for _, t := range s.tables {
			switch {
			case t.Has("Id", "Producer"):
				for _, r := range t.Rows {
					if !r.Placeholder() {
						out.Evidence = append(out.Evidence, meta.Evidence{ID: r.Cell("Id"), Subject: meta.Subject(r.Cell("Subject")), Kind: meta.EvidenceKind(r.Cell("Kind")),
							Producer: r.Cell("Producer"), Independence: meta.Independence(r.Cell("Independence")), How: r.Cell("How established"),
							Repeatable: meta.Repeatable(r.Cell("Repeatable")), ObtainedAt: r.Cell("Obtained at"), ValidUntil: r.Cell("Valid until")})
					}
				}
			case t.Has("Id", "Outcome", "Criterion"):
				for _, r := range t.Rows {
					if !r.Placeholder() {
						out.Verdicts = append(out.Verdicts, meta.Verdict{ID: r.Cell("Id"), Unit: r.Cell("Unit"), Criterion: r.Cell("Criterion"),
							Outcome: meta.VerdictOutcome(r.Cell("Outcome")), Evidence: ids(r.Cell("Evidence")), State: meta.VerdictState(r.Cell("State")),
							Window: r.Cell("Window"), Baseline: r.Cell("Baseline")})
					}
				}
			case t.Has("Standing", "Ground"):
				for _, r := range t.Rows {
					if !r.Placeholder() {
						out.Raised = append(out.Raised, meta.Escalation{N: d.int("escalation", r.Cell("#")), Standing: r.Cell("Standing"), Ground: r.Cell("Ground"),
							Against: r.Cell("Raised against"), To: r.Cell("To")})
					}
				}
			case t.Has("Escalation", "Resolved by"):
				for _, r := range t.Rows {
					if !r.Placeholder() {
						out.Settled = append(out.Settled, meta.Settlement{Escalation: r.Cell("Escalation"), ResolvedBy: r.Cell("Resolved by"), Note: r.Cell("Note")})
					}
				}
			case t.Has("Refused by", "Ground"):
				for _, r := range t.Rows {
					if !r.Placeholder() {
						out.Refusals = append(out.Refusals, meta.Refusal{ID: r.Cell("Id"), Object: r.Cell("Object"), Boundary: r.Cell("Boundary"),
							By: meta.RefusedBy(r.Cell("Refused by")), Ground: r.Cell("Ground"), At: r.Cell("At")})
					}
				}
			default:
				other.tables = append(other.tables, t)
			}
		}
		if len(other.tables) > 0 || (s.title != "" && len(s.prose) > 0) {
			other.title, other.prose = s.title, s.prose
			if other.title == "" {
				other.title = "Notes"
			}
			out.Sections = append(out.Sections, asSection(other))
		}
	}
	return out, d.problems
}

// DecodeLanding reads a Markdown 06-landing.
func DecodeLanding(name, text string) (meta.Landing, []string) {
	d := &decoder{stage: Landing}
	out := meta.Landing{Schema: meta.SchemaVersion, Name: name}
	for _, s := range sectionsOf(text) {
		for _, t := range s.tables {
			switch {
			case t.Has("Order", "Unit"):
				for _, r := range t.Rows {
					if !r.Placeholder() {
						out.Entries = append(out.Entries, meta.Entry{Order: d.int("order", r.Cell("Order")), Unit: r.Cell("Unit"), VerdictState: r.Cell("Verdict state"),
							StillValid: r.Cell("Evidence still valid"), EnteredAt: r.Cell("Entered at"), Invalidated: r.Cell("Invalidated by this entry"),
							Reestablished: r.Cell("Re-established before next")})
					}
				}
			case t.Has("Occurred"):
				for _, r := range t.Rows {
					if !r.Placeholder() && !(r.Cell("Occurred") == "no" && r.Cell("Area of effect halted") == "" && r.Cell("Resolved by") == "") {
						out.Unplanned = append(out.Unplanned, meta.Unplanned{Occurred: r.Cell("Occurred"), Halted: r.Cell("Area of effect halted"), ResolvedBy: r.Cell("Resolved by")})
					}
				}
			default:
				d.fail("a table %v landing has no place for", t.Headers)
			}
		}
		if len(s.prose) > 0 {
			title := s.title
			if title == "" {
				title = "Notes"
			}
			out.Notes = append(out.Notes, meta.Section{Title: title, Text: strings.Join(s.prose, "\n\n")})
		}
	}
	return out, d.problems
}

// DecodeRecord reads a Markdown record.
func DecodeRecord(text string) ([]meta.Crossing, []string) {
	d := &decoder{stage: "record"}
	var out []meta.Crossing
	for _, t := range ParseTables(text) {
		if !t.Has("#", "Stage") {
			continue
		}
		for _, r := range t.Rows {
			if r.Placeholder() {
				continue
			}
			out = append(out, meta.Crossing{N: d.int("ordinal", r.Cell("#")), At: r.Cell("At"), Stage: r.Cell("Stage"), Object: r.Cell("Object"),
				Digest: r.Cell("Digest"), Note: r.Cell("Note")})
		}
	}
	return out, d.problems
}
