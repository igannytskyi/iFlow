# Planning and execution

How a specified change becomes candidates: what a plan is, what must hold of every plan before anything is carried, how each unit of it is carried and what follows from what comes of it, and what every agent is handed. Areas 2 and 4; area 3 admits what area 2 planned, area 5 judges what area 4 made.

This is the design those areas are built to. Where the code does not yet do what it says, the last column of each table says so; that column is the work outstanding, and nothing else is.

## Terms

| Term | Meaning |
|---|---|
| unit | One piece of work, carried by one agent in one copy of the estate, kept as one candidate. Its **kind** is `contract`, `tests` or `code` |
| scope | The files a unit may change. Nothing outside it is written |
| provision | A name a code unit adds and others build on: its file, its name, its kind (function, method, type, field, variable, constant) and its signature |
| uses | The names of other units' provisions a unit builds on |
| reads | Files outside a unit's scope its agent is handed whole: files of the estate, or files another unit writes |
| contract | The unit that declares every provision, without behaviour, before anything else is written |
| chain | Code units that change a file in common, carried in order |
| the last unit | The last code unit. It completes the change: it may change any file a code unit may, and must pass the change's tests and then the whole suite |
| carrying | One attempt at a unit by its executor. A unit is carried again for a reason; each carrying has a generation |
| outcome | What came of a carrying (below) |

## 1. What a plan is

A plan is an ordered list of units and, implied by it, a graph of what each waits for.

```
contract ──► tests ──► code (chain A) ─┐
    │          │       code (chain B) ─┼──► the last unit
    └──────────┴──────►code (chain C) ─┘
```

- The **contract** comes first where it exists (P9). It waits for nothing.
- Each **tests** unit waits for the contract.
- Each **code** unit but the last waits for the contract, every tests unit, and the unit before it in its chain.
- The **last unit** waits for every other unit.

Units that wait for nothing undone are carried at once; this is the only source of parallel work, and it is computed, not chosen.

## 2. What must hold of every plan

A plan that breaks any of these is not carried. It goes back to its planner with which, and why, at most as many times as the planner is asked; if no plan holds, area 2 stops and says so (§2.3). Nothing is mended in its place.

### 2.1 The invariants

| | Invariant | Why | Checked now |
|---|---|---|---|
| P1 | At most one contract unit, first; then the tests units; then at least one code unit | The order is the dependency graph | built so by the framework; not checked as such |
| P2 | Every unit names its kind, a non-empty scope, an objective and at least one task | A unit with none of these cannot be carried | `unitsOf` |
| P3 | A tests unit's scope is among the specification's new tests; a code unit's, among its scope; the contract's, the files its provisions name | Nothing writes outside what was specified | `unitsOf`; the contract's by construction |
| P4 | Every new test file is written by exactly one tests unit | A test file no unit writes judges nothing; two writing it overwrite each other | written by some unit: `unitsOf`; by exactly one: **not checked** |
| P5 | Every criterion is carried by at least one code unit; a tests unit carries exactly criteria the new tests judge; every unit carries at least one criterion | Every unit that enters needs a verdict (A6); every criterion needs code that makes it hold | first: mended, with a note, by giving it to the last unit; second: `unitsOf`; third: mended by giving the suite's criteria — **both mendings to be replaced by sending back** |
| P6 | Every provision has a name, a file in its unit's scope, a kind other than described, and a signature | A provision that cannot be checked is a guess | `provisionProblem` |
| P7 | Every provision is a name its file does not declare already | A provision of an existing name would be declared again by the contract and break what uses it | `providedAlready` |
| P8 | No name is provided twice in one package | Two units would declare it twice | **not checked** |
| P9 | A contract exists exactly when tests units exist and code units provide at least one name; it declares every provision of the plan | Tests written first must build against what they call | `withContract` — today only where two or more code units provide; one code unit is P9's open case |
| P10 | Every name a unit uses is provided by an earlier code unit or defined in the estate | Nothing builds on what does not exist yet | `orderOf` |
| P11 | Every file a unit reads exists in the estate, or another unit writes it; a tests unit reads only files that exist or that the contract writes | Tests are carried before any code but the contract's | first half: `unitsOf`; tests' half: **not checked** |
| P12 | No unit's scope holds what judges the change: the estate's runner, or a test of the suite | What judges a change lies outside what it may touch (R7, R8) | the specification's scope, at area 1; **not re-checked on the plan** |

