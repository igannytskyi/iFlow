# Assurance — evidence-plan

## Evidence

| Id | Subject | Kind | Producer | Independence | How established | Repeatable | Obtained at | Valid until |
|---|---|---|---|---|---|---|---|---|
| EV-008-01-01 | candidate | test-run | tests/test_evidence_plan.py, run in an isolated tree with every candidate applied | independent-by-precommitment | written and its failure per criterion recorded before CA-008-02 existed | yes | 2026-09-10 | next change to tools/check.py |
| EV-008-01-02 | candidate | test-run | the six arbiters of the earlier changes | independent-by-precommitment | all fixed before this change was intended | yes | 2026-09-10 | next change to tools/check.py |

## Verdicts

| Id | Unit | Criterion | Outcome | Evidence | State | Window | Baseline |
|---|---|---|---|---|---|---|---|
| VE-008-01 | WU-008-02 | CR-008-01 | met | EV-008-01-01 | settled | | |
| VE-008-02 | WU-008-02 | CR-008-02 | met | EV-008-01-01 | settled | | |
| VE-008-03 | WU-008-02 | CR-008-03 | met | EV-008-01-02 | settled | | |
| VE-008-04 | WU-008-01 | CR-008-01 | met | EV-008-01-01 | settled | | |
| VE-008-05 | WU-008-03 | CR-008-02 | met | EV-008-01-01 | settled | | |

## Escalations

| # | Standing | Ground | Raised against | To |
|---|---|---|---|---|
| 1 | open | R16 requires a plan to name the area of effect it was derived from, and checks that the field is not empty. It cannot check that the plan actually follows from that area rather than from the change — the fourth rule to share the one limit, which by now is plainly a single defect and not four | framework | Illia Gannytskyi |
