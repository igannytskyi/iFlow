# Admission — arbiter-acceptance

| Field | Value |
|---|---|
| Specification digest | 2cbcf4245c378fdf |

## Decisions

| Unit | Outcome | Deciding condition | Confidence of the edges relied on | Escalates at |
|---|---|---|---|---|
| WU-005-01 | admitted | permission | high | |
| WU-005-02 | admitted | conflict | high | |
| WU-005-03 | admitted | conflict | high | |

## Conflicts

| Units | Intersecting region | Evidence it would invalidate | Resolution |
|---|---|---|---|
| WU-005-02, WU-005-03 | none | none | ordering |

## Prior state captured

| Criterion | Artefact | Captured to |
|---|---|---|
| CR-005-03 | tools/check.py before this change | baseline/check.py.before |
| CR-005-01 | framework/RULES.md before this change | baseline/RULES.md.before |
| CR-005-03 | the gate's verdict on every existing change | baseline/gate-before.txt |

## Grants

| Id | Unit | Permitted operations | Targets | Expires |
|---|---|---|---|---|
| GR-005-01 | WU-005-01 | read, write | tests/ | with the run |
| GR-005-02 | WU-005-02 | read, write | tools/check.py | with the run |
| GR-005-03 | WU-005-03 | read, write | framework/RULES.md | with the run |
