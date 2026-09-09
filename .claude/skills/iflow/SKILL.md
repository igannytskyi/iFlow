---
name: iflow
description: Carry a change through iFlow — intent, specification, plan, admission, execution, assurance, landing. Use when the user says /iflow, asks to start or continue a change under iFlow, wants a specification with acceptance criteria written before execution, wants to admit or hold work, wants a verdict on a candidate, or asks for the status of a change. Also use when they ask to check a change folder against the rules.
---

# iFlow

One change lives in `changes/<slug>/` as seven files, written in order. Read `framework/CONVENTIONS.md` before writing any of them; every code is defined there and a value not listed there is an error, not a variation. Read `framework/RULES.md` for what will be checked and what deliberately will not.

Copy the matching file from `templates/` and fill it. Never invent a column.

## Verbs

| Verb | Writes | What it does |
|---|---|---|
| `intent` | `00-intent.md` | Records what is to be achieved and who may decide it against a competing intent. One or two sentences, in the vocabulary of the domain |
| `plan` | `01-specification.md`, `02-plan.md` | Fixes criteria, termination, scope and arbiter; then derives work units with their classes and areas of effect |
| `admit` | `03-admission.md` | Settles conflict, allowance and permission together, records the specification digest, issues grants |
| `run` | `04-execution.md` | Records what executed, its version, its terminal state and the candidate it produced |
| `verify` | `05-assurance.md` | Gathers evidence and renders a verdict per criterion |
| `land` | `06-landing.md` | Enters accepted candidates while their evidence still holds |
| `status` | — | Reads the folder and reports where the change stands and what blocks it |
| `check` | — | Runs `python3 tools/check.py changes/<slug>` and reports it verbatim |

Every verb appends its crossing to `record.md`. A stage present without its line fails the check.

## Rules that bind you while doing this

**Criteria are fixed before execution and never after.** If execution shows the criteria were wrong, that is a new specification, not an edit to the old one — the digest recorded at admission will catch an edit anyway.

**Never weaken a criterion, a test or a schema to make something pass.** The arbiter is outside the scope by construction. If a change genuinely needs the arbiter to move, that is its own work unit, judged on its own, never by the unit that depends on it.

**Capture the prior state at admission.** Where a criterion could only ever be evidenced by the artefact being changed, capture that artefact's prior state into `baseline/` before execution starts. Afterwards there is nothing left to compare against, and any evidence you produce will be circular.

**Write candidates into `candidates/<id>/`, never into the live tree.** Landing is the only stage that applies anything. An execution that edits in place leaves invariant 2 with nothing to enforce it.

**Nothing lands on an undecided criterion.** Undecided means acceptance was not established, and entry presumes it was. A deferred verdict is different — it is a decision awaiting its window, and it lands while the change stays reversible.

**Independence has two grounds, and you must name which.** `independent-by-executor` — produced by something other than what produced the candidate. `independent-by-precommitment` — fixed, and its own acceptance recorded, before the candidate existed. Evidence produced from inside the scope is `not-independent` whatever else is claimed.

**`undecided` is a correct answer.** Use it whenever evidence could not be obtained. Conflating it with `failed` destroys the only signal that says the arbiter is too weak to decide.

**A held unit is written down.** So is a refusal, a missing piece of evidence, and a unit awaiting another authority. Silence is the only unacceptable outcome.

**Prefer a deterministic executor.** Where the class admits a transformation — a rewrite rule, a codemod, a type-checked rename — using an agent instead is a defect: it makes a reproducible result unreproducible and costs more. Record which was used and its version.

**Class demotion is not negotiable.** An equivalence claim that rests on documentation or on a person is `attested`, and the class is `C1T`, never `C1P`.

## Before you present anything

Run the check and report what it says, including when it passes. A verdict, a landing or a status presented without it is not finished work.
