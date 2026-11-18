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

// SchemaVersion is the version of the model a document is written in; a
// document whose shape has changed since has a version of its own —
// PlanVersion. A document of an older version is read into this one, and
// iflow migrate writes it so; one of a newer version is refused.
const SchemaVersion = 1

// PlanVersion is the version of a plan: 2 since a unit says what it writes
// and its contract is named entries, not prose.
const PlanVersion = 2

// Version is the version of the model this program writes a document in.
func Version(doc any) int {
	switch doc.(type) {
	case Plan, *Plan:
		return PlanVersion
	}
	return SchemaVersion
}

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
	Priority  string `json:"priority,omitempty" doc:"how this intent ranks against others"`
	Deadline  string `json:"deadline,omitempty" doc:"when it is wanted by, or none"`
	// Statement is what is to be achieved, in the vocabulary of the domain.
	Statement string       `json:"statement" doc:"what is to be achieved, in the vocabulary of the domain"`
	Status    IntentStatus `json:"status" doc:"where the intent stands"`
}

// Specification is 01-specification.json: how the change is judged. The
// change described for a reader is change.md, which this does not hold.
type Specification struct {
	Schema int    `json:"schema" doc:"the version of the data model this was written in"`
	ID     string `json:"id" doc:"SPEC-<nnn>"`
	Name   string `json:"name" doc:"the change's folder name, its slug"`
	Intent string `json:"intent" doc:"the intent it specifies, INT-<nnn>"`
	// Class is the default class of the change's units.
	Class Class `json:"default_class" doc:"the class a unit of the change has unless the plan gives it another"`
	// Prose is what the specification says besides its tables, for a
	// specification written before change.md held the change described.
	Prose       string        `json:"prose,omitempty" doc:"what the document says besides its fields, for one written before the model held it"`
	Criteria    []Criterion   `json:"criteria" doc:"the acceptance criteria"`
	Termination []Termination `json:"termination" doc:"the conditions on which work stops without acceptance"`
	Scope       Scope         `json:"scope" doc:"what the change may touch, as paths"`
	// Arbiters are the paths that hold what judges the change.
	Arbiters []Arbiter `json:"arbiters" doc:"the paths holding what judges the change"`
	// Inputs are what the arbiters read, and whether each is in the scope:
	// an input in the scope has its prior state captured at admission (R7b).
	Inputs []ArbiterInput `json:"arbiter_inputs,omitempty" doc:"what the arbiters read, and whether each is in the scope"`
	// NewTests are the tests written for this change before its code, and
	// SuiteTests those already there that judge it.
	NewTests   []string `json:"new_tests,omitempty" doc:"the tests written for this change, before its code"`
	SuiteTests []string `json:"existing_tests,omitempty" doc:"the tests already there that judge it"`
	// Sections are the further sections a study's specification keeps —
	// its population, hypothesis, measures — as headed prose and tables.
	Sections []Section `json:"sections,omitempty" doc:"headed parts the model has no field for, kept as they were written"`
}

// Criterion is one acceptance criterion: when these conditions hold, then
// this must be true.
type Criterion struct {
	ID        string       `json:"id" doc:"CR-<nnn>-<nn>"`
	Criterion string       `json:"criterion" doc:"one line, in the vocabulary of the intent, never of the implementation"`
	When      string       `json:"when,omitempty" doc:"the conditions under which it is judged"`
	Then      string       `json:"then,omitempty" doc:"what must be true"`
	Plan      PlanNeed     `json:"plan,omitempty" doc:"whether settling it needs an evidence plan"`
	Procedure Procedure    `json:"procedure" doc:"how the criterion is decided"`
	Evidence  EvidenceKind `json:"required_evidence" doc:"what kind of evidence settles the criterion"`
	Threshold string       `json:"threshold,omitempty" doc:"the bound a result must meet"`
	New       bool         `json:"judged_by_new_tests,omitempty" doc:"whether tests written for this change, before its code, judge it"`
}

// Termination is a condition on which work stops without acceptance.
type Termination struct {
	Condition string `json:"condition" doc:"the condition"`
	Action    string `json:"action" doc:"stop, or escalate"`
}

// Scope is what the change may and may not touch.
type Scope struct {
	Included []string `json:"included" doc:"paths the change may touch"`
	Excluded []string `json:"excluded,omitempty" doc:"paths it may not touch, though near"`
}

