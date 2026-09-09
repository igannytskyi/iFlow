# Admission — <name>

| Field | Value |
|---|---|
| Specification digest | <recorded here; R1 compares against it forever after> |

## Decisions

| Unit | Outcome | Deciding condition | Confidence of the edges relied on | Escalates at |
|---|---|---|---|---|
| WU-<nnn>-01 | admitted · held · refused · awaiting-authority | conflict · allowance · permission | high · medium · low | |

## Conflicts

| Units | Intersecting region | Evidence it would invalidate | Resolution |
|---|---|---|---|
| | | | ordering · escalation |

A cycle among held units is a defect and is reported immediately, not waited out.

## Grants

| Id | Unit | Permitted operations | Targets | Expires |
|---|---|---|---|---|
| GR-<nnn>-01 | WU-<nnn>-01 | read · write · execute | | with the run |

A grant is the narrowest set sufficient for the unit's scope. Anything wider is refused, not warned about.
