# Admission — repeatability

| Field | Value |
|---|---|
| Specification digest | 9d928e5d3f67108f |

## Decisions

| Unit | Outcome | Deciding condition | Confidence of the edges relied on | Escalates at |
|---|---|---|---|---|
| WU-006-01 | admitted | permission | high | |
| WU-006-02 | admitted | conflict | high | |
| WU-006-03 | admitted | conflict | high | |

## Conflicts

| Units | Intersecting region | Evidence it would invalidate | Resolution |
|---|---|---|---|
| WU-006-02, WU-006-03 | none | none | ordering |

## Prior state captured

| Criterion | Artefact | Captured to |
|---|---|---|
| CR-006-03 | tools/check.py before this change | baseline/check.py.before |
| CR-006-01 | framework/CONVENTIONS.md before this change | baseline/CONVENTIONS.md.before |
| CR-006-03 | the gate's verdict on every existing change | baseline/gate-before.txt |

## Grants

| Id | Unit | Permitted operations | Targets | Expires |
|---|---|---|---|---|
| GR-006-01 | WU-006-01 | read, write | tests/ | with the run |
| GR-006-02 | WU-006-02 | read, write | tools/check.py | with the run |
| GR-006-03 | WU-006-03 | read, write | framework/CONVENTIONS.md | with the run |
