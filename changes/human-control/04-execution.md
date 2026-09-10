# Execution — human-control

| Unit | Executor | Version | Terminal state | Candidate | Trace |
|---|---|---|---|---|---|
| WU-009-01 | Claude Opus 5 | claude-opus-5 | candidate | CA-009-01 | this file |
| WU-009-02 | Claude Opus 5 | claude-opus-5 | candidate | CA-009-02 | this file |
| WU-009-03 | Claude Opus 5 | claude-opus-5 | candidate | CA-009-03 | this file |
| WU-009-04 | Claude Opus 5 | claude-opus-5 | candidate | CA-009-04 | this file |

## Candidates

| Id | Unit | Stored at | Artefacts | Produced at |
|---|---|---|---|---|
| CA-009-01 | WU-009-01 | candidates/CA-009-01/ | tests/test_human_control.py | 2026-09-10 |
| CA-009-02 | WU-009-02 | candidates/CA-009-02/ | tools/check.py — R1 revised, R17 and R18 added | 2026-09-10 |
| CA-009-03 | WU-009-03 | candidates/CA-009-03/ | framework/RULES.md and CONVENTIONS.md | 2026-09-10 |
| CA-009-04 | WU-009-04 | candidates/CA-009-04/ | tests/test_rules_enforced.py — the enforcement scan follows pointers | 2026-09-10 |
| CA-009-05 | WU-009-05 | candidates/CA-009-01/ | tests/test_human_control.py — the fixture is constructed, not borrowed | 2026-09-10 |

## Arbiter acceptance

| Unit | Criterion | Before repair |
|---|---|---|
| WU-009-01 | CR-009-01 | failed, a refused object carrying a met verdict went undetected |
| WU-009-01 | CR-009-02 | failed, an edit before any candidate was treated as a violation |
| WU-009-01 | CR-009-03 | failed, silence about the review went undetected |
| WU-009-01 | CR-009-04 | passed, as it must — nothing had changed yet |
| WU-009-04 | CR-003-01 | failed, reporting B3 as claimed mechanical and never emitted |
| WU-009-05 | CR-009-02 | failed, the borrowed fixture reported a violation where the case was a re-admission |
