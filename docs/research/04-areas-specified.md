# iFlow — The Areas, Specified

**Status:** approved, provisional
**Version:** 2.3 — 2026-09-02
**Schema:** [03-schema.md](./03-schema.md) — parameters P1–P10
**Areas:** [02-areas.md](./02-areas.md)

Each area is stated against the ten parameters. Object names are used consistently across areas; the object catalogue itself is a separate document.

---

## Part I — The path a change travels

### 1. Intent and Criteria

- **P1 Inputs.** `Intent`, `EstateModel`, the catalogue of `ChangeClass`.
- **P2 Outputs.** `Specification`, containing `ChangeClass`, `AcceptanceCriteria`, `TerminationCondition` and `Scope`.
- **P3 Decision rule.** A specification is complete when every criterion is either decidable by machine or explicitly marked as requiring a person. What is decidable is a property of the `ChangeClass`, not of the individual intent.
- **P4 Completion.** Completeness reached, or ambiguity declared irreducible.
- **P5 Invariant.** Criteria are fixed before execution and are not altered by it.
- **P6 Failure semantics.** An intent that cannot be expressed as criteria is a failure of this area, not a poor specification passed downstream.
- **P7 Evidence emitted.** What each criterion is grounded in, and who affirmed it.
- **P8 Cost and stopping.** Clarifying exchanges with a person; stop when further exchange stops raising decidability.
- **P9 Authority required.** Read the estate model. No write.
- **P10 Escalation.** An undecidable criterion, or a contradiction between criteria.

### 2. Work Formation

- **P1 Inputs.** `Specification`, `EstateModel`.
- **P2 Outputs.** A `ChangePlan` over a set of `WorkUnit`, each carrying its own `ChangeClass`, `AreaOfEffect`, inherited `AcceptanceCriteria` and `Scope`. Where phases must be separated in time, the plan carries the wait conditions and the requirement that every intermediate state is a valid, shippable system — a criterion belonging to no single unit. Where the specification is not decidable as it stands, the output includes **preparatory units** — a reproduction, characterization of current behaviour in an inadequately observed region — whose own acceptance is decidable and on which the original unit depends.
- **P3 Decision rule.** Divide until each unit's area of effect is computable and its criteria are decidable within it; do not divide past the point where criteria cease to be verifiable.
- **P4 Completion.** The units together cover the specification, and none exceeds the size at which completion probability falls below the declared bound.
- **P5 Invariant.** The union of the units' criteria implies the specification's criteria — nothing acceptable is lost in the division. **This holds only where the estate query that produced the scope was complete**; where completeness is not derivable, the residue is stated on the specification rather than assumed away.
- **P6 Failure semantics.** A specification that cannot be covered by units with computable areas of effect returns to area 1; it is not passed on in parts. Emitting a unit that cannot be accepted, where a preparatory unit would have made it acceptable, is a failure of this area.
- **P7 Evidence emitted.** The derivation of each unit's boundary and area of effect from the estate model.
- **P8 Cost and stopping.** Estate queries and analysis per unit; stop when further division stops reducing area of effect.
- **P9 Authority required.** Read the estate model. No write.
- **P10 Escalation.** Coverage incomplete, or an area of effect that cannot be bounded.

### 3. Admission

- **P1 Inputs.** Pending `WorkUnit`s with their `AreaOfEffect`s, `EstateModel` including work in flight, `Budget`, policy.
- **P2 Outputs.** `AdmissionDecision`, `Grant`, `Conflict`.
- **P3 Decision rule.** A unit is admitted when its area of effect does not intersect that of an unfinished unit in a way that would invalidate either's evidence, an allocation exists for it, and a grant no wider than its `Scope` can be issued. Failing any of the three, it is held rather than started.
- **P4 Completion.** Every pending unit is admitted, held with a stated reason, or refused.
- **P5 Invariant.** No resources are committed to a unit that could not have been accepted had it succeeded.
- **P6 Failure semantics.** Admitting a unit that later proves to have been in conflict is a failure of this area, not of landing. Holding a unit that could have run is a lesser failure and must be visible as queueing rather than as silence.
- **P7 Evidence emitted.** For each decision, which of the three conditions decided it, against what, and at what confidence — a conflict found through a matched contract edge is only as certain as that edge.
- **P8 Cost and stopping.** The gate must cost far less than the work it withholds; a unit held beyond a stated age is escalated rather than held indefinitely.
- **P9 Authority required.** Issue grants within policy; refuse and hold work. No ability to widen policy.
- **P10 Escalation.** A unit held past its age limit; a conflict that no ordering resolves; a required grant wider than policy allows.

### 4. Execution

