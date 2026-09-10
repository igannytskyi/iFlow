# Admission — human-control

| Field | Value |
|---|---|
| Specification digest | 6e1348be7ff82be0 |

## Criteria review

| Reviewed before admission | By | At | Note |
|---|---|---|---|
| yes | Illia Gannytskyi | 2026-09-10 | the three gaps were named by the reviewer, and the criteria were written from that reading |

## Decisions

| Unit | Outcome | Deciding condition | Confidence of the edges relied on | Escalates at |
|---|---|---|---|---|
| WU-009-01 | admitted | permission | high | |
| WU-009-02 | admitted | conflict | high | |
| WU-009-03 | admitted | conflict | high | |

## Conflicts

| Units | Intersecting region | Evidence it would invalidate | Resolution |
|---|---|---|---|
| WU-009-02, WU-009-03 | none | none | ordering |

## Evidence plan

| Criterion | Observed | Method | Unchanged means | Derived from | Fixed at |
|---|---|---|---|---|---|

## Prior state captured

| Criterion | Artefact | Captured to |
|---|---|---|
| CR-009-04 | tools/check.py before this change | baseline/check.py.before |
| CR-009-04 | the gate's verdict on every existing change | baseline/gate-before.txt |

## Grants

| Id | Unit | Permitted operations | Targets | Expires |
|---|---|---|---|---|
| GR-009-01 | WU-009-01 | read, write | tests/ | with the run |
| GR-009-02 | WU-009-02 | read, write | tools/check.py | with the run |
| GR-009-03 | WU-009-03 | read, write | framework/ | with the run |
