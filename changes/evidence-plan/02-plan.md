# Plan — evidence-plan

| Field | Value |
|---|---|
| Specification | SPEC-008 |
| Phases | 1 |
| Intermediate states valid | yes |
| Residue on abandonment | none |

## Work units

| Id | Phase | Class | Scope | Area of effect | Criteria | Depends on | Preparatory for |
|---|---|---|---|---|---|---|---|
| WU-008-01 | 1 | C3 | tests/ | the arbiter only, derived, high | CR-008-01, CR-008-02, CR-008-03 | | WU-008-02 |
| WU-008-02 | 1 | C3 | tools/check.py | the gate and every change folder it reads, derived, high | CR-008-01, CR-008-02, CR-008-03 | WU-008-01 | |
| WU-008-03 | 1 | C3 | framework/RULES.md, framework/CONVENTIONS.md | the claims and codes, derived, high | CR-008-01, CR-008-02 | WU-008-01 | |

## Coverage

| Question | Answer |
|---|---|
| Do the units together cover the specification? | yes |
| Was the estate query that produced the scope complete? | yes — three files |
