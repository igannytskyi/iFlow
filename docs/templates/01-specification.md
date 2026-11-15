# Specification — <name>

| Field | Value |
|---|---|
| Id | SPEC-<nnn> |
| Intent | INT-<nnn> |
| Default class | C1P · C1T · C2 · C3 · C4 · C5 |
| Digest at admission | <filled by admit; empty before> |

<one sentence summing the change up>

## The change

The change described for whoever reads it before accepting it, and for agents who build from it seeing only their part. Sections with nothing to say are left out.

### Context
<the problem, why the change is wanted, what exists now>

### Goals
- <what the change achieves>

### Non-goals
- <what it deliberately does not do>

### How a person uses it
- <every path, step by step: the ordinary one and each alternative>

### Requirements
- FR-1 <one testable statement>

### Quality — security, errors, data, performance
- <as far as it matters here>

### Design
- <data kept and where; interfaces with their methods and signatures; routes or commands with inputs and outputs; which file holds what; what tests stand in for>

### Edge cases
- <wrong input, repetition, expiry, absence, refusal, concurrency — each with what happens>

### Dependencies
- <inside the estate or out>

### Risks
- <what could go wrong or be misread>

### Open questions
- <only what the person who asked must decide>

### How it is tested
- <which tests judge which criteria, at which entry point, with which fakes>

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
