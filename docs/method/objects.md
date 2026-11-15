# Objects

What each area hands on is two things, and they are kept apart:

- **the object itself** — the document, the code, the output — which a person reads and an agent builds from;
- **its record** — the method's metadata about it, in the stage files' tables, which the gate checks.

Mixing them made both worse: a description squeezed into a table cell, a table no one could read as a description. Each object below has its own file, its own structure, and is shown in the workbench above its record.

| Area | The object | Kept in | Its record |
|---|---|---|---|
| 1 | The specification — the change described | `change.md` | `01-specification.md` |
| 2 | The plan, and one ticket per unit | `plan.md`, `units/WU-…md` | `02-plan.md` |
| 3 | The prior state of every file the change may touch | `baseline/` | `03-admission.md` |
| 4 | Each unit's candidate, what its agent was handed, and what it did | `candidates/CA-…/files/`, `pack.md`, `trace.jsonl`, `summary.txt` | `04-execution.md` |
| 5 | The output every verdict rests on | `evidence/EV-…txt`, `candidates/CA-…/failing-before.txt` | `05-assurance.md` |
| 6 | What entered the project | the candidates' files, in the estate | `06-landing.md` |
| 8 | Every crossing, and what each agent was handed | `record.md`, `handed/` | — |
| 9 | What each piece of work consumed | `costs.jsonl` | — |
| 10 | What the person decided, for every later author | `changes/decisions.md` | — |

## The specification — `change.md`

Written by the author, read by a reviewer, accepted — or changed by hand, or sent back with notes — by the person. A product requirements document and a design document together. A section with nothing to say is left out.

1. **The statement** — the intent in one sentence, and a sentence summing the change up.
2. **Context** — the problem, why the change is wanted, what exists now.
3. **Goals** and **Non-goals**.
4. **How a person uses it** — every path, step by step, the ordinary one and each alternative.
5. **Requirements** — functional, numbered `FR-1…`, one testable statement each.
6. **Quality** — security, error handling, data integrity, limits, performance, as far as they matter.
7. **Technology** — languages, libraries, protocols, services, and why, as far as the estate does not already decide them.
8. **Design** — data kept and where; interfaces with their methods and signatures; routes or commands with inputs and outputs; which file holds what; what tests stand in for.
9. **Edge cases** — each with what happens.
10. **Dependencies**, **Risks**.
11. **Open questions** — only what the person must decide.
12. **How it is tested** — which tests judge which criteria, at which entry point, with which fakes.
13. **Acceptance criteria** — each `CR-…` in words: when, then.
14. **What comes after** — the rest of a task too large for one change, each part a task of its own.

Accepted, it is fixed (R1): a changed intent is a new one. The criteria it lists are the record's; editing the description by hand does not change them.

## The plan — `plan.md` and `units/`

Written by the planner, read by a reviewer.

`plan.md`: **Approach** — how the change is built as a whole, the steps in order, what could go wrong — then every unit.

`units/WU-…md` — one ticket a unit, all its agent is told of the change:

1. **Objective** — what the unit is for.
2. **Serves** — the requirements and criteria.
3. **Relies on** — what exists, or what an earlier unit provides.
4. **Tasks** — in order.
5. **Provides** — every name another unit uses, with its full signature.
6. **Constraints** — what not to touch.
7. **Edge cases**.
8. **Done when** — the check, and what it shows.
9. **The contract between the units** — every unit's *Provides*, so that the tests call what the code defines and no unit defines what another provides.

## The candidate — `candidates/CA-…/`

- `files/` — the unit's files, whole, as it left them; shown as a difference from `baseline/`.
- `pack.md` — what its agent was handed: its ticket and the least of the estate the work needs.
- `trace.jsonl` — every step, with what each model turn cost.
- `summary.txt` — what the agent said it did, in a sentence.
- `failing-before.txt` — for tests written first: their run against the estate as it stood, failing.

## What was handed — `handed/`

Every author, reviewer and planner call keeps its instructions and its prompt, word for word, numbered in order; an executor's is its candidate's `pack.md`. The cost record of each call names the file (`kept`) and its size (`handed`), and the gate holds them to it (R23).
