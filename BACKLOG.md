# Backlog

What has been agreed and not done, what is waiting on a decision, and what was rejected and why. A list that stops things being dropped between conversations.

**What goes in here.** Work agreed and not done. Decisions waiting on a person. Things rejected, with the reason — because a rejection without its reason gets proposed again.

**What does not.** The reasoning that led to any of it. Reasoning either survives into `docs/iflow.md`, where it is part of the method, or it does not survive at all. A backlog that records every exchange becomes the second decaying corpus this method exists to prevent.

## Agreed, not done

| # | Item | Raised |
|---|---|---|
| 1 | **The one limit shared by five rules.** R12, R13, R15, R16 and R18 each check that something was recorded and none can check that the record is honest. One defect, recorded five times as five | 09-10, open as `human-control#1` |
| 2 | **Verification in an isolated tree is not enough** — an arbiter can pass there and fail where the change lands | 09-10, open as `human-control#2` |
| 3 | **Nothing requires an arbiter to say it tests a proxy** for its criterion rather than the criterion itself. That proxy has broken three times | 09-10, open as `human-control#3` |
| 4 | **No cross-change view** — what is in flight, held, awaiting another authority, blocked. Computed from the folders in flight and gone with them; it stores nothing | 09-10 |
| 5 | **Nothing reviews the escalations as a set** — five turned out to be one defect, noticed only by reading them together | 09-10 |
| 6 | **Estate tooling for area 1.** A code graph per repository, consumed rather than built, with the layer above it that nobody ships: cross-repository contract edges matched and confirmed by telemetry, confidence per edge, provenance per statement, freshness per region, observational adequacy per region, reachability beyond the estate, and an overlay of changes in flight. One query: what does this change affect, and at what confidence | 09-10 |
| 7 | **Refresh on demand, not on a timer.** The index records the commit it was built from per repository; a query names a region; a region whose source has moved is refreshed before the query is answered. Forge events say what changed; local clones hold the content, because a parser needs files and pulling hundreds of repositories over an API is slower than a fetch | 09-10 |
| 8 | **Discover repositories from the forge, keep a list of exclusions.** A hand-maintained list of inclusions rots the moment someone creates a repository and forgets it | 09-10 |
| 9 | **Criteria are objects while a change is in flight and history after it lands.** The gate cannot check what lives only in a tracker, and a landed criterion is history for which a tracker is the right home. This is why a change folder is transient by design | 09-10 |
| 10 | **A baseline measured on real work** — the instruments are described and not one figure has been taken | 09-09, blocked on having an estate |

## Waiting on a decision

| # | Question | Why it matters |
|---|---|---|
| A | Is the check of an intent against the estate **advisory or blocking**? | Blocking is stronger and closer to the method, but it makes the quality of the graph a precondition for working at all — on an unobserved estate it stops nearly everything. A third option: blocking only where the graph is confident, advisory where it is not, so the gate tightens itself as the graph improves and the share of each becomes a measurable quantity |

## Rejected, with the reason

| Item | Why not |
|---|---|
| **A living specification** — a current statement of what the system requires, folded from the deltas of every landed change | It is derived from the record of changes rather than from the system, so relative to current code it is an assertion about the past. Where a derived statement and an asserted one conflict the derived one prevails, so the estate model always wins and the comparison has a foregone answer. Worse, it is a second corpus coupled to history rather than to the code that invalidates it — the decay this method exists to prevent, one level up. The part of the problem that is real, standing obligations not derivable from code, is `Testimony`: it already expires, carries a falsifier, and never outranks a statement derived from current code |

## Done since this list began

| Item | Landed |
|---|---|
| The repository holds the framework and nothing else; arbiters build their fixtures instead of borrowing them | v3.0.0 |
| Seven probes for arriving at a criterion, one of them mechanical | v2.5.0 |
| `Refusal` as an object; immutability from a candidate; the criteria review point | v2.2.0 |
| Diagrams of the phases, artefacts and human touchpoints | v2.1.0 |
| `EvidencePlan`, the B′ gate, scenario-form criteria | v2.0.0 |
| Escalations closable across changes | v1.12.0 |
