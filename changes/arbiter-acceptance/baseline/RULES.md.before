# Rules

The nine invariants of [../docs/iflow.md](../docs/iflow.md) §4, restated so that each names the check that proves it. A rule with no check is an aspiration, and is marked as such.

`M` = checked mechanically by `tools/check.py`, and the gate emits that rule's name when it fails. `H` = not checked mechanically, and the third column says why not rather than leaving the exemption bare.

| # | Rule | Check | How |
|---|---|---|---|
| **R1** | A specification is immutable after admission | `M` | Its digest is recorded at admission and recomputed on every later run |
| **R2** | A candidate is never applied by what produced it | `M` | Every candidate named in execution exists under `changes/<slug>/candidates/<id>/`; landing is the only stage that records an entry |
| **R3** | Evidence states one of the two grounds of independence, and how | `M` | Every evidence row carries a defined `independence` value and a non-empty basis; `not-independent` cannot support a `met` outcome |
| **R3b** | The artefact under change cannot attest to its own behaviour | `M` | Evidence whose producer names a path inside the scope must be `not-independent` |
| **R10** | Nothing enters on an undecided criterion | `M` | A unit with any `undecided` verdict has no entry in landing; `deferred` is not `undecided` |
| **R4** | A trace is not writable by its subject | `M` | The record's ordinals must form 1..N in order, so a line removed or reordered is detected. A line removed from the tail is not: that needs a digest chain the record does not carry |
| **R5** | Evidence about a transformation is amortized, and invalidated by any edit to it | `H` | Evidence rows carry no digest of the transformation they attest to, so there is nothing to compare against. Enforceable once they do; claimed mechanical before it was true |
| **R6** | A deferred verdict records its window and its baseline | `M` | Both fields must be present on a deferred verdict. Whether the horizon it names is real is not derivable from artifacts and is not claimed here |
| **R7** | Scope and the arbiter are disjoint | `M` | The scope's paths are intersected with the arbiter's paths; any overlap is an error |
| **R7b** | Scope is disjoint from what the arbiter reads, or the mitigations are in place | `M` | A specification declares its arbiter's inputs; an input inside the scope must have its prior state captured at admission. A specification declaring no inputs predates the obligation and is noted, not failed |
| **R8** | A change to the arbiter is a separate unit, never accepted by the unit depending on it | `M` | A candidate whose artefacts name an arbiter path is allowed only where its unit is preparatory for another |
| **R9** | The framework is not its own arbiter | `H` | Changes to `framework/`, `tools/` and the record are arbitrated outside iFlow |

## Rules of the boundary

From §7.

| # | Rule | Check | How it is checked, or why it is not |
|---|---|---|---|
| **B1** | Only objects cross | `M` | Stage files must parse as tables, every coded value must be one CONVENTIONS defines, and every column a check binds to must be present — a renamed column disables a check silently otherwise |
| **B2** | A consumer never reaches back to ask | `M` | A stage may only cite identifiers defined in an earlier stage |
| **B3** | A consumer may refuse, and refusal is the producer's failure | `H` | Refusal is not yet an object: no artefact records one, so there is nothing to check. Claimed mechanical before it was true |
| **B4** | Every crossing is recorded | `M` | A stage present without its line in the record is an error |

## What is deliberately not enforced

Stated so that absence is not mistaken for oversight.

- **Whether a criterion was the right criterion.** Acceptance is conformance, not correctness of intent. No check can reach it; it surfaces as a lag to discovery.
- **Whether an attested statement is true.** Testimony carries its source and its falsifier. Nothing here can test it.
- **Whether a reversibility horizon is real.** R6 records the claim and cannot verify it.
