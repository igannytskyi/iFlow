# Assurance — criteria-technique

## Evidence

| Id | Subject | Kind | Producer | Independence | How established | Repeatable | Obtained at | Valid until |
|---|---|---|---|---|---|---|---|---|
| EV-010-01-01 | candidate | test-run | framework/tests/test_criteria_technique.py, run in an isolated tree with every candidate applied | independent-by-precommitment | written, corrected and its failure recorded before CA-010-02 existed | yes | 2026-09-10 | next change to framework/check.py |
| EV-010-01-02 | candidate | test-run | the eight arbiters of the earlier changes | independent-by-precommitment | all fixed before this change was intended | yes | 2026-09-10 | next change to framework/check.py |
| EV-010-01-03 | candidate | static-analysis | baseline/rules.md.before compared against CA-010-03 | independent-by-precommitment | captured at admission, before any edit, so a deleted claim shows as a difference | yes | 2026-09-10 | — |

## Verdicts

| Id | Unit | Criterion | Outcome | Evidence | State | Window | Baseline |
|---|---|---|---|---|---|---|---|
| VE-010-01 | WU-010-02 | CR-010-01 | met | EV-010-01-01 | settled | | |
| VE-010-02 | WU-010-03 | CR-010-02 | met | EV-010-01-03 | settled | | |
| VE-010-03 | WU-010-02 | CR-010-03 | met | EV-010-01-02 | settled | | |
| VE-010-04 | WU-010-01 | CR-010-01 | met | EV-010-01-01 | settled | | |

## Escalations

| # | Standing | Ground | Raised against | To |
|---|---|---|---|---|
| 1 | open | Six of the seven probes are questions, and the method has no way to tell whether anyone asked them. The criteria review records that someone read the criteria, not what they asked while reading. Recording the answers would make the technique a form to fill in, which is what was avoided; leaving them unrecorded means their value can never be measured. Both cannot be had | framework | Illia Gannytskyi |
