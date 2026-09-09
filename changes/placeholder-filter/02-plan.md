# Plan — placeholder-filter

| Field | Value |
|---|---|
| Specification | SPEC-002 |
| Phases | 1 |
| Intermediate states valid | yes |
| Residue on abandonment | none — the reproduction stands on its own and is worth keeping either way |

## Work units

| Id | Phase | Class | Scope | Area of effect | Criteria | Depends on | Preparatory for |
|---|---|---|---|---|---|---|---|
| WU-002-01 | 1 | C3 | tests/ | the arbiter only; no behaviour of the gate, derived, high | CR-002-01, CR-002-02, CR-002-03 | | WU-002-02 |
| WU-002-02 | 1 | C2 | tools/check.py | the gate's row filter and every check reading it, derived, high | CR-002-01, CR-002-02, CR-002-03 | WU-002-01 | |

WU-002-01 is preparatory. There is no observational adequacy here at all — the repository has no tests — so the reproduction must exist before the repair can be judged. Its own acceptance is decidable: it must fail on the unmodified gate, and fail for the stated reason rather than incidentally.

## Coverage

| Question | Answer |
|---|---|
| Do the units together cover the specification? | yes |
| Was the estate query that produced the scope complete? | yes — one file, read directly |
