# Rules

The nine invariants of [../docs/iflow.md](../docs/iflow.md) §4, restated so that each names the check that proves it. A rule with no check is an aspiration, and is marked as such.

`M` = checked mechanically by `tools/check.py`. `H` = checked by a person, because no artifact carries what would decide it.

| # | Rule | Check | How |
|---|---|---|---|
| **R1** | A specification is immutable after admission | `M` | Its digest is recorded at admission and recomputed on every later run |
| **R2** | A candidate is never applied by what produced it | `M` | Every candidate named in execution exists under `changes/<slug>/candidates/<id>/`; landing is the only stage that records an entry |
| **R3** | Evidence states one of the two grounds of independence, and how | `M` | Every evidence row carries a defined `independence` value and a non-empty basis; `not-independent` cannot support a `met` outcome |
| **R3b** | The artefact under change cannot attest to its own behaviour | `M` | Evidence whose producer names a path inside the scope must be `not-independent` |
| **R10** | Nothing enters on an undecided criterion | `M` | A unit with any `undecided` verdict has no entry in landing; `deferred` is not `undecided` |
| **R4** | A trace is not writable by its subject | `M` | The record is append-only: prior lines must match their previous digest |
| **R5** | Evidence about a transformation is amortized, and invalidated by any edit to it | `M` | Transformation evidence carries the transformation's digest; a changed digest invalidates every verdict citing it |
| **R6** | A deferred verdict lives inside a reversibility horizon | `H` | The window and the horizon are both recorded; whether the horizon is real cannot be derived from artifacts |
| **R7** | Scope and the arbiter are disjoint | `M` | The scope's paths are intersected with the arbiter's paths; any overlap is an error |
| **R8** | A change to the arbiter is a separate unit, never accepted by the unit depending on it | `M` | A unit whose candidate touches arbiter paths is rejected; such work is a unit of its own |
| **R9** | The framework is not its own arbiter | `H` | Changes to `framework/`, `tools/` and the record are arbitrated outside iFlow |

## Rules of the boundary

From §7. All four are mechanical.

| # | Rule | How it is checked |
|---|---|---|
| **B1** | Only objects cross | Each stage file must parse against its template's tables; free prose in a data column is an error |
| **B2** | A consumer never reaches back to ask | A stage file may only cite identifiers defined in an earlier stage |
| **B3** | A consumer may refuse, and refusal is the producer's failure | A refusal is recorded against the producing stage, not as a request |
| **B4** | Every crossing is recorded | Each stage transition appends one line to the record; a stage present without its line is an error |

## What is deliberately not enforced

Stated so that absence is not mistaken for oversight.

- **Whether a criterion was the right criterion.** Acceptance is conformance, not correctness of intent. No check can reach it; it surfaces as a lag to discovery.
- **Whether an attested statement is true.** Testimony carries its source and its falsifier. Nothing here can test it.
- **Whether a reversibility horizon is real.** R6 records the claim and cannot verify it.
