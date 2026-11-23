# Planning and execution

## The idea

A change moves left to right, and each step only narrows what the one before it decided:

1. **Intent.** The person says what they want, loosely.
2. **Specification.** An author turns it into a specification: what the change is to do, the criteria by which it is judged, and the scope it may touch — a package's folder where its files are not known one by one. It says *what*, not *how*: a function or a file is named only where the task names it. To write it, the author is handed the project's present state and what the project is for: where the task's words land, how those names are declared, the project's own README and AGENTS.md, and the person's earlier decisions. A reviewer checks the specification, then the person accepts it.
3. **Plan.** A planner divides the specification into units. Every unit serves criteria the specification already states. A unit invents nothing: it carries out a decision or checks one.
4. **Execution.** Agents carry the units, each in its own copy of the estate.
5. **Assurance.** The result is judged against the criteria, by tests written before the code and by the person where only a person can see.
6. **Landing.** What holds lands, in the plan's order.

**What is strict, and what is a hint.** Strict is what the person decides — the specification's criteria and scope — and the evidence that they hold: the test ledger, the runner's report, the whole suite, the person's affirmation. Everything about *how* — the plan's division into units, the files of each, the names they give each other, what they read — is foresight of code not yet read, handed to the agents as a hint. An agent who reads the code and finds the hint wrong does what the code needs, within the specification's scope, and that is recorded and shown to the person, never a failure. A run stops only where the evidence shows the criteria cannot be met as stated.

The reason to divide at all is that **each agent's piece fits its effective context**. That is the part of the window a model actually uses well, and it is far smaller than the window itself. Within it, an agent writes less, but writes it well.

The evidence (November 2026) supports this:

- Quality falls with input length even when the model retrieves everything it needs. It falls well inside the window a model claims: by 14–85% in one study, and to half at 32K tokens for 10 of 12 models in another.
- Text that is near the subject but wrong, such as a neighbouring file, does more harm than length alone.
- Narrowing what a code agent sees raises what it resolves.

The known cost of dividing is that agents decide the same thing differently. So **the shared decisions travel whole to every unit**: the specification's decisions and the contract of names and signatures. Only the code each agent is handed is cut down to what its unit needs.

## The plan

The plan has three kinds of unit:

- **contract**: declares every name the units give each other, without behaviour. The framework writes it from the plan's provisions — file, name, kind, the type a field or method belongs to, its declaration — each language by its own declarer (Go, Python, JavaScript); a function or method fails as not yet done when called. Only where a file's language has no declarer is it handed to an agent;
- **tests**: tests written first, which fail before the change;
- **code**: the code itself.

```
contract ──► tests ──► code (chain A) ─┐
                       code (chain B) ─┼──► the last code unit
                       code (chain C) ─┘
```

Code units that change a file in common form a chain and are carried in order. A code unit also waits for every earlier one whose files it reads or whose names it calls: the contract declares a name, but only the unit that writes it gives it the behaviour the later unit's tests ask for. A criterion is judged at the code unit that serves it last: that unit waits for every other that serves it, and its check is the build and then the criterion's tests, as the test ledger names them. Units that wait for nothing of each other are carried at the same time; that is the only source of parallel work, and it is computed from the plan, not chosen. The last code unit completes the change: it must pass the change's tests and then the whole suite.

A plan that breaks a rule of R24 below goes back to its planner, told which and why: those rules keep the evidence sound — tests before code, every criterion served and judged, nothing writing what judges it. P1–P12 are one function of the plan and the specification, `meta.PlanBreaches`, and the gate holds every plan to it as rule R24. What depends on the estate as it stood — P7, P10's estate half, P11, P13 — is foresight: a hint the estate shows wrong is set aside and said, and the plan stands. The planner gets at most three tries. If no plan holds, area 2 stops and says why; nothing is made up in the planner's place.

