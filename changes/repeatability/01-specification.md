# Specification — repeatability

| Field | Value |
|---|---|
| Id | SPEC-006 |
| Intent | INT-006 |
| Default class | C3 |

The framework divides knowledge by whether it is derived from artefacts or attested by someone, and treats a record of a past run as though the first always applied. It does not. What decides the status of such a record is whether the run can be repeated: a deterministic transformation on fixed inputs can be re-run to the same result, and an agent's run cannot. The second is testimony about something that happened once, however carefully written down.

## Acceptance criteria

| Id | Criterion | Procedure | Required evidence | Threshold |
|---|---|---|---|---|
| CR-006-01 | Evidence says whether the run behind it can be produced again | machine | test-run | absence detected, or noted where it predates |
| CR-006-02 | A result is not accepted on the strength of a run nobody can repeat alone | machine | test-run | detected on a real case |
| CR-006-03 | The gate's verdict on the changes that already exist is unchanged | machine | test-run | identical, this repair the only variable |

## Termination

| Condition | Action |
|---|---|
| Repeatability cannot be determined from the artefacts for some kind of evidence | escalate, and record it as unknown rather than assume either way |

## Scope

| Included | Excluded |
|---|---|
| tools/check.py | changes/ |
| framework/CONVENTIONS.md | templates/ |

## Arbiter

| Path | What it arbitrates |
|---|---|
| tests/ | CR-006-01, CR-006-02, CR-006-03 |

## Arbiter reads

| Input | In scope? |
|---|---|
| tools/check.py | yes — the gate is both subject and input |
| changes/ | no |
