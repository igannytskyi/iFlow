# Admission — greet-the-world

| Field | Value |
|---|---|
| Specification digest | 2595638150f37955 |

## Criteria review

| Reviewed before admission | By | At | Note |
|---|---|---|---|
| yes | the example's author | 2026-11-14 | the three criteria read before any work |

## Decisions

| Unit | Outcome | Deciding condition | Confidence of the edges relied on |
|---|---|---|---|
| WU-001-01 | admitted | permission | high |
| WU-001-02 | admitted | conflict | high |

## Conflicts

| Units | Intersecting region | Evidence it would invalidate | Resolution |
|---|---|---|---|
| WU-001-01, WU-001-02 | none | none | ordering |

## Evidence plan

| Criterion | Observed | Method | Unchanged means | Derived from | Fixed at |
|---|---|---|---|---|---|
| CR-001-01 | greeting.go, the greeting with no name | the reproduction, run once before the repair and once after | the greeting still reads "Hello, !" | the area of effect of greeting.go, derived, high | 2026-11-14 |

## Prior state captured

| Criterion | Artefact | Captured to |
|---|---|---|
| CR-001-01 | greeting.go as it stood | baseline/greeting.go.before |
