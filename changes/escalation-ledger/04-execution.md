# Execution — escalation-ledger

| Unit | Executor | Version | Terminal state | Candidate | Trace |
|---|---|---|---|---|---|
| WU-007-01 | Claude Opus 5 | claude-opus-5 | candidate | CA-007-01 | this file |
| WU-007-02 | Claude Opus 5 | claude-opus-5 | candidate | CA-007-02 | this file |
| WU-007-03 | Claude Opus 5 | claude-opus-5 | candidate | CA-007-03 | this file |

## Candidates

| Id | Unit | Stored at | Artefacts | Produced at |
|---|---|---|---|---|
| CA-007-01 | WU-007-01 | candidates/CA-007-01/ | tests/test_escalation_ledger.py | 2026-09-10 |
| CA-007-02 | WU-007-02 | candidates/CA-007-02/ | tools/check.py, adding the ledger and R15 | 2026-09-10 |
| CA-007-03 | WU-007-03 | candidates/CA-007-03/ | framework/RULES.md and CONVENTIONS.md | 2026-09-10 |
| CA-007-04 | WU-007-04 | candidates/CA-007-04/ | tests/test_rules_enforced.py, emission scan widened | 2026-09-10 |

## Arbiter acceptance

| Unit | Criterion | Before repair |
|---|---|---|
| WU-007-01 | CR-007-01 | failed, no ledger existed to list what was owed |
| WU-007-01 | CR-007-02 | failed, settling something never owed went undetected |
| WU-007-01 | CR-007-03 | passed, as it must — nothing had changed yet |
| WU-007-04 | CR-003-01 | failed, reporting R15 as claimed mechanical and never emitted |

WU-007-04's row records what the earlier arbiter did once WU-007-02 had landed: it named R15 as unenforced, which was the proxy breaking rather than the rule failing. R12 caught the absence of this row before the change could be presented — the rule finding a gap in the change that introduced the unit it applies to.