### 2.2 Where they are checked

Each invariant is checked once, by one pure function of the plan and the estate — `meta.PlanProblems(plan, estate)` — returning every breach, each named by its invariant. The planner's answer is held to it before area 2 ends, and the gate holds the plan document to it as a rule, so that a plan written or changed any other way is held to the same. Today the checks above live in `flow` in five functions, two invariants are mended instead of sent back, and the gate checks none of them.

### 2.3 When no plan holds

The planner is asked at most three times, each time told every breach of its last answer. If none holds, area 2 stops, saying the last breaches; the person reopens the change from its plan with what to do differently. No plan is derived in the planner's place.

## 3. Carrying a unit

### 3.1 What a unit is carried on

A unit's agent works in a copy of the estate holding the candidates of every unit it waits for, near and far, in the plan's order. It is handed what §5 says, and has the tools of §5.2.

### 3.2 What comes of a carrying

Every carrying ends in exactly one outcome. Where more than one could be said, the first in this order is:

| Outcome | When | Read from |
|---|---|---|
| `holds` | A tests unit's tests pass on the estate without the contract | the run of its tests, twice: on the contract, then on the estate alone |
| `contract-acts` | A tests unit's tests pass on the contract but not on the estate alone | the same two runs |
| `tests-wrong` | A code unit's carrying ended blaming a file a tests unit wrote — by its agent naming it (`cannot` with files), by its check failing only there, or by spending its steps while the change's tests still fail there | the result's `Blames`, matched to tests units' files |
| `contract-wrong` | A tests or code unit blames a file only the contract wrote | the result's `Blames`, matched to the contract's files |
| `earlier-wrong` | A code unit blames a file only an earlier code unit wrote | the result's `Blames`, matched to code units' files |
| `cannot` | Its agent says it cannot be done as planned, blaming no unit's file | the result's `Cannot` |
| `candidate` | Its check passed and every name it provides is declared — whether or not it changed anything (a unit may find its work done) | the result's terminal, and the code's declarations |
| `failed` | Anything else | — |

### 3.3 What follows

| Outcome | What follows | At most | Then |
|---|---|---|---|
| `candidate` | kept; the units waiting on it may go | — | — |
| `holds` | the change is closed as an answer: nothing to change | — | — |
| `contract-acts` | the contract carried again, told why; then every tests unit | once | `failed` |
| `contract-wrong` | the contract carried again, told why; then the units that wait on it and were carried | once | `failed` |
| `tests-wrong` | the tests unit carried again, told why; then the unit that blamed it | once per tests unit | `failed` |
| `earlier-wrong` | the earlier unit carried again, told why; then the unit that blamed it | once per unit | `failed` |
| `cannot` | the plan goes back to its planner, told why; areas 2–4 again | once per run | `failed` |
| `failed` | area 4 stops, saying why; the change can be resumed | — | — |

A unit sent to be carried again while it is being carried is carried again once that carrying ends; what that carrying made is set aside. Every reason to carry again has its bound, and a run of `n` units is carried at most `n × (1 + bounds)` times before it lands or stops: area 4 always ends.

`contract-wrong` and `earlier-wrong` are **not yet distinguished**: today a blame that matches no tests unit is `failed`.

### 3.4 A unit's own check

| Kind | Its check, while carried | Done when |
|---|---|---|
| contract | the estate builds | it builds, and every provision is declared |
| tests | its tests build, run none | they build; then they fail on the estate, and are named by the runner |
| code | the estate builds | it builds, and every name it provides is declared |
| the last unit | the change's own tests, then the whole suite | both pass |

After every edit the agent is told whether the files it changed still build, in its language's own check.

### 3.5 Reading what tools say

What a build or a test run says — which tests failed, which file and line, whether anything ran — is read from each tool's machine-readable report where it has one: `go test -json`, JUnit XML from pytest, node's test runner and Gradle, cargo's JSON. **Today it is read from their human output by patterns in the language table**; every pattern there is to be replaced by a report the tool writes for machines.

