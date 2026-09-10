# Assurance — escalation-ledger

## Evidence

| Id | Subject | Kind | Producer | Independence | How established | Repeatable | Obtained at | Valid until |
|---|---|---|---|---|---|---|---|---|
| EV-007-01-01 | candidate | test-run | tests/test_escalation_ledger.py, run in an isolated tree with every candidate applied | independent-by-precommitment | written and its failure per criterion recorded before CA-007-02 existed | yes | 2026-09-10 | next change to tools/check.py |
| EV-007-01-02 | candidate | test-run | the five arbiters of the earlier changes | independent-by-precommitment | all fixed before this change was intended | yes | 2026-09-10 | next change to tools/check.py |

## Verdicts

| Id | Unit | Criterion | Outcome | Evidence | State | Window | Baseline |
|---|---|---|---|---|---|---|---|
| VE-007-01 | WU-007-02 | CR-007-01 | met | EV-007-01-01 | settled | | |
| VE-007-02 | WU-007-02 | CR-007-02 | met | EV-007-01-01 | settled | | |
| VE-007-03 | WU-007-02 | CR-007-03 | met | EV-007-01-02 | settled | | |
| VE-007-04 | WU-007-01 | CR-007-01 | met | EV-007-01-01 | settled | | |
| VE-007-05 | WU-007-03 | CR-007-02 | met | EV-007-01-01 | settled | | |
| VE-007-06 | WU-007-04 | CR-007-03 | met | EV-007-01-02 | settled | | |

## Escalations closed

Two debts were settled runs ago and went on being owed because nothing could close them from outside the change that raised them. That is the defect this change repairs, and these are its first two uses.

| Escalation | Resolved by | Note |
|---|---|---|
| rules-enforced#1 | arbiter-influence | Y5 extended disjointness from paths to what the arbiter reads, and R7b enforces it |
| arbiter-influence#2 | arbiter-acceptance | Y8 required an arbiter to be shown failing for its stated reason, enforced as R11 and R12 |

## Escalations

| # | Standing | Ground | Raised against | To |
|---|---|---|---|---|
| 1 | closed | An escalation raised in one change cannot be closed by another. Repaired here | framework | Illia Gannytskyi |
| 2 | open | Entry happened before every standing arbiter had been run, so an invalidated verdict was found after the fact rather than before. Nothing in the gate requires the whole set of arbiters to pass before a landing is recorded — each change checks its own folder | framework | Illia Gannytskyi |
| 3 | open | The ledger reads the standing recorded where an escalation was raised, and a closure recorded elsewhere. Neither says whether the thing was actually addressed — closure is a claim by whoever wrote the row, exactly like the pre-acceptance record of R12 and the repeatability of R13. Three rules now share one limit, which suggests it is not three defects but one | framework | Illia Gannytskyi |
