# Plan — criteria-technique

| Field | Value |
|---|---|
| Specification | SPEC-010 |
| Phases | 1 |
| Intermediate states valid | yes |
| Residue on abandonment | none |

## Work units

| Id | Phase | Class | Scope | Area of effect | Criteria | Depends on | Preparatory for |
|---|---|---|---|---|---|---|---|
| WU-010-01 | 1 | C3 | framework/tests/ | the arbiter only, derived, high | CR-010-01, CR-010-02, CR-010-03 | | WU-010-02 |
| WU-010-02 | 1 | C3 | framework/check.py | the gate and every change folder it reads, derived, high | CR-010-01, CR-010-03 | WU-010-01 | |
| WU-010-03 | 1 | C3 | framework/criteria.md, framework/rules.md | the technique, and what the method claims about it, derived, high | CR-010-02 | WU-010-01 | |

## Coverage

| Question | Answer |
|---|---|
| Do the units together cover the specification? | yes |
| Was the estate query that produced the scope complete? | yes — two existing files and one new |
