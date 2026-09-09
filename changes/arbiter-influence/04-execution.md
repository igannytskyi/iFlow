# Execution — arbiter-influence

| Unit | Executor | Version | Terminal state | Candidate | Trace |
|---|---|---|---|---|---|
| WU-004-01 | Claude Opus 5 | claude-opus-5 | candidate | CA-004-01 | this file |
| WU-004-02 | Claude Opus 5 | claude-opus-5 | candidate | CA-004-02 | this file |
| WU-004-03 | Claude Opus 5 | claude-opus-5 | candidate | CA-004-03 | this file |
| WU-004-04 | Claude Opus 5 | claude-opus-5 | candidate | CA-004-04 | this file |

## Candidates

| Id | Unit | Stored at | Artefacts | Produced at |
|---|---|---|---|---|
| CA-004-01 | WU-004-01 | candidates/CA-004-01/ | tests/test_arbiter_influence.py | 2026-09-09 |
| CA-004-02 | WU-004-02 | candidates/CA-004-02/ | tools/check.py, adding R7b and the column dependencies | 2026-09-09 |
| CA-004-03 | WU-004-03 | candidates/CA-004-03/ | framework/RULES.md, R7b added and B1 widened | 2026-09-09 |
| CA-004-04 | WU-004-04 | candidates/CA-004-04/ | tests/test_rules_enforced.py, comparison narrowed to verdicts | 2026-09-09 |

## Note on the arbiter's own acceptance

CA-004-01 failed on its first run for four criteria, one of which — CR-004-04 — failed because the baseline had been captured while this change's own folder was half-created, so "everything under changes/" was not a stable definition of what to compare. A spurious failure is not the stated reason. The arbiter was corrected to name the three pre-existing changes explicitly, re-run, and then failed on exactly the three criteria describing the defect. This is the second run in which the arbiter needed correcting before it could be accepted, and both times for the same kind of reason: the comparison was specified more loosely than the criterion it serves.
