# Admission — rules-enforced

| Field | Value |
|---|---|
| Specification digest | ba4b181f9ecb5ef7 |

## Decisions

| Unit | Outcome | Deciding condition | Confidence of the edges relied on | Escalates at |
|---|---|---|---|---|
| WU-003-01 | admitted | permission | high | |
| WU-003-02 | admitted | conflict | high | |
| WU-003-03 | admitted | conflict | high | |

## Conflicts

| Units | Intersecting region | Evidence it would invalidate | Resolution |
|---|---|---|---|
| WU-003-02, WU-003-03 | none; one touches the gate, the other the rules | none | ordering |

## Prior state captured

| Criterion | Artefact | Captured to |
|---|---|---|
| CR-003-01 | framework/RULES.md before any reclassification | baseline/RULES.md.before |
| CR-003-04 | tools/check.py as it stands before the repair | baseline/check.py.before |
| CR-003-04 | the gate's verdict on the existing changes | baseline/gate-before.txt |

Captured before execution, as Y1 now requires. Without the first of these, CR-003-01 could be met by deleting the claims instead of honouring them, and nothing would show it.

## Grants

| Id | Unit | Permitted operations | Targets | Expires |
|---|---|---|---|---|
| GR-003-01 | WU-003-01 | read, write | tests/ | with the run |
| GR-003-02 | WU-003-02 | read, write | tools/check.py | with the run |
| GR-003-03 | WU-003-03 | read, write | framework/RULES.md | with the run |
