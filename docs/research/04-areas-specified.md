# iFlow — The Areas, Specified

**Status:** approved, provisional
**Version:** 1.0 — 2026-09-02
**Schema:** [03-schema.md](./03-schema.md) — parameters P1–P10
**Areas:** [02-areas.md](./02-areas.md)

Each area is stated against the ten parameters. Object names are used consistently across areas; the object catalogue itself is a separate document.

---

## Part I — The path a change travels

### 1. Intent and Criteria

- **P1 Inputs.** `Intent`, `EstateModel`.
- **P2 Outputs.** `Specification`, containing `AcceptanceCriteria`, `TerminationCondition` and `Scope`.
- **P3 Decision rule.** A specification is complete when every criterion is either decidable by machine or explicitly marked as requiring a person.
- **P4 Completion.** Completeness reached, or ambiguity declared irreducible.
- **P5 Invariant.** Criteria are fixed before execution and are not altered by it.
- **P6 Failure semantics.** An intent that cannot be expressed as criteria is a failure of this area, not a poor specification passed downstream.
- **P7 Evidence emitted.** What each criterion is grounded in, and who affirmed it.
- **P8 Cost and stopping.** Clarifying exchanges with a person; stop when further exchange stops raising decidability.
- **P9 Authority required.** Read the estate model. No write.
- **P10 Escalation.** An undecidable criterion, or a contradiction between criteria.

### 2. Work Formation

- **P1 Inputs.** `Specification`, `EstateModel`.
- **P2 Outputs.** A set of `WorkUnit`, each carrying `AreaOfEffect`, inherited `AcceptanceCriteria` and `Scope`.
- **P3 Decision rule.** Divide until each unit's area of effect is computable and its criteria are decidable within it; do not divide past the point where criteria cease to be verifiable.
- **P4 Completion.** The units together cover the specification, and none exceeds the size at which completion probability falls below the declared bound.
- **P5 Invariant.** The union of the units' criteria implies the specification's criteria — nothing acceptable is lost in the division.
- **P6 Failure semantics.** A specification that cannot be covered by units with computable areas of effect returns to area 1; it is not passed on in parts.
- **P7 Evidence emitted.** The derivation of each unit's boundary and area of effect from the estate model.
- **P8 Cost and stopping.** Estate queries and analysis per unit; stop when further division stops reducing area of effect.
- **P9 Authority required.** Read the estate model. No write.
- **P10 Escalation.** Coverage incomplete, or an area of effect that cannot be bounded.

### 3. Execution

- **P1 Inputs.** `WorkUnit`, `ContextBundle`, `Grant`.
- **P2 Outputs.** `Candidate`, `Trace`.
- **P3 Decision rule.** Act within `Scope` and stop on `TerminationCondition`; produce a candidate, never a change to the live system.
- **P4 Completion.** A candidate exists, or the termination condition is met, or the step budget is exhausted.
- **P5 Invariant.** No effect outside the declared `Scope` and `Grant`; the estate itself is not modified.
- **P6 Failure semantics.** Substrate failure — tool error, capacity exhaustion, timeout — is distinguished from task failure, the inability to satisfy the criteria. The two are retried differently and only the second is informative about the work.
- **P7 Evidence emitted.** `Trace`, together with the identity and version of the executor at the time of the run.
- **P8 Cost and stopping.** Tokens, steps, tool calls, elapsed time; a stopping rule for non-convergence.
- **P9 Authority required.** Exactly the `Grant` bound to the work unit, enforced outside the agent.
- **P10 Escalation.** Repeated substrate failure, or task failure where the criteria were judged achievable.

### 4. Assurance

- **P1 Inputs.** `Candidate`, `Specification`, `EstateModel`, `Trace`.
- **P2 Outputs.** `Verdict`, with the `Evidence` supporting it.
- **P3 Decision rule.** Accept only when every criterion is supported by evidence produced independently of the executor; otherwise reject or leave undecided.
- **P4 Completion.** A verdict exists for every criterion in the specification.
- **P5 Invariant.** Evidence is not produced by the agent that produced the candidate.
- **P6 Failure semantics.** Inability to obtain evidence is an *undecided* verdict, not a rejection, and the two must not be conflated.
- **P7 Evidence emitted.** The evidence itself, and how each item was obtained.
- **P8 Cost and stopping.** Verification runs; stop when the cost of assurance exceeds the value of the change, which is itself a decision that must be recorded.
- **P9 Authority required.** Execute tests and analyses in isolated environments. No write to the live system.
- **P10 Escalation.** An undecided verdict, or a criterion marked as requiring a person.

### 5. Landing

- **P1 Inputs.** Accepted `Candidate`s, their `AreaOfEffect`s, `EstateModel` including work still in flight.
- **P2 Outputs.** `LandingPlan`, `Conflict`.
- **P3 Decision rule.** Two units conflict when their areas of effect intersect such that either's evidence is invalidated; conflicting units are ordered rather than admitted together.
- **P4 Completion.** Every accepted candidate has landed, been held, or been reversed.
- **P5 Invariant.** Nothing lands while the evidence supporting it has expired or been invalidated by another landing.
- **P6 Failure semantics.** Joint incorrectness discovered after landing is a failure of this area, not of assurance.
- **P7 Evidence emitted.** The conflict analysis and the resulting order.
- **P8 Cost and stopping.** Work held waiting, and re-verification after invalidation.
- **P9 Authority required.** Write to the live system, narrowly and per target.
- **P10 Escalation.** A conflict that cannot be resolved by ordering; a reversal that fails.

---

## Part II — Foundations

### 6. Estate Representation

- **P1 Inputs.** Code, configuration, version history, build and deployment records, runtime telemetry, `Lesson`.
- **P2 Outputs.** `EstateModel`; answers to queries, chief among them `AreaOfEffect`.
- **P3 Decision rule.** A statement enters the model with its provenance and confidence; where a derived statement and an asserted one conflict, the derived one prevails.
- **P4 Completion.** Never complete. Measured by freshness, not by coverage.
- **P5 Invariant.** Every statement carries provenance, confidence and validity. Nothing is served as fact without a source.
- **P6 Failure semantics.** Staleness is detected and reported; serving a stale answer as current is the failure mode of this area.
- **P7 Evidence emitted.** For every answer, what confirms it.
- **P8 Cost and stopping.** Indexing and re-derivation, incremental rather than whole-estate.
- **P9 Authority required.** Read across artifacts and telemetry. No write.
- **P10 Escalation.** A question that cannot be answered to the confidence the asking decision requires.

### 7. Record

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

### 8. Economy

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

### 9. Accumulation

- **P1 Inputs.** `Trace`, `Verdict`, `Conflict`, and decisions made by people during escalation.
- **P2 Outputs.** `Lesson`, bound to places in the estate.
- **P3 Decision rule.** Capture only as a byproduct of a decision already being made; never as a separate request for someone to write something down.
- **P4 Completion.** Continuous.
- **P5 Invariant.** A lesson carries what it was drawn from and expires with it, and never outranks a statement derived from current code.
- **P6 Failure semantics.** An unfalsifiable lesson is worse than none and is refused at capture.
- **P7 Evidence emitted.** The decision the lesson was drawn from.
- **P8 Cost and stopping.** Measured as the cost of a second task in the same area relative to the first.
- **P9 Authority required.** Read traces and verdicts; write only to the lesson store.
- **P10 Escalation.** None. Accumulation must never block work.

### 10. Human Boundary

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

### 11. Authority

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
