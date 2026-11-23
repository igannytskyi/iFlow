# Working in this repository

Instructions for any agent — Claude Code, Codex, Cursor, Aider, Copilot, Gemini CLI, Windsurf or another. This file is the single source; `CLAUDE.md` and the skill under `.claude/skills/iflow/` point here rather than repeating it, because two copies of a rule drift apart and that is the failure this whole method exists to prevent.

## What this repository is

A method for accepting change: a person states what is to be achieved and what would make the result acceptable, autonomous executors do the work, and a change is accepted on evidence rather than on anyone having read it.

| | |
|---|---|
| `docs/` | Everything written: the research, the process drawn, the vocabulary, the codes, the rules, the artefact shapes |
| `cmd/iflow`, `internal/` | The program: the gate and its rules, the estate, the harness, the flow through the areas, the workbench |
| `examples/greeting` | A small estate with changes carried through the method |

Read [`docs/method/glossary.md`](docs/method/glossary.md) to learn what the words mean, and [`docs/method/conventions.md`](docs/method/conventions.md) before writing any artefact. Every code is defined there and nowhere else.

## How a change is carried

One change lives in `changes/<slug>/` as seven documents written in order, plus an append-only record. Each document is JSON in the shape of its type in [`internal/meta`](internal/meta/meta.go), described in [`docs/method/data-model.md`](docs/method/data-model.md) and checked when it is written; its Markdown beside it is rendered for people and never read back. Write documents with `artifact.Save` and the record with `artifact.AppendRecord`, never by hand; the templates in `docs/templates/` show the rendered shape. Never invent a field: add it to the type, its description and the schema.

| Step | Writes | What it does |
|---|---|---|
| intent | `00-intent.json` | What is to be achieved, and who may decide it against a competing intent |
| plan | `01-specification.json`, `02-plan.json` | Fixes criteria, termination, scope and arbiter; derives work units with their classes and areas of effect |
| admit | `03-admission.json` | Settles conflict, allowance and permission together, records the specification digest, fixes any evidence plan, issues grants |
| run | `04-execution.json` | Records what executed, its version, its terminal state and the candidate it produced |
| verify | `05-assurance.json` | Gathers evidence and renders a verdict per criterion |
| land | `06-landing.json` | Enters accepted candidates while their evidence still holds |

Every step appends its crossing to `record.jsonl`. A step present without its line fails the check. A change written before the data model is carried into it with `iflow migrate`.

A task can also be carried by the program itself: `iflow run <estate> "<task>"`, or typed into `iflow desk`. A model states the intent and criteria, a person accepts them, and the plan, the admission, the execution by an agent in a copy of the estate, the evidence and the landing follow on their own, each written as the artefact above and checked by the gate at its boundary. What each piece of work cost is kept beside the change in `costs.jsonl`, and what each agent did, step by step, beside its candidate.

Nothing enters on a run nobody made. Before a landing stands, the arbiters must have been run against this framework and every claim that a run repeats must have been repeated against exactly that command — `iflow check --arbiters` and `iflow check --repeat <slug>` do both and leave a note beside the framework, derived and disposable, which any edit to what was run voids. In a repository whose changes are to the framework itself, that means running them again after each one.

```bash
iflow check <changes/slug>              # one change
iflow check --status [dir]              # every change, at a glance
iflow check --arbiters                  # run every arbiter
iflow check --mutate                    # break each rule, see who notices
iflow check --repeat <slug>             # run what claims to repeat, twice
iflow check --unused                    # fields nobody reads, codes nobody checks
iflow check --escalations               # what the method still owes itself
iflow measure <estate>...               # what the model is worth, measured: reach and quotation
iflow baseline <estate>                 # the present way of working, reconstructed from its history

iflow plan <changes/slug>               # what the estate says about this specification
iflow plan <changes/slug> --draft       # the derivable half of its plan
```

Before a plan is written, ask the estate what it already answers about the specification: what the scope reaches, what could judge a change there, which consumers no change here reaches, and what unfinished work holds the same ground. A region nothing judges makes a criterion about behaviour there undecidable as it stands, and calls for a preparatory unit before the work itself. How the work divides, what each unit is called and which class it falls in are judgement, are left blank, and are said to be.

## What the estate is

Before a change is planned, what the system is now is derived from it rather than described about it: what a change here reaches, how well behaviour there is pinned down, what crosses between repositories, and how far each of those answers is to be trusted. Nothing here is maintained — the index is a cache with an invalidation rule, never a corpus with a publication date.

```bash
iflow estate affects <path>...          # what a change here reaches
iflow estate observability              # every region, worst first
iflow estate contracts <dir>            # what crosses between repositories
iflow estate reachability <dir> <repo>  # who consumes this, who cannot be reached
iflow estate freshness                  # what the answers were derived from
iflow estate refresh                    # derive again only what moved
iflow estate corrections                # what landings taught that reading cannot find
iflow estate readers                    # which languages are read here
iflow estate coverage                   # what share of this the index reaches — an instrument for comparing two indexes, not a measure of the model: this representation is never complete, and what it is measured by is freshness
iflow estate context <symbol|path>      # the code to change, and what it touches
```

Every language is read through its own tree-sitter grammar, built into the program: Python, Go, TypeScript, JavaScript, Java, C#, Rust, Ruby, PHP, C, C++, Kotlin, Scala and shell. A language nothing reads is reported unseen, never absent — `iflow estate readers` says which are read.