// Arbiter is one path holding what judges the change.
type Arbiter struct {
	Path       string `json:"path" doc:"a path, relative to the estate"`
	Arbitrates string `json:"arbitrates" doc:"the criteria it judges"`
}

// ArbiterInput is something an arbiter reads.
type ArbiterInput struct {
	Input   string `json:"input" doc:"a path an arbiter reads"`
	InScope string `json:"in_scope" doc:"yes or no, and why"`
}

// Section is a headed part of a document that has no type of its own: its
// prose, and its tables as they were written.
type Section struct {
	Title  string      `json:"title" doc:"the heading"`
	Text   string      `json:"text,omitempty" doc:"its prose"`
	Tables []TableData `json:"tables,omitempty" doc:"its tables"`
}

// TableData is a table kept as it was written.
type TableData struct {
	Headers []string   `json:"headers" doc:"the column headings"`
	Rows    [][]string `json:"rows" doc:"the rows, each cell under its heading"`
}

// Plan is 02-plan.json: the change divided into units of work.
type Plan struct {
	Schema        int    `json:"schema" doc:"the version of the data model this was written in"`
	Name          string `json:"name" doc:"the change's folder name, its slug"`
	Specification string `json:"specification" doc:"SPEC-<nnn>"`
	Phases        int    `json:"phases" doc:"how many phases the change lands in; 1 if everything can land at once"`
	// IntermediateValid says whether every phase boundary leaves a
	// shippable system.
	IntermediateValid string     `json:"intermediate_states_valid" doc:"whether every phase boundary leaves a shippable system, and why"`
	Residue           string     `json:"residue_on_abandonment,omitempty" doc:"what is left if the change is abandoned: a rollback, or the intent that disposes of it"`
	Units             []WorkUnit `json:"units" doc:"the units of work"`
	// Coverage is the plan's answers to what the method asks of every plan,
	// or, in a plan written before those questions were asked, its prose.
	Coverage     []Answer `json:"coverage,omitempty" doc:"the plan's answers to what the method asks of every plan"`
	CoverageText string   `json:"coverage_text,omitempty" doc:"how the units cover the specification, in prose"`
	// Planner is who divided the work.
	Planner string `json:"planner,omitempty" doc:"who divided the work"`
	// Notes are what the plan says in prose beside its tables.
	Notes []Section `json:"notes,omitempty" doc:"what the document says in prose beside its tables, by heading"`
}

// WorkUnit is one unit of work.
type WorkUnit struct {
	ID             string   `json:"id" doc:"WU-<nnn>-<nn>"`
	Phase          int      `json:"phase" doc:"the phase it lands in"`
	Class          Class    `json:"class" doc:"the unit's change class"`
	Scope          []string `json:"scope" doc:"what the change may touch, as paths"`
	AreaOfEffect   string   `json:"area_of_effect,omitempty" doc:"how it was computed, and at what confidence"`
	Criteria       []string `json:"criteria" doc:"the acceptance criteria"`
	DependsOn      []string `json:"depends_on,omitempty" doc:"the units it relies on"`
	PreparatoryFor []string `json:"preparatory_for,omitempty" doc:"the criteria whose arbiter this unit writes, before their code"`
	// Kind is what the unit writes.
	Kind UnitKind `json:"kind" doc:"what the unit writes: the contract, the tests that judge the change, or its code"`
	// What the unit's agent is told: its title, its brief — the ticket in
	// units/ — the names it gives the others, and the names of theirs it
	// uses.
	Title    string      `json:"title,omitempty" doc:"the unit in a few words"`
	Brief    string      `json:"brief,omitempty" doc:"its ticket: objective, what it serves and relies on, tasks, constraints, edge cases, when it is done"`
	Provides []Provision `json:"provides,omitempty" doc:"the names it gives the other units: the contract between them"`
	Uses     []string    `json:"uses,omitempty" doc:"the names other units of the plan provide that it uses"`
	Reads    []string    `json:"reads,omitempty" doc:"files of the estate outside its scope that it builds on, handed to its agent whole"`
}

