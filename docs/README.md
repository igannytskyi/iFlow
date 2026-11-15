# Documentation

Everything written about iFlow is here. What it is about, what it is made of, and the shapes of what it produces.

| | |
|---|---|
| [`iflow.md`](iflow.md) | The research: the goal, the objects, the change classes, the thirteen areas, the loop, the validation runs, how the claim is measured. Why everything is shaped as it is |
| [`process.md`](process.md) | The same, drawn: the path, the artefacts, where a person touches it, what to do when something is wrong |
| [`method/glossary.md`](method/glossary.md) | What every word means, in a sentence or two. For a person |
| [`method/conventions.md`](method/conventions.md) | Every code, defined once. Nothing anywhere may use a value this file does not define, and the gate validates against it |
| [`method/criteria.md`](method/criteria.md) | How to arrive at a criterion: seven probes, one mechanical and six questions |
| [`method/objects.md`](method/objects.md) | What each area hands on, as the thing itself and as its record, and the structure of each |
| [`method/rules.md`](method/rules.md) | Every rule, each naming the check that proves it — and, where a rule is not checked, why not |
| [`templates/`](templates/) | The seven artefacts a change travels through, and its append-only record |

The program reads `iflow.md`, the method documents and the templates as they are written here — the areas, objects, contracts and rules the workbench shows, and the codes the gate validates — so there is no second copy of any of them in code.

## Why this shape

Each document has a separate reason to change. `conventions.md` changes when a new value becomes possible. `rules.md` changes when something new becomes checkable, or when a claim of enforcement turns out to be untrue. A template changes when an artefact gains a field. The gate changes when a rule becomes mechanical, and its arbiters — the Go tests beside it — change when one is found to test something other than its criterion.

`iflow check --mutate` removes each rule in turn and runs the arbiters against the result: a rule whose loss nothing notices is not being tested, whatever the arbiters claim. `iflow check --unused` lists every column the gate does not read and every code nothing validates; it does not call them ceremony — most exist for a person — but before it nobody could list them. `iflow check --repeat` runs whatever a piece of evidence claims can be produced again, twice, and compares.

## What is not here

**No stored fixture.** The arbiters build a complete, valid change from the shapes defined here when they need one and throw it away afterwards. A fixture kept on disk goes stale the moment a template gains a field, and nothing notices; one built from the current shapes fails loudly instead.
