# Objects

What each area hands on is two things, and they are kept apart:

- **the object itself** — the document, the code, the output — which a person reads and an agent builds from;
- **its record** — the method's metadata about it, in the stage documents of the [data model](data-model.md), which the gate checks.

Mixing them made both worse: a description squeezed into a table cell, a table no one could read as a description. Each object below has its own file, its own structure, and is shown in the workbench above its record.

| Area | The object | Kept in | Its record |
|---|---|---|---|
| 1 | The specification — the change described | `change.md` | `01-specification.json` |
| 2 | The plan, and one ticket per unit | `plan.md`, `units/WU-…md` | `02-plan.json` |
| 3 | The prior state of every file the change may touch | `baseline/` | `03-admission.json` |
| 4 | Each unit's candidate, what its agent was handed, and what it did | `candidates/CA-…/files/`, `pack.md`, `trace.jsonl`, `summary.txt` | `04-execution.json` |
| 5 | The output every verdict rests on | `evidence/EV-…txt`, `candidates/CA-…/failing-before.txt` | `05-assurance.json` |
| 6 | What entered the project | the candidates' files, in the estate | `06-landing.json` |
| 8 | Every crossing, and what each agent was handed | `record.jsonl`, `handed/` | — |
| 9 | What each piece of work consumed | `costs.jsonl` | — |
| 10 | What the person decided, for every later author | `changes/decisions.md` | — |

## The specification — `change.md`

Written by the author, read by a reviewer, accepted — or changed by hand, or sent back with notes — by the person. It is the whole specification of the change, as the best engineering organisations write one: a product requirements document, a design document and a request for comments together. It begins with the intent and holds everything about what is wanted and how; the record beside it holds how it is judged. A section with nothing to say is left out.

1. **Intent** — the task in one sentence, as accepted.
2. **Summary** — what changes, for whom and why, readable alone.
3. **Context and problem** — the motivation, and what exists now.
4. **Goals** and **Non-goals** — a non-goal is what could reasonably have been a goal and deliberately is not.
5. **Users** and **User stories** — "As a …, I want …, so that …".
6. **How a person uses it** — every path, step by step.
7. **Functional requirements** — numbered `FR-1…`, one testable statement each.
8. **Non-functional requirements** — security, privacy, errors, data integrity, limits, performance, accessibility.
9. **Design** — **Architecture** (the parts and how they talk), **Data model** (what is kept, its fields, where, how long), **Interfaces** (every route, command or function another part calls: inputs, outputs, errors, signatures), **Components and files** (which file holds what), **Technology** (and why).
10. **Edge cases and errors** — each with what happens.
11. **Alternatives considered** — and why not.
12. **Dependencies** and **Assumptions**.
13. **Risks** — and how each is contained.
14. **Observability** — what is logged or measured to see it work in use.
15. **Rollout and rollback** — how it ships, migrates, and is undone.
16. **Test strategy** — which tests judge which criteria, at which entry point, with which fakes.
17. **Success metrics** — how, in use, it is known to have achieved its goals.
18. **Open questions** — only what the person must decide.
19. **Acceptance criteria** — each `CR-…`: when, then.
20. **Termination** — when work stops without acceptance, and what then.
21. **What comes after** — the rest of a task too large for one change.

Accepted, it is fixed (R1): a changed intent is a new one. Editing the description by hand does not change the criteria, which are the record's. A task carried before specifications were written whole can have one written after the fact, from what was asked, its record and the code; it says so.

Sources for the structure: [Design docs at Google](https://www.industrialempathy.com/posts/design-docs-at-google/), [Companies using RFCs or design docs](https://blog.pragmaticengineer.com/rfcs-and-design-docs/), common product requirements templates.

## The plan — `plan.md` and `units/`

Written by the planner, read by a reviewer.

`plan.md`: **Approach** — how the change is built as a whole, the steps in order, what could go wrong — then every unit.

The units come in this order:

1. **The contract**, where the tests and the code are written apart and the code provides names — added by the framework, not the planner: one unit that declares every such name, without behaviour, in the file its provision names. Everything after it builds against an interface that compiles, so no unit guesses what another will declare. It is judged by what the estate's own suite judges: declared, nothing else moves.
2. **The tests**, written against the contract, before any behaviour exists, and failing until it does.
3. **The code units**. Those that change no file in common are carried at once, a chain each — units that share a file are one chain, in order — and taken in the plan's order as if carried one by one. The last unit completes the change: it runs the change's own tests, then the whole suite, and may mend any code the change wrote.

`units/WU-…md` — one ticket a unit, what its agent is told to do; beside it the agent is handed the specification's own decisions (below, under the candidate):

1. **Objective** — what the unit is for.
2. **Serves** — the requirements and criteria.
3. **Relies on** — what exists, or what an earlier unit provides; the names it uses of other units' are kept apart, as data (`uses`), and the plan is sent back where one is provided only after it, or by no unit and nowhere in the estate.
4. **Tasks** — in order.
5. **Provides** — every name another unit uses, with its full signature.
6. **Constraints** — what not to touch.
7. **Edge cases**.
8. **Done when** — the check, and what it shows.
9. **The contract between the units** — every unit's *Provides*: each a named entry — file, name, kind, signature — so that the tests call what the code defines, no unit defines what another provides, and a unit is not done until every name it provides is in its files.

## The candidate — `candidates/CA-…/`

- `files/` — the unit's files, whole, as it left them; shown as a difference from `baseline/`.
- `pack.md` — what its agent was handed: its ticket; the specification's decisions, word for word — its requirements, data model, interfaces, components and edge cases, and for a tests unit its test strategy — so that the units, written apart, build from one source and none decides again what the specification decided; and the least of the estate the work needs.
- `trace.jsonl` — every step, with what each model turn cost.
- `summary.txt` — what the agent said it did, in a sentence.
- `failing-before.txt` — for tests written first: their run against the estate as it stood, failing.

## What was handed — `handed/`

Every author, reviewer and planner call keeps its instructions and its prompt, word for word, numbered in order; an executor's is its candidate's `pack.md`. The cost record of each call names the file (`kept`) and its size (`handed`), and the gate holds them to it (R23).
