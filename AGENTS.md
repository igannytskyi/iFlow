# Working in this repository

Instructions for any agent — Claude Code, Codex, Cursor, Aider, Copilot, Gemini CLI, Windsurf or another. This file is the single source; `CLAUDE.md` and the skill under `.claude/skills/iflow/` point here rather than repeating it, because two copies of a rule drift apart and that is the failure this whole method exists to prevent.

## What this repository is

A method for accepting change: a person states what is to be achieved and what would make the result acceptable, autonomous executors do the work, and a change is accepted on evidence rather than on anyone having read it.

| | |
|---|---|
| `docs/iflow.md` | Why any of this is shaped the way it is |
| `docs/process.md` | The same, drawn — the path, the artefacts, where a person touches it |
| `framework/` | The method itself: the vocabulary, the rules, the artefact shapes, the gate, the arbiters |
| `changes/` | What the method has produced. `clock-instant/` is a worked example; the rest are real |

Read [`framework/glossary.md`](framework/glossary.md) to learn what the words mean, and `framework/conventions.md` before writing any artefact. Every code is defined there and nowhere else.

## How a change is carried

One change lives in `changes/<slug>/` as seven files written in order, plus an append-only `record.md`. Copy the matching file from `framework/templates/` and fill it. Never invent a column.

| Step | Writes | What it does |
|---|---|---|
| intent | `00-intent.md` | What is to be achieved, and who may decide it against a competing intent |
| plan | `01-specification.md`, `02-plan.md` | Fixes criteria, termination, scope and arbiter; derives work units with their classes and areas of effect |
| admit | `03-admission.md` | Settles conflict, allowance and permission together, records the specification digest, fixes any evidence plan, issues grants |
| run | `04-execution.md` | Records what executed, its version, its terminal state and the candidate it produced |
| verify | `05-assurance.md` | Gathers evidence and renders a verdict per criterion |
| land | `06-landing.md` | Enters accepted candidates while their evidence still holds |

Every step appends its crossing to `record.md`. A step present without its line fails the check.

```bash
python3 framework/check.py changes/<slug>     # one change
python3 framework/check.py --status [dir]     # every change, at a glance
python3 framework/check.py --arbiters         # run every arbiter
python3 framework/check.py --mutate           # break each rule, see who notices
python3 framework/check.py --repeat <slug>    # run what claims to repeat, twice
python3 framework/check.py --unused           # fields nobody reads, codes nobody checks
python3 framework/check.py --escalations      # what the method still owes itself
```

## What the estate is

Before a change is planned, what the system is now is derived from it rather than described about it: what a change here reaches, how well behaviour there is pinned down, what crosses between repositories, and how far each of those answers is to be trusted. Nothing here is maintained — the index is a cache with an invalidation rule, never a corpus with a publication date.

```bash
python3 framework/estate.py affects <path>...     # what a change here reaches
python3 framework/estate.py observability         # every region, worst first
python3 framework/estate.py contracts <dir>       # what crosses between repositories
python3 framework/estate.py reachability <dir> <repo>   # who consumes this, who cannot be reached
python3 framework/estate.py freshness             # what the answers were derived from
python3 framework/estate.py refresh               # derive again only what moved
python3 framework/estate.py readers               # which languages are read here
python3 framework/estate.py coverage              # how much of this the index reaches
```

Python is read by the parser that ships with Python, so an estate written in it needs nothing installed. Every other language is read through its own tree-sitter grammar, installed beside the readers and never committed:

```bash
python3 -m pip install --target framework/readers/_lib -r framework/readers/requirements.txt
```

What is unread is derived from what is installed, not declared: install a grammar and those files move out of the unseen count; remove it and they move back. A language nothing reads is reported unseen, never absent.

Run it from the repository or the estate it is asked about, not from here. Derived material goes beside that repository in `.estate/`; to keep an estate's material together instead, name one directory:

```bash
export IFLOW_ESTATE=changes/<slug>/estate        # cache/ and answers/ go here
export IFLOW_ESTATE_ROOT=changes/<slug>/estate/repos
```

| | |
|---|---|
| `<home>/repos/` | The repositories the change is decided against |
| `<home>/cache/` | One index per repository, keyed by the commit each region was last touched by |
| `<home>/answers/` | What the tooling said, and from which commit — read afterwards to see whether the model was right |

The cache is disposable; the answers are not.

## What binds you

**Never invent a code.** If a value is not in `framework/conventions.md`, stop and say so. Adding one is a change to the method, decided outside it, not a decision to take mid-task.

**Criteria are fixed before execution and never after.** Correcting them is free while no candidate exists — that is a re-admission, and it is recorded. Once a candidate exists they are frozen, and a correction is a new specification under the same intent.

**A criterion is written in the vocabulary of the intent, never of the implementation.** *When these conditions hold, then this observable fact holds* — not *this line was added*. One that names a code artefact can be satisfied by editing that artefact. `framework/criteria.md` has seven probes for telling a good criterion from a bad one before any work exists; use them when writing criteria and when reviewing them.

**Never weaken a criterion, a test or a schema to make something pass.** Whatever judges a change lies outside what the change may touch. If the judging apparatus genuinely must move, that is its own unit, judged on its own, never by the unit that benefits from it.

**Capture the prior state before execution.** Where a criterion could only ever be evidenced by the artefact being changed, capture that artefact's prior state into `baseline/` first. Afterwards there is nothing left to compare against and any evidence you produce is circular.

**Write candidates into `candidates/<id>/`, never into the live tree.** Landing is the only step that applies anything. Execution that edits in place leaves nothing to enforce the separation.

**Name which ground of independence evidence rests on.** Either something other than the executor produced it, or it was fixed and its own acceptance recorded before the candidate existed. Evidence produced from inside the scope of the change is not independent whatever else is claimed.

**Say whether a run can be repeated.** A deterministic transformation on fixed inputs can be re-run to the same result and its record is derived; an agent's run cannot, and its record is testimony. Nothing is accepted on an unrepeatable run alone.

**Prefer a deterministic executor.** Where a rewrite rule, a codemod or a type-checked rename will do, using an agent instead is a defect: it makes a reproducible result unreproducible and costs more. Record which was used, and its version.

**`undecided` is a correct answer.** Use it when evidence could not be obtained. Conflating it with `failed` destroys the only signal that says the judging apparatus is too weak. Nothing lands on an undecided criterion.

**Record what you could not do.** A held unit, a refusal, a missing piece of evidence, work awaiting someone else's authority — all are outcomes and all are written down. Silence is the one unacceptable result.

**A person may refuse at any boundary.** A refusal returns the object to its producer as *that producer's* failure, not as a request to try again, and nothing refused is accepted.

## Before presenting anything

Run the check and report what it says, including when it passes. A verdict, a landing or a status presented without it is not finished work.
