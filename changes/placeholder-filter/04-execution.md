# Execution — placeholder-filter

| Unit | Executor | Version | Terminal state | Candidate | Trace |
|---|---|---|---|---|---|
| WU-002-01 | Claude Opus 5 | claude-opus-5 | candidate | CA-002-01 | this file |
| WU-002-02 | Claude Opus 5 | claude-opus-5 | candidate | CA-002-02 | this file |

No deterministic transformation exists for either unit, so an agent was the correct executor rather than a defect.

## Candidates

| Id | Unit | Artefacts | Produced at |
|---|---|---|---|
| CA-002-01 | WU-002-01 | tests/test_row_filter.py | 2026-09-09 |
| CA-002-02 | WU-002-02 | tools/check.py, the row filter | 2026-09-09 |

## Deviation

Both candidates were written straight into the working tree. This increment has no candidate store, so execution and landing are not separated here, and invariant 2 was not enforceable. Recorded rather than hidden; see the escalation in 05-assurance.