// Provision is one name a unit gives the others: where it is declared, as
// what, and exactly how.
type Provision struct {
	File      string   `json:"file,omitempty" doc:"the file it is declared in; empty only for one described"`
	Name      string   `json:"name,omitempty" doc:"the name the other units call it by; empty only for one described"`
	Kind      DeclKind `json:"kind" doc:"what the code declares it as"`
	Signature string   `json:"signature" doc:"its declaration as the estate's language writes it, or, described, what was said of it"`
}

// Answer is a question the method asks, and the answer given.
type Answer struct {
	Question string `json:"question" doc:"the question"`
	Answer   string `json:"answer" doc:"the answer"`
}

// Admission is 03-admission.json: what was admitted, on what, and what was
// fixed before execution.
type Admission struct {
	Schema int    `json:"schema" doc:"the version of the data model this was written in"`
	Name   string `json:"name" doc:"the change's folder name, its slug"`
	// Digest is the specification's digest at admission; R1 compares
	// against it ever after.
	Digest string `json:"specification_digest" doc:"the specification's digest at admission; R1 compares against it ever after"`
	// Reviews are who read the criteria before admission.
	Reviews   []Review       `json:"criteria_review,omitempty" doc:"whether a person read the criteria before admission, and who"`
	Decisions []Decision     `json:"decisions" doc:"what admission decided of each unit"`
	Conflicts []Conflict     `json:"conflicts,omitempty" doc:"where units' areas of effect meet, and how it is resolved"`
	Plans     []EvidencePlan `json:"evidence_plan,omitempty" doc:"what will be observed to settle each criterion that needs a plan"`
	Prior     []Capture      `json:"prior_state" doc:"the prior state of what the arbiters read, captured before execution"`
	Grants    []Grant        `json:"grants,omitempty" doc:"what each unit was permitted to do"`
	// Notes are what the admission says in prose beside its tables.
	Notes []Section `json:"notes,omitempty" doc:"what the document says in prose beside its tables, by heading"`
}

// Review is a person's reading of the criteria before admission.
type Review struct {
	Reviewed Reviewed `json:"reviewed_before_admission" doc:"yes or no"`
	By       string   `json:"by" doc:"who"`
	At       string   `json:"at" doc:"when, as a date"`
	Note     string   `json:"note,omitempty" doc:"a note"`
}

// Decision is what admission decided of one unit.
type Decision struct {
	Unit       string           `json:"unit" doc:"a work unit, WU-<nnn>-<nn>"`
	Outcome    AdmissionOutcome `json:"outcome" doc:"the outcome"`
	Condition  string           `json:"deciding_condition" doc:"conflict, allowance or permission, and what of it"`
	Confidence string           `json:"confidence_of_edges" doc:"the confidence of the edges of the estate relied on"`
	Escalates  string           `json:"escalates_at,omitempty" doc:"when a held unit is escalated to a person"`
}

// Conflict is two units' areas of effect meeting.
type Conflict struct {
	Units       string `json:"units" doc:"the units of work"`
	Region      string `json:"intersecting_region" doc:"where the areas of effect meet"`
	Invalidates string `json:"evidence_it_would_invalidate" doc:"what evidence one landing would void for the other"`
	Resolution  string `json:"resolution" doc:"how the conflict is resolved"`
}

// EvidencePlan is what will be observed to settle one criterion, and how;
// fixed at admission, derived from the area of effect, never the candidate.
type EvidencePlan struct {
	Criterion   string `json:"criterion" doc:"a criterion, CR-<nnn>-<nn>"`
	Observed    string `json:"observed" doc:"which component, which interfaces"`
	Method      string `json:"method" doc:"environment, load, repetitions, tolerated variance"`
	Unchanged   string `json:"unchanged_means,omitempty" doc:"what counts as unchanged, against which captured prior state"`
	DerivedFrom string `json:"derived_from" doc:"the area of effect, and its confidence"`
	FixedAt     string `json:"fixed_at,omitempty" doc:"when the plan stopped being editable"`
}

// Capture is the prior state of something an arbiter reads, kept before
// execution.
type Capture struct {
	Criterion  string `json:"criterion" doc:"a criterion, CR-<nnn>-<nn>"`
	Artefact   string `json:"artefact" doc:"what was captured"`
	CapturedTo string `json:"captured_to" doc:"baseline/<name>"`
}

