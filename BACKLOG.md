# Backlog

What we have agreed to do and not yet done. Not a plan and not a roadmap — a list that stops things being dropped between turns. An item leaves here when it lands or when it is explicitly abandoned.

## Agreed, not done

| # | Item | Raised | State |
|---|---|---|---|
| 1 | The one limit shared by R12, R13, R15, R16 and R18: the gate checks that something was recorded, never that the record is honest. Five rules, one defect | 2026-09-10 | open as `human-control#1` |
| 2 | Verification in an isolated tree is not enough — an arbiter can pass there and fail where the change lands | 2026-09-10 | open as `human-control#2` |
| 3 | Nothing requires an arbiter to say that it tests a proxy for its criterion rather than the criterion itself. That proxy has now broken three times | 2026-09-10 | open as `human-control#3` |
| 5 | **No cross-change view** — the ledger shows escalations and nothing shows what is in flight, held, awaiting an authority or blocked. Invisible at one change, mandatory at fifty. OpenSpec carries 128 files of changes in flight and has three commands for looking at them | 2026-09-10 | from the comparison |
| 6 | **Nothing reviews the escalations as a set** — five turned out to be one defect, noticed only by looking | 2026-09-10 | from the comparison |
| 8 | Estate representation as a standalone contract with two consumers — iFlow and the estimation framework. OpenSpec ships this as Stores: planning in a repository of its own, one change spanning three code repositories, requirements owned by one team and read-only to others | 2026-09-09 | deferred to implementation |
| 9 | A baseline measured on real work — §10 describes the instruments and not one figure has been taken | 2026-09-09 | blocked on having an estate |

## Rejected, with the reason

| Item | Why not |
|---|---|
| A living specification — a current statement of what the system requires, folded from the deltas of every landed change | It is derived from the record of changes, not from the system, so relative to current code it is an assertion about the past. Where a derived statement and an asserted one conflict, the derived one prevails — so the estate model always wins and the comparison has a foregone answer. Worse, it is a second corpus coupled to history rather than to the code that invalidates it, which is the decay this method exists to prevent, one level up. The part of the problem that is real — standing obligations not derivable from code, such as a contractual or regulatory requirement — is `Testimony`, which already expires, carries a falsifier and never outranks a statement derived from current code |

## Done since this list began

| Item | Landed |
|---|---|
| Seven probes for arriving at a criterion, one of them mechanical | v2.5.0 |
| `Refusal` as an object; immutability from a candidate; the criteria review point | v2.2.0 |
| Diagrams of the phases, artefacts and human touchpoints | v2.1.0 |
| `EvidencePlan`, the B′ gate, scenario-form criteria | v2.0.0 |
| Escalations closable across changes | v1.12.0 |
