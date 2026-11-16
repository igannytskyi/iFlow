# Execution — greet-the-world

| Unit | Executor | Version | Terminal state | Candidate |
|---|---|---|---|---|
| WU-001-01 | Claude Opus 5.5 | claude-opus-5-5 | candidate | CA-001-01 |
| WU-001-02 | Claude Opus 5.5 | claude-opus-5-5 | candidate | CA-001-02 |

## Candidates

| Id | Unit | Stored at | Artefacts | Produced at |
|---|---|---|---|---|
| CA-001-01 | WU-001-01 | candidates/CA-001-01/ | the reproduction, greeting_world_test.go, and its failing run | 2026-11-14 |
| CA-001-02 | WU-001-02 | candidates/CA-001-02/ | the repair to the greeting, its patch, and its passing run | 2026-11-14 |

The reproduction was run against the greeting as it stood and failed for the stated reason: it greeted an empty name. The repair was run beside it in a copy of the estate, never in the live tree.
