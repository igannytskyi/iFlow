# Admission — clock-instant

| Field | Value |
|---|---|
| Specification digest | 1a8f9e40d94a809f |

## Decisions

| Unit | Outcome | Deciding condition | Confidence of the edges relied on | Escalates at |
|---|---|---|---|---|
| WU-001-01 | admitted | permission | high | |
| WU-001-02 | held | conflict | medium | 3 days |

## Conflicts

| Units | Intersecting region | Evidence it would invalidate | Resolution |
|---|---|---|---|
| WU-001-02, WU-014-03 | src/orders/clock.py | CR-001-02 for both | ordering |

## Grants

| Id | Unit | Permitted operations | Targets | Expires |
|---|---|---|---|---|
| GR-001-01 | WU-001-01 | read, write | src/billing | with the run |
