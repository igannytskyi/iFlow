# Plan — greet-the-world

| Field | Value |
|---|---|
| Specification | SPEC-001 |
| Phases | 1 |
| Intermediate states valid | yes — the reproduction alone fails and nothing ships on it; the repair alone is untested |
| Residue on abandonment | none — remove the reproduction |

## Work units

| Id | Phase | Class | Scope | Area of effect | Criteria | Depends on | Preparatory for |
|---|---|---|---|---|---|---|---|
| WU-001-01 | 1 | C2 | greeting_world_test.go | the arbiter only, derived, high | CR-001-01 | | WU-001-02 |
| WU-001-02 | 1 | C2 | greeting.go | the package, derived, high — one file, one function | CR-001-01, CR-001-02, CR-001-03 | WU-001-01 | |

## Coverage

The two units cover all three criteria. The package is one file and everything in it is reached.
