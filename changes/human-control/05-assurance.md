# Assurance — human-control

## Evidence

| Id | Subject | Kind | Producer | Independence | How established | Repeatable | Obtained at | Valid until |
|---|---|---|---|---|---|---|---|---|
| EV-009-01-01 | candidate | test-run | tests/test_human_control.py, run in an isolated tree with every candidate applied | independent-by-precommitment | written and its failure per criterion recorded before CA-009-02 existed | yes | 2026-09-10 | next change to tools/check.py |
| EV-009-01-02 | candidate | test-run | the seven arbiters of the earlier changes | independent-by-precommitment | all fixed before this change was intended | yes | 2026-09-10 | next change to tools/check.py |

## Verdicts

| Id | Unit | Criterion | Outcome | Evidence | State | Window | Baseline |
|---|---|---|---|---|---|---|---|
| VE-009-01 | WU-009-02 | CR-009-01 | met | EV-009-01-01 | settled | | |
| VE-009-02 | WU-009-02 | CR-009-02 | met | EV-009-01-01 | settled | | |
| VE-009-03 | WU-009-02 | CR-009-03 | met | EV-009-01-01 | settled | | |
| VE-009-04 | WU-009-02 | CR-009-04 | met | EV-009-01-02 | settled | | |
| VE-009-05 | WU-009-01 | CR-009-01 | met | EV-009-01-01 | settled | | |
| VE-009-06 | WU-009-03 | CR-009-03 | met | EV-009-01-01 | settled | | |
| VE-009-07 | WU-009-04 | CR-009-04 | met | EV-009-01-02 | settled | | |
| VE-009-08 | WU-009-05 | CR-009-02 | met | EV-009-01-01 | settled | | |

## Escalations closed

| Escalation | Resolved by | Note |
|---|---|---|
| arbiter-acceptance#2 | this change | not settled: reopened as ground 1 below, since it is one defect shared by five rules rather than one per rule |

## Escalations

| # | Standing | Ground | Raised against | To |
|---|---|---|---|---|
| 1 | open | Five rules now share one limit — R12, R13, R15, R16 and R18 each check that something was recorded and none can check that the record is honest. A pre-acceptance outcome, a claim of repeatability, a settled debt, the derivation of a plan, a criteria review: all are testimony about something already over. This is one defect, and the framework has been recording it five times as five | framework | Illia Gannytskyi |
| 2 | open | Verification in an isolated tree is not sufficient: this change's own arbiter passed there and failed where the change landed, because its fixture borrowed a folder whose shape the landing changed. Nothing requires the arbiters to be run at the landing point as well | framework | Illia Gannytskyi |
| 3 | open | The enforcement scan of SPEC-003 has broken three times — absolute paths, a folder set, and now a rule enforced under another rule's name. Each time the proxy was wrong and the rule was fine. Nothing requires an arbiter to state that it tests a *proxy* for its criterion rather than the criterion itself | framework | Illia Gannytskyi |
