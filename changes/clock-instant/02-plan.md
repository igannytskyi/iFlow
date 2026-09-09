# Plan — clock-instant

| Field | Value |
|---|---|
| Specification | SPEC-001 |
| Phases | 1 |
| Intermediate states valid | yes |
| Residue on abandonment | none — a single phase, reversible by revert |

## Work units

| Id | Phase | Class | Scope | Area of effect | Criteria | Depends on | Preparatory for |
|---|---|---|---|---|---|---|---|
| WU-001-01 | 1 | C1T | src/billing | call sites and their dependents, derived from the source index, medium | CR-001-01, CR-001-02 | | |
| WU-001-02 | 1 | C1T | src/orders | call sites and their dependents, derived from the source index, medium | CR-001-01, CR-001-02 | | |

## Coverage

| Question | Answer |
|---|---|
| Do the units together cover the specification? | yes, for statically resolvable uses |
| Was the estate query that produced the scope complete? | no — reflection is not resolvable, residue stated on SPEC-001 |