- **P1 Inputs.** `WorkUnit`, `ContextBundle`, `Grant` and `AdmissionDecision` from area 3.
- **P2 Outputs.** `Candidate`, `Trace`.
- **P3 Decision rule.** Act within `Scope` and stop on `TerminationCondition`; produce a candidate, never a change to the live system. The executor is whatever satisfies the class: **where the class permits a deterministic transformation, using an agent instead is a defect** — it makes a reproducible result unreproducible and costs more.
- **P4 Completion.** A candidate exists, or the termination condition is met, or the step budget is exhausted.
- **P5 Invariant.** No effect outside the declared `Scope` and `Grant`; the estate itself is not modified.
- **P6 Failure semantics.** Substrate failure — tool error, capacity exhaustion, timeout — is distinguished from task failure, the inability to satisfy the criteria. The two are retried differently and only the second is informative about the work.
- **P7 Evidence emitted.** `Trace`, together with the identity and version of the executor at the time of the run.
- **P8 Cost and stopping.** Tokens, steps, tool calls, elapsed time; a stopping rule for non-convergence.
- **P9 Authority required.** Exactly the `Grant` bound to the work unit, enforced outside the agent.
- **P10 Escalation.** Repeated substrate failure, or task failure where the criteria were judged achievable.

### 5. Assurance

- **P1 Inputs.** `Candidate`, `Specification`, `EstateModel`, `Trace`.
- **P2 Outputs.** `Verdict`, with the `Evidence` supporting it.
- **P3 Decision rule.** Accept only when every criterion is supported by evidence produced independently of the executor; otherwise reject or leave undecided. Which evidence counts as sufficient is fixed per `ChangeClass` in advance, not chosen per candidate.
- **P4 Completion.** A verdict exists for every criterion in the specification. A verdict on an effect that does not exist before exposure is **deferred rather than absent**: it is opened here with its observation window and baseline, and closed by area 13 after landing.
- **P5 Invariant.** Evidence is not produced by the agent that produced the candidate.
- **P6 Failure semantics.** Inability to obtain evidence is an *undecided* verdict, not a rejection, and the two must not be conflated.
- **P7 Evidence emitted.** The evidence itself, and how each item was obtained.
- **P8 Cost and stopping.** Verification runs; stop when the cost of assurance exceeds the value of the change, which is itself a decision that must be recorded.
- **P9 Authority required.** Execute tests and analyses in isolated environments, and **read production observations**, which a C4 criterion cannot be decided without. No write anywhere.
- **P10 Escalation.** An undecided verdict, or a criterion marked as requiring a person.

### 6. Landing

- **P1 Inputs.** Accepted `Candidate`s with their `Verdict`s and `Evidence`, `EstateModel`.
- **P2 Outputs.** `LandingPlan`, and the landed change itself.
- **P3 Decision rule.** A candidate enters only while the evidence supporting its verdict is still valid; where an earlier entry has invalidated it, the affected evidence is re-established before entry rather than the candidate being dropped.
- **P4 Completion.** Every accepted candidate has entered, awaits re-establishment of evidence, or has been reversed.
- **P5 Invariant.** Nothing takes effect on evidence that has expired or been invalidated by another entry. A candidate carrying a deferred verdict enters only while it remains reversible, and remains reversible until that verdict closes.
- **P6 Failure semantics.** Joint incorrectness in production is a failure of this area, and must be traceable to a specific invalidation that went unnoticed. Conflict that should have prevented the work from starting is a failure of area 3, not of this one.
- **P7 Evidence emitted.** What was invalidated by each entry, and what was re-established before the next.
- **P8 Cost and stopping.** Re-verification after invalidation, and work waiting on it.
- **P9 Authority required.** Write to the live system, narrowly and per target.
- **P10 Escalation.** Evidence that cannot be re-established; a reversal that fails.

---

## Part II — Foundations

### 7. Estate Representation

- **P1 Inputs.** Code, configuration, version history, build and deployment records, runtime telemetry, `Testimony`.
- **P2 Outputs.** `EstateModel`; answers to queries, chief among them `AreaOfEffect`, the observational adequacy of a region, and which consumers of the estate lie beyond the reach of any change to it.
- **P3 Decision rule.** A statement enters the model with its provenance and confidence; where a derived statement and an asserted one conflict, the derived one prevails.
- **P4 Completion.** Never complete. Measured by freshness, not by coverage.
- **P5 Invariant.** Every statement carries provenance, confidence and validity. Nothing is served as fact without a source.
- **P6 Failure semantics.** Staleness is detected and reported; serving a stale answer as current is the failure mode of this area.
- **P7 Evidence emitted.** For every answer, what confirms it.
- **P8 Cost and stopping.** Indexing and re-derivation, incremental rather than whole-estate.
- **P9 Authority required.** Read across artifacts and telemetry. No write.
- **P10 Escalation.** A question that cannot be answered to the confidence the asking decision requires.

### 8. Record

- **P1 Inputs.** Everything every area emits.
- **P2 Outputs.** `Trace`, and the durable binding between a `Verdict` and the evidence that supported it.
- **P3 Decision rule.** Record unconditionally. Nothing is written selectively on the basis of expected interest.
- **P4 Completion.** A record closes when the work it covers reaches a terminal state.
- **P5 Invariant.** Append-only, and not writable by the agent it describes.
- **P6 Failure semantics.** Failure to record is failure of the work: work that cannot be recorded must not proceed.
- **P7 Evidence emitted.** The record is the evidence; its own integrity must be attestable.
- **P8 Cost and stopping.** Storage and retention, bound to the lifetime of the decisions the record supports.
- **P9 Authority required.** Write to the record store only.
- **P10 Escalation.** Record integrity broken or the store unavailable.

