---
name: iflow
description: Carry a change through iFlow — intent, specification, plan, admission, execution, assurance, landing. Use when the user says /iflow, asks to start or continue a change under iFlow, wants a specification with acceptance criteria written before execution, wants to admit or hold work, wants a verdict on a candidate, or asks for the status of a change. Also use when they ask to check a change folder against the rules.
---

# iFlow

Carry one change through the path: intent, specification, plan, admission, execution, assurance, landing.

**The rules that bind you are in [AGENTS.md](../../../AGENTS.md), and they are not repeated here.** Read it before writing anything. Read `framework/conventions.md` for every code, and `framework/rules.md` for what will be checked and what deliberately will not.

One change lives in `changes/<slug>/` as seven files written in order. Copy the matching file from `framework/templates/` and fill it. Never invent a column.

## Verbs

| Verb | Writes |
|---|---|
| `intent` | `00-intent.md` |
| `plan` | `01-specification.md`, `02-plan.md` |
| `admit` | `03-admission.md` |
| `run` | `04-execution.md` |
| `verify` | `05-assurance.md` |
| `land` | `06-landing.md` |
| `status` | — reads the folder and reports where the change stands and what blocks it |
| `check` | — runs `python3 framework/check.py changes/<slug>` and reports it verbatim |

Every verb appends its crossing to `record.md`.

## Before you present anything

Run the check and report what it says, including when it passes. A verdict, a landing or a status presented without it is not finished work.
