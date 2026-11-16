# Assurance — trim-name-whitespace

## Evidence

| Id | Subject | Kind | Producer | Independence | How established | Repeatable | Obtained at | Valid until |
|---|---|---|---|---|---|---|---|---|
| EV-002-01-01 | candidate | test-run | `test.sh ^(TestGreetTreatsWhitespaceOnlyNameAsAbsent\|TestGreetTrimsSurroundingWhitespaceFromName)$` | independent-by-precommitment | written by WU-002-01 and failed against the unchanged estate before the change existed — candidates/CA-002-01/failing-before.txt | yes | 2026-11-15 | next change to greeting.go |
| EV-002-02-01 | candidate | test-run | `test.sh` | independent-by-executor | the estate's own suite, written before this change and outside everything it touched | yes | 2026-11-15 | next change to the estate |

## Verdicts

| Id | Unit | Criterion | Outcome | Evidence | State | Window | Baseline |
|---|---|---|---|---|---|---|---|
| VE-002-01 | WU-002-01 | CR-002-01 | met | EV-002-01-01 | settled |  |  |
| VE-002-02 | WU-002-01 | CR-002-02 | met | EV-002-01-01 | settled |  |  |
| VE-002-03 | WU-002-02 | CR-002-01 | met | EV-002-01-01 | settled |  |  |
| VE-002-04 | WU-002-02 | CR-002-02 | met | EV-002-01-01 | settled |  |  |
| VE-002-05 | WU-002-02 | CR-002-03 | met | EV-002-02-01 | settled |  |  |
