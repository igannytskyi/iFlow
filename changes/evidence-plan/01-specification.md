# Specification — evidence-plan

| Field | Value |
|---|---|
| Id | SPEC-008 |
| Intent | INT-008 |
| Default class | C3 |

A criterion written at intake is decidable in principle and says nothing about what to run. For a defect whose cause is unknown, which interfaces must hold, what load is representative and what *as before* is measured against are all consequences of a diagnosis that has not happened. Nothing in the framework carries that, so either the criteria get revised after the fix is understood — which the hypothesis forbids — or the second half of a defect criterion has no subject at all.

## Acceptance criteria

| Id | Criterion | When | Then | Plan | Procedure | Required evidence | Threshold |
|---|---|---|---|---|---|---|---|
| CR-008-01 | a criterion needing an evidence plan has one before it is executed against | a criterion declares that it needs an evidence plan | a plan naming what is observed, by what method, and what it was derived from exists before the units judged by it execute | not-required | machine | test-run | detected on a real case |
| CR-008-02 | a plan says what it was derived from | an evidence plan names no area of effect | that is detected, because a plan derived from the change rather than from the estate is the failure this exists to prevent | not-required | machine | test-run | detected |
| CR-008-03 | the gate's verdict on existing changes does not move | the gate runs against every change that already exists | its verdict is unchanged, this repair being the only variable | not-required | machine | test-run | identical |

The scenario columns are **additive**. `Criterion` stays as the one-line statement the gate already binds to; `When` and `Then` make it testable without renaming anything. The first attempt replaced `Criterion` outright, and B1 refused the specification rather than checking nothing — Y6 catching the change that introduces the shape it governs.

## Termination

| Condition | Action |
|---|---|
| Whether a criterion needs a plan cannot be decided without knowing the diagnosis | escalate |

## Scope

| Included | Excluded |
|---|---|
| tools/check.py | changes/ |
| framework/RULES.md | |
| framework/CONVENTIONS.md | |

## Arbiter

| Path | What it arbitrates |
|---|---|
| tests/ | CR-008-01, CR-008-02, CR-008-03 |

## Arbiter reads

| Input | In scope? |
|---|---|
| tools/check.py | yes — the gate is both subject and input |
| changes/ | no |
