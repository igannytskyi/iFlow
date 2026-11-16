# iFlow

A framework for software development and maintenance carried out at scale by autonomous agents.

**Goal.** To make the development and maintenance of software an activity carried out at any scale by autonomous agents: a person contributes the intent and the criteria of an acceptable result, is free not to take part in how it is achieved, and is able to verify it at will; and the volume of such work is limited only by the resources one is willing to spend on it.

**Status.** Working, early. One Go program carries a task from a sentence to a landed change: agents state the criteria, a person accepts them, and every area after that runs by itself, judged at each boundary by a mechanical gate.

## Install

Go 1.27 or later, and a C compiler for the language grammars and the window.

```bash
go install ./cmd/iflow
```

Each piece of work is one agent, asked once: the author of the specification, a reviewer of it, the planner, a reviewer of the plan, and one executor per unit. Which model each role uses is kept in `models.json` in the user's configuration directory (on a Mac, `~/Library/Application Support/iflow/models.json`), written on the first start, as `provider:model`:

```json
"roles": {"author": "openrouter:z-ai/glm-5.3-flash", "reviewer": "openrouter:z-ai/glm-5.3-flash",
          "planner": "openrouter:z-ai/glm-5.3-flash", "executor": "openrouter:z-ai/glm-5.3-flash"}
```

Providers are any server speaking the OpenAI chat protocol (OpenRouter and DeepSeek are described; a local Ollama is one more entry), or `claude-code:<model>`. A key is never kept in the file: it is read from the variable the provider names (`OPENROUTER_API_KEY`) or, on a Mac, from the keychain, where its owner puts it once:

```bash
security add-generic-password -s iflow.openrouter -a iflow -w
```

`iflow models` says which model does what and whether its key is found. The environment overrides:

| Variable | What it does |
|---|---|
| `IFLOW_MODEL` | every role on this model, as `provider:model` |
| `IFLOW_EXECUTOR=agent` | units handed to Claude Code's own agent instead of one-shot agents |
| `IFLOW_BUDGET_USD` | the most one unit of work may spend; 2 when unset |
| `IFLOW_PACK_TOKENS` | the most of the estate one agent is handed; 16000 when unset |

## Use

```bash
iflow desk <project>...                 # the workbench: type a task, watch it carried area by area
iflow run [--accept] <project> <task>   # the same without a window
iflow models                            # which model does each piece of work
```

The workbench shows each area as the task passes through it, every object it hands on, what the gate said at each boundary, what the agent did step by step, and what each step cost — model, tokens and money.

```bash
iflow check <changes/slug>              # one change, against every rule
iflow check --status [dir]              # every change, at a glance
iflow check --repeat <slug>             # run what claims to repeat, twice
iflow check --escalations               # what the method still owes itself
iflow check --arbiters                  # run every arbiter of the framework
iflow check --mutate                    # break each rule, see who notices
iflow check --unused                    # fields nobody reads, codes nobody checks
iflow plan <changes/slug> [--draft]     # what the estate says about a specification
iflow estate <question>                 # what a change reaches, what judges it, the code to change
iflow baseline <project>... [--method]  # the present way of working, and the method beside it
iflow measure <project>...              # what the estate model is worth
iflow adopt openspec|speckit <folder>   # another method's change, read as intent and criteria
```

## The repository

| | |
|---|---|
| [`docs/`](docs/) | All of the documentation: the research, the process drawn, the method's vocabulary, codes and rules, and the shapes of the artefacts. Start at [`docs/README.md`](docs/README.md) |
| [`AGENTS.md`](AGENTS.md) | How any agent works in this repository. Tool-neutral and the single source; `CLAUDE.md` and the skill point at it |
| `cmd/iflow` | The program |
| `internal/` | The framework: the gate and its rules, the estate and the language readers, the harness, the flow through the areas, measurement, the workbench |
| `examples/greeting` | A small estate with changes carried through the method, the first written by hand and the second by agents |

A change carried through the method lives in a folder of its own — seven artefacts, an append-only record, its candidates, its evidence and what it cost. That folder is the output of using the framework on one estate and is kept with that estate, not here.

## Licence

Documentation is under [CC BY-SA 4.0](LICENSES/CC-BY-SA-4.0.txt) — read, cite, translate and adapt it, including commercially, with attribution and under the same terms. Source code is under [BUSL-1.1](LICENSES/BUSL-1.1.txt): production use is permitted except as a hosted service or in competition with the Licensor, and each version converts to AGPL-3.0-or-later on its Change Date. See [LICENSE](LICENSE).

Copyright (c) 2026 Illia Gannytskyi. The Licensor retains the right to license this work on other terms.
