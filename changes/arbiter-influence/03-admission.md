# Admission — arbiter-influence

| Field | Value |
|---|---|
| Specification digest | cd94ccb61c692b68 |

## Decisions

| Unit | Outcome | Deciding condition | Confidence of the edges relied on | Escalates at |
|---|---|---|---|---|
| WU-004-01 | admitted | permission | high | |
| WU-004-02 | admitted | conflict | high | |
| WU-004-03 | admitted | conflict | high | |

## Conflicts

| Units | Intersecting region | Evidence it would invalidate | Resolution |
|---|---|---|---|
| WU-004-02, WU-004-03 | none; one touches the gate, the other the rules | none | ordering |

## Prior state captured

| Criterion | Artefact | Captured to |
|---|---|---|
| CR-004-01 | framework/RULES.md before this change | baseline/RULES.md.before |
| CR-004-04 | tools/check.py before this change | baseline/check.py.before |
| CR-004-04 | the gate's verdict on every existing change | baseline/gate-before.txt |

## Grants

| Id | Unit | Permitted operations | Targets | Expires |
|---|---|---|---|---|
| GR-004-01 | WU-004-01 | read, write | tests/ | with the run |
| GR-004-02 | WU-004-02 | read, write | tools/check.py | with the run |
| GR-004-03 | WU-004-03 | read, write | framework/RULES.md | with the run |