| | Rule | Checked |
|---|---|---|
| P1 | Contract first, then tests units, then at least one code unit | R24 |
| P2 | Every unit has a kind, a scope, an objective and tasks | R24 |
| P3 | A unit writes only what the specification lets its kind write | R24 |
| P4 | Every new test file is written by exactly one tests unit | R24 |
| P5 | Every criterion is served by a code unit; every unit serves one; a tests unit only what the new tests judge | R24 |
| P6 | Every name provided has a file in scope, a kind and a signature | R24 |
| P7 | No unit provides a name its package already declares | a hint: such a provision is set aside, and the unit changes the name as its tasks say |
| P8 | No name is provided twice in one package | R24 |
| P9 | A contract exists whenever tests and code are written apart | R24 |
| P10 | A unit uses only names the estate defines or an earlier unit provides | R24 within the plan; against the estate, a hint |
| P11 | A unit reads only files that exist or another unit writes | a hint: a read of nothing is set aside |
| P12 | No unit's scope holds what judges the change | R24 |
| P13 | A unit fits one agent: its brief, the specification's decisions and the files it changes whole, within the budget | a hint: what does not fit is handed by its outline and read by parts |

## Carrying a unit

A unit is carried in a copy of the estate holding exactly the candidates of the units it waits for, and no others. What its agent is handed, and what it asks of the estate as it works, comes from that copy.

What comes of each carrying is one outcome, and the outcome decides what happens next:

| Outcome | What follows |
|---|---|
| candidate | kept; the units waiting on it may go |
| holds — the tests pass before any code | the change is closed: nothing to do |
| contract acts — the tests pass only with the contract | contract and tests carried again, once; it cannot happen where the framework declares the contract |
| the contract cannot be declared as the plan states it | the plan goes back to its planner, with why, once |
| tests wrong — a code unit blames a test file | that tests unit and the blaming unit carried again, once; wrong again, the specification goes back to its author with why, once, and the person accepts it again |
| a file beyond the unit — the agent writes a file of the specification's scope its unit was not given | given it, where nothing judges it and no unit carried apart from it writes it; recorded in the execution (`widened`) and shown to the person; refused otherwise, with why |
| cannot — the unit cannot be done as planned | the plan goes back to its planner, once; where what must change is a file outside the specification's scope, the specification goes back to its author, once |
| failed | area 4 stops and says why; the change can be resumed |

Every reason to carry a unit again has a bound, so area 4 always ends. Where a bound is reached because what a unit was built from is at fault, the change goes back to the step that decided it — the specification or the plan — handed what it decided and why a later step could not build on it, once a run for each; it is not mended where it failed.

## What each agent is handed

Everything below fits one budget: 16000 tokens unless `IFLOW_PACK_TOKENS` says otherwise.

| | Author | Planner | Contract | Tests | Code |
|---|---|---|---|---|---|
| the intent | ✓ | ✓ | ✓ | ✓ | ✓ |
| the project's own words (README, AGENTS.md), earlier decisions | ✓ | | | | |
| the estate's map, by names | ✓ | ✓ | | | |
| the specification, whole | | ✓ | | | |
| its decisions and the unit's criteria | | | ✓ | ✓ | ✓ |
| the contract | | | ✓ | ✓ | ✓ |
| its files and what it reads, whole | | | ✓ | ✓ | ✓ |
| the tests it must pass, whole, by the test ledger | | | | | ✓ |
| what earlier units wrote, by path | | | | ✓ | ✓ |
| around them: signatures and names only | | | ✓ | ✓ | ✓ |

A tests unit answers, besides its tests, with its part of the **test ledger**: each test it wrote, its file, and the criterion it judges. Its check holds the ledger to what it wrote; the ledger is kept in the execution document, and every later step that needs to know which test judges what reads it there ([handoffs.md](handoffs.md)).

What else it needs, the agent reads with its tools: files by parts, where a name is defined, who uses it.

The budget is spent in this order:

1. its goal;
2. the tests it must pass;
3. the files it changes, whole, unless one alone is longer than the whole budget;
4. what it reads, reduced to an outline as soon as it would crowd out the rest;
5. signatures and names.

The tokens each agent was handed are recorded with its costs.

## Open

1. Outcomes for blaming the contract or an earlier code unit, each with its bound.
2. A test that drives the scheduler through random outcomes and shows that it always ends.
3. Tools' machine-readable reports in place of patterns over their output.
4. A bench task on an estate large enough for the budget to matter. Today's bench hands 2–5k tokens a unit, so it cannot show the budget's effect.
