# Record — escalation-ledger

Append-only. Each crossing of a stage boundary is one line.

| # | At | Stage | Object | Digest | Note |
|---|---|---|---|---|---|
| 1 | 2026-09-10 | intent | INT-007 | | two escalations open that were resolved runs ago |
| 2 | 2026-09-10 | specification | SPEC-007 | | three criteria |
| 3 | 2026-09-10 | plan | SPEC-007 | | three units |
| 4 | 2026-09-10 | admission | WU-007-01 | | admitted |
| 5 | 2026-09-10 | admission | WU-007-02 | | admitted |
| 6 | 2026-09-10 | admission | WU-007-03 | | admitted |
| 7 | 2026-09-10 | admission | CR-007-03 | | prior gate captured before execution |
| 8 | 2026-09-10 | execution | CA-007-01 | | arbiter written; failed on the two stated criteria |
| 9 | 2026-09-10 | execution | CA-007-02 | | ledger and R15 implemented |
| 10 | 2026-09-10 | execution | CA-007-03 | | escalation addressing recorded in the conventions |
| 11 | 2026-09-10 | assurance | VE-007-01 | | met, verified in an isolated tree |
| 12 | 2026-09-10 | assurance | — | | two debts settled: rules-enforced#1, arbiter-influence#2 |
| 13 | 2026-09-10 | assurance | — | | escalation 2 raised: three rules now share one limit |
| 14 | 2026-09-10 | landing | WU-007-01 | | entered |
| 15 | 2026-09-10 | landing | WU-007-03 | | entered |
| 16 | 2026-09-10 | landing | WU-007-02 | | entered |
| 17 | 2026-09-10 | execution | CA-007-04 | | earlier arbiter's emission scan widened; unplanned unit under invariant 8 |
| 18 | 2026-09-10 | landing | WU-007-04 | | entered, re-establishing CR-003-01 after the fact |
