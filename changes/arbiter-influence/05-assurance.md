# Assurance — arbiter-influence

## Evidence

| Id | Subject | Kind | Producer | Independence | How established | Obtained at | Valid until |
|---|---|---|---|---|---|---|---|
| EV-004-01-01 | candidate | test-run | tests/test_arbiter_influence.py, run in an isolated tree with every candidate applied | independent-by-precommitment | written, corrected and its failure recorded before CA-004-02 and CA-004-03 existed | 2026-09-09 | next change to tools/check.py or framework/RULES.md |
| EV-004-01-02 | candidate | static-analysis | baseline/RULES.md.before compared against CA-004-03 | independent-by-precommitment | the prior rules were captured at admission, so a deleted claim shows as a difference | 2026-09-09 | — |
| EV-004-01-03 | candidate | test-run | tests/test_rules_enforced.py and tests/test_row_filter.py, the arbiters of the two earlier changes | independent-by-precommitment | both were fixed before this change was intended | 2026-09-09 | next change to tools/check.py |

## Verdicts

| Id | Unit | Criterion | Outcome | Evidence | State | Window | Baseline |
|---|---|---|---|---|---|---|---|
| VE-004-01 | WU-004-02 | CR-004-01 | met | EV-004-01-01 | settled | | |
| VE-004-02 | WU-004-02 | CR-004-02 | met | EV-004-01-01 | settled | | |
| VE-004-03 | WU-004-02 | CR-004-03 | met | EV-004-01-01 | settled | | |
| VE-004-04 | WU-004-02 | CR-004-04 | met | EV-004-01-03 | settled | | |
| VE-004-05 | WU-004-03 | CR-004-01 | met | EV-004-01-02 | settled | | |
| VE-004-06 | WU-004-01 | CR-004-02 | met | EV-004-01-01 | settled | | |
| VE-004-07 | WU-004-04 | CR-004-04 | met | EV-004-01-03 | settled | | |

VE-004-07 judges the correction to the earlier arbiter against CR-003-04 as admitted in SPEC-003 — that the gate's *verdict* on existing changes is unchanged. The correction narrows the comparison to verdicts, which is what that criterion says; it does not relax it. It is judged separately from WU-004-02, which benefits from it.

## Escalations

| # | Standing | Ground | Raised against | To |
|---|---|---|---|---|
| 1 | closed | Path disjointness is not influence disjointness. Applied to the research document as Y5 and enforced here as R7b | framework | Illia Gannytskyi |
| 2 | open | Twice now the arbiter has needed correcting before acceptance, both times because its comparison was specified more loosely than the criterion it serves. A criterion says "the verdict is unchanged" and the arbiter compared every line printed. Nothing in the framework requires an arbiter to state which part of its criterion it tests | framework | Illia Gannytskyi |
