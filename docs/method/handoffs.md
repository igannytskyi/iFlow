# Handoffs

What passes from one step to the next, in what form, and who reads it. A handoff is sound when what the later step needs is in the object as data, written by the step that knows it and checked as it is written. A handoff is unsound when the later step must recover it by reading text: a regular expression over a tool's output, a guess at which file a path means, a model's prose. Every defect the real-size bench found (11-21 to 11-23) was at an unsound handoff.

Single-agent harnesses have no such handoffs: one agent holds the task, the code and the test output in one context, and recovers what it needs by reading. Dividing the work, which is what keeps each agent inside its effective context, is only sound if every boundary carries what the next side needs as data.

## The chain, as it stood on 11-23

| # | From → to | What passes | Form | Sound |
|---|---|---|---|---|
| 1 | person → author | the intent | text | yes: it is the person's words |
| 2 | estate → author | names, declarations, project's own words | data from the estate | yes |
| 3 | author → plan | criteria (id, when, then, judged by new tests), scope, new test files, existing tests, relies_on | typed JSON, checked | yes, since relies_on is declared by the framework (11-23) |
| 4 | plan → units | each unit: kind, scope, criteria, provides, uses, reads | typed JSON, R24 | mostly: criteria ids are written by the planner, not chosen from the specification's |
| 5 | plan → scheduler | which unit waits for which | derived from scope, reads, uses | yes, since reads and uses order code units (11-23) |
| 6 | tests unit → everyone after | **which tests it wrote, and which criterion each judges** | **not passed at all**: names recovered by a pattern over the files; the criterion of each test never stated | **no** |
| 7 | code unit ← its check | **whether the criteria it serves now hold** | **not asked**: a code unit before the last is checked by the build only | **no** |
| 8 | test run → framework | which tests passed, which failed, where | **a pattern over the runner's human output** (`--- FAIL`, `_test.go:61`) | **no**: a panic, a timeout, a goroutine's failure, a path as the copy holds it, all escape it |
| 9 | agent → framework | the unit cannot be done; the tests are wrong; which files | free text, and paths the framework then guesses at (by base name, by suffix) | **no** |
| 10 | candidates → assurance | each criterion met or not | the pass of **all** tests of a tests unit stands for every criterion it served | **no**: one failing test fails every criterion of its unit; a criterion with no test of its own is "met" by the suite |

## What follows from the unsound ones

- **6 → 8 → 9 → 10 are one missing object: the test ledger.** With no record of which test judges which criterion and was written by which unit, every later step that needs it reconstructs it from text, and each reconstruction has failed once: test names by pattern (fuzz tests missed), failures by pattern (a panic missed), the blamed file by path (an absolute path missed), the criterion by unit (all-or-nothing).
- **7 makes the last unit the real executor.** No code unit before it is told whether its criteria hold, so everything that does not hold is found at the end, by one agent with the whole change in its scope — the single-agent harness we divided the work to avoid, at the moment it is least able to help.

## The sound chain

One object, the **test ledger**, written once and read by every later step:

```
criterion CR-n  ──(specification)──►  judged by new tests in file F
      │
      ▼  (plan)
tests unit T writes F, for criteria [CR…]       code units serve [CR…]
      │
      ▼  (T's answer, checked by the framework)
ledger: test name · file · criterion · written by T
      │
      ├──► code unit U: its check runs the tests of every criterion U is the last to serve,
      │     read from the runner's machine-readable report
      │
      ├──► a test that fails: its record says which criterion and which tests unit — no guess
      │
      └──► assurance: each criterion met iff each of its own tests passed
```

1. **The specification** already names, per criterion, whether new tests judge it, and the new test files. The plan's criteria are chosen from the specification's ids (an enumeration in the planner's schema), never written.
2. **A tests unit answers with its ledger**: for each test it wrote, its name, its file and the criterion it judges. The framework checks the answer against what was written, by the language's own parser (go/ast for Go, Python's ast for Python), not a pattern: every name is a test in that file; every criterion of the unit has at least one test; every test judges a criterion of the unit. A ledger that does not hold goes back to the tests unit, with each break named.
3. **The ledger is kept in the execution document** (`meta.Execution.Tests`), beside the candidates, typed and checked by the gate.
4. **A criterion is judged at the code unit that serves it last**, in the order the scheduler carries them; that unit waits for every other unit that serves the same criterion. Its check is the build, then the tests of those criteria. Earlier units are checked by the build. The last unit runs the whole suite, as now.
5. **Test results are read from the runner's machine-readable report**: `go test -json` for Go, JUnit XML for pytest, node's test runner, Gradle and Maven; for a runner without one, the framework says it cannot judge per test, and falls back to the run as a whole, never to a pattern over prose.
6. **What an agent says of a failure is said in the ledger's terms**: the tests it holds wrong are named by test name, from the list it is given; the files outside its scope by path, from the estate. The framework checks each against the ledger and the estate, and refuses a name it does not hold.
7. **Assurance reads the ledger**: a verdict per criterion, from its own tests; a criterion judged by new tests with none in the ledger is not met.

## The order of the work

1. The test ledger: its type, the tests unit's answer, its check, kept in the execution document. (6) — done 11-23: a tests unit writes `iflow-tests.json`, held by its check to what it wrote (Go read by go/ast), kept as `meta.Execution.Tests`; the test names every later step runs are the ledger's; a run resumed from an execution without it carries its tests units again.
2. Machine-readable test reports, Go first, the others by the report each runner already writes. (8) — done 11-23: `go test -json` (a build that fails is read as such, not as tests missing), JUnit XML for pytest and node; the patterns over the runners' prose (`--- FAIL`, `_test.go:61`) are gone; cargo and a script of the estate's own report no test by test, and their run is judged as a whole.
3. A code unit's check: the tests of the criteria it is last to serve. (7) — done 11-23: it builds first, then runs those tests; it waits for every other unit serving them; it is handed whole the test files of the criteria it serves, the tests that must pass after it named.
4. Failures in the ledger's terms: the blame by test name, mapped through the ledger. (9)
5. Assurance per criterion. (10)
6. The planner's criteria as an enumeration of the specification's. (4)

Each step removes a reconstruction; none adds one. When all six stand, no step of the flow reads another step's text to find out what it did.
