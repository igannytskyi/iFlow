# Conventions

Every code used anywhere in iFlow is defined here, once. Tools validate against exactly these values; a value not listed here is an error, not a variation.

Derived from [../docs/iflow.md](../docs/iflow.md). Where the two disagree, the research document is the authority and this file is the defect.

## Identifiers

| Object | Pattern | Example |
|---|---|---|
| Intent | `INT-<nnn>` | `INT-003` |
| Specification | `SPEC-<nnn>` | `SPEC-003` |
| Criterion | `CR-<nnn>-<nn>` | `CR-003-02` |
| Work unit | `WU-<nnn>-<nn>` | `WU-003-07` |
| Grant | `GR-<nnn>-<nn>` | `GR-003-07` |
| Candidate | `CA-<nnn>-<nn>` | `CA-003-07` |
| Evidence | `EV-<nnn>-<nn>-<nn>` | `EV-003-07-01` |
| Verdict | `VE-<nnn>-<nn>` | `VE-003-07` |

The `<nnn>` of a unit, grant, candidate, verdict and evidence is always the intent it descends from. An identifier is stable: it survives re-creation of what it names, per O1.

## Change class

| Code | Class | Arbiter |
|---|---|---|
| `C1P` | Behaviour-preserving, proved | The transformation's own proof |
| `C1T` | Behaviour-preserving, tested | The prior system, observed |
| `C2` | Defect repair | A reproduction, plus C1's arbiter for the rest |
| `C3` | Contract-bounded | The stated contract |
| `C4` | Observable-effect | Production against a baseline; verdict deferred |
| `C5` | Judgment-bound | A person; no arbiter can be built |

A class is carried by the **work unit**. A specification carries only a default. An equivalence claim resting on testimony makes the class `C1T`, never `C1P`.

## Provenance

| Code | Meaning |
|---|---|
| `derived` | Established mechanically from an artifact. States which artifact and which method |
| `attested` | Held on a person's word — `Testimony`. States who and when |
| `produced` | Output of an agent or transformation. States which executor and version |

## Confidence

| Code | Meaning |
|---|---|
| `high` | Derived, and the derivation is total over its input |
| `medium` | Derived, but the derivation has known blind spots — reflection, dynamic dispatch, configuration |
| `low` | Attested, matched, or inferred |

`high` is not available to an `attested` statement at any time, by definition.

## Criterion

A criterion is stated as a scenario: **when** these conditions hold, **then** this must be true. The scenario columns are additive — `Criterion` remains the one-line statement, `When` and `Then` make it testable without renaming it.

| Field | Values |
|---|---|
| Decision procedure | `machine` · `human` |
| Required evidence kind | see below |
| Plan | `required` · `not-required` — whether settling it needs an evidence plan |

A criterion is written in the vocabulary of the symptom, never of the implementation: *when a null shipping address is submitted, then the order is rejected rather than 500* — not *the null check is added*.

## Evidence plan

What will be observed to settle one criterion, and how. It cannot be written at intake where the subject follows from a diagnosis that has not happened: which interfaces are in the area of effect, what load is representative, what environment, how many repetitions, what variance is tolerated, and what *as before* is measured against.

| Field | Meaning |
|---|---|
| Criterion | the criterion it settles |
| Observed | which component, which interfaces |
| Method | environment, load, repetitions, tolerated variance |
| Unchanged means | what counts as unchanged, and against which captured prior state |
| Derived from | the area of effect it was computed from, and that area's confidence |
| Fixed at | the moment it stopped being editable |

**A plan is derived from the area of effect, never from the candidate** — the candidate does not exist yet, and a plan shaped around one settles nothing. It is fixed at its own moment, later than the specification, and is immutable from then. It expires with the area of effect it was derived from.

A specification is complete when every criterion is `machine`, or is `human` and says so explicitly. A criterion is written in the vocabulary of the intent, not of the implementation.

## Evidence

