# Execution — evidence-plan

| Unit | Executor | Version | Terminal state | Candidate | Trace |
|---|---|---|---|---|---|
| WU-008-01 | Claude Opus 5 | claude-opus-5 | candidate | CA-008-01 | this file |
| WU-008-02 | Claude Opus 5 | claude-opus-5 | candidate | CA-008-02 | this file |
| WU-008-03 | Claude Opus 5 | claude-opus-5 | candidate | CA-008-03 | this file |

## Candidates

| Id | Unit | Stored at | Artefacts | Produced at |
|---|---|---|---|---|
| CA-008-01 | WU-008-01 | candidates/CA-008-01/ | tests/test_evidence_plan.py | 2026-09-10 |
| CA-008-02 | WU-008-02 | candidates/CA-008-02/ | tools/check.py, adding R16 | 2026-09-10 |
| CA-008-03 | WU-008-03 | candidates/CA-008-03/ | framework/RULES.md and CONVENTIONS.md | 2026-09-10 |

## Arbiter acceptance

| Unit | Criterion | Before repair |
|---|---|---|
| WU-008-01 | CR-008-01 | failed, a criterion needing a plan and having none went undetected |
| WU-008-01 | CR-008-02 | failed, a plan with nothing behind it went undetected |
| WU-008-01 | CR-008-03 | passed, as it must — nothing had changed yet |

## Deviation

The specification was first written with `When` and `Then` replacing `Criterion`, and B1 refused it: the column a check binds to was gone, so that check would not have run. The scenario columns were made additive instead, and the digest recorded at admission was taken again — legitimately, because nothing had executed against the earlier text. **Y6 caught the change that introduces the shape it governs**, which is the third time a rule has found a defect in the change adding it.
