# The framework

Everything the method *is* lives here. What it *produced* is in `../changes/`. What it is *about* is in `../docs/`. Those three are kept apart on purpose: the method should be readable without reading its output, and its output should be readable without reading the research behind it.

| | |
|---|---|
| `conventions.md` | Every code, defined once. Nothing anywhere may use a value this file does not define |
| `criteria.md` | How to arrive at a criterion — seven probes, one of them mechanical and six of them questions |
| `rules.md` | Every rule, each naming the check that proves it — and, where a rule is not checked, why not |
| `templates/` | The seven artefacts a change travels through, plus the append-only record |
| `check.py` | The gate. Pure Python, no dependencies. Reads a change folder and says what is wrong |
| `tests/` | The arbiters. Each was written before the thing it judges and shown to fail for its stated reason |

```bash
python3 framework/check.py changes/<slug>     # one change
python3 framework/check.py --escalations      # what the framework owes itself
```

## Why this shape

The method is a document, a vocabulary, a set of artefact shapes, one program that enforces them, and the arbiters that prove that program does what it claims. Those five things are the whole framework, and each is a separate file because each has a separate reason to change.

`conventions.md` changes when a new value becomes possible. `rules.md` changes when something new becomes checkable, or when a claim of enforcement turns out to be untrue. `templates/` change when an artefact gains a field. `check.py` changes when a rule becomes mechanical. `tests/` change when an arbiter is found to test something other than its criterion — which has happened repeatedly, and always turned out to be the arbiter's fault rather than the rule's.

The skill an agent loads lives at `../.claude/skills/iflow/`, because the tool requires that location. It is the only part of the framework not in this folder, and it is a summary of what is here rather than a source of it.
