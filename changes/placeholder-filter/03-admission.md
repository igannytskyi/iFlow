# Admission — placeholder-filter

| Field | Value |
|---|---|
| Specification digest | 606ad70c2c41b03e |

## Decisions

| Unit | Outcome | Deciding condition | Confidence of the edges relied on | Escalates at |
|---|---|---|---|---|
| WU-002-01 | admitted | permission | high | |
| WU-002-02 | admitted | conflict | high | |

WU-002-02 is admitted behind WU-002-01, which it depends on. No other work is in flight in either area of effect.

## Conflicts

| Units | Intersecting region | Evidence it would invalidate | Resolution |
|---|---|---|---|
| WU-002-01, WU-002-02 | none; the reproduction is in the arbiter, the repair is in the scope | none | ordering |

## Grants

| Id | Unit | Permitted operations | Targets | Expires |
|---|---|---|---|---|
| GR-002-01 | WU-002-01 | read, write | tests/ | with the run |
| GR-002-02 | WU-002-02 | read, write | tools/check.py | with the run |
