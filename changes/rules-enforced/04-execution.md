# Execution — rules-enforced

| Unit | Executor | Version | Terminal state | Candidate | Trace |
|---|---|---|---|---|---|
| WU-003-01 | Claude Opus 5 | claude-opus-5 | candidate | CA-003-01 | this file |
| WU-003-02 | Claude Opus 5 | claude-opus-5 | candidate | CA-003-02 | this file |
| WU-003-03 | Claude Opus 5 | claude-opus-5 | candidate | CA-003-03 | this file |

No deterministic transformation exists for any of the three, so an agent was the correct executor.

## Candidates

| Id | Unit | Stored at | Artefacts | Produced at |
|---|---|---|---|---|
| CA-003-01 | WU-003-01 | candidates/CA-003-01/ | tests/test_rules_enforced.py | 2026-09-09 |
| CA-003-02 | WU-003-02 | candidates/CA-003-02/ | tools/check.py, adding R4 and R8 | 2026-09-09 |
| CA-003-03 | WU-003-03 | candidates/CA-003-03/ | framework/RULES.md, reclassified | 2026-09-09 |

## Note on the arbiter's own acceptance

CA-003-01 failed on its first run against the unrepaired state for three criteria, one of which — CR-003-04 — failed on absolute paths rather than on anything about the subject. A spurious failure is not the stated reason, so the arbiter was corrected and re-run before being accepted. It then failed on exactly CR-003-01 and CR-003-02 and passed the other two, which is what a reproduction is supposed to look like.
