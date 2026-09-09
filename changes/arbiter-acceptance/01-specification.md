# Specification — arbiter-acceptance

| Field | Value |
|---|---|
| Id | SPEC-005 |
| Intent | INT-005 |
| Default class | C3 |

Twice in succession an arbiter had to be corrected before acceptance, both times because it compared more than its criterion named and so failed for a reason unrelated to the subject. The requirement that a reproduction fail for the stated reason already existed, but only for defect repair. It belongs to every arbiter. A second gap follows from the same place: nothing checks that each criterion is claimed by an arbiter at all, so a criterion could be tested by nothing and the gate would not notice.

## Acceptance criteria

| Id | Criterion | Procedure | Required evidence | Threshold |
|---|---|---|---|---|
| CR-005-01 | A criterion that nothing claims to arbitrate is detected | machine | test-run | detected on a real case |
| CR-005-02 | Where a unit builds an arbiter, its own pre-acceptance outcome is recorded per criterion | machine | test-run | absence detected, or noted where it predates |
| CR-005-03 | The gate's verdict on the changes that already exist is unchanged | machine | test-run | identical, this repair the only variable |

## Termination

| Condition | Action |
|---|---|
| An arbiter's fidelity to its criterion cannot be checked from artefacts | escalate, and say so in the rules rather than claim it |

## Scope

| Included | Excluded |
|---|---|
| tools/check.py | changes/ |
| framework/RULES.md | templates/04-execution.md |

## Arbiter

| Path | What it arbitrates |
|---|---|
| tests/ | CR-005-01, CR-005-02, CR-005-03 |

## Arbiter reads

| Input | In scope? |
|---|---|
| tools/check.py | yes — the gate is both subject and input |
| changes/ | no |
