# Admission — criteria-technique

| Field | Value |
|---|---|
| Specification digest | 69921e0e7c2a9c7c |

## Criteria review

| Reviewed before admission | By | At | Note |
|---|---|---|---|
| yes | Illia Gannytskyi | 2026-09-10 | the gap was named by the reviewer from the comparison with other frameworks |

## Decisions

| Unit | Outcome | Deciding condition | Confidence of the edges relied on | Escalates at |
|---|---|---|---|---|
| WU-010-01 | admitted | permission | high | |
| WU-010-02 | admitted | conflict | high | |
| WU-010-03 | admitted | conflict | high | |

## Conflicts

| Units | Intersecting region | Evidence it would invalidate | Resolution |
|---|---|---|---|
| WU-010-02, WU-010-03 | none | none | ordering |

## Evidence plan

| Criterion | Observed | Method | Unchanged means | Derived from | Fixed at |
|---|---|---|---|---|---|

## Prior state captured

| Criterion | Artefact | Captured to |
|---|---|---|
| CR-010-03 | framework/check.py before this change | baseline/check.py.before |
| CR-010-02 | framework/rules.md before this change | baseline/rules.md.before |
| CR-010-03 | the gate's verdict on every existing change | baseline/gate-before.txt |

## Grants

| Id | Unit | Permitted operations | Targets | Expires |
|---|---|---|---|---|
| GR-010-01 | WU-010-01 | read, write | framework/tests/ | with the run |
| GR-010-02 | WU-010-02 | read, write | framework/check.py | with the run |
| GR-010-03 | WU-010-03 | read, write | framework/criteria.md, framework/rules.md | with the run |
