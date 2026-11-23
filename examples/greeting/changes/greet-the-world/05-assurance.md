# Assurance — greet-the-world

## Evidence

| Id | Subject | Kind | Producer | Independence | How established | Repeatable | Obtained at | Valid until |
|---|---|---|---|---|---|---|---|---|
| EV-001-01-01 | candidate | test-run | `test.sh TestGreetTheWorld` | independent-by-precommitment | the reproduction was written and failed against the unchanged greeting before the repair existed — candidates/CA-001-01/failing-before.txt | yes | 2026-11-14 | next change to greeting.go |
| EV-001-02-01 | candidate | test-run | `test.sh` | independent-by-executor | the estate's own suite, written before this change and lying outside everything it touched | yes | 2026-11-14 | next change to greeting.go |

## Verdicts

| Id | Unit | Criterion | Outcome | Evidence | State | Window | Baseline |
|---|---|---|---|---|---|---|---|
| VE-001-01 | WU-001-02 | CR-001-01 | met | EV-001-01-01 | settled |  |  |
| VE-001-02 | WU-001-02 | CR-001-02 | met | EV-001-02-01 | settled |  |  |
| VE-001-03 | WU-001-02 | CR-001-03 | met | EV-001-02-01 | settled |  |  |
| VE-001-04 | WU-001-01 | CR-001-01 | met | EV-001-01-01 | settled |  |  |

## Escalations

| # | Standing | Ground | Raised against | To |
|---|---|---|---|---|
| 1 | open | A name made only of spaces is greeted as it is, and no criterion says whether that is a name — the criteria under-determine the result there, which belongs to area 1 | area 1 | the example's author |

## Escalations closed

| Escalation | Resolved by | Note |
|---|---|---|
| greet-the-world#1 | Illia Gannytskyi, by a decision | resolved by trim-name-whitespace, which landed after it: a name made only of whitespace is now absent, and Greet greets the world |
