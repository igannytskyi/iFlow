# Execution — placeholder-filter

| Unit | Executor | Version | Terminal state | Candidate | Trace |
|---|---|---|---|---|---|
| WU-002-01 | Claude Opus 5 | claude-opus-5 | candidate | CA-002-01 | this file |
| WU-002-02 | Claude Opus 5 | claude-opus-5 | candidate | CA-002-02 | this file |

No deterministic transformation exists for either unit, so an agent was the correct executor rather than a defect.

## Candidates

| Id | Unit | Stored at | Artefacts | Produced at |
|---|---|---|---|---|
| CA-002-01 | WU-002-01 | candidates/CA-002-01/ | tests/test_row_filter.py | 2026-09-09 |
| CA-002-02 | WU-002-02 | candidates/CA-002-02/ | tools/check.py, the row filter | 2026-09-09 |

## Deviation

Both candidates were originally written straight into the working tree: the increment had no candidate store, so execution and landing were not separated and invariant 2 had nothing to enforce it. That gap is escalation 3, and it is now closed — a store exists, R2 checks it, and the candidates above were placed in it from version control, which had preserved exactly what each unit proposed.
