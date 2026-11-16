package meta

import "slices"

// Every code a field may hold, as docs/method/conventions.md defines it and
// nowhere else. A value not listed is an error, not a variation.

// IntentStatus is where an intent stands.
type IntentStatus string

// Class is a change class: what can arbitrate the change.
type Class string

// PlanNeed says whether settling a criterion needs an evidence plan.
type PlanNeed string

// Procedure is how a criterion is decided.
type Procedure string

// EvidenceKind is what kind of evidence settles a criterion.
type EvidenceKind string

// Reviewed says whether the criteria were read before admission.
type Reviewed string

// AdmissionOutcome is what admission decided of a unit.
type AdmissionOutcome string

// Terminal is how a unit's run ended.
type Terminal string

// Subject is what a piece of evidence is about.
type Subject string

// Independence is how far evidence stands apart from what made the candidate.
type Independence string

// Repeatable says whether the run behind a piece of evidence can be made again.
type Repeatable string

// VerdictOutcome is what a verdict found.
type VerdictOutcome string

// VerdictState says whether a verdict is settled or waits on later evidence.
type VerdictState string

// RefusedBy is who returned an object across a boundary.
type RefusedBy string

// Provenance is how a statement is known.
type Provenance string

// Confidence is how far a derivation can be trusted.
type Confidence string

// Codes is every code by its type's name in the model, with the values it
// may hold, in the order conventions.md gives them.
var Codes = map[string][]string{
	"IntentStatus":     {"open", "satisfied", "withdrawn"},
	"Class":            {"C1P", "C1T", "C2", "C3", "C4", "C5"},
	"PlanNeed":         {"required", "not-required"},
	"Procedure":        {"machine", "human"},
	"EvidenceKind":     {"test-run", "static-analysis", "runtime-observation", "human-affirmation", "transformation-proof"},
	"Reviewed":         {"yes", "no"},
	"AdmissionOutcome": {"admitted", "held", "refused", "awaiting-authority"},
	"Terminal":         {"candidate", "terminated", "exhausted", "failed-substrate", "failed-task", "cancelled"},
	"Subject":          {"candidate", "transformation"},
	"Independence":     {"independent-by-executor", "independent-by-precommitment", "not-independent"},
	"Repeatable":       {"yes", "no", "unknown"},
	"VerdictOutcome":   {"met", "failed", "undecided"},
	"VerdictState":     {"settled", "deferred"},
	"RefusedBy":        {"person", "area"},
	"Provenance":       {"derived", "attested", "produced"},
	"Confidence":       {"high", "medium", "low"},
}

// Allowed reports whether v is one of the values of the code named.
func Allowed(code, v string) bool { return slices.Contains(Codes[code], v) }

// The values a writer names in code.
const (
	Open      IntentStatus = "open"
	Satisfied IntentStatus = "satisfied"
	Withdrawn IntentStatus = "withdrawn"

	Required    PlanNeed = "required"
	NotRequired PlanNeed = "not-required"

	Machine Procedure = "machine"
	Human   Procedure = "human"

	TestRun EvidenceKind = "test-run"

	Admitted AdmissionOutcome = "admitted"
	Held     AdmissionOutcome = "held"

	CandidateEnd Terminal = "candidate"

	OfCandidate Subject = "candidate"

	ByExecutor      Independence = "independent-by-executor"
	ByPrecommitment Independence = "independent-by-precommitment"
	NotIndependent  Independence = "not-independent"

	Yes     Repeatable = "yes"
	No      Repeatable = "no"
	Unknown Repeatable = "unknown"

	Met       VerdictOutcome = "met"
	Failed    VerdictOutcome = "failed"
	Undecided VerdictOutcome = "undecided"

	Settled  VerdictState = "settled"
	Deferred VerdictState = "deferred"
)
