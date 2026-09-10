# Specification — criteria-technique

| Field | Value |
|---|---|
| Id | SPEC-010 |
| Intent | INT-010 |
| Default class | C3 |

The method checks that a criterion is decidable and written in the vocabulary of the intent. It offers nothing for arriving at one. That is the gap with the largest consequence, because acceptance establishes conformance and never correctness of intent — a bad criterion is accepted correctly, and no downstream rule can catch it.

What is wanted is not a menu of postures but a set of probes, each aimed at a named way a criterion goes wrong, each answerable before any work exists. And the method's own honesty rule applies to the result: whatever of it is not mechanically checked must say so rather than claim otherwise.

## Acceptance criteria

| Id | Criterion | When | Then | Plan | Procedure | Required evidence | Threshold |
|---|---|---|---|---|---|---|---|
| CR-010-01 | a criterion that can be satisfied by editing what it names is detected | a criterion's statement names something inside the region the change may touch | that is reported, because such a criterion is met by editing the thing it names rather than by achieving anything | not-required | machine | test-run | detected on a real case |
| CR-010-02 | nothing added here claims an enforcement it lacks | the framework's rules are read against the checks that exist | every rule claiming mechanical enforcement is enforced, and every probe that is not says so | not-required | machine | test-run | no rule claims what it lacks |
| CR-010-03 | the gate's verdict on existing changes does not move | the gate runs against every change that already exists | its verdict is unchanged, this addition being the only variable | not-required | machine | test-run | identical |

## Termination

| Condition | Action |
|---|---|
| A probe cannot be stated so that it is answerable before work exists | drop it — a probe answerable only afterwards is a review, not a technique |

## Scope

| Included | Excluded |
|---|---|
| framework/criteria.md | changes/ |
| framework/check.py | |
| framework/rules.md | |

## Arbiter

| Path | What it arbitrates |
|---|---|
| framework/tests/ | CR-010-01, CR-010-02, CR-010-03 |

## Arbiter reads

| Input | In scope? |
|---|---|
| framework/check.py | yes — the gate is both subject and input |
| framework/rules.md | yes — the same |
| changes/ | no |
