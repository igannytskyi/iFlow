# Plan — human-control

| Field | Value |
|---|---|
| Specification | SPEC-009 |
| Phases | 1 |
| Intermediate states valid | yes |
| Residue on abandonment | none |

## Work units

| Id | Phase | Class | Scope | Area of effect | Criteria | Depends on | Preparatory for |
|---|---|---|---|---|---|---|---|
| WU-009-01 | 1 | C3 | tests/ | the arbiter only, derived, high | CR-009-01, CR-009-02, CR-009-03, CR-009-04 | | WU-009-02 |
| WU-009-02 | 1 | C3 | tools/check.py | the gate and every change folder it reads, derived, high | CR-009-01, CR-009-02, CR-009-03, CR-009-04 | WU-009-01 | |
| WU-009-03 | 1 | C3 | framework/RULES.md, framework/CONVENTIONS.md | the claims and codes, derived, high | CR-009-01, CR-009-02, CR-009-03 | WU-009-01 | |
| WU-009-04 | 1 | C3 | tests/test_rules_enforced.py | the arbiter of an earlier change, derived, high | CR-003-01 | | WU-009-03 |
| WU-009-05 | 1 | C3 | tests/test_human_control.py | this change's own arbiter, derived, high | CR-009-02 | | WU-009-02 |

WU-009-04 was not planned. B3 becomes mechanical here and is enforced by R17 rather than under its own name, and the earlier arbiter required the name itself to appear in the gate — a proxy for "enforced" that has now broken three times without the rule ever being at fault. Under invariant 8 the correction is its own unit, judged against CR-003-01 as admitted in SPEC-003, and preparatory for WU-009-03, which could not stand while an earlier verdict was invalidated.

## Coverage

| Question | Answer |
|---|---|
| Do the units together cover the specification? | yes |
| Was the estate query that produced the scope complete? | yes — three files |

WU-009-05 was not planned either. This change's own arbiter built the case "nothing has executed yet" by borrowing the live folder, which passed while that folder had no candidate and failed the moment it gained one. The fixture is now constructed — later stages removed, record truncated to the admission crossing — so it says what it means regardless of how far anything has progressed.
