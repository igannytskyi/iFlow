# Admission — <name>

| Field | Value |
|---|---|
| Specification digest | <recorded here; R1 compares against it forever after> |

## Decisions

| Unit | Outcome | Deciding condition | Confidence of the edges relied on | Escalates at |
|---|---|---|---|---|
| WU-<nnn>-01 | admitted · held · refused · awaiting-authority | conflict · allowance · permission | high · medium · low | |

## Conflicts

| Units | Intersecting region | Evidence it would invalidate | Resolution |
|---|---|---|---|
| | | | ordering · escalation |

A cycle among held units is a defect and is reported immediately, not waited out.

## Evidence plan

Required for every criterion marked `required`. Written from the area of effect once that is known, never from the candidate — which does not exist yet.

| Criterion | Observed | Method | Unchanged means | Derived from | Fixed at |
|---|---|---|---|---|---|
| CR-<nnn>-01 | <which component, which interfaces> | <environment, load, repetitions, tolerated variance> | <what counts as unchanged, against which captured prior state> | <the area of effect, and its confidence> | |

## Prior state captured

Where a criterion could only ever be evidenced by the artefact being changed, its prior state is captured here — after execution there is nothing left to compare against.

| Criterion | Artefact | Captured to |
|---|---|---|
| CR-<nnn>-01 | | baseline/<name> |

## Grants

| Id | Unit | Permitted operations | Targets | Expires |
|---|---|---|---|---|
| GR-<nnn>-01 | WU-<nnn>-01 | read · write · execute | | with the run |

A grant is the narrowest set sufficient for the unit's scope. Anything wider is refused, not warned about.
