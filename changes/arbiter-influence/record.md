# Record — arbiter-influence

Append-only. Each crossing of a stage boundary is one line.

| # | At | Stage | Object | Digest | Note |
|---|---|---|---|---|---|
| 1 | 2026-09-09 | intent | INT-004 | | path disjointness is not influence disjointness |
| 2 | 2026-09-09 | specification | SPEC-004 | | first specification to declare what its arbiter reads |
| 3 | 2026-09-09 | plan | SPEC-004 | | three units |
| 4 | 2026-09-09 | admission | WU-004-01 | | admitted |
| 5 | 2026-09-09 | admission | WU-004-02 | | admitted |
| 6 | 2026-09-09 | admission | WU-004-03 | | admitted |
| 7 | 2026-09-09 | admission | CR-004-01 | | prior rules and gate captured before execution |
| 8 | 2026-09-09 | execution | CA-004-01 | | arbiter written; corrected once for a spurious baseline, then failed on the stated reasons |
| 9 | 2026-09-09 | execution | CA-004-02 | | R7b and the column-dependency check added to the gate |
| 10 | 2026-09-09 | execution | CA-004-03 | | R7b recorded, B1 widened |
| 11 | 2026-09-09 | execution | CA-004-04 | | earlier arbiter narrowed to verdicts; unplanned unit under invariant 8 |
| 12 | 2026-09-09 | assurance | VE-004-01 | | met, all candidates verified in an isolated tree |
| 13 | 2026-09-09 | assurance | VE-004-07 | | met, judged against CR-003-04 separately from the unit that benefits |
| 14 | 2026-09-09 | assurance | — | | escalation 2 raised: an arbiter may test more than its criterion says |
| 15 | 2026-09-09 | landing | WU-004-04 | | entered first, re-establishing CR-003-04 |
| 16 | 2026-09-09 | landing | WU-004-02 | | entered |
| 17 | 2026-09-09 | landing | WU-004-04 | | R8 fired: the preparatory relationship to WU-004-02 had been left unstated |
