# The framework

Everything the method *is* lives here. What it is *about* is in `../docs/`. What it *produces* is not versioned at all: a change carried through the method is the output of using it on one estate, every adopter has their own, and nothing in the method requires a particular place for it.

| | |
|---|---|
| `conventions.md` | Every code, defined once. Nothing anywhere may use a value this file does not define |
| `criteria.md` | How to arrive at a criterion — seven probes, one of them mechanical and six of them questions |
| `rules.md` | Every rule, each naming the check that proves it — and, where a rule is not checked, why not |
| `templates/` | The seven artefacts a change travels through, plus the append-only record |
| `check.py` | The gate. Pure Python, no dependencies. Reads a change folder and says what is wrong |
| `tests/` | The arbiters. Each was written before the thing it judges and shown to fail for its stated reason |
| `tests/harness.py` | Builds a valid change when one is needed and stores none. A fixture kept on disk drifts away from the templates in silence; one built at run time cannot |

```bash
python3 framework/check.py changes/<slug>     # one change
python3 framework/check.py --status [dir]     # every change, at a glance
python3 framework/check.py --arbiters         # run every arbiter
python3 framework/check.py --mutate           # break each rule, see who notices
python3 framework/check.py --repeat <slug>    # run what claims to repeat, twice
python3 framework/check.py --escalations      # what the framework owes itself
```

Running every arbiter is one action so that it can be done where a change lands, and not only in the isolated tree where a fixture may still agree with the world.

`--mutate` removes each rule in turn and runs every arbiter against the result. A rule whose loss nothing notices is not being tested, whatever the arbiters claim. It replaced a record in which a unit attested that its arbiter had once failed for its stated reason — the demonstration is repeated now rather than believed about a run that has ended, and on its first use it found four rules that nothing was testing.

`--repeat` runs whatever a piece of evidence claims can be produced again, twice, and compares. A claim to repeat must name something that can be re-run — one that names nothing cannot be tested and is therefore not a claim. This is the second record the method stopped believing and started re-deriving.

## Why this shape

The method is a document, a vocabulary, a set of artefact shapes, one program that enforces them, and the arbiters that prove that program does what it claims. Those five things are the whole framework, and each is a separate file because each has a separate reason to change.

`conventions.md` changes when a new value becomes possible. `rules.md` changes when something new becomes checkable, or when a claim of enforcement turns out to be untrue. `templates/` change when an artefact gains a field. `check.py` changes when a rule becomes mechanical. `tests/` change when an arbiter is found to test something other than its criterion — which has happened repeatedly, and always turned out to be the arbiter's fault rather than the rule's.

The skill an agent loads lives at `../.claude/skills/iflow/`, because the tool requires that location. It is the only part of the framework not in this folder, and it is a summary of what is here rather than a source of it.

## What is not here

**No stored fixture.** The arbiters build a complete, valid change when they need one and throw it away afterwards. A fixture kept on disk goes stale the moment a template gains a field, and nothing notices; a fixture built from the current shapes fails loudly instead.

**No worked example.** The templates are the shape, the diagrams are the flow, and an example folder would be a third copy of both — kept in step by hand, which is how it stops being true.
