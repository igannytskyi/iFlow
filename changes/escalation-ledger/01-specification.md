# Specification — escalation-ledger

| Field | Value |
|---|---|
| Id | SPEC-007 |
| Intent | INT-007 |
| Default class | C3 |

An escalation is raised inside one change and is almost always resolved by another, because resolving it means changing the framework. Nothing can close it from there: its identifier is an ordinal local to the folder that raised it, and no artefact anywhere says it has been settled. Two are open now that were resolved runs ago. The list of open escalations is the only place the framework's debt to itself is visible, and it is currently wrong.

## Acceptance criteria

| Id | Criterion | Procedure | Required evidence | Threshold |
|---|---|---|---|---|
| CR-007-01 | An escalation resolved by a later change stops appearing as owed | machine | test-run | closed elsewhere is reported closed |
| CR-007-02 | Settling something that was never owed, or was owed once and already settled, is detected | machine | test-run | both detected |
| CR-007-03 | The gate's verdict on the changes that already exist is unchanged | machine | test-run | identical, this repair the only variable |

## Termination

| Condition | Action |
|---|---|
| An escalation cannot be addressed without editing a change that has already landed | escalate |

## Scope

| Included | Excluded |
|---|---|
| tools/check.py | changes/ |
| framework/RULES.md | templates/ |
| framework/CONVENTIONS.md | |

## Arbiter

| Path | What it arbitrates |
|---|---|
| tests/ | CR-007-01, CR-007-02, CR-007-03 |

## Arbiter reads

| Input | In scope? |
|---|---|
| tools/check.py | yes — the gate is both subject and input |
| changes/ | no |
