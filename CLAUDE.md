# iFlow — agent rules

This repository holds the iFlow research document and the first working increment: the conventions, artifact formats, rules and checks by which a change travels from an intent to a landing.

Read [docs/iflow.md](docs/iflow.md) for why any of this is shaped the way it is. Read [framework/conventions.md](framework/conventions.md) before writing any artifact — every code is defined there and nowhere else.

## How to behave

**One change per folder.** `changes/<slug>/` holds the six stage files. Never write a stage file before the one before it exists and passes `framework/check.py`.

**Never invent a code.** If a value is not in CONVENTIONS.md, stop and say so. Adding a value is a change to the framework, arbitrated outside iFlow (R9), not a decision to take mid-task.

**Criteria before execution, always.** Do not write `04-execution.md` for a unit whose criteria are not already fixed in `01-specification.md`. If execution reveals that the criteria were wrong, that returns to area 1 as a new specification — it never edits the old one (R1).

**Never soften a criterion to make it pass.** If a criterion cannot be met, the outcome is `failed-task`, and if it cannot be decided, `undecided`. Both are correct answers. A criterion that is edited so a candidate passes is the failure this whole framework exists to prevent (R7, R8).

**Say undecided.** `undecided` is not a hedge and not a failure. It is the honest verdict when evidence could not be obtained, and conflating it with `failed` destroys the only signal that says the arbiter is too weak.

**Record what you could not do.** A held unit, a refused admission, a missing piece of evidence — all of these are outcomes and all are written down. Silence is the one thing that is never acceptable.

## Verbs

The skill in `.claude/skills/iflow/` defines: `intent`, `plan`, `admit`, `run`, `verify`, `land`, `status`, `check`. They map one to one onto areas 1–6 of the research document.

## Before presenting anything

Run `python3 framework/check.py changes/<slug>` and report what it says. Do not present a verdict, a landing, or a status without it.
