# iFlow — Object Catalogue

**Status:** approved, provisional
**Version:** 1.1 — 2026-09-02
**Requirements on objects:** [03-schema.md](./03-schema.md), Part A
**Objects are used in:** [04-areas-specified.md](./04-areas-specified.md)

This document defines the objects that pass between areas. It defines no area and adds no schema requirement.

Per `03` Part A, **identity (O1), provenance (O3), confidence (O4) and ownership (O6) apply to every object without exception** and are not repeated below. Each entry states what the object is, the fields that constitute its schema (O2), and its validity (O5), because those are what differ.

---

## Group I — Intent and specification

| Object | What it is | Schema (O2) | Validity (O5) |
|---|---|---|---|
| **`Intent`** | A person's statement of what is to be achieved. The only object originating outside the system. | statement; requester; priority; deadline | Until satisfied or withdrawn |
| **`ChangeClass`** | A category of change defined by *how its acceptance is decided*, not by what it touches. The typology is [06-change-classes.md](./06-change-classes.md). | name; decidability (full, partial, none); admissible evidence kinds; required evidence set; default scope shape; default budget profile | Long-lived; revised only by area 13 on measurement |
| **`Criterion`** | One condition on an acceptable result. | statement; decision procedure (machine or human); required evidence kind; threshold | Fixed with its specification |
| **`AcceptanceCriteria`** | The set of criteria for one specification. | criteria; completeness marker | Fixed with its specification |
| **`TerminationCondition`** | Conditions under which work stops without acceptance. | condition; action on trigger | Fixed with its specification |
| **`Scope`** | The declared region of the estate a change may touch. | included; excluded; derived-from class default | Fixed with its specification |
| **`Specification`** | The complete statement of a change to be made and judged. | `Intent`; `ChangeClass`; `AcceptanceCriteria`; `TerminationCondition`; `Scope` | **Immutable after admission.** A changed intent produces a new specification; it does not edit an existing one |

## Group II — The estate

| Object | What it is | Schema (O2) | Validity (O5) |
|---|---|---|---|
| **`Statement`** | One assertion about the estate. The atom of the estate model. | subject; relation; object; source artifact; derivation method | Until the source artifact changes; validity is per statement, not per model |
| **`EstateModel`** | The body of statements together with the queries answerable over it. Not a document. | statements; query interface; freshness per region; **observational adequacy per region** — how well behaviour there can be pinned down | Never wholly valid or wholly stale; measured by region |
| **`AreaOfEffect`** | The region a given change can affect. | node set; derivation; computed-at; estate version | **Short.** Invalidated by any landing intersecting it |
| **`Testimony`** | A non-derivable statement drawn from a decision a person was already making. | statement; decision it was drawn from; bound places; falsifier; expiry | Expires with what it was drawn from; never outranks a statement derived from current code |

## Group III — Work

| Object | What it is | Schema (O2) | Validity (O5) |
|---|---|---|---|
| **`WorkUnit`** | A bounded, executable piece of work with its own criteria. | specification ref; `Scope`; `AreaOfEffect`; inherited criteria; dependencies; size estimate | Until admitted, or until its area of effect is invalidated while it waits |
| **`ContextBundle`** | The subset of knowledge supplied to an agent for one unit. | statements included; selection rule; estate version; budget consumed | One execution only; never reused across runs |
| **`AdmissionDecision`** | The decision to commit resources to a unit, or not. | unit; outcome (admitted, held, refused); deciding condition; decided-at | Held decisions expire into escalation at a stated age |
| **`Grant`** | Permissions bound to one unit of work. | unit; permitted operations; targets; credential reference; expiry | Expires with the run; never renewed by the agent holding it |
| **`Conflict`** | A detected interference between two units. | units; intersecting region; evidence it would invalidate | Until one of the two units terminates |

## Group IV — Result and evidence

| Object | What it is | Schema (O2) | Validity (O5) |
|---|---|---|---|
| **`Candidate`** | A proposed change. Never applied by the area that produced it. | unit; artifact set; executor identity and version; produced-at | Until its supporting evidence expires |
| **`Trace`** | The record of one run. | run id; steps; tool calls; input and output digests; executor version; timestamps | Retained for the lifetime of the decisions it supports. Append-only; not writable by its subject |
| **`Evidence`** | An artifact supporting one claim about a candidate. | claim; kind (test run, static analysis, runtime observation, human affirmation); producer; **independence from the executor, and how it is established**; obtained-at | **Has a shelf life.** Bound to the estate version and executor version it was obtained against |
| **`Verdict`** | The acceptance decision on a candidate. | candidate; per-criterion outcome (met, failed, **undecided**); evidence refs; overall outcome; decided-by; **state (settled or deferred)**; observation window and baseline, where deferred | Until invalidated by a landing that touches its area of effect. **A deferred verdict holds its candidate reversible until it closes**, and a verdict that never closes is a failure, not a permanent state |
| **`LandingPlan`** | The order in which accepted candidates enter the live system. | ordered entries; expected invalidations; re-establishment required before each | One landing cycle |

## Group V — Governance

| Object | What it is | Schema (O2) | Validity (O5) |
|---|---|---|---|
| **`Budget`** | A declared allowance against which work is admitted. | owner; period; limit; consumed; weighting by class | One accounting period |
| **`CostRecord`** | Measured consumption attributable to one unit. | unit; tokens; steps; elapsed; money; attributed-to | Permanent |
| **`Escalation`** | A request for human involvement, with its ground. | origin area; ground; unit; raised-at; respondent; resolution; **behaviour on no response** | Expires; expiry is itself a recorded outcome |
| **`Baseline`** | The present way of working, measured on the same workload. | workload description; period; values; method | Re-established when the workload changes materially |
| **`Metric`** | One measured figure. | name; population; `ChangeClass`; period; value; method | One period |

---

## Invariants across objects

Four rules bind the catalogue together. Each restates, at the level of objects, a decision already made at the level of areas.

1. **A `Specification` is immutable after admission.** Otherwise criteria are shaped by what execution turned out to produce, and the hypothesis in `02` fails at its first condition.
2. **A `Candidate` is never applied by the area that produced it.** Execution proposes; landing disposes.
3. **`Evidence` records its independence from the executor, and how that independence is established.** Evidence that cannot state this does not count toward a verdict.
4. **A `Trace` is not writable by its subject.** A record an agent can edit is not a record.
5. **A deferred `Verdict` and reversibility are inseparable.** A change may be landed on an open verdict only while it can still be withdrawn. Losing reversibility closes the verdict by forcing a decision, it does not extend it.

## What is not an object

Conversation, prose descriptions, dashboards and reports are not objects. They may be rendered *from* objects, and nothing may be passed between areas in that form. This is the composability test of `03` Part C stated negatively.
