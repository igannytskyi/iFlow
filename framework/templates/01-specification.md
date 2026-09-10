# Specification — <name>

| Field | Value |
|---|---|
| Id | SPEC-<nnn> |
| Intent | INT-<nnn> |
| Default class | C1P · C1T · C2 · C3 · C4 · C5 |
| Digest at admission | <filled by admit; empty before> |

## Acceptance criteria

| Id | Criterion | When | Then | Plan | Procedure | Required evidence | Threshold |
|---|---|---|---|---|---|---|---|
| CR-<nnn>-01 | <one line, in the vocabulary of the intent, never of the implementation> | <conditions under which it is judged> | <what must be true> | required · not-required | machine · human | test-run · static-analysis · runtime-observation · human-affirmation · transformation-proof | |

## Termination

| Condition | Action |
|---|---|
| <what makes work stop without acceptance> | stop · escalate |

## Scope

| Included | Excluded |
|---|---|
| <paths that may be touched> | <paths that may not> |

## Arbiter

Paths holding what will judge this change — tests, schemas, telemetry definitions, criteria. **Must not intersect Scope/Included** (R7).

| Path | What it arbitrates |
|---|---|
| | |
