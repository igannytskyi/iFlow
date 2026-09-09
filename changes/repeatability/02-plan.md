# Plan — repeatability

| Field | Value |
|---|---|
| Specification | SPEC-006 |
| Phases | 1 |
| Intermediate states valid | yes |
| Residue on abandonment | none |

## Work units

| Id | Phase | Class | Scope | Area of effect | Criteria | Depends on | Preparatory for |
|---|---|---|---|---|---|---|---|
| WU-006-01 | 1 | C3 | tests/ | the arbiter only, derived, high | CR-006-01, CR-006-02, CR-006-03 | | WU-006-02 |
| WU-006-02 | 1 | C3 | tools/check.py | the gate and every change folder it reads, derived, high | CR-006-01, CR-006-02, CR-006-03 | WU-006-01 | |
| WU-006-03 | 1 | C3 | framework/CONVENTIONS.md | the codes every artefact is written against, derived, high | CR-006-01 | WU-006-01 | |

## Coverage

| Question | Answer |
|---|---|
| Do the units together cover the specification? | yes |
| Was the estate query that produced the scope complete? | yes — two files |
