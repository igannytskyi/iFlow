# Execution — repeatability

| Unit | Executor | Version | Terminal state | Candidate | Trace |
|---|---|---|---|---|---|
| WU-006-01 | Claude Opus 5 | claude-opus-5 | candidate | CA-006-01 | this file |
| WU-006-02 | Claude Opus 5 | claude-opus-5 | candidate | CA-006-02 | this file |
| WU-006-03 | Claude Opus 5 | claude-opus-5 | candidate | CA-006-03 | this file |

## Candidates

| Id | Unit | Stored at | Artefacts | Produced at |
|---|---|---|---|---|
| CA-006-01 | WU-006-01 | candidates/CA-006-01/ | tests/test_repeatability.py | 2026-09-09 |
| CA-006-02 | WU-006-02 | candidates/CA-006-02/ | tools/check.py, adding R13 and R14 | 2026-09-09 |
| CA-006-03 | WU-006-03 | candidates/CA-006-03/ | framework/CONVENTIONS.md and RULES.md | 2026-09-09 |

## Arbiter acceptance

| Unit | Criterion | Before repair |
|---|---|---|
| WU-006-01 | CR-006-01 | failed, on missing repeatability going undetected |
| WU-006-01 | CR-006-02 | failed, on a verdict met over an unrepeatable run going undetected |
| WU-006-01 | CR-006-03 | passed, as it must — nothing had changed yet |

This record is itself an account of a past run by an agent, so by the rule this change introduces it is testimony, not derivation. That is why the arbiter it describes is a deterministic script: the script can be re-run against the captured prior gate, and this table cannot.
