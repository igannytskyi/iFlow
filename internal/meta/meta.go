// Package meta is the data model of a change: one type for each document a
// change keeps, every field named, typed and described, every code one of the
// values docs/method/conventions.md defines. A document is kept as JSON in
// the shape of its type — that is the record — and rendered as Markdown
// beside it for a person, never read back. docs/method/data-model.md
// describes the model; docs/method/schema/ holds the JSON Schema generated
// from it.
package meta

// The data model of a change: one type for each document a change keeps,
// every field named, typed and described, every code one of the values
// docs/method/conventions.md defines. A document is kept as JSON in the
// shape of its type — that is the record — and rendered as Markdown beside
// it for a person, never read back. docs/method/data-model.md describes the
// model, and docs/method/schema/ holds the JSON Schema generated from it.

// SchemaVersion is the version of the model a document was written in. A
// document of an older version is read by Migrate into this one; a document
// of a newer version is refused.
const SchemaVersion = 1

// Intent is 00-intent.json: what is to be achieved, said once.
type Intent struct {
	Schema int `json:"schema" doc:"the version of the data model this was written in"`
	// ID is INT-<nnn>.
	ID   string `json:"id" doc:"INT-<nnn>"`
	Name string `json:"name" doc:"the change's folder name, its slug"`
	// StatedBy is who stated the intent.
	StatedBy string `json:"stated_by" doc:"who stated the intent"`
	// Authority is who may decide this intent against a competing one.
	Authority string `json:"deciding_authority,omitempty" doc:"who may decide this intent against a competing one"`
	Priority  string `json:"priority,omitempty"`
	Deadline  string `json:"deadline,omitempty"`
	// Statement is what is to be achieved, in the vocabulary of the domain.
	Statement string       `json:"statement" doc:"what is to be achieved, in the vocabulary of the domain"`
	Status    IntentStatus `json:"status"`
}

// Specification is 01-specification.json: how the change is judged. The
// change described for a reader is change.md, which this does not hold.
type Specification struct {
	Schema int    `json:"schema"`
	ID     string `json:"id" doc:"SPEC-<nnn>"`
	Name   string `json:"name"`
	Intent string `json:"intent" doc:"the intent it specifies, INT-<nnn>"`
	// Class is the default class of the change's units.
	Class Class `json:"default_class"`
	// Prose is what the specification says besides its tables, for a
	// specification written before change.md held the change described.
	Prose       string        `json:"prose,omitempty"`
	Criteria    []Criterion   `json:"criteria"`
	Termination []Termination `json:"termination"`
	Scope       Scope         `json:"scope"`
	// Arbiters are the paths that hold what judges the change.
	Arbiters []Arbiter `json:"arbiters"`
	// Inputs are what the arbiters read, and whether each is in the scope:
	// an input in the scope has its prior state captured at admission (R7b).
	Inputs []ArbiterInput `json:"arbiter_inputs,omitempty"`
	// Sections are the further sections a study's specification keeps —
	// its population, hypothesis, measures — as headed prose and tables.
	Sections []Section `json:"sections,omitempty"`
}

// Criterion is one acceptance criterion: when these conditions hold, then
// this must be true.
type Criterion struct {
	ID        string       `json:"id" doc:"CR-<nnn>-<nn>"`
	Criterion string       `json:"criterion" doc:"one line, in the vocabulary of the intent, never of the implementation"`
	When      string       `json:"when,omitempty" doc:"the conditions under which it is judged"`
	Then      string       `json:"then,omitempty" doc:"what must be true"`
	Plan      PlanNeed     `json:"plan,omitempty" doc:"whether settling it needs an evidence plan"`
	Procedure Procedure    `json:"procedure"`
	Evidence  EvidenceKind `json:"required_evidence"`
	Threshold string       `json:"threshold,omitempty"`
}

// Termination is a condition on which work stops without acceptance.
type Termination struct {
	Condition string `json:"condition"`
	Action    string `json:"action"`
}

// Scope is what the change may and may not touch.
type Scope struct {
	Included []string `json:"included"`
	Excluded []string `json:"excluded,omitempty"`
}

// Arbiter is one path holding what judges the change.
type Arbiter struct {
	Path       string `json:"path"`
	Arbitrates string `json:"arbitrates" doc:"the criteria it judges"`
}

// ArbiterInput is something an arbiter reads.
type ArbiterInput struct {
	Input   string `json:"input"`
	InScope string `json:"in_scope" doc:"yes or no, and why"`
}

// Section is a headed part of a document that has no type of its own: its
// prose, and its tables as they were written.
type Section struct {
	Title  string      `json:"title"`
	Text   string      `json:"text,omitempty"`
	Tables []TableData `json:"tables,omitempty"`
}

// TableData is a table kept as it was written.
type TableData struct {
	Headers []string   `json:"headers"`
	Rows    [][]string `json:"rows"`
}

