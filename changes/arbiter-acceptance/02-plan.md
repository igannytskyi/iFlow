# Plan — arbiter-acceptance

| Field | Value |
|---|---|
| Specification | SPEC-005 |
| Phases | 1 |
| Intermediate states valid | yes |
| Residue on abandonment | none |

## Work units

| Id | Phase | Class | Scope | Area of effect | Criteria | Depends on | Preparatory for |
|---|---|---|---|---|---|---|---|
| WU-005-01 | 1 | C3 | tests/ | the arbiter only, derived, high | CR-005-01, CR-005-02, CR-005-03 | | WU-005-02 |
| WU-005-02 | 1 | C3 | tools/check.py | the gate and every change folder it reads, derived, high | CR-005-01, CR-005-02, CR-005-03 | WU-005-01 | |
| WU-005-03 | 1 | C3 | framework/RULES.md | the claims the framework makes, derived, high | CR-005-01, CR-005-02 | WU-005-01 | |

## Coverage

| Question | Answer |
|---|---|
| Do the units together cover the specification? | yes |
| Was the estate query that produced the scope complete? | yes — two files |