// Grant is what a unit was permitted to do.
type Grant struct {
	ID         string `json:"id" doc:"GR-<nnn>-<nn>"`
	Unit       string `json:"unit" doc:"a work unit, WU-<nnn>-<nn>"`
	Operations string `json:"permitted_operations" doc:"what the unit may do"`
	Targets    string `json:"targets" doc:"where it may do it"`
	Expires    string `json:"expires" doc:"when the permission ends"`
}

// Execution is 04-execution.json: each unit carried out, and what it left.
type Execution struct {
	Schema     int         `json:"schema" doc:"the version of the data model this was written in"`
	Name       string      `json:"name" doc:"the change's folder name, its slug"`
	Runs       []Run       `json:"runs" doc:"each unit's run by its executor"`
	Candidates []Candidate `json:"candidates" doc:"what each unit proposes, kept apart from the estate"`
	// Reproductions are, for a repair, how each criterion stood before it.
	Reproductions []Reproduction `json:"reproductions,omitempty" doc:"for a repair, how each criterion stood before it"`
	Prose         string         `json:"prose,omitempty" doc:"what the document says besides its fields, for one written before the model held it"`
}

// Run is one unit's run by its executor.
type Run struct {
	Unit      string   `json:"unit" doc:"a work unit, WU-<nnn>-<nn>"`
	Executor  string   `json:"executor" doc:"what carried the unit out"`
	Version   string   `json:"version" doc:"the executor's version"`
	Terminal  Terminal `json:"terminal_state" doc:"how the unit's run ended"`
	Candidate string   `json:"candidate,omitempty" doc:"the candidate it left, CA-<nnn>-<nn>"`
	Trace     string   `json:"trace,omitempty" doc:"where the run's steps are kept"`
}

// Candidate is what a unit proposes, kept apart and never applied here.
type Candidate struct {
	ID         string `json:"id" doc:"CA-<nnn>-<nn>"`
	Unit       string `json:"unit" doc:"a work unit, WU-<nnn>-<nn>"`
	StoredAt   string `json:"stored_at" doc:"the candidate's folder, inside the change"`
	Artefacts  string `json:"artefacts" doc:"what the candidate proposes, never applied here"`
	ProducedAt string `json:"produced_at" doc:"when it was produced"`
}

// Reproduction is how a criterion stood before a repair.
type Reproduction struct {
	Unit         string `json:"unit" doc:"a work unit, WU-<nnn>-<nn>"`
	Criterion    string `json:"criterion" doc:"a criterion, CR-<nnn>-<nn>"`
	BeforeRepair string `json:"before_repair" doc:"how the criterion stood before the repair"`
}

// Assurance is 05-assurance.json: the evidence, the verdicts on it, and what
// the change raised and settled.
type Assurance struct {
	Schema   int        `json:"schema" doc:"the version of the data model this was written in"`
	Name     string     `json:"name" doc:"the change's folder name, its slug"`
	Evidence []Evidence `json:"evidence" doc:"the evidence"`
	Verdicts []Verdict  `json:"verdicts" doc:"the verdicts, one for each unit on each criterion"`
	// Raised are the escalations this change raised: the method asking a
	// person, or itself, for what no change here can settle.
	Raised []Escalation `json:"escalations_raised,omitempty" doc:"the escalations this change raised"`
	// Settled are the escalations of earlier changes this one closed.
	Settled  []Settlement `json:"escalations_closed,omitempty" doc:"the escalations of earlier changes this change closed"`
	Refusals []Refusal    `json:"refusals,omitempty" doc:"objects returned across a boundary"`
	Sections []Section    `json:"sections,omitempty" doc:"headed parts the model has no field for, kept as they were written"`
}

// Evidence is one output a verdict may rest on.
type Evidence struct {
	ID           string       `json:"id" doc:"EV-<nnn>-<nn>-<nn>"`
	Subject      Subject      `json:"subject" doc:"what the evidence is about"`
	Kind         EvidenceKind `json:"kind" doc:"what kind of evidence it is"`
	Producer     string       `json:"producer" doc:"what produced it, a command in backticks where it can be run again"`
	Independence Independence `json:"independence" doc:"how far it stands apart from what made the candidate"`
	How          string       `json:"how_established" doc:"how its independence was established"`
	Repeatable   Repeatable   `json:"repeatable,omitempty" doc:"whether the run behind it can be made again; left out by evidence written before the method asked"`
	ObtainedAt   string       `json:"obtained_at" doc:"when it was obtained"`
	ValidUntil   string       `json:"valid_until,omitempty" doc:"what voids it"`
}

