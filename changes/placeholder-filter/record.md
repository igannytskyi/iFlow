# Record — placeholder-filter

Append-only. Each crossing of a stage boundary is one line.

| # | At | Stage | Object | Digest | Note |
|---|---|---|---|---|---|
| 1 | 2026-09-09 | intent | INT-002 | | a criterion can go unjudged while the gate reports passed |
| 2 | 2026-09-09 | specification | SPEC-002 | | class C2; arbiter is tests/, disjoint from tools/check.py |
| 3 | 2026-09-09 | plan | SPEC-002 | | two units; WU-002-01 preparatory, no tests exist yet |
| 4 | 2026-09-09 | admission | WU-002-01 | | admitted |
| 5 | 2026-09-09 | admission | WU-002-02 | | admitted behind WU-002-01 |
| 6 | 2026-09-09 | execution | CA-002-01 | | reproduction written; fails on the unmodified gate for the stated reason |
| 7 | 2026-09-09 | execution | CA-002-02 | | repair to the row filter |
| 8 | 2026-09-09 | assurance | VE-002-01 | | met |
| 9 | 2026-09-09 | assurance | VE-002-02 | | met |
| 10 | 2026-09-09 | assurance | VE-002-03 | | undecided; evidence would be circular |
| 11 | 2026-09-09 | landing | WU-002-01 | | entered |
| 12 | 2026-09-09 | landing | WU-002-02 | | entered with an open escalation |
| 13 | 2026-09-09 | landing | VE-002-03 | | escalation 4 raised: entry on an undecided verdict is unforbidden |
