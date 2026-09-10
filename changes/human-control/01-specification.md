# Specification — human-control

| Field | Value |
|---|---|
| Id | SPEC-009 |
| Intent | INT-009 |
| Default class | C3 |

Three gaps, all about the person rather than the machinery.

The framework has escalation — the system deciding it needs someone — and nothing for the reverse: a person who looked, disagreed, and wants the work returned. That mechanism was named as B3 and reclassified as unenforceable because refusal is not an object.

Immutability is stated from admission, but its reason is that criteria must not be shaped by what execution produced. Before a candidate exists that risk does not exist, so the rule is stricter than the thing it protects, and correcting a criterion costs a whole new intent when it should cost an edit.

And nothing invites anyone to read the criteria before admission — the one moment where a minute of attention is cheap, nothing has been spent, and the wrong-question failure is still preventable. Whether that minute was spent is not recorded, so nobody can ever find out whether it helps.

## Acceptance criteria

| Id | Criterion | When | Then | Plan | Procedure | Required evidence | Threshold |
|---|---|---|---|---|---|---|---|
| CR-009-01 | a refusal is recorded and its producer is not treated as complete | a consumer refuses an object at a boundary | the refusal names the object, the boundary and its ground, and nothing downstream of it is accepted | not-required | machine | test-run | detected on a real case |
| CR-009-02 | correcting criteria is free until a candidate exists and impossible after | a specification differs from the digest recorded at admission | it is a recorded re-admission where no candidate exists, and a violation where one does | not-required | machine | test-run | both cases distinguished |
| CR-009-03 | whether a person read the criteria before admission is recorded either way | a change reaches admission | admission says whether the criteria were reviewed beforehand, and by whom, or says plainly that they were not | not-required | machine | test-run | silence detected |
| CR-009-04 | the gate's verdict on existing changes does not move | the gate runs against every change that already exists | its verdict is unchanged, this repair being the only variable | not-required | machine | test-run | identical |

## Termination

| Condition | Action |
|---|---|
| Recording a review cannot be distinguished from requiring one | escalate — the point is to measure whether it helps, never to mandate it |

## Scope

| Included | Excluded |
|---|---|
| tools/check.py | changes/ |
| framework/RULES.md | |
| framework/CONVENTIONS.md | |

## Arbiter

| Path | What it arbitrates |
|---|---|
| tests/ | CR-009-01, CR-009-02, CR-009-03, CR-009-04 |

## Arbiter reads

| Input | In scope? |
|---|---|
| tools/check.py | yes — the gate is both subject and input |
| changes/ | no |
