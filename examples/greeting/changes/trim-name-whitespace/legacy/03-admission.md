# Admission — trim-name-whitespace

| Field | Value |
|---|---|
| Specification digest | 9f0f6ead216c8a5e |

## Criteria review

| Reviewed before admission | By | At | Note |
|---|---|---|---|
| yes | Illia Gannytskyi (accepted with --accept, without reading) | 2026-11-15 | accepted in the workbench before any work |

## Decisions

| Unit | Outcome | Deciding condition | Confidence of the edges relied on |
|---|---|---|---|
| WU-002-01 | admitted | permission | medium |
| WU-002-02 | admitted | conflict | medium |

## Conflicts

| Units | Intersecting region | Evidence it would invalidate | Resolution |
|---|---|---|---|
| WU-002-01, WU-002-02 | none | none | ordering |

## Evidence plan

| Criterion | Observed | Method | Unchanged means | Derived from | Fixed at |
|---|---|---|---|---|---|
| CR-002-01 | greeting.go — the behaviour the criterion names | the tests written first, run once before the change and once after | the behaviour stands as it does now | the area of effect of greeting.go, derived, medium | 2026-11-15 |
| CR-002-02 | greeting.go — the behaviour the criterion names | the tests written first, run once before the change and once after | the behaviour stands as it does now | the area of effect of greeting.go, derived, medium | 2026-11-15 |

## Prior state captured

| Criterion | Artefact | Captured to |
|---|---|---|
| CR-002-01 | greeting.go as it stood | baseline/greeting.go.before |
