# iFlow

A framework for software development and maintenance carried out at scale by autonomous agents.

**Goal.** To make the development and maintenance of software an activity carried out at any scale by autonomous agents: a person contributes the intent and the criteria of an acceptable result, is free not to take part in how it is achieved, and is able to verify it at will; and the volume of such work is limited only by the resources one is willing to spend on it.

**Status.** Research. Nothing is implemented yet.

## The document

The research is one document: [`docs/iflow.md`](docs/iflow.md) — goal, research framework, the descriptive schema, the objects, the change classes, the thirteen areas, the feedback loop, five validation runs, how the claim is measured, and what to build first.

## Drawn

[`docs/process.md`](docs/process.md) — the path and its artefacts, where a person actually touches it, the defect flow, what to do when something is wrong, and the mapping to epic, issue, DoR, DoD and testing.

## The first increment

The working part: conventions, artifact formats, rules and a mechanical gate, for one change travelling the path from intent to landing.

| | |
|---|---|
| [`framework/CONVENTIONS.md`](framework/CONVENTIONS.md) | Every code, defined once. Tools validate against exactly these values |
| [`framework/RULES.md`](framework/RULES.md) | The nine invariants, each naming the check that proves it — and what is deliberately not enforced |
| [`templates/`](templates/) | The seven artifacts of the path, plus the append-only record |
| [`tools/check.py`](tools/check.py) | The gate. Pure Python, no dependencies |
| [`changes/clock-instant/`](changes/clock-instant/) | A complete worked example — read this first |
| [`.claude/skills/iflow/`](.claude/skills/iflow/) | The verbs: intent, plan, admit, run, verify, land, status, check |

```bash
python3 tools/check.py changes/clock-instant
```

## Versioning

One release per substantive revision of the document.

## Licence

Documentation is under [CC BY-SA 4.0](LICENSES/CC-BY-SA-4.0.txt) — read, cite, translate and adapt it, including commercially, with attribution and under the same terms. Source code, present and future, is under [BUSL-1.1](LICENSES/BUSL-1.1.txt): production use is permitted except as a hosted service or in competition with the Licensor, and each version converts to AGPL-3.0-or-later on its Change Date. See [LICENSE](LICENSE).

Copyright (c) 2026 Illia Gannytskyi. The Licensor retains the right to license this work on other terms.