## 4. After area 4

Area 5 judges the candidates on the evidence of the tests written first, the whole suite, and — for what only a person can see — the person's affirmation. A person who does not affirm sends the change back to area 4, with what they said, at most twice (§3.3's table, one row more: `refused`, from admission again). Area 6 lands the candidates in the plan's order.

## 5. What each agent is handed

### 5.1 The matrix

| Handed | Author | Spec reviewer | Planner | Plan reviewer | Contract | Tests | Code | Last unit |
|---|---|---|---|---|---|---|---|---|
| the intent, as stated | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| the estate's files and what each defines | ✓ | | ✓ | | | | | |
| where the task's words land in the estate | ✓ | | | | | | | |
| the estate's runner, a script whole | ✓ | | | | | | | |
| the person's earlier decisions | ✓ | | | | | | | |
| the specification, whole | | ✓ | ✓ | ✓ | | | | |
| the specification's decisions: requirements, data model, interfaces, components, edge cases | | | | | ✓ | ✓ | ✓ | ✓ |
| its test strategy | | | | | | ✓ | | |
| the criteria the unit serves | | | | | ✓ | ✓ | ✓ | ✓ |
| what exists where the change works, by first lines | | | ✓ | ✓ | | | | |
| the plan, whole | | | | ✓ | | | | |
| its own ticket | | | | | ✓ | ✓ | ✓ | ✓ |
| every provision of the plan (the contract) | | | | | ✓ | ✓ | ✓ | ✓ |
| its scope's files, whole | | | | | ✓ | ✓ | ✓ | ✓ |
| what it reads, whole | | | | | ✓ | ✓ | ✓ | ✓ |
| what uses and is used by its scope | | | | | ✓ | ✓ | ✓ | ✓ |
| the contract's candidate | | | | | | ✓ | ✓ | ✓ |
| the tests' candidates (must pass) | | | | | | | ✓ | ✓ |
| its chain's earlier candidates | | | | | | | ✓ | ✓ |
| every other unit's candidate | | | | | | | | ✓ |
| why it is carried again | | | | | ✓ | ✓ | ✓ | ✓ |
| what the person said, not affirming | | | | | | | ✓ | ✓ |

Nothing an agent is not handed reaches it: no agent sees the person, no tests agent sees code that has behaviour, no agent sees another unit's ticket.

What a unit is handed of other units' candidates is exactly what its copy of the estate holds — the candidates of the units it waits for. **Today a code unit is handed every earlier unit's candidate that exists when it starts**, which, with chains carried at once, depends on which finished first: what it reads and what it works on can differ, and from one run to the next.

### 5.2 What an agent can do

An executor's agent reads files and the libraries' source, searches, asks the estate's model where a name is defined and who uses it, runs the change's tests by name and its check, and writes only within its scope. It says a unit cannot be done as planned with `cannot`, naming the files to blame where it can. What it may and may not do is enforced by the harness, never by its instructions.

## 6. The work this document leaves

In the order it is to be done:

1. `meta.PlanProblems`: P1–P12 as one function, a gate rule holding the plan document to it, a table test of one breach each; `unitsOf`'s checks and the two mendings of P5 replaced by it.
2. What a unit is handed of other units' candidates made the candidates it is carried on, and no others.
3. P9 for one code unit: the contract wherever tests and code are written apart.
4. `contract-wrong` and `earlier-wrong` as outcomes, and the bound on each.
5. A test that drives the scheduler through random sequences of outcomes and shows that it always ends, never keeps a candidate twice, and never carries a unit past its bound.
6. Tools' machine reports in place of the patterns in the language table.
7. `flow` divided by responsibility: the plan and its problems, the scheduler, what is handed, what is recorded.

## 7. For the person to decide

- The gate rule holding a plan to P1–P12 is a new rule of the method. Its identifier, and whether it holds changes carried before the plan had kinds, are the person's to decide.
- Whether a plan that breaks P5 by leaving a criterion to no code unit is sent back, as everything else is, or given to the last unit with a note, as now.
