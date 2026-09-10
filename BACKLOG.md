# Backlog

What has been agreed and not done, what is waiting on a decision, and what was rejected and why. A list that stops things being dropped between conversations.

**What goes in here.** Work agreed and not done. Decisions waiting on a person. Things rejected, with the reason — because a rejection without its reason gets proposed again.

**What does not.** The reasoning that led to any of it. Reasoning either survives into `docs/iflow.md`, where it is part of the method, or it does not survive at all. A backlog that records every exchange becomes the second decaying corpus this method exists to prevent.

## Agreed, not done

| # | Item | Raised |
|---|---|---|
| 1 | **The one limit shared by five rules.** R12, R13, R15, R16 and R18 each check that something was recorded and none can check that the record is honest. One defect, recorded five times as five | 09-10, open as `human-control#1` |
| 4 | **This file is a hand copy of a ledger it cannot read.** The escalation ledger is computed from change folders, which are no longer versioned; anyone cloning the repository sees only this file, and the two will drift. Either escalations do not belong in change folders, or this file is derived from them | 09-10, open as `self-observation#1` |
| 5 | **Nothing reviews the escalations as a set** — five turned out to be one defect, noticed only by reading them together. `--status` now shows how many each change owes, which makes accumulation visible without saying anything about what they share | 09-10 |
| 6 | **Estate tooling for area 1.** A code graph per repository, consumed rather than built, with the layer above it that nobody ships: cross-repository contract edges matched and confirmed by telemetry, confidence per edge, provenance per statement, freshness per region, observational adequacy per region, reachability beyond the estate, and an overlay of changes in flight. One query: what does this change affect, and at what confidence | 09-10 |
| 7 | **Refresh on demand, not on a timer.** The index records the commit it was built from per repository; a query names a region; a region whose source has moved is refreshed before the query is answered. Forge events say what changed; local clones hold the content, because a parser needs files and pulling hundreds of repositories over an API is slower than a fetch | 09-10 |
| 8 | **The estate carries what it does not know.** A configured list does not disappear — discovery needs a seed, and a repository created in another group, another namespace or another forge never appears in any listing. What changes is scale: a handful of groups rotting yearly rather than hundreds of repositories rotting weekly. Completeness is never assumed, and an unindexed region is *coverage unknown*, not *coverage absent*. Gaps announce themselves as unattributable edges — traffic from a service that maps to no repository, an artefact in the registry with no indexed source, an import pointing outside — by the same mechanism that makes a surprising landing evidence against the model | 09-10 |
| 10 | **A baseline measured on real work** — the instruments are described and not one figure has been taken | 09-09, blocked on having an estate |

## Rejected, with the reason

| Item | Why not |
|---|---|
| **The estate as a gate on intent** — refusing an intent the graph does not recognise | The estate model produces statements with provenance and confidence; verdicts belong to assurance and refusals to admission and the human boundary. A fifth gate would refuse work on a knowingly incomplete picture, which is a defect anywhere else in this method. The graph *determines* rather than refuses: area of effect, observational adequacy, the class, and what evidence exists — and what follows from those is already handled by the gates that exist |
| **A living specification** — a current statement of what the system requires, folded from the deltas of every landed change | It is derived from the record of changes rather than from the system, so relative to current code it is an assertion about the past. Where a derived statement and an asserted one conflict the derived one prevails, so the estate model always wins and the comparison has a foregone answer. Worse, it is a second corpus coupled to history rather than to the code that invalidates it — the decay this method exists to prevent, one level up. The part of the problem that is real, standing obligations not derivable from code, is `Testimony`: it already expires, carries a falsifier, and never outranks a statement derived from current code |

## Done since this list began

| Item | Landed |
|---|---|
| A view across changes, one action to run every arbiter, and an arbiter that says whether it tests its criterion or a proxy | v3.1.0 |
| The repository holds the framework and nothing else; arbiters build their fixtures instead of borrowing them | v3.0.0 |
| Seven probes for arriving at a criterion, one of them mechanical | v2.5.0 |
| `Refusal` as an object; immutability from a candidate; the criteria review point | v2.2.0 |
| Diagrams of the phases, artefacts and human touchpoints | v2.1.0 |
| `EvidencePlan`, the B′ gate, scenario-form criteria | v2.0.0 |
| Escalations closable across changes | v1.12.0 |
