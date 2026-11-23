# Record — trim-name-whitespace

Append-only. Each crossing of a stage boundary is one line.

| # | At | Stage | Object | Note |
|---|---|---|---|---|
| 1 | 2026-11-15 | intent | INT-002 | Когда имя передано с пробелами по краям, приветствие использует имя без этих пробелов. Имя только из пробелов считается отсутствующим, и тогда приветствуется мир. |
| 2 | 2026-11-15 | specification | SPEC-002 | class C3; 3 criteria |
| 3 | 2026-11-15 | plan | SPEC-002 | 2 unit(s) |
| 4 | 2026-11-15 | admission | WU-002-01 | admitted |
| 5 | 2026-11-15 | admission | WU-002-02 | admitted |
| 6 | 2026-11-15 | execution | CA-002-01 | I don't have approval to run commands in this session. The test file is written and ready — please run `/private/var/folders/vk/hkg1rbx97bg5h8bs6f7cyhyh0000gn/T/iflow-ws-1177295712/test.sh` yourself to confirm it fails as expected against the current `Greet` implementation (which does no trimming).  I added `greeting_trim_test.go` with two tests: `TestGreetTrimsSurroundingWhitespaceFromName` (CR-002-01, checks names with leading/trailing whitespace are greeted trimmed) and `TestGreetTreatsWhitespaceOnlyNameAsAbsent` (CR-002-02, checks whitespace-only names fall back to "Hello, world!"). Both should fail against the current `Greet` implementation since it only special-cases the exact empty string and never trims. |
| 7 | 2026-11-15 | execution | CA-002-02 | All tests pass. I added `strings.TrimSpace(name)` before the empty-check in `Greet`, so whitespace-padded names are trimmed and whitespace-only names fall through to the existing "world" default. |
| 8 | 2026-11-15 | assurance | VE-002-01 | met |
| 9 | 2026-11-15 | assurance | VE-002-02 | met |
| 10 | 2026-11-15 | assurance | VE-002-03 | met |
| 11 | 2026-11-15 | assurance | VE-002-04 | met |
| 12 | 2026-11-15 | assurance | VE-002-05 | met |
| 13 | 2026-11-15 | landing | WU-002-01 | entered |
| 14 | 2026-11-15 | landing | WU-002-02 | entered |
