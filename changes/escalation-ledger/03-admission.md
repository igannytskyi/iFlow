# Admission — escalation-ledger

| Field | Value |
|---|---|
| Specification digest | 3582550257f82ba7 |

## Decisions

| Unit | Outcome | Deciding condition | Confidence of the edges relied on | Escalates at |
|---|---|---|---|---|
| WU-007-01 | admitted | permission | high | |
| WU-007-02 | admitted | conflict | high | |
| WU-007-03 | admitted | conflict | high | |

## Conflicts

| Units | Intersecting region | Evidence it would invalidate | Resolution |
|---|---|---|---|
| WU-007-02, WU-007-03 | none | none | ordering |

## Prior state captured

| Criterion | Artefact | Captured to |
|---|---|---|
| CR-007-03 | tools/check.py before this change | baseline/check.py.before |
| CR-007-03 | the gate's verdict on every existing change | baseline/gate-before.txt |

## Grants

| Id | Unit | Permitted operations | Targets | Expires |
|---|---|---|---|---|
| GR-007-01 | WU-007-01 | read, write | tests/ | with the run |
| GR-007-02 | WU-007-02 | read, write | tools/check.py | with the run |
| GR-007-03 | WU-007-03 | read, write | framework/ | with the run |
