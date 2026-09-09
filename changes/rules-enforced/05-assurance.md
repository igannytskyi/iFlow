# Assurance — rules-enforced

## Evidence

| Id | Subject | Kind | Producer | Independence | How established | Obtained at | Valid until |
|---|---|---|---|---|---|---|---|
| EV-003-01-01 | candidate | test-run | tests/test_rules_enforced.py, run in an isolated tree with all three candidates applied | independent-by-precommitment | written and its failure recorded before CA-003-02 and CA-003-03 existed; it could not have been shaped to fit them | 2026-09-09 | next change to tools/check.py or framework/RULES.md |
| EV-003-01-02 | candidate | test-run | tests/test_row_filter.py, the arbiter of the previous change | independent-by-precommitment | fixed before this change was intended, so it cannot have been bent around it | 2026-09-09 | next change to tools/check.py |
| EV-003-01-03 | candidate | static-analysis | baseline/RULES.md.before compared against CA-003-03 | independent-by-precommitment | the prior rules were captured at admission, before any reclassification, so a deleted claim would show as a difference | 2026-09-09 | — |

## Verdicts

| Id | Unit | Criterion | Outcome | Evidence | State | Window | Baseline |
|---|---|---|---|---|---|---|---|
| VE-003-01 | WU-003-01 | CR-003-01 | met | EV-003-01-01 | settled | | |
| VE-003-02 | WU-003-02 | CR-003-02 | met | EV-003-01-01 | settled | | |
| VE-003-03 | WU-003-03 | CR-003-03 | met | EV-003-01-03 | settled | | |
| VE-003-04 | WU-003-02 | CR-003-04 | met | EV-003-01-02 | settled | | |

CR-003-03 is judged on EV-003-01-03 and not on the arbiter alone. The arbiter reads the same file the unit changed, so on its own it would show only that the claims and the checks now agree — which deleting the claims would also achieve. The comparison against the state captured at admission is what distinguishes honouring a claim from removing it.

## Escalations

| # | Standing | Ground | Raised against | To |
|---|---|---|---|---|
| 1 | open | R7 requires the scope and the arbiter to be disjoint as paths, and they are. But the arbiter reads `framework/RULES.md`, which is inside the scope, so a file an executor may edit determines what the arbiter concludes. Disjointness of paths is not disjointness of influence. Mitigated here by separating the unit, requiring a reason for every exemption, and capturing the prior state — but the rule as written does not require any of that | framework | Illia Gannytskyi |
