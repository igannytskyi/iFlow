# Assurance — <name>

## Evidence

| Id | Subject | Kind | Producer | Independence | How established | Repeatable | Obtained at | Valid until |
|---|---|---|---|---|---|---|---|---|
| EV-<nnn>-01-01 | candidate · transformation | | | independent-by-executor · independent-by-precommitment · not-independent | | yes · no · unknown | | |

Evidence about a `transformation` carries that transformation's digest and is amortized across every application. `not-independent` evidence cannot support a `met` outcome, and neither can evidence whose run cannot be produced again.

## Verdicts

| Id | Unit | Criterion | Outcome | Evidence | State | Window | Baseline |
|---|---|---|---|---|---|---|---|
| VE-<nnn>-01 | WU-<nnn>-01 | CR-<nnn>-01 | met · failed · undecided | EV-<nnn>-01-01 | settled · deferred | | |

`undecided` is not `failed`. A deferred verdict names its observation window and its baseline, and holds its candidate reversible until it closes.

## Escalations closed

Only where this change settles a debt raised by another. An escalation is addressed as `<slug>#<n>`.

| Escalation | Resolved by | Note |
|---|---|---|
| <slug>#<n> | this change | |

## Refusals

Only where a person or an area returned an object to its producer. A refusal is the producer's failure, not a request to retry, and nothing it refused is accepted.

| Id | Object | Boundary | Refused by | Ground | At |
|---|---|---|---|---|---|
| RF-<nnn>-01 | <the object returned> | execution to assurance | person · area | <why> | |
