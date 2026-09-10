# Execution — criteria-technique

| Unit | Executor | Version | Terminal state | Candidate | Trace |
|---|---|---|---|---|---|
| WU-010-01 | Claude Opus 5 | claude-opus-5 | candidate | CA-010-01 | this file |
| WU-010-02 | Claude Opus 5 | claude-opus-5 | candidate | CA-010-02 | this file |
| WU-010-03 | Claude Opus 5 | claude-opus-5 | candidate | CA-010-03 | this file |

## Candidates

| Id | Unit | Stored at | Artefacts | Produced at |
|---|---|---|---|---|
| CA-010-01 | WU-010-01 | candidates/CA-010-01/ | framework/tests/test_criteria_technique.py | 2026-09-10 |
| CA-010-02 | WU-010-02 | candidates/CA-010-02/ | framework/check.py, adding R19 | 2026-09-10 |
| CA-010-03 | WU-010-03 | candidates/CA-010-03/ | framework/criteria.md and rules.md | 2026-09-10 |

## Arbiter acceptance

| Unit | Criterion | Before repair |
|---|---|---|
| WU-010-01 | CR-010-01 | failed, a criterion naming what the change may touch went undetected |
| WU-010-01 | CR-010-02 | failed on first writing — see below — then passed once it could fail |
| WU-010-01 | CR-010-03 | passed, as it must — nothing had changed yet |

## Deviations

Two, both caught by the method rather than by care.

The gate refused admission: `framework/rules.md` was declared an input the arbiter reads and lies inside the scope, and its prior state had not been captured. Without that capture the criterion could have been met by deleting claims instead of honouring them, and nothing would have shown it.

The second criterion passed vacuously on first writing: it compared the real rules against the real gate, and with nothing yet added there was nothing for it to catch. A check that cannot fail is not evidence of anything. It now makes two assertions — that a rule invented for the purpose is caught, and that the real rules contain none — of which only the first can fail by construction.
