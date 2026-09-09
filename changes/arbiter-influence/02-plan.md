# Plan — arbiter-influence

| Field | Value |
|---|---|
| Specification | SPEC-004 |
| Phases | 1 |
| Intermediate states valid | yes |
| Residue on abandonment | none |

## Work units

| Id | Phase | Class | Scope | Area of effect | Criteria | Depends on | Preparatory for |
|---|---|---|---|---|---|---|---|
| WU-004-01 | 1 | C3 | tests/ | the arbiter only, derived, high | CR-004-01, CR-004-02, CR-004-03, CR-004-04 | | WU-004-02 |
| WU-004-02 | 1 | C3 | tools/check.py | the gate and every change folder it reads, derived, high | CR-004-01, CR-004-02, CR-004-03, CR-004-04 | WU-004-01 | |
| WU-004-03 | 1 | C3 | framework/RULES.md | the claims the framework makes, derived, high | CR-004-01, CR-004-02 | WU-004-01 | |
| WU-004-04 | 1 | C3 | tests/test_rules_enforced.py | the arbiter of the previous change, derived, high | CR-003-04 | | WU-004-02 |

WU-004-04 was not planned. It exists because WU-004-02 adds an advisory note to the gate's output, and the arbiter of the previous change compared the whole output rather than the verdict its criterion names — so a standing verdict was invalidated. Under §6 the evidence is re-established before entry rather than the candidate dropped, and under invariant 8 the correction is its own unit, judged against CR-003-04 as already admitted in SPEC-003 and never accepted by WU-004-02, which benefits from it.

It is recorded as preparatory for WU-004-02, and that is the literal truth rather than a way past R8: WU-004-02 could not enter until this unit had re-established the verdict its entry would otherwise have invalidated. R8 fired on the first attempt, when the relationship had been left unstated, and the artefact was wrong rather than the rule.

## Coverage

| Question | Answer |
|---|---|
| Do the units together cover the specification? | yes |
| Was the estate query that produced the scope complete? | yes — two files |