// Verdict is one unit judged on one criterion.
type Verdict struct {
	ID        string         `json:"id" doc:"VE-<nnn>-<nn>"`
	Unit      string         `json:"unit" doc:"a work unit, WU-<nnn>-<nn>"`
	Criterion string         `json:"criterion" doc:"a criterion, CR-<nnn>-<nn>"`
	Outcome   VerdictOutcome `json:"outcome" doc:"the outcome"`
	Evidence  []string       `json:"evidence" doc:"the evidence"`
	State     VerdictState   `json:"state" doc:"settled, or deferred to a window and a baseline"`
	Window    string         `json:"window,omitempty" doc:"for a deferred verdict, when it will be settled"`
	Baseline  string         `json:"baseline,omitempty" doc:"for a deferred verdict, what it will be compared against"`
}

// Escalation is one escalation a change raised, addressed <slug>#<n>.
type Escalation struct {
	N        int    `json:"n" doc:"its ordinal"`
	Standing string `json:"standing" doc:"open or closed"`
	Ground   string `json:"ground" doc:"why"`
	Against  string `json:"raised_against" doc:"the area or framework it is raised against"`
	To       string `json:"to" doc:"who must settle it"`
}

// Settlement is an earlier change's escalation this change closed.
type Settlement struct {
	Escalation string `json:"escalation" doc:"<slug>#<n>"`
	ResolvedBy string `json:"resolved_by" doc:"what resolved it"`
	Note       string `json:"note,omitempty" doc:"a note"`
}

// Refusal is an object returned across a boundary.
type Refusal struct {
	ID       string    `json:"id" doc:"RF-<nnn>-<nn>"`
	Object   string    `json:"object" doc:"the object"`
	Boundary string    `json:"boundary" doc:"the boundary it was returned across"`
	By       RefusedBy `json:"refused_by" doc:"who returned it"`
	Ground   string    `json:"ground" doc:"why"`
	At       string    `json:"at" doc:"when, as a date"`
}

// Landing is 06-landing.json: what entered the estate, in what order.
type Landing struct {
	Schema    int         `json:"schema" doc:"the version of the data model this was written in"`
	Name      string      `json:"name" doc:"the change's folder name, its slug"`
	Entries   []Entry     `json:"entries" doc:"each unit entering the estate, in order"`
	Unplanned []Unplanned `json:"unplanned_state,omitempty" doc:"states no plan foresaw, and how each was resolved"`
	// Notes are what the landing says in prose beside its tables.
	Notes []Section `json:"notes,omitempty" doc:"what the document says in prose beside its tables, by heading"`
}

// Entry is one unit entering the estate.
type Entry struct {
	Order         int    `json:"order" doc:"its place in the order of entry"`
	Unit          string `json:"unit" doc:"a work unit, WU-<nnn>-<nn>"`
	VerdictState  string `json:"verdict_state" doc:"the state of the verdicts it entered on"`
	StillValid    string `json:"evidence_still_valid" doc:"yes or no: whether its evidence still held when it entered"`
	EnteredAt     string `json:"entered_at" doc:"when it entered"`
	Invalidated   string `json:"invalidated_by_this_entry" doc:"what this entry voided of other units' evidence"`
	Reestablished string `json:"reestablished_before_next,omitempty" doc:"how what it voided was established again before the next entry"`
}

// Unplanned is a state no plan foresaw, and how it was resolved.
type Unplanned struct {
	Occurred   string `json:"occurred" doc:"what occurred"`
	Halted     string `json:"area_of_effect_halted" doc:"what was halted"`
	ResolvedBy string `json:"resolved_by" doc:"what resolved it"`
}

// Crossing is one line of the record: a stage boundary crossed. The record
// is record.jsonl, one crossing a line, appended to and never rewritten.
type Crossing struct {
	N      int    `json:"n" doc:"the ordinal: 1, 2, 3 … in order, none missing"`
	At     string `json:"at" doc:"when, as a date"`
	Stage  string `json:"stage" doc:"the stage, or closed, or migrated"`
	Object string `json:"object" doc:"the object"`
	Digest string `json:"digest,omitempty" doc:"a digest the crossing fixes"`
	Note   string `json:"note,omitempty" doc:"a note"`
}
