# Plan — rules-enforced

| Field | Value |
|---|---|
| Specification | SPEC-003 |
| Phases | 1 |
| Intermediate states valid | yes |
| Residue on abandonment | none — each unit stands alone and is worth keeping separately |

## Work units

| Id | Phase | Class | Scope | Area of effect | Criteria | Depends on | Preparatory for |
|---|---|---|---|---|---|---|---|
| WU-003-01 | 1 | C3 | tests/ | the arbiter only, derived, high | CR-003-01, CR-003-02, CR-003-03, CR-003-04 | | WU-003-02, WU-003-03 |
| WU-003-02 | 1 | C3 | tools/check.py | the gate and every change folder it reads, derived, high | CR-003-01, CR-003-02, CR-003-04 | WU-003-01 | |
| WU-003-03 | 1 | C3 | framework/RULES.md | the claims the framework makes about itself, derived, high | CR-003-01, CR-003-03 | WU-003-01 | |

WU-003-01 is preparatory: nothing here is decidable until an arbiter exists that reads the rules and the gate and compares them.

WU-003-03 weakens a claim — it moves R5 and B3 from mechanical to human. Under invariant 8 that is its own unit, judged on CR-003-03 alone, and never accepted by WU-003-02 which benefits from it.

## Coverage

| Question | Answer |
|---|---|
| Do the units together cover the specification? | yes |
| Was the estate query that produced the scope complete? | yes — two files, read directly |