`context` is the one shaped for doing the work rather than deciding about it: it answers with the code itself — the definition quoted with its line numbers, the call sites with the lines around them, what the subject calls that lives here, and what would judge a change to it — firmest first, cut to a budget it names. The others answer with paths, confidences and areas, which is what a decision needs and not what a change needs.

Run it from the repository or the estate it is asked about, not from here. Derived material goes beside that repository in `.estate/`; to keep an estate's material together instead, name one directory:

```bash
export IFLOW_ESTATE=changes/<slug>/estate        # cache/ goes here
export IFLOW_ESTATE_ROOT=changes/<slug>/estate/repos
```

| | |
|---|---|
| `<home>/repos/` | The repositories the change is decided against |
| `<home>/cache/` | One index per repository, keyed by the contents of each file and the commit that last touched it, and the corrections landings taught |

The index is disposable; the corrections are not. Being wrong is not detectable from the source: where work had to touch ground nothing connects to its scope, the edge is real and no re-reading finds it. The gate refuses such a landing until it is recorded, and from then on it is answered with as observed rather than derived.

## What binds you

**Never invent a code.** If a value is not in `docs/method/conventions.md`, stop and say so. Adding one is a change to the method, decided outside it, not a decision to take mid-task.

**Criteria are fixed before execution and never after.** Correcting them is free while no candidate exists — that is a re-admission, and it is recorded. Once a candidate exists they are frozen, and a correction is a new specification under the same intent.

**A criterion is written in the vocabulary of the intent, never of the implementation.** *When these conditions hold, then this observable fact holds* — not *this line was added*. One that names a code artefact can be satisfied by editing that artefact. `docs/method/criteria.md` has seven probes for telling a good criterion from a bad one before any work exists; use them when writing criteria and when reviewing them.

**Never weaken a criterion, a test or a schema to make something pass.** Whatever judges a change lies outside what the change may touch. If the judging apparatus genuinely must move, that is its own unit, judged on its own, never by the unit that benefits from it.

**Capture the prior state before execution.** Where a criterion could only ever be evidenced by the artefact being changed, capture that artefact's prior state into `baseline/` first. Afterwards there is nothing left to compare against and any evidence you produce is circular.

**Write candidates into `candidates/<id>/`, never into the live tree.** Landing is the only step that applies anything. Execution that edits in place leaves nothing to enforce the separation.

**Name which ground of independence evidence rests on.** Either something other than the executor produced it, or it was fixed and its own acceptance recorded before the candidate existed. Evidence produced from inside the scope of the change is not independent whatever else is claimed.

**Say whether a run can be repeated.** A deterministic transformation on fixed inputs can be re-run to the same result and its record is derived; an agent's run cannot, and its record is testimony. Nothing is accepted on an unrepeatable run alone.

**Prefer a deterministic executor.** Where a rewrite rule, a codemod or a type-checked rename will do, using an agent instead is a defect: it makes a reproducible result unreproducible and costs more. Record which was used, and its version.

**`undecided` is a correct answer.** Use it when evidence could not be obtained. Conflating it with `failed` destroys the only signal that says the judging apparatus is too weak. Nothing lands on an undecided criterion.

**Record what you could not do.** A held unit, a refusal, a missing piece of evidence, work awaiting someone else's authority — all are outcomes and all are written down. Silence is the one unacceptable result.

**A person may refuse at any boundary.** A refusal returns the object to its producer as *that producer's* failure, not as a request to try again, and nothing refused is accepted.

## The standard the framework's own code is held to

iFlow is a framework, built to the best practices of Go — its idioms, its patterns, sound algorithms — and every change to it is measured against that, not against whether it works today.

**Everything that crosses between areas, units and agents is typed data in the model.** A role, a contract, a dependency, an outcome is a field with a type, a schema and a validation — never a convention in prose, a prefix in a title, a marker in a prompt, or a regular expression run over what an agent wrote. Where something is inferred from text today, that is a defect to remove, not a pattern to repeat.

**No workarounds, no special cases, no guessing.** A failure is fixed where it arises, by the model being made able to say what was meant; a fix that recognises one symptom — one wording, one language, one task — is not a fix. If the right fix is larger than the task, say so and record it in `docs/backlog.md` rather than land the patch.

**One place for each decision.** The order units run in, what a language needs, how a run is judged: each is defined once, as data or one function, and read everywhere else.

**Measured, not believed.** A change to how tasks are carried is run on the reference tasks (`iflow bench`) before and after; one that lowers what lands, or raises what it costs without cause, is not kept.

**Checked free before paid.** A paid run of the bench is a measure of the models, never the first test of a change to the framework. Before one: `make test`, the real runs kept under `internal/flow/testdata/realruns` among it, and a replay of every recorded run (`iflow bench --replay`). A change to how an agent is bounded — its steps, its reading, its budget — is set from the traces real runs left, not from one run's look; what a real run reveals becomes a kept real run or a recording, so that it is caught free the next time.

**Strict is what the person decides and the evidence it holds.** The specification says what the change does and how it is judged, never how; its criteria and scope are strict, and so are the test ledger, the runner's report and the person's affirmation. The plan is made on the code, by a planner that reads it, and the agents follow it; where it is wrong it goes back to the step that decided it — the planner, or the specification's author where the scope is too narrow — never mended where it failed.

## Before presenting anything

Run the check and report what it says, including when it passes. A verdict, a landing or a status presented without it is not finished work.