| Field | Values |
|---|---|
| Subject | `candidate` · `transformation` |
| Kind | `test-run` · `static-analysis` · `runtime-observation` · `human-affirmation` · `transformation-proof` |
| Independence | `independent-by-executor` · `independent-by-precommitment` · `not-independent` — and how it is established |
| Repeatable | `yes` · `no` · `unknown` — whether the run behind it can be produced again to the same result |

Independence has exactly two grounds. `independent-by-executor` — produced by something other than the executor that produced the candidate. `independent-by-precommitment` — fixed, and its own acceptance recorded, before that candidate existed, so that it could not have been shaped to fit. The second is the stronger: a second executor may share the first's blind spots, while a thing written before the work cannot have been bent around it.

Evidence whose producer lies **inside the scope of the change** is `not-independent` whatever else is claimed, because the artefact under change cannot attest to its own behaviour.

A record of a past run is `derived` only where that run can be produced again to the same result: a deterministic transformation on fixed inputs qualifies, an agent's run does not. Where it cannot be repeated the record is `attested` — testimony about something that happened once — and `unknown` is an honest answer that is treated as `no`. **A verdict is not `met` on unrepeatable evidence alone.**

Evidence about a `transformation` is amortized across every application of it, and any edit to that transformation invalidates every verdict resting on it.

## Verdict

| Field | Values |
|---|---|
| Per-criterion outcome | `met` · `failed` · `undecided` |
| State | `settled` · `deferred` |

`undecided` is not `failed`. A `deferred` verdict carries an observation window and a baseline, and holds its candidate reversible until it closes.

## Admission outcome

| Code | Meaning |
|---|---|
| `admitted` | Conflict, allowance and permission all settled |
| `held` | One of the three failed; states which, and the age at which it escalates |
| `refused` | It cannot be admitted at all; states why |
| `awaiting-authority` | Blocked on another organization. Not a hold: no gate here controls the outcome |

## Unit terminal state

| Code | Meaning |
|---|---|
| `candidate` | A candidate was produced |
| `terminated` | The termination condition fired as written |
| `exhausted` | The step budget ran out. Diagnosed before it is retried |
| `failed-substrate` | Tool error, capacity, timeout. Says nothing about the work |
| `failed-task` | The criteria could not be satisfied. Says something about the work |
| `cancelled` | The intent was withdrawn. Still charged |

## Scope

A scope lists paths that may be touched and paths that may not. It must be **disjoint from the arbiter** — the tests, schemas, telemetry definitions and criteria by which this change will be judged. A scope intersecting its own arbiter is malformed and is rejected at admission, not warned about.

## Escalation

An escalation is addressed as `<change-slug>#<n>` — its number is an ordinal local to the change that raised it, and a change that resolves one is almost never the change that raised it, because resolving usually means changing the framework.

| Field | Values |
|---|---|
| Standing | `open` · `closed` — as recorded where it was raised |
| Settled by | a change records `<slug>#<n>` in its own **Escalations closed** table |

What the framework still owes itself is `--escalations`: every escalation raised, minus every one a later change has settled. A change may close its own escalation by marking it `closed` where it was raised; anything resolved elsewhere is settled from there.

## Refusal

A person or an area may return an object to its producer at any boundary. A refusal is **the producer's failure**, not a request to try again, and it is recorded where it was made.

| Field | Values |
|---|---|
| Object | what is returned — a candidate, a plan, a specification |
| Boundary | which stage returned it to which |
| Refused by | `person` · `area` |
| Ground | why. A refusal without one is not a refusal |

Nothing refused is accepted: no verdict is `met` on a refused object or on work whose candidate was refused. **Escalation is the system asking for a person; refusal is a person acting.** The first fires by rule; the second is available at any time and costs nothing when unused.

## Criteria review

Reading the criteria before admission is the cheapest minute in the path: one small artefact, nothing yet spent, and the only point at which the right answer to the wrong question is still preventable.

| Field | Values |
|---|---|
| Reviewed before admission | `yes` · `no` |
| By | who, where `yes` |

**Recording `no` is legitimate.** The rule exists so the question can be answered later — whether reading the criteria beforehand actually lowers the rate of wrongly accepted change is not known, and cannot be found out while nobody records whether it happened.
