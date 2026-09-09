# Specification — arbiter-influence

| Field | Value |
|---|---|
| Id | SPEC-004 |
| Intent | INT-004 |
| Default class | C3 |

Two defects of one family. The first: disjointness between a change and its arbiter is stated over paths, while what actually decides an arbiter's conclusion is everything it reads — and an input inside the scope is as good as a writable arbiter. The second: every check in the gate binds to the name of a column, so renaming a column disables that check silently and the gate goes on reporting that it passed. That is the defect of run 6 for the third time, now in the gate's own binding.

## Acceptance criteria

| Id | Criterion | Procedure | Required evidence | Threshold |
|---|---|---|---|---|
| CR-004-01 | Where a change may edit something its arbiter reads, that is detected and the prescribed mitigations are required | machine | test-run | detected on a real case |
| CR-004-02 | A check whose column is missing says so instead of passing silently | machine | test-run | every dependency named and absence reported |
| CR-004-03 | An obligation added after a specification was admitted is reported as predating it, not as a violation | machine | test-run | no retroactive failure |
| CR-004-04 | The gate's verdict on the changes that already exist is unchanged | machine | test-run | identical, this repair the only variable |

## Termination

| Condition | Action |
|---|---|
| A dependency cannot be named without changing a specification already admitted | escalate |

## Scope

| Included | Excluded |
|---|---|
| tools/check.py | changes/ |
| framework/RULES.md | templates/01-specification.md |

## Arbiter

| Path | What it arbitrates |
|---|---|
| tests/ | CR-004-01, CR-004-02, CR-004-03, CR-004-04 |

## Arbiter reads

| Input | In scope? |
|---|---|
| framework/RULES.md | yes — the exposure this change exists to name |
| tools/check.py | yes — the same |
| changes/ | no |
