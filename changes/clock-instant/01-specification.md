# Specification — clock-instant

| Field | Value |
|---|---|
| Id | SPEC-001 |
| Intent | INT-001 |
| Default class | C1T |

The library documents the two calls as returning the same value. That claim is `attested`, not derived, so the class is C1T and not C1P.

## Acceptance criteria

| Id | Criterion | Procedure | Required evidence | Threshold |
|---|---|---|---|---|
| CR-001-01 | No statically resolvable use of the deprecated call remains in scope | machine | static-analysis | zero occurrences |
| CR-001-02 | Every service still reports the same instant for the same input as before the change | machine | test-run | no observable difference |

Reflective and string-formed invocation is not statically decidable. CR-001-01 is written to what is decidable, and the residue is stated here rather than absorbed into a confident total.

## Termination

| Condition | Action |
|---|---|
| A use the transformation cannot resolve mechanically | stop |

## Scope

| Included | Excluded |
|---|---|
| src/ | tests/ |
| lib/ | schema/ |

## Arbiter

| Path | What it arbitrates |
|---|---|
| tests/ | CR-001-02 |
| schema/ | CR-001-01 |
