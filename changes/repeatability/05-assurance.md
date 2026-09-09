# Assurance — repeatability

## Evidence

| Id | Subject | Kind | Producer | Independence | How established | Repeatable | Obtained at | Valid until |
|---|---|---|---|---|---|---|---|---|
| EV-006-01-01 | candidate | test-run | tests/test_repeatability.py, run in an isolated tree with every candidate applied | independent-by-precommitment | written and its failure per criterion recorded before CA-006-02 and CA-006-03 existed | yes | 2026-09-09 | next change to tools/check.py |
| EV-006-01-02 | candidate | test-run | the four arbiters of the earlier changes | independent-by-precommitment | all fixed before this change was intended | yes | 2026-09-09 | next change to tools/check.py |
| EV-006-01-03 | candidate | static-analysis | baseline/CONVENTIONS.md.before compared against CA-006-03 | independent-by-precommitment | captured at admission, before any edit | yes | 2026-09-09 | — |

Every piece of evidence here is a deterministic script run against inputs fixed in the repository, so each can be produced again. That is what `yes` claims and nothing more.

## Verdicts

| Id | Unit | Criterion | Outcome | Evidence | State | Window | Baseline |
|---|---|---|---|---|---|---|---|
| VE-006-01 | WU-006-02 | CR-006-01 | met | EV-006-01-01 | settled | | |
| VE-006-02 | WU-006-02 | CR-006-02 | met | EV-006-01-01 | settled | | |
| VE-006-03 | WU-006-02 | CR-006-03 | met | EV-006-01-02 | settled | | |
| VE-006-04 | WU-006-01 | CR-006-01 | met | EV-006-01-01 | settled | | |
| VE-006-05 | WU-006-03 | CR-006-01 | met | EV-006-01-03 | settled | | |

## Escalations

| # | Standing | Ground | Raised against | To |
|---|---|---|---|---|
| 1 | closed | A record of a past run is derived only where the run can be repeated. Applied as Y9 and enforced as R13 and R14 | framework | Illia Gannytskyi |
| 2 | open | Repeatability is recorded by whoever writes the row, and the gate checks that a value is present, not that it is honest. The same limit as R12: a property of a run that has ended cannot be verified from the record of it. Unlike R12 this one has a way out — a repeatable run can simply be repeated — but nothing in the framework requires anyone to do so | framework | Illia Gannytskyi |
