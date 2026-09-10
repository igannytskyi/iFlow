# Admission — evidence-plan

| Field | Value |
|---|---|
| Specification digest | 084ed03520093ef5 |

## Decisions

| Unit | Outcome | Deciding condition | Confidence of the edges relied on | Escalates at |
|---|---|---|---|---|
| WU-008-01 | admitted | permission | high | |
| WU-008-02 | admitted | conflict | high | |
| WU-008-03 | admitted | conflict | high | |

## Conflicts

| Units | Intersecting region | Evidence it would invalidate | Resolution |
|---|---|---|---|
| WU-008-02, WU-008-03 | none | none | ordering |

## Evidence plan

No criterion here needs one: each names its own subject and threshold, and the area of effect is two files read directly. The section is present to show the shape, and its absence elsewhere is what R16 notes rather than fails.

| Criterion | Observed | Method | Unchanged means | Derived from | Fixed at |
|---|---|---|---|---|---|

## Prior state captured

| Criterion | Artefact | Captured to |
|---|---|---|
| CR-008-03 | tools/check.py before this change | baseline/check.py.before |
| CR-008-03 | the gate's verdict on every existing change | baseline/gate-before.txt |

## Grants

| Id | Unit | Permitted operations | Targets | Expires |
|---|---|---|---|---|
| GR-008-01 | WU-008-01 | read, write | tests/ | with the run |
| GR-008-02 | WU-008-02 | read, write | tools/check.py | with the run |
| GR-008-03 | WU-008-03 | read, write | framework/ | with the run |
