# Assurance — arbiter-acceptance

## Evidence

| Id | Subject | Kind | Producer | Independence | How established | Obtained at | Valid until |
|---|---|---|---|---|---|---|---|
| EV-005-01-01 | candidate | test-run | tests/test_arbiter_acceptance.py, run in an isolated tree with every candidate applied | independent-by-precommitment | written, corrected, and its failure per criterion recorded before CA-005-02 and CA-005-03 existed | 2026-09-09 | next change to tools/check.py or framework/RULES.md |
| EV-005-01-02 | candidate | test-run | the three arbiters of the earlier changes | independent-by-precommitment | all fixed before this change was intended | 2026-09-09 | next change to tools/check.py |

## Verdicts

| Id | Unit | Criterion | Outcome | Evidence | State | Window | Baseline |
|---|---|---|---|---|---|---|---|
| VE-005-01 | WU-005-02 | CR-005-01 | met | EV-005-01-01 | settled | | |
| VE-005-02 | WU-005-02 | CR-005-02 | met | EV-005-01-01 | settled | | |
| VE-005-03 | WU-005-02 | CR-005-03 | met | EV-005-01-02 | settled | | |
| VE-005-04 | WU-005-01 | CR-005-01 | met | EV-005-01-01 | settled | | |
| VE-005-05 | WU-005-03 | CR-005-02 | met | EV-005-01-01 | settled | | |

## Escalations

| # | Standing | Ground | Raised against | To |
|---|---|---|---|---|
| 1 | closed | An arbiter must be shown to fail for the stated reason, not only a C2 reproduction. Applied as Y8 and enforced as R11 and R12 | framework | Illia Gannytskyi |
| 2 | open | R12 checks that a pre-acceptance outcome was recorded. It cannot check that the recorded outcome is true, and nothing can: the run that produced it is gone. The record is testimony about a past run, and the framework treats it as though it were derived | framework | Illia Gannytskyi |
