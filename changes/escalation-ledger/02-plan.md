# Plan — escalation-ledger

| Field | Value |
|---|---|
| Specification | SPEC-007 |
| Phases | 1 |
| Intermediate states valid | yes |
| Residue on abandonment | none |

## Work units

| Id | Phase | Class | Scope | Area of effect | Criteria | Depends on | Preparatory for |
|---|---|---|---|---|---|---|---|
| WU-007-01 | 1 | C3 | tests/ | the arbiter only, derived, high | CR-007-01, CR-007-02, CR-007-03 | | WU-007-02 |
| WU-007-02 | 1 | C3 | tools/check.py | the gate and the ledger it computes, derived, high | CR-007-01, CR-007-02, CR-007-03 | WU-007-01 | |
| WU-007-03 | 1 | C3 | framework/RULES.md, framework/CONVENTIONS.md | the claims and codes, derived, high | CR-007-01, CR-007-02 | WU-007-01 | |
| WU-007-04 | 1 | C3 | tests/test_rules_enforced.py | the arbiter of an earlier change, derived, high | CR-003-04 | | WU-007-02 |

WU-007-04 was not planned. R15 is computed across every change rather than inside one folder, so the gate emits it outside the per-folder failure path — and the earlier arbiter scanned only that path, taking it for a proxy of "enforced". The proxy broke rather than the rule. Under invariant 8 the correction is its own unit, judged against CR-003-01 as admitted in SPEC-003, and it is preparatory for WU-007-02 because that unit could not stand while an earlier verdict was invalidated.

## Coverage

| Question | Answer |
|---|---|
| Do the units together cover the specification? | yes |
| Was the estate query that produced the scope complete? | yes — three files |
