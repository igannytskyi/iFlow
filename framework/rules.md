# Rules

The nine invariants of [../docs/iflow.md](../docs/iflow.md) §4, restated so that each names the check that proves it. A rule with no check is an aspiration, and is marked as such.

`M` = checked mechanically by `framework/check.py`, and the gate emits that rule's name when it fails. `H` = not checked mechanically, and the third column says why not rather than leaving the exemption bare.

| # | Rule | Check | How |
|---|---|---|---|
| **R1** | A specification is immutable **from the moment a candidate exists** | `M` | Its digest is recorded at admission and recomputed on every later run. A difference where a candidate exists is a violation; where none does it is a re-admission, permitted but never silent — the record must carry a specification crossing after the admission one |
| **R2** | A candidate is never applied by what produced it | `M` | Every candidate named in execution exists under `changes/<slug>/candidates/<id>/`; landing is the only stage that records an entry |
| **R3** | Evidence states one of the two grounds of independence, and how | `M` | Every evidence row carries a defined `independence` value and a non-empty basis; `not-independent` cannot support a `met` outcome |
| **R3b** | The artefact under change cannot attest to its own behaviour | `M` | Evidence whose producer names a path inside the scope must be `not-independent` |
| **R17** | A refusal names what it returns and why, and nothing it refused is accepted | `M` | A refusal carries an object, a boundary, who refused and a ground. No verdict is `met` on a refused object, nor on work whose candidate was refused. A refusal is the producer's failure, not a request to retry |
| **R18** | Whether a person read the criteria before admission is recorded, either way | `M` | Admission says `yes` with a name, or `no`. Recording `no` is legitimate; the rule exists to make the question answerable, never to mandate the reading. A change that says nothing predates the obligation and is noted |
| **R12** | A rule nothing would notice the loss of is not being tested | `M` | `check.py --mutate` removes each rule in turn and runs every arbiter against the result. A rule whose removal nothing notices is reported. This replaced a record in which a unit attested that its arbiter had once failed for its stated reason: the demonstration is repeated now rather than believed about a run that has ended |
| **R20** | An arbiter says which of its criteria it tests directly and which through a proxy | `M` | Checked by `check.py --arbiters`. A proxy stands for its criterion and is not it: this one has broken three times, each time silently, and each time the rule was fine. A proxy declared without saying what it stands for is an error |
| **R19** | A criterion does not name what the change may touch | `M` | The criterion's text is searched for the paths in its own scope. A criterion naming one can be met by editing that thing rather than by achieving anything. The probe is deliberately crude; the six other probes in `criteria.md` are questions, not checks, and are listed there as such |
| **R16** | A criterion that needs an evidence plan has one, derived from the estate | `M` | The specification says per criterion whether a plan is needed; a needed plan must exist in admission and must name what is observed, by what method, and the area of effect it was derived from. A specification that does not declare this predates the obligation and is noted |
| **R15** | A debt is settled once, and only if it was owed | `M` | Computed across every change by `check.py --escalations`, not by the per-folder gate, which cannot see beyond the folder it was given. Settling something never raised, or settling it twice, is an error |
| **R13** | A record of a run says whether the run can be produced again | `M` | Every evidence row carries a defined `Repeatable` value. A change whose evidence has no such column predates the obligation and is noted |
| **R14** | A result does not rest on an unrepeatable run alone | `M` | A `met` verdict citing only evidence marked `no` or `unknown` is an error |
| **R11** | Every criterion is arbitrated by something named | `M` | The criteria in the specification are intersected with what the arbiter table claims; a criterion nothing claims is not being tested and nothing else would say so |
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
| **B3** | A consumer may refuse, and refusal is the producer's failure | `M` | Enforced as R17 |
| **B4** | Every crossing is recorded | `M` | A stage present without its line in the record is an error |

## What is deliberately not enforced

- **Six of the seven probes for arriving at a criterion.** `criteria.md` names one mechanical check and six questions. The questions are answerable only by whoever writes or reviews the criteria, and are recorded here so that their absence from the gate is not mistaken for their absence from the method.

Stated so that absence is not mistaken for oversight.

- **Whether a criterion was the right criterion.** Acceptance is conformance, not correctness of intent. No check can reach it; it surfaces as a lag to discovery.
- **Whether an attested statement is true.** Testimony carries its source and its falsifier. Nothing here can test it.
- **Whether a reversibility horizon is real.** R6 records the claim and cannot verify it.
