# Plan — trim-name-whitespace

| Field | Value |
|---|---|
| Specification | SPEC-002 |
| Phases | 1 |
| Intermediate states valid | yes — the tests alone fail and nothing ships on them |
| Residue on abandonment | none — remove the new tests |

## Work units

| Id | Phase | Class | Scope | Area of effect | Criteria | Depends on | Preparatory for |
|---|---|---|---|---|---|---|---|
| WU-002-01 | 1 | C3 | greeting_trim_test.go | the arbiter only, derived, high | CR-002-01, CR-002-02 |  | WU-002-02 |
| WU-002-02 | 1 | C3 | greeting.go | 0 file(s) reached firmly, 2 on a name alone, derived, medium | CR-002-01, CR-002-02, CR-002-03 | WU-002-01 |  |

## Coverage

The units cover all 3 criteria. The estate reached 2 file(s) from the scope, 0 of them firmly.
