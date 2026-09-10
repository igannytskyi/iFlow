# iFlow

A framework for software development and maintenance carried out at scale by autonomous agents.

**Goal.** To make the development and maintenance of software an activity carried out at any scale by autonomous agents: a person contributes the intent and the criteria of an acceptable result, is free not to take part in how it is achieved, and is able to verify it at will; and the volume of such work is limited only by the resources one is willing to spend on it.

**Status.** Research. Nothing is implemented yet.

## The document

The research is one document: [`docs/iflow.md`](docs/iflow.md) — goal, research framework, the descriptive schema, the objects, the change classes, the thirteen areas, the feedback loop, five validation runs, how the claim is measured, and what to build first.

## Drawn

[`docs/process.md`](docs/process.md) — the path and its artefacts, where a person actually touches it, the defect flow, what to do when something is wrong, and the mapping to epic, issue, DoR, DoD and testing.

## Working in it

[`AGENTS.md`](AGENTS.md) — how any agent works in this repository: the path a change travels, and what binds whoever carries it. Tool-neutral and the single source; `CLAUDE.md` and the skill point at it rather than repeating it.

## The framework

Everything the method is lives in one folder: [`framework/`](framework/) — the vocabulary, the rules, the artefact shapes, the gate that enforces them and the arbiters that prove the gate does what it claims. Its own [README](framework/README.md) says why it is shaped that way.

A change carried through the method lives in a folder of its own — seven artefacts and an append-only record. That folder is the output of using the framework and is not versioned here; every adopter has their own, and nothing in the method requires a particular name for it. What an artefact looks like is in [`framework/templates/`](framework/templates/).

```bash
python3 framework/check.py changes/<slug>     # one change
python3 framework/check.py --status [dir]     # every change, at a glance
python3 framework/check.py --arbiters         # run every arbiter
python3 framework/check.py --mutate           # break each rule, see who notices
python3 framework/check.py --repeat <slug>    # run what claims to repeat, twice
python3 framework/check.py --unused           # fields nobody reads, codes nobody checks
python3 framework/check.py --escalations      # what the framework owes itself
```

## Versioning

One release per substantive revision of the document.

## Licence

Documentation is under [CC BY-SA 4.0](LICENSES/CC-BY-SA-4.0.txt) — read, cite, translate and adapt it, including commercially, with attribution and under the same terms. Source code, present and future, is under [BUSL-1.1](LICENSES/BUSL-1.1.txt): production use is permitted except as a hosted service or in competition with the Licensor, and each version converts to AGPL-3.0-or-later on its Change Date. See [LICENSE](LICENSE).

Copyright (c) 2026 Illia Gannytskyi. The Licensor retains the right to license this work on other terms.