// Plan is 02-plan.json: the change divided into units of work.
type Plan struct {
	Schema        int    `json:"schema"`
	Name          string `json:"name"`
	Specification string `json:"specification" doc:"SPEC-<nnn>"`
	Phases        int    `json:"phases"`
	// IntermediateValid says whether every phase boundary leaves a
	// shippable system.
	IntermediateValid string     `json:"intermediate_states_valid"`
	Residue           string     `json:"residue_on_abandonment,omitempty"`
	Units             []WorkUnit `json:"units"`
	// Coverage is the plan's answers to what the method asks of every plan,
	// or, in a plan written before those questions were asked, its prose.
	Coverage     []Answer `json:"coverage,omitempty"`
	CoverageText string   `json:"coverage_text,omitempty"`
	// Planner is who divided the work.
	Planner string `json:"planner,omitempty"`
	// Notes are what the plan says in prose beside its tables.
	Notes []Section `json:"notes,omitempty"`
}

// WorkUnit is one unit of work.
type WorkUnit struct {
	ID             string   `json:"id" doc:"WU-<nnn>-<nn>"`
	Phase          int      `json:"phase"`
	Class          Class    `json:"class"`
	Scope          []string `json:"scope"`
	AreaOfEffect   string   `json:"area_of_effect,omitempty" doc:"how it was computed, and at what confidence"`
	Criteria       []string `json:"criteria"`
	DependsOn      []string `json:"depends_on,omitempty"`
	PreparatoryFor []string `json:"preparatory_for,omitempty" doc:"the criteria whose arbiter this unit writes, before their code"`
}

// Answer is a question the method asks, and the answer given.
type Answer struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// Admission is 03-admission.json: what was admitted, on what, and what was
// fixed before execution.
type Admission struct {
	Schema int    `json:"schema"`
	Name   string `json:"name"`
	// Digest is the specification's digest at admission; R1 compares
	// against it ever after.
	Digest string `json:"specification_digest"`
	// Reviews are who read the criteria before admission.
	Reviews   []Review       `json:"criteria_review,omitempty"`
	Decisions []Decision     `json:"decisions"`
	Conflicts []Conflict     `json:"conflicts,omitempty"`
	Plans     []EvidencePlan `json:"evidence_plan,omitempty"`
	Prior     []Capture      `json:"prior_state"`
	Grants    []Grant        `json:"grants,omitempty"`
	// Notes are what the admission says in prose beside its tables.
	Notes []Section `json:"notes,omitempty"`
}

// Review is a person's reading of the criteria before admission.
type Review struct {
	Reviewed Reviewed `json:"reviewed_before_admission"`
	By       string   `json:"by"`
	At       string   `json:"at"`
	Note     string   `json:"note,omitempty"`
}

// Decision is what admission decided of one unit.
type Decision struct {
	Unit       string           `json:"unit"`
	Outcome    AdmissionOutcome `json:"outcome"`
	Condition  string           `json:"deciding_condition" doc:"conflict, allowance or permission, and what of it"`
	Confidence string           `json:"confidence_of_edges" doc:"the confidence of the edges of the estate relied on"`
	Escalates  string           `json:"escalates_at,omitempty"`
}

// Conflict is two units' areas of effect meeting.
type Conflict struct {
	Units       string `json:"units"`
	Region      string `json:"intersecting_region"`
	Invalidates string `json:"evidence_it_would_invalidate"`
	Resolution  string `json:"resolution"`
}

// EvidencePlan is what will be observed to settle one criterion, and how;
// fixed at admission, derived from the area of effect, never the candidate.
type EvidencePlan struct {
	Criterion   string `json:"criterion"`
	Observed    string `json:"observed" doc:"which component, which interfaces"`
	Method      string `json:"method" doc:"environment, load, repetitions, tolerated variance"`
	Unchanged   string `json:"unchanged_means,omitempty" doc:"what counts as unchanged, against which captured prior state"`
	DerivedFrom string `json:"derived_from" doc:"the area of effect, and its confidence"`
	FixedAt     string `json:"fixed_at,omitempty"`
}

// Capture is the prior state of something an arbiter reads, kept before
// execution.
type Capture struct {
	Criterion  string `json:"criterion"`
	Artefact   string `json:"artefact"`
	CapturedTo string `json:"captured_to" doc:"baseline/<name>"`
}

// Grant is what a unit was permitted to do.
type Grant struct {
	ID         string `json:"id" doc:"GR-<nnn>-<nn>"`
	Unit       string `json:"unit"`
	Operations string `json:"permitted_operations"`
	Targets    string `json:"targets"`
	Expires    string `json:"expires"`
}

// Execution is 04-execution.json: each unit carried out, and what it left.
type Execution struct {
	Schema     int         `json:"schema"`
	Name       string      `json:"name"`
	Runs       []Run       `json:"runs"`
	Candidates []Candidate `json:"candidates"`
	// Reproductions are, for a repair, how each criterion stood before it.
	Reproductions []Reproduction `json:"reproductions,omitempty"`
	Prose         string         `json:"prose,omitempty"`
}

