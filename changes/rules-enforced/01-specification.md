# Specification — rules-enforced

| Field | Value |
|---|---|
| Id | SPEC-003 |
| Intent | INT-003 |
| Default class | C3 |

Four rules — R4, R5, R8, B3 — are marked mechanical and no check emits them. One, R6, is marked human while the gate does emit it. A rule that claims an enforcement it lacks is worse than an acknowledged gap, because it is relied upon; this is the defect Y3 named, recurring elsewhere in the same document that named it.

## Acceptance criteria

| Id | Criterion | Procedure | Required evidence | Threshold |
|---|---|---|---|---|
| CR-003-01 | Every rule the framework says it enforces mechanically is enforced | machine | test-run | no rule claims what it lacks |
| CR-003-02 | A violation of each newly enforced rule is actually detected | machine | test-run | each fires on a real violation |
| CR-003-03 | Every rule not enforced mechanically says why it cannot be | machine | test-run | no bare exemption |
| CR-003-04 | The gate's verdict on the changes that already exist is unchanged | machine | test-run | identical, with this repair the only variable |

## Termination

| Condition | Action |
|---|---|
| A rule can be neither enforced nor honestly reclassified | escalate |

## Scope

| Included | Excluded |
|---|---|
| tools/check.py | templates/ |
| framework/RULES.md | changes/ |

## Arbiter

| Path | What it arbitrates |
|---|---|
| tests/ | CR-003-01, CR-003-02, CR-003-03, CR-003-04 |

`framework/RULES.md` is in scope and is also read by the arbiter, so an executor could satisfy CR-003-01 by deleting claims rather than by honouring them. Three things block that path: the reclassification is its own work unit under invariant 8, never accepted by the unit that depends on it; CR-003-03 requires a stated reason for every exemption; and the prior state of the file is captured at admission, so any deletion is visible as a difference. The exposure is recorded rather than assumed away.