### 9. Economy

- **P1 Inputs.** `Budget`, `CostRecord`, pending `WorkUnit`s, the priority of the intents behind them.
- **P2 Outputs.** Admission decisions, allocations, stop signals.
- **P3 Decision rule.** Work is admitted only against an existing allocation; exceeding a limit blocks before the spend rather than being reported after it.
- **P4 Completion.** Continuous; settled per accounting period.
- **P5 Invariant.** No work runs without an allocation attributable to it.
- **P6 Failure semantics.** Unattributable spend is a failure — an agent whose cost cannot be attributed must not run.
- **P7 Evidence emitted.** Cost per unit of verified change, by class of work.
- **P8 Cost and stopping.** The accounting itself must be negligible against what it governs.
- **P9 Authority required.** Refuse and stop work. No ability to alter budgets.
- **P10 Escalation.** Exhaustion against work classified as mandatory, such as a production defect.

### 10. Accumulation

- **P1 Inputs.** `Trace`, `Verdict`, `Conflict`, and decisions made by people during escalation.
- **P2 Outputs.** `Testimony`, bound to places in the estate.
- **P3 Decision rule.** Capture only as a byproduct of a decision already being made; never as a separate request for someone to write something down.
- **P4 Completion.** Continuous.
- **P5 Invariant.** Testimony carries what it was drawn from and expires with it, and never outranks a statement derived from current code.
- **P6 Failure semantics.** Unfalsifiable testimony is worse than none and is refused at capture.
- **P7 Evidence emitted.** The decision the testimony was drawn from.
- **P8 Cost and stopping.** Measured as the cost of a second task in the same area relative to the first.
- **P9 Authority required.** Read traces and verdicts; write only to the testimony store.
- **P10 Escalation.** None. Accumulation must never block work.

### 11. Human Boundary

- **P1 Inputs.** `Escalation` from any area; historical verdicts and their outcomes.
- **P2 Outputs.** Routing of an escalation to a person, and revision of the escalation rules themselves.
- **P3 Decision rule.** A person is drawn in when a decision is undecidable on evidence, when authority is required, or when the consequence exceeds a declared threshold.
- **P4 Completion.** Every escalation resolved or expired, with expiry itself being a recorded outcome.
- **P5 Invariant.** A case that met the escalation rule and did not reach a person is detectable afterwards.
- **P6 Failure semantics.** Silent non-escalation is the failure mode of this area; late escalation is the lesser one.
- **P7 Evidence emitted.** Participation per unit of change, and its trend over time.
- **P8 Cost and stopping.** Human time, accounted as the scarcest resource there is.
- **P9 Authority required.** Interrupt and hold work.
- **P10 Escalation.** This area is the escalation target. What happens when a person does not respond must itself be defined.

### 12. Authority

- **P1 Inputs.** `WorkUnit`, declared policy.
- **P2 Outputs.** `Grant`, bound to one work unit.
- **P3 Decision rule.** A grant is the narrowest set of permissions sufficient for the declared `Scope`; anything wider is refused rather than warned about.
- **P4 Completion.** A grant exists before execution starts, or execution does not start.
- **P5 Invariant.** Enforcement lies outside the agent. An agent cannot widen its own grant.
- **P6 Failure semantics.** A refused action is a normal outcome, reported to the requester rather than retried by another route.
- **P7 Evidence emitted.** Every attempted action against its grant, allowed or refused.
- **P8 Cost and stopping.** Negligible by requirement; enforcement must not become a throughput constraint.
- **P9 Authority required.** Issue and revoke grants. This is the root of authority and must itself be governed by people.
- **P10 Escalation.** A request for a grant wider than policy allows.

### 13. Measurement

- **P1 Inputs.** `Trace`, `Verdict`, `CostRecord`, `Escalation`, landed changes and their later outcomes, `Baseline`.
- **P2 Outputs.** `Metric` series, per `ChangeClass`.
- **P3 Decision rule.** A measurement counts only against a population and a class stated in advance; a figure without both is not published.
- **P4 Completion.** Continuous, settled per period.
- **P5 Invariant.** The baseline is measured on the same workload as the comparison, otherwise no comparison is made at all.
- **P6 Failure semantics.** A metric that cannot detect the failure it is supposed to detect is worse than none. The governing case: a wrongly accepted change is by construction unread, so it can only be discovered later — from a defect, an incident or a reversal — and the lag between acceptance and discovery is itself the instrument's resolution.
- **P7 Evidence emitted.** How each figure was obtained, over what population, in what period.
- **P8 Cost and stopping.** Measurement must not perturb what it measures, and its cost is accounted against the work it observes.
- **P9 Authority required.** Read records and outcomes. No write anywhere else.
- **P10 Escalation.** A metric diverging from its target for longer than a stated period; loss of the baseline.
