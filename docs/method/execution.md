# Planning and execution

## The idea

A change moves left to right, and each step only narrows what the one before it decided:

1. **Intent.** The person says what they want, loosely.
2. **Specification.** An author turns it into a specification. To write it, the author is handed the project's present state and what the project is for as a whole, not only the task: the estate's files and names, where the task's words land, the project's own README and AGENTS.md, and the person's earlier decisions. A reviewer checks the specification, then the person accepts it.
3. **Plan.** A planner divides the specification into units. Every unit serves criteria the specification already states. A unit invents nothing: it carries out a decision or checks one.
4. **Execution.** Agents carry the units, each in its own copy of the estate.
5. **Assurance.** The result is judged against the criteria, by tests written before the code and by the person where only a person can see.
6. **Landing.** What holds lands, in the plan's order.

The reason to divide at all is that **each agent's piece fits its effective context**. That is the part of the window a model actually uses well, and it is far smaller than the window itself. Within it, an agent writes less, but writes it well.

The evidence (November 2026) supports this:

- Quality falls with input length even when the model retrieves everything it needs. It falls well inside the window a model claims: by 14–85% in one study, and to half at 32K tokens for 10 of 12 models in another.
- Text that is near the subject but wrong, such as a neighbouring file, does more harm than length alone.
- Narrowing what a code agent sees raises what it resolves.

The known cost of dividing is that agents decide the same thing differently. So **the shared decisions travel whole to every unit**: the specification's decisions and the contract of names and signatures. Only the code each agent is handed is cut down to what its unit needs.

## The plan

The plan has three kinds of unit:

- **contract**: declares every name the units give each other, without behaviour;
- **tests**: tests written first, which fail before the change;
- **code**: the code itself.

```
contract ──► tests ──► code (chain A) ─┐
                       code (chain B) ─┼──► the last code unit
                       code (chain C) ─┘
```

Code units that change a file in common form a chain and are carried in order. Separate chains are carried at the same time; that is the only source of parallel work, and it is computed from the plan, not chosen. The last code unit completes the change: it must pass the change's tests and then the whole suite.

A plan that breaks any of the rules below goes back to its planner, told which rule and why. The planner gets at most three tries. If no plan holds, area 2 stops and says why; nothing is made up in the planner's place.

| | Rule | Checked |
|---|---|---|
| P1 | Contract first, then tests units, then at least one code unit | by construction |
| P2 | Every unit has a kind, a scope, an objective and tasks | `unitsOf` |
| P3 | A unit writes only what the specification lets its kind write | `unitsOf` |
| P4 | Every new test file is written by exactly one tests unit | at least one: `unitsOf`; exactly one: **open** |
| P5 | Every criterion is served by a code unit; every unit serves one; a tests unit only what the new tests judge | `unitsOf` |
| P6 | Every name provided has a file in scope, a kind and a signature | `provisionProblem` |
| P7 | No unit provides a name its file already declares | `providedAlready` |
| P8 | No name is provided twice in one package | **open** |
| P9 | A contract exists whenever tests and code are written apart | `withContract`; one code unit: **open** |
| P10 | A unit uses only names the estate defines or an earlier unit provides | `orderOf` |
| P11 | A unit reads only files that exist or another unit writes | `unitsOf`; tests' half: **open** |
| P12 | No unit's scope holds what judges the change | at area 1 only |
| P13 | A unit fits one agent: its brief, the specification's decisions and its files whole, within the budget | `oversized`; a unit of one file too long for it is let be, and is read by parts |

## Carrying a unit

A unit is carried in a copy of the estate holding exactly the candidates of the units it waits for, and no others. What its agent is handed, and what it asks of the estate as it works, comes from that copy.

What comes of each carrying is one outcome, and the outcome decides what happens next:

| Outcome | What follows |
|---|---|
| candidate | kept; the units waiting on it may go |
| holds — the tests pass before any code | the change is closed: nothing to do |
| contract acts — the tests pass only with the contract | contract and tests carried again, once |
| tests wrong — a code unit blames a test file | that tests unit and the blaming unit carried again, once |
| cannot — the unit cannot be done as planned | the plan goes back to its planner, once |
| failed | area 4 stops and says why; the change can be resumed |

Every reason to carry a unit again has a bound, so area 4 always ends.

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
| the tests it must pass, whole | | | | | ✓ |
| what earlier units wrote, by path | | | | ✓ | ✓ |
| around them: signatures and names only | | | ✓ | ✓ | ✓ |

What else it needs, the agent reads with its tools: files by parts, where a name is defined, who uses it.

The budget is spent in this order:

1. its goal;
2. the tests it must pass;
3. the files it changes, whole, unless one alone is longer than the whole budget;
4. what it reads, reduced to an outline as soon as it would crowd out the rest;
5. signatures and names.

The tokens each agent was handed are recorded with its costs.

## Open

1. P1–P13 as one function of the plan and the estate, held by the gate as rule R24; P4, P8, P11 and P12 checked.
2. P9 for one code unit.
3. Outcomes for blaming the contract or an earlier code unit, each with its bound.
4. A test that drives the scheduler through random outcomes and shows that it always ends.
5. Tools' machine-readable reports in place of patterns over their output.
6. A bench task on an estate large enough for the budget to matter. Today's bench hands 2–5k tokens a unit, so it cannot show the budget's effect.
