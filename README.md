# iFlow

A framework for software development and maintenance carried out at scale by autonomous agents.

**Goal.** To make the development and maintenance of software an activity carried out at any scale by autonomous agents: a person contributes the intent and the criteria of an acceptable result, is free not to take part in how it is achieved, and is able to verify it at will; and the volume of such work is limited only by the resources one is willing to spend on it.

**Status.** Working, early. One Go program carries a task from a sentence to a landed change: agents state the criteria, a person accepts them, and every area after that runs by itself, judged at each boundary by a mechanical gate.

## Install

Go 1.27 or later, and a C compiler for the language grammars and the window.

```bash
make install      # builds bin/iflow, puts it in ~/Applications/iFlow.app if there is one, and replaces
                  # the running service with this build, carrying its runs on
```

`make` builds only, `make test` checks everything. Put `bin/` on your PATH, or run `./bin/iflow`.

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
| `IFLOW_PACK_TOKENS` | the most one agent is handed — its goal, the tests it must pass, its files and what surrounds them; 16000 when unset |

Runs are carried by the service, so these are read when it starts: after changing one, `iflow service restart`.

## Use

```bash
iflow tui [project]                     # the path in a terminal: tasks, areas, the conversation beside them
iflow desk <project>...                 # the same in a window
iflow run [--accept|--ask] <project> <task>  # carry a task, printing each step
iflow models                            # which model does each piece of work
iflow stop <change>                     # stop the run carrying a change, wherever it was started
iflow resume [--ask] <change>           # carry a stopped change on from where it stopped
iflow reopen [--ask] <change> <1|2> <what is to change>  # carry a change again in its own folder,
                                        # from its specification (1) or its plan (2)
iflow settle <change> <n> <decision>    # close an escalation the change raised by your decision
iflow service status                    # the service: which build, since when, how many runs
```

Every run is carried by one service, whichever screen started it: the window, the terminal and `iflow run` see the same runs, and a run goes on when its screen closes. The first screen opened starts the service; a screen of a later build replaces it, stopping its runs where they can be carried on and carrying them on in the new one — so a new build never needs the service killed by hand. `iflow service status|start|stop|restart|log` does the same by hand.

The workbench shows each area as the task passes through it, every object it hands on, what the gate said at each boundary, what the agent did step by step, and what each step cost — model, tokens and money.

### Starting the workbench

From a clone of this repository:

```bash
make install                            # bin/iflow, the app, the service
iflow models                            # every role should say "ready"; if not, add the key as above
iflow desk ~/code/my-project            # opens the window with that project in it
```

`iflow desk` with no project opens the projects it remembers from last time, or the examples that come with the framework when there are none. More projects are added with **+** beside *Projects* in the window, and are remembered. If `iflow` is not found, run it as `./bin/iflow desk`.

In the window:

- **+ New intent** under a project, or Cmd/Ctrl+N: type what should change, Enter starts it; the specification waits for you to accept it or send it back.
- The box at the foot is **Chat** or **Intent**. Chat talks with the model about what is on view: each task has its own conversation, handed that task's specification, plan and record, and the model may read the repository before it answers. *Make it an intent* turns an answer into a task.
- A task at work can be stopped, wherever it was started; one that stopped is resumed from where it stopped — a specification not yet accepted is put to you again as it is, a plan made is carried on from admission, units all carried are judged and landed — or carried again with something changed, or closed.
- A task's problems and escalations open from the chip under its heading. Each can be fixed in the task itself — carried again from its specification or from its plan, with what is to change — or by a new intent; an escalation can also be settled by a decision, kept with your others.
- Cmd/Ctrl+1…9 choose a project; Escape closes the open object, handover, foundation page, then the settings.
- **Settings** at the bottom left: which model does each piece of work, and what each costs.

On a Mac the window can also live in an app bundle; after copying a new `iflow` into it, sign the bundle again (`codesign --force -s <identity> <bundle>`), or macOS starts the program without showing its window. An estate may keep its own test runner as `test.sh` at its root: run with no arguments it runs every test; run with names, one an argument — each a regular expression matching the test's name — it runs those tests. Without one, the runner of the estate's language is used (`go test`, pytest, `node --test`). Its icon is drawn by `go run ./cmd/iflow-icon icon.png`; the command's documentation says how to make the bundle's `Contents/Resources/iFlow.icns` from it, which `CFBundleIconFile` in `Info.plist` names.

```bash
iflow check <changes/slug>              # one change, against every rule
iflow bench [--set review=1] [--label x] # the reference tasks (examples/bench.json), every
                                        # question answered yes: what each came to and cost
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
