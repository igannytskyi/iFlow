# Execution — arbiter-acceptance

| Unit | Executor | Version | Terminal state | Candidate | Trace |
|---|---|---|---|---|---|
| WU-005-01 | Claude Opus 5 | claude-opus-5 | candidate | CA-005-01 | this file |
| WU-005-02 | Claude Opus 5 | claude-opus-5 | candidate | CA-005-02 | this file |
| WU-005-03 | Claude Opus 5 | claude-opus-5 | candidate | CA-005-03 | this file |

## Candidates

| Id | Unit | Stored at | Artefacts | Produced at |
|---|---|---|---|---|
| CA-005-01 | WU-005-01 | candidates/CA-005-01/ | tests/test_arbiter_acceptance.py | 2026-09-09 |
| CA-005-02 | WU-005-02 | candidates/CA-005-02/ | tools/check.py, adding R11 and R12 | 2026-09-09 |
| CA-005-03 | WU-005-03 | candidates/CA-005-03/ | framework/RULES.md, R11 and R12 recorded | 2026-09-09 |

## Arbiter acceptance

What the arbiter did against the unrepaired gate, per criterion. This is the record R12 requires, and it is the first change to carry it.

| Unit | Criterion | Before repair |
|---|---|---|
| WU-005-01 | CR-005-01 | failed, on an unarbitrated criterion going undetected |
| WU-005-01 | CR-005-02 | failed, on a missing pre-acceptance record going undetected |
| WU-005-01 | CR-005-03 | passed, as it must — nothing had changed yet |

CR-005-02 passed vacuously on the first attempt: the check had nothing to look at, because this change's own execution stage did not yet exist. A check that cannot fail is not evidence of anything, so the fixture was rebuilt from an existing change with the section added — making it a folder that is subject to the obligation rather than one predating it — and the criterion then failed for the reason it names. That correction is Y8 applied to the arbiter that enforces Y8.