// Run is one unit's run by its executor.
type Run struct {
	Unit      string   `json:"unit"`
	Executor  string   `json:"executor"`
	Version   string   `json:"version"`
	Terminal  Terminal `json:"terminal_state"`
	Candidate string   `json:"candidate,omitempty"`
	Trace     string   `json:"trace,omitempty"`
}

// Candidate is what a unit proposes, kept apart and never applied here.
type Candidate struct {
	ID         string `json:"id" doc:"CA-<nnn>-<nn>"`
	Unit       string `json:"unit"`
	StoredAt   string `json:"stored_at"`
	Artefacts  string `json:"artefacts"`
	ProducedAt string `json:"produced_at"`
}

// Reproduction is how a criterion stood before a repair.
type Reproduction struct {
	Unit         string `json:"unit"`
	Criterion    string `json:"criterion"`
	BeforeRepair string `json:"before_repair"`
}

// Assurance is 05-assurance.json: the evidence, the verdicts on it, and what
// the change raised and settled.
type Assurance struct {
	Schema   int        `json:"schema"`
	Name     string     `json:"name"`
	Evidence []Evidence `json:"evidence"`
	Verdicts []Verdict  `json:"verdicts"`
	// Raised are the escalations this change raised: the method asking a
	// person, or itself, for what no change here can settle.
	Raised []Escalation `json:"escalations_raised,omitempty"`
	// Settled are the escalations of earlier changes this one closed.
	Settled  []Settlement `json:"escalations_closed,omitempty"`
	Refusals []Refusal    `json:"refusals,omitempty"`
	Sections []Section    `json:"sections,omitempty"`
}

// Evidence is one output a verdict may rest on.
type Evidence struct {
	ID           string       `json:"id" doc:"EV-<nnn>-<nn>-<nn>"`
	Subject      Subject      `json:"subject"`
	Kind         EvidenceKind `json:"kind"`
	Producer     string       `json:"producer" doc:"what produced it, a command in backticks where it can be run again"`
	Independence Independence `json:"independence"`
	How          string       `json:"how_established"`
	Repeatable   Repeatable   `json:"repeatable,omitempty"`
	ObtainedAt   string       `json:"obtained_at"`
	ValidUntil   string       `json:"valid_until,omitempty"`
}

// Verdict is one unit judged on one criterion.
type Verdict struct {
	ID        string         `json:"id" doc:"VE-<nnn>-<nn>"`
	Unit      string         `json:"unit"`
	Criterion string         `json:"criterion"`
	Outcome   VerdictOutcome `json:"outcome"`
	Evidence  []string       `json:"evidence"`
	State     VerdictState   `json:"state"`
	Window    string         `json:"window,omitempty"`
	Baseline  string         `json:"baseline,omitempty"`
}

// Escalation is one escalation a change raised, addressed <slug>#<n>.
type Escalation struct {
	N        int    `json:"n"`
	Standing string `json:"standing" doc:"open or closed"`
	Ground   string `json:"ground"`
	Against  string `json:"raised_against"`
	To       string `json:"to"`
}

// Settlement is an earlier change's escalation this change closed.
type Settlement struct {
	Escalation string `json:"escalation" doc:"<slug>#<n>"`
	ResolvedBy string `json:"resolved_by"`
	Note       string `json:"note,omitempty"`
}

// Refusal is an object returned across a boundary.
type Refusal struct {
	ID       string    `json:"id" doc:"RF-<nnn>-<nn>"`
	Object   string    `json:"object"`
	Boundary string    `json:"boundary"`
	By       RefusedBy `json:"refused_by"`
	Ground   string    `json:"ground"`
	At       string    `json:"at"`
}

// Landing is 06-landing.json: what entered the estate, in what order.
type Landing struct {
	Schema    int         `json:"schema"`
	Name      string      `json:"name"`
	Entries   []Entry     `json:"entries"`
	Unplanned []Unplanned `json:"unplanned_state,omitempty"`
	// Notes are what the landing says in prose beside its tables.
	Notes []Section `json:"notes,omitempty"`
}

// Entry is one unit entering the estate.
type Entry struct {
	Order         int    `json:"order"`
	Unit          string `json:"unit"`
	VerdictState  string `json:"verdict_state"`
	StillValid    string `json:"evidence_still_valid"`
	EnteredAt     string `json:"entered_at"`
	Invalidated   string `json:"invalidated_by_this_entry"`
	Reestablished string `json:"reestablished_before_next,omitempty"`
}

// Unplanned is a state no plan foresaw, and how it was resolved.
type Unplanned struct {
	Occurred   string `json:"occurred"`
	Halted     string `json:"area_of_effect_halted"`
	ResolvedBy string `json:"resolved_by"`
}

// Crossing is one line of the record: a stage boundary crossed. The record
// is record.jsonl, one crossing a line, appended to and never rewritten.
type Crossing struct {
	N      int    `json:"n" doc:"the ordinal: 1, 2, 3 … in order, none missing"`
	At     string `json:"at"`
	Stage  string `json:"stage"`
	Object string `json:"object"`
	Digest string `json:"digest,omitempty"`
	Note   string `json:"note,omitempty"`
}
