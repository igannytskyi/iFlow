# Specification — placeholder-filter

| Field | Value |
|---|---|
| Id | SPEC-002 |
| Intent | INT-002 |
| Default class | C2 |

The gate drops rows it takes for unfilled template placeholders. The test is too coarse: any row whose joined text contains both an opening and a closing angle bracket is discarded. An acceptance criterion stating a two-sided bound — "queue depth stays > 0 and < 100" — is therefore discarded, no verdict is ever demanded for it, and the gate reports `passed`.

This is the failure mode the framework exists to prevent, in the gate itself: not a wrong answer, but a check that silently did not run.

## Acceptance criteria

| Id | Criterion | Procedure | Required evidence | Threshold |
|---|---|---|---|---|
| CR-002-01 | A row a person wrote is judged, whatever punctuation its text contains | machine | test-run | no written row is dropped |
| CR-002-02 | An unfilled template row is still not mistaken for data | machine | test-run | every template row dropped |
| CR-002-03 | The gate's verdict on every change that already exists is unchanged | machine | test-run | identical output |

## Termination

| Condition | Action |
|---|---|
| Written rows and unfilled rows cannot be told apart without changing the templates themselves | stop |

## Scope

| Included | Excluded |
|---|---|
| tools/check.py | templates/ |

## Arbiter

| Path | What it arbitrates |
|---|---|
| tests/ | CR-002-01, CR-002-02, CR-002-03 |
