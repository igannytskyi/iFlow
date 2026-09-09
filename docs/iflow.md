# iFlow

**Status:** research, provisional
**Version:** 1.3 — 2026-09-09

A single document. It supersedes the eight it was assembled from; the git history holds those.

---

# 1. Goal

> To make the development and maintenance of software an activity carried out at any scale by autonomous agents: a person contributes the intent and the criteria of an acceptable result, is free not to take part in how it is achieved, and is able to verify it at will; and the volume of such work is limited only by the resources one is willing to spend on it.

Restated in clauses, cited throughout as the ground of every derivation:

- **(a)** the work is carried out at any scale by autonomous agents;
- **(b)** a person contributes the intent and the criteria of an acceptable result;
- **(c)** that person is free not to take part in how the result is achieved;
- **(d)** that person is able to verify it at will;
- **(e)** the volume of work is limited only by the resources one is willing to spend.

---

# 2. Research framework

**Object of study.** The process by which an organization creates and changes software systems whose scale and heterogeneity exceed what any individual participant can comprehend.

**Subject of study.** The conditions and mechanisms under which that process can be carried out by autonomous agents while intent, criteria and the possibility of verification remain with a person.

**Hypothesis.** A change can be accepted without a person who understands the system **if and only if** the criteria of acceptability are stated before execution and conformance to them is established by evidence produced independently of the executor. Each condition is load-bearing: criteria stated after the fact describe a result rather than judge it, and evidence produced by an executor about its own work establishes nothing.

**Methods.** Derivation of requirements from the goal; comparative analysis of existing systems and of the practice of large-scale automated change; testing the derived structure against documented industry failure modes; end-to-end runs of a single change through the whole structure, whose purpose is to break it.

**Claim to novelty.** Three positions, none of which is current practice. Acceptance without a reader is treated as the governing constraint rather than as a downstream quality concern. Knowledge of the system is divided by *derivability from artifacts* rather than by document type, and the non-derivable remainder is treated as harvestable only as a byproduct of decisions people are already making. The boundary of necessary human participation is treated as a computed and contracting quantity rather than as a fixed configuration.

**Practical significance.** The framework separates the parts of the problem that platform vendors are commoditizing, and which should therefore be consumed rather than built, from the parts that remain specific to how an organization defines *correct* and *done*, and which must therefore be built.

---

# 3. How this is described

## 3.1 Requirements on any object

An object is anything that passes from one area to another. Without every one of these it cannot be handed over mechanically.

| | Requirement | Meaning | Why |
|---|---|---|---|
| **O1** | Identity | A stable identifier surviving re-creation, citable from a record or a decision | Otherwise a decision cannot be tied to what it was made about |
| **O2** | Schema | Fixed structure, not free text | An area whose output is prose does not compose with the next |
| **O3** | Provenance | Derived from an artifact, asserted by a person, or produced by an agent | Distinguishes the established from the supposed |
| **O4** | Confidence | A stated degree of certainty, on every object | A supposition presented as fact silently corrupts everything downstream |
| **O5** | Validity | Until when it holds, and what invalidates it | The substrate is non-stationary; evidence has a shelf life |
| **O6** | Ownership | The place in the estate it pertains to | So it surfaces where it applies and expires with what it describes |

O1, O3, O4 and O6 apply universally and are not restated per object in §4.

## 3.2 Parameters of any area

Ten. Without any one of them an area cannot run unattended.

| | Parameter | What must be stated | From |
|---|---|---|---|
| **P1** | Inputs | Which objects are consumed, and whence | (a), (c) — an unnamed input is supplied by a person |
| **P2** | Outputs | Which objects are produced, in what form | (a) — an output that is not an object cannot be consumed |
| **P3** | Decision rule | What decision is made here, by what rule | (c) — a decision without a rule requires a person |
| **P4** | Completion | When work on an item is finished, including when it stops without success | (b) — criteria cover termination, not only acceptance |
| **P5** | Invariant | What holds throughout, regardless of outcome | This is what is checked, as against what is declared |
| **P6** | Failure semantics | How the area fails, and how failure differs from a wrong result | (a) — automation without defined failure yields silent corruption instead of an error |
| **P7** | Evidence emitted | What it leaves so its own work can be checked later | (d) |
| **P8** | Cost and stopping | What is consumed, how measured, when cut off | (e) |
| **P9** | Authority required | Which permissions its actions need | (c) |
| **P10** | Escalation | When it must draw in a person | (c), (d) |

## 3.3 The test

> **The output of an area is consumed by the next with no person in between.**

If a person is needed to carry a result across a boundary, the output is not an object with a schema, and no automation inside the areas repairs that.

Conversation, prose, dashboards and reports are therefore **not objects**. They may be rendered *from* objects; nothing passes between areas in that form.

---

# 4. Objects

## Intent and specification

| Object | What it is | Schema | Validity |
|---|---|---|---|
| **`Intent`** | A person's statement of what is to be achieved. The only object originating outside the system | statement; requester; **the authority that may decide it against a competing intent**; priority; deadline | Until satisfied or withdrawn |
| **`ChangeClass`** | A category of change defined by *how its acceptance is decided*, not by what it touches. See §5 | name; decidability; admissible evidence kinds; required evidence set; default scope shape; default budget profile | Long-lived; revised only by area 13 on measurement |
| **`Criterion`** | One condition on an acceptable result | statement; decision procedure (machine or human); required evidence kind; threshold | Fixed with its specification |
| **`AcceptanceCriteria`** | The criteria for one specification | criteria; completeness marker | Fixed with its specification |
| **`TerminationCondition`** | When work stops without acceptance | condition; action on trigger | Fixed with its specification |
| **`Scope`** | The declared region a change may touch | included; excluded; class default; **disjointness from the instrument** — the tests, schemas, telemetry definitions and criteria by which the change will be judged | Fixed with its specification |
| **`Specification`** | The complete statement of a change to be made and judged | `Intent`; **default** `ChangeClass`; `AcceptanceCriteria`; `TerminationCondition`; `Scope` | **Immutable after admission.** A changed intent produces a new specification, never an edit |

## The estate

| Object | What it is | Schema | Validity |
|---|---|---|---|
| **`Statement`** | One assertion about the estate; the atom of the model | subject; relation; object; source artifact; derivation method | Until the source artifact changes — validity is per statement, not per model |
| **`EstateModel`** | The body of statements and the queries answerable over it. Not a document | statements; query interface; freshness per region; observational adequacy per region; **reachability** — which consumers no change to the estate can reach | Never wholly valid or wholly stale; measured by region |
| **`AreaOfEffect`** | The region a change can affect | node set; derivation; computed-at; estate version | **Short.** Invalidated by any landing intersecting it |
| **`Testimony`** | A claim held on a person's word rather than derived, drawn from a decision they were already making | statement; the decision it came from; bound places; falsifier; expiry | Expires with what it was drawn from; never outranks a statement derived from current code |

## Work

| Object | What it is | Schema | Validity |
|---|---|---|---|
| **`WorkUnit`** | A bounded, executable piece of work with its own criteria | specification ref; **its own `ChangeClass`**; `Scope`; `AreaOfEffect`; inherited criteria; dependencies; size estimate | Until admitted, or until its area of effect is invalidated while waiting |
| **`ChangePlan`** | The ordered phases realising one specification where they cannot all land at once | phases; unit membership; wait conditions, including waits on observation; **criterion that every intermediate state is a valid, shippable system**; rollback position per phase; **disposal of residue on abandonment** | Until every phase lands or the plan is abandoned; abandonment leaves the system at a named intermediate state, never mid-phase, and **never leaves that state unowned** |
| **`ContextBundle`** | The knowledge supplied to an executor for one unit | statements included; selection rule; estate version; budget consumed | One execution only; never reused |
| **`AdmissionDecision`** | The decision to commit resources, or not | unit; outcome (admitted, held, refused); deciding condition; **the confidence of the estate edges it rested on**; decided-at | Held decisions expire into escalation at a stated age |
| **`Grant`** | Permissions bound to one unit | unit; permitted operations; targets; credential reference; expiry | Expires with the run; never renewed by the agent holding it |
| **`Conflict`** | A detected interference between two units | units; intersecting region; the evidence it would invalidate | Until one of the two terminates |

## Result and evidence

| Object | What it is | Schema | Validity |
|---|---|---|---|
| **`Candidate`** | A proposed change; never applied by what produced it | unit; artifact set; executor identity and version; produced-at | Until its supporting evidence expires |
| **`Trace`** | The record of one run | run id; steps; tool calls; input and output digests; executor version; timestamps | Retained for the lifetime of the decisions it supports. Append-only; not writable by its subject |
| **`Evidence`** | An artifact supporting one claim about a candidate **or about a transformation** | claim; **subject (candidate or transformation)**; kind (test run, static analysis, runtime observation, human affirmation, proof of a transformation's property); producer; **independence from the executor, and how established**; obtained-at | **Has a shelf life**, bound to the estate version and executor version it was obtained against |
| **`Verdict`** | The acceptance decision on a candidate | candidate; per-criterion outcome (met, failed, **undecided**); evidence refs; overall; decided-by; **state (settled or deferred)**; observation window and baseline where deferred | Until invalidated by a landing touching its area of effect. A verdict that never closes is a failure, not a permanent state |
| **`LandingPlan`** | The order in which accepted candidates enter | ordered entries; expected invalidations; re-establishment required before each | One landing cycle |

## Governance

| Object | What it is | Schema | Validity |
|---|---|---|---|
| **`Budget`** | The allowance against which work is admitted | owner; period; limit; consumed; weighting by class | One accounting period |
| **`CostRecord`** | Consumption attributable to one unit | unit; tokens; steps; elapsed; money; attributed-to | Permanent |
| **`Escalation`** | A request for human involvement, with its ground | origin area; ground; unit; raised-at; respondent; resolution; **behaviour on no response** | Expires; expiry is itself a recorded outcome |
| **`Baseline`** | The present way of working, measured on the same workload | workload description; period; values; method | Re-established when the workload changes materially |
| **`Metric`** | One measured figure | name; population; `ChangeClass`; period; value; method | One period |

## Invariants across objects

1. **A `Specification` is immutable after admission.** Otherwise criteria are shaped by what execution turned out to produce, and the hypothesis fails at its first condition.
2. **A `Candidate` is never applied by the area that produced it.** Execution proposes; landing disposes.
3. **`Evidence` records its independence from the executor and how that is established.** Evidence that cannot state this does not count toward a verdict.
4. **A `Trace` is not writable by its subject.** A record an agent can edit is not a record.
5. **Evidence about a transformation is amortized across its applications.** Establishing a property of a transformation once, rather than of each candidate, is what stops assurance cost scaling with volume. Any edit to the transformation invalidates every verdict resting on it.
6. **A deferred `Verdict` lives inside a reversibility horizon.** Reversibility is not a property a change has or lacks; it shortens as other work builds on the change. A deferred verdict is admissible only while its observation window fits inside that horizon. Where the horizon expires first, a person decides at that moment — never quietly abandoned, never extended past the point of no return.
7. **`Scope` and the instrument are disjoint.** Whatever will judge a change — its tests, its schemas, its telemetry definitions, its criteria — lies outside the region that change may touch. A `Grant` permitting an executor to write to its own instrument is malformed, and evidence gathered through an instrument the executor could reach establishes nothing.
8. **A change to the instrument is a separate unit, accepted separately, and never by the unit that depends on it.** Some changes legitimately require the instrument to move; that move is itself work, with its own criteria and its own acceptance. Allowing one unit to widen its instrument and then pass through it is invariant 3 evaded rather than satisfied.

---

# 5. Change classes

A class is defined by **how the acceptance of a change is decided** — not by what it touches, how large it is, or who asked. Two changes in the same file belong to different classes if one can be accepted on evidence and the other cannot.

A class is assigned **per `WorkUnit`**; the `Specification` carries only a default. One intent legitimately spans several: a contract change is C1 for the producer's existing callers, C3 per consumer, C4 while old-shape traffic drains, and C5 at the residue.

Acceptance requires an **oracle** — something that can pronounce on a result independently of what produced it. Classes are ordered by which oracle decides them.

| | Class | The claim | Oracle | Decidability |
|---|---|---|---|---|
| **C1** | Behaviour-preserving | Nothing observable changed | The prior system itself | Full, given observational adequacy |
| **C2** | Defect repair | This wrong behaviour is now right, and nothing else changed | A reproduction, plus C1's oracle for the rest | Full, given a reproduction exists |
| **C3** | Contract-bounded | The system now satisfies this stated contract | The contract — types, schema, interface, property, policy | Full within the contract |
| **C4** | Observable-effect | The deployed system behaves better against a measured quantity | Production observation against a baseline | Partial, and **delayed** |
| **C5** | Judgment-bound | This is what was wanted | A person | None, by construction |

**C1 — Behaviour-preserving.** Dependency and version upgrades, framework migrations, mechanical refactoring, dead-code removal, moves and renames. Two sub-modes: **proved**, where the transformation is behaviour-preserving by construction and acceptance costs nothing because nothing need be run; and **tested**, where behaviour is compared before and after.

The proof rests on an equivalence claim, and **that claim must be recorded with its provenance.** Where it is derived — the compiler establishes it, the transformation is total over the semantic tree — the class holds. Where it comes from documentation or a person it is `Testimony`, and **an equivalence claim resting on testimony demotes the change to C1 tested.** Otherwise the strongest guarantee in the catalogue rests on an unexamined assertion that no downstream oracle can catch, because the proof is what replaced the oracle.

The binding difficulty in C1 is never the oracle but **observational adequacy**. A second limit is inherent: where a language permits reflection or string-formed invocation, the set of call sites is not statically decidable. Criteria are written to what is decidable, with the residue stated, never to what merely sounds complete.

**C2 — Defect repair.** Two halves, both needing evidence: the reported behaviour is corrected, and nothing else moved — the second half is C1. Decidability is conditional on a reproduction, whose own acceptance is decidable: the test must fail on the unmodified system, and fail for the stated reason rather than incidentally. **A defect without a reproduction is not a C2 change; it is a request for a reproduction, followed by one.**

**C3 — Contract-bounded.** The criterion is an explicit statement the result must satisfy. What remains undecided is whether the contract was the right one; that residue belongs to area 1, where the criterion was written, and must not be smuggled into area 5 as though assurance could settle it.

**C4 — Observable-effect.** The quantity does not exist until the change is exposed, so acceptance splits: before landing, only safety; after landing, effect against a baseline, completing or reversing the verdict. **C4 requires reversibility.** An irreversible change whose acceptance depends on observed effect is not C4 — it is C5, and needs a person before it lands.

Two further constraints, and the second bounds concurrency itself. The observation window must fit not only inside the reversibility horizon but inside the **decay of its own baseline**: a baseline measured before forty other changes landed no longer describes a world without this one. And an observed effect is attributable to a particular change only where that change is the sole variable in its area of effect, or where a comparison group exists. **Without isolation or a control, a C4 verdict cannot close positively** — it closes undecided and escalates. This is the first place where concurrency is limited by the class rather than by resources: two C4 changes in the same area of effect cannot run at once at any budget.

**C5 — Judgment-bound.** New user-facing behaviour, product decisions, anything whose criterion is desirability. No oracle exists and none can be built. The task is not to decide but to **reduce what must be judged**: establish everything establishable, hand the person a bounded decision rather than a diff, record the decision as testimony. **C5 does not become automatable. It becomes cheaper to judge.**

## Modifiers

These cut across the classes and must not be turned into any.

**Observational adequacy** — whether behaviour in a region can be pinned down. Unknown by default in a brownfield estate, and it decides whether C1 and C2 are decidable *here* rather than in principle.

**Reachability** — whether every consumer of a changed contract can be changed at all. Shipped applications, third parties, anything already in someone else's hands. **A contract change with an unreachable consumer cannot complete without a human decision, and that is knowable at area 1, before any work is done.**

**Blast radius** — how far effects travel before anything detects them. Governs admission, not acceptance.

**Reversibility** — whether a change can be withdrawn cheaply, and **for how long**; the horizon shortens as other work builds on it.

## Order of capability

**C1 proved → C1 tested → C2 → C3 → C4 → C5.** Each oracle is weaker than the last. The first is where the strongest guarantees and the existing industrial practice are. The last never arrives and should not be aimed at.

---

# 6. The areas

Thirteen. Areas 1–6 are sequential — the path a change travels. Areas 7–13 are not steps at any point on that path; they hold across all of it at once. Each is derived from a clause of the goal, and stated as object, subject, aim, criterion of resolution, then P1–P10.

## Part I — the path a change travels

### 1. Intent and Criteria — *from (b)*

**Object.** The act of stating what is to be changed and what result would be acceptable. **Subject.** The form and completeness of that statement, sufficient for execution and judgement without further participation by its author. **Aim.** A representation of intent and criteria fit for machine execution and machine judgement.
**Criterion of resolution.** An executor raises no questions against the specification, and a judge decides without consulting its author.

- **P1** `Intent`, `EstateModel`, the catalogue of `ChangeClass`.
- **P2** `Specification`, containing a default `ChangeClass`, `AcceptanceCriteria`, `TerminationCondition`, `Scope`.
- **P3** A specification is complete when every criterion is either decidable by machine or explicitly marked as requiring a person. What is decidable is a property of the class, not of the individual intent.
- **P4** Completeness reached, or ambiguity declared irreducible.
- **P5** Criteria are fixed before execution and are not altered by it.
- **P6** An intent that cannot be expressed as criteria is a failure of this area, not a poor specification passed downstream. Criteria that admit two materially different acceptable results are incomplete, and the discovery of that fact anywhere downstream returns here.
- **P7** What each criterion is grounded in, and who affirmed it.
- **P8** Clarifying exchanges with a person; stop when further exchange stops raising decidability.
- **P9** Read the estate model. No write.
- **P10** An undecidable criterion, or a contradiction between criteria.

### 2. Work Formation — *from (a), at any scale*

**Object.** The conversion of an intent into units of work within a system spanning many components. **Subject.** The dependence of a unit's executability and reliability on its boundaries and size. **Aim.** A rule by which units with a known area of effect are derived from an intent and knowledge of the estate.
**Criterion of resolution.** For a typical intent the plan is obtained reproducibly without a person, and covers the specification.

- **P1** `Specification`, `EstateModel`.
- **P2** A `ChangePlan` over a set of `WorkUnit`, each with its own `ChangeClass`, `AreaOfEffect`, inherited criteria and `Scope`. Where phases must be separated in time, the plan carries the wait conditions and the requirement that every intermediate state is a valid, shippable system — a criterion belonging to no single unit. Where a specification is not decidable as it stands, the plan includes **preparatory units** — a reproduction, characterization of behaviour in an inadequately observed region — whose own acceptance is decidable and on which the original unit depends.
- **P3** Divide until each unit's area of effect is computable and its criteria decidable within it; do not divide past the point where criteria cease to be verifiable.
- **P4** The units cover the specification, and none exceeds the size at which completion probability falls below the declared bound.
- **P5** The union of unit criteria implies the specification's criteria — **but only where the estate query producing the scope was complete**; where completeness is not derivable, the residue is stated on the specification rather than assumed away.
- **P6** A specification that cannot be covered returns to area 1; it is not passed on in parts. Emitting a unit that cannot be accepted, where a preparatory unit would have made it acceptable, is a failure of this area.
- **P7** The derivation of each unit's boundary and area of effect.
- **P8** Estate queries per unit; stop when further division stops reducing area of effect.
- **P9** Read the estate model. No write.
- **P10** Coverage incomplete, or an area of effect that cannot be bounded.

### 3. Admission — *from (a) and (e)*

**Object.** The decision to start work on a unit, or not. **Subject.** The conditions that must hold before resources are committed. **Aim.** A single gate at which conflict, allowance and permission are settled together, before execution rather than after it.
**Criterion of resolution.** Work that could not have been accepted is not started, and the reason it was not started is recorded.

*Why separate.* Conflict detected after execution has already been paid for. Three foundations — landing, economy and authority — meet the path at exactly one point, and that point is a decision to commit resources.

- **P1** Pending `WorkUnit`s with their `AreaOfEffect`s, `EstateModel` including work in flight, `Budget`, policy.
- **P2** `AdmissionDecision`, `Grant`, `Conflict`.
- **P3** A unit is admitted when its area of effect does not intersect an unfinished unit's such that either's evidence would be invalidated, an allocation exists, and a grant no wider than its `Scope` can be issued. Failing any of the three it is held, not started. **A cycle among held units is detected structurally and immediately, not discovered by timeout**: age-based expiry is for contention, a cycle is a defect.
- **P4** Every pending unit admitted, held with a stated reason, or refused.
- **P5** No resources are committed to a unit that could not have been accepted had it succeeded.
- **P6** Admitting a unit later found to be in conflict is a failure of this area, not of landing. Holding one that could have run is a lesser failure, and must be visible as queueing rather than as silence.
- **P7** Which of the three conditions decided it, against what, and **at what confidence** — a conflict found through a matched contract edge is only as certain as that edge.
- **P8** The gate must cost far less than the work it withholds; a unit held past a stated age escalates rather than waiting indefinitely.
- **P9** Issue grants within policy; refuse and hold work. No ability to widen policy.
- **P10** A unit held past its age limit; a conflict no ordering resolves, which escalates **to an authority over both intents, whose absence is itself a finding reported here rather than at the point of deadlock**; a grant wider than policy allows.

### 4. Execution — *from (a), by autonomous agents*

**Object.** The work of an executor on one unit. An autonomous agent is one kind of executor; a deterministic transformation is another, preferred wherever the class admits it. **Subject.** The behaviour of execution on a substrate that is unreliable and non-stationary. **Aim.** Execution whose failures are detectable and whose results remain comparable across versions of the executor.
**Criterion of resolution.** A failure of execution is distinguishable from a failure of the intent, and every result carries what produced it and when.

- **P1** `WorkUnit`, `ContextBundle`, `Grant` and `AdmissionDecision`.
- **P2** `Candidate`, `Trace`.
- **P3** Act within `Scope`, stop on `TerminationCondition`, produce a candidate and never a change to the live system. **Where the class permits a deterministic transformation, using an agent instead is a defect** — it makes a reproducible result unreproducible and costs more.
- **P4** A candidate exists, the termination condition is met, the step budget is exhausted, or the unit is **cancelled** — a terminal state distinct from failure, reached when the intent behind it is withdrawn. Cancelled work is still charged: area 9 forbids unattributable spend regardless of why the work stopped.
- **P5** No effect outside the declared `Scope` and `Grant`; the estate itself is not modified.
- **P6** Substrate failure — tool error, capacity exhaustion, timeout — is distinguished from task failure, the inability to satisfy the criteria. They are retried differently and only the second is informative about the work.
- **P7** `Trace`, with the identity and version of the executor at the time of the run.
- **P8** Tokens, steps, tool calls, elapsed time; a stopping rule for non-convergence. **A budget exhaustion is diagnosed before it is retried**, or the system pays repeatedly for a missing input.
- **P9** Exactly the `Grant`, enforced outside the agent.
- **P10** Repeated substrate failure, or task failure where the criteria were judged achievable. **Two executors producing different candidates that both satisfy the criteria is not adjudicated here**: it is evidence that the criteria under-determine the result, and it escalates to area 1. Running a unit twice on purpose is therefore a cheap probe of criteria completeness.

### 5. Assurance — *from (b) and (c)*

**Object.** The establishment of conformance between a result and its criteria. **Subject.** The composition and sufficiency of the evidence that replaces a person reading the result. **Aim.** An acceptance decision reached on evidence.
**Criterion of resolution.** The share of changes accepted without human reading is measurable, and the share wrongly accepted is bounded and observable.

- **P1** `Candidate`, `Specification`, `EstateModel`, `Trace`.
- **P2** `Verdict`, with the `Evidence` supporting it.
- **P3** Accept only when every criterion is supported by evidence produced independently of the executor; otherwise reject or leave undecided. Which evidence suffices is fixed per `ChangeClass` in advance, not chosen per candidate.
- **P4** A verdict exists for every criterion. A verdict on an effect that does not exist before exposure is **deferred rather than absent**: opened here with its observation window and baseline, closed by area 13 after landing.
- **P5** Evidence is not produced by the agent that produced the candidate.
- **P6** Inability to obtain evidence is an **undecided** verdict, not a rejection; the two must not be conflated.
- **P7** The evidence itself, and how each item was obtained.
- **P8** Verification runs; stop when the cost of assurance exceeds the value of the change — a decision that is itself recorded.
- **P9** Tests and analyses in isolated environments, and **read production observations**, without which no C4 criterion is decidable. No write anywhere.
- **P10** An undecided verdict, or a criterion marked as requiring a person.

### 6. Landing — *from (a), since scale means simultaneity*

**Object.** The entry of accepted changes into a system in operation. **Subject.** Whether evidence gathered before entry still holds at the moment of entry. **Aim.** Entry under which nothing takes effect on evidence another entry has already invalidated.
**Criterion of resolution.** Joint incorrectness in production is traceable to a specific invalidation that was missed.

*Conflict detection is not here.* It is area 3, because it must precede the commitment of resources.

- **P1** Accepted `Candidate`s with their `Verdict`s and `Evidence`, `EstateModel`.
- **P2** `LandingPlan`, and the landed change. **A reversal is a change**: it carries its own criteria, evidence and plan, and is not a privileged instant operation exempt from them.
- **P3** A candidate enters only while its supporting evidence is valid; where an earlier entry invalidated it, the evidence is re-established before entry rather than the candidate dropped.
- **P4** Every accepted candidate has entered, awaits re-establishment, or has been reversed.
- **P5** Nothing takes effect on expired or invalidated evidence. A candidate carrying a deferred verdict enters only while it remains reversible, and remains reversible until that verdict closes.
- **P6** Joint incorrectness in production is a failure of this area. Conflict that should have prevented the work from starting is a failure of area 3. **A partial failure leaves the system in a state no plan declared valid**; that state halts every further entry within its area of effect until it is resolved, and the halt is the failure's first consequence rather than a decision someone makes later.
- **P7** What each entry invalidated, and what was re-established before the next.
- **P8** Re-verification after invalidation, and work waiting on it.
- **P9** Write to the live system, narrowly and per target.
- **P10** Evidence that cannot be re-established; a reversal that fails.

## Part II — foundations

### 7. Estate Representation — *from (a), autonomy*

**Object.** An organization's knowledge of its own software systems. **Subject.** The provenance and trustworthiness of that knowledge where the system's self-description is unreliable. **Aim.** A representation answering questions on demand, each answer carrying its provenance and confidence.
**Criterion of resolution.** *What does this change affect* is answered together with what confirms the answer.

- **P1** Code, configuration, version history, build and deployment records, runtime telemetry, `Testimony`, **and the outcomes of landings that contradicted their own predictions**.
- **P2** `EstateModel`; answers to queries, chief among them `AreaOfEffect`, the observational adequacy of a region, and which consumers lie beyond the reach of any change.
- **P3** A statement enters with its provenance and confidence; where a derived statement and an asserted one conflict, **the derived one prevails**.
- **P4** Never complete. Measured by freshness, not coverage.
- **P5** Every statement carries provenance, confidence and validity. Nothing is served as fact without a source.
- **P6** Serving a stale answer as current is the failure mode of this area. Staleness is detectable from the source; **being wrong is not**, and is corrected only from outside: a landing whose effects fall beyond its predicted `AreaOfEffect` is evidence against the model and must be fed back as a correction, not merely handled as an incident. This is the only mechanism by which a false contract edge is ever removed.
- **P7** For every answer, what confirms it.
- **P8** Indexing and re-derivation, incremental rather than whole-estate.
- **P9** Read across artifacts and telemetry. No write.
- **P10** A question unanswerable to the confidence the asking decision requires.

### 8. Record — *from (d)*

**Object.** The trace left by agent work. **Subject.** The composition of a record sufficient to reconstruct any step after the fact. **Aim.** An always-on record making verification possible at any moment.
**Criterion of resolution.** Any step of any run is reconstructible without recourse to the executor that performed it.

- **P1** Everything every area emits. **P2** `Trace`, and the durable binding between a `Verdict` and its evidence.
- **P3** Record unconditionally; nothing is written selectively on expected interest.
- **P4** A record closes when the work it covers reaches a terminal state.
- **P5** Append-only, and not writable by the agent it describes.
- **P6** **Failure to record is failure of the work**: work that cannot be recorded must not proceed.
- **P7** The record is the evidence; its own integrity must be attestable.
- **P8** Storage and retention, bound to the lifetime of the decisions it supports.
- **P9** Write to the record store only. **P10** Integrity broken or store unavailable.

### 9. Economy — *from (e)*

**Object.** The consumption of resources by agent work. **Subject.** The mechanism bounding consumption against unbounded demand. **Aim.** Volume governed by declared limits rather than reported after the fact.
**Criterion of resolution.** Exceeding a limit is prevented before the spend, not discovered after it.

- **P1** `Budget`, `CostRecord`, pending units, the priority of the intents behind them, **and the capacity of the people escalations are served by**. **P2** Admission decisions, allocations, stop signals.
- **P3** Work is admitted only against an existing allocation; exceeding a limit blocks before the spend.
- **P4** Continuous; settled per accounting period.
- **P5** No work runs without an allocation attributable to it.
- **P6** Unattributable spend is a failure — an agent whose cost cannot be attributed must not run.
- **P7** Cost per unit of verified change, by class, **counting human time alongside machine resources** — the goal names attention as the scarce resource, so accounting that omits it measures the wrong constraint. An escalation that cannot be served within its window forces an explicit, recorded choice between stopping the work and proceeding at a stated lower assurance; it never defaults.
- **P8** The accounting must be negligible against what it governs.
- **P9** Refuse and stop work. No ability to alter budgets. **P10** Exhaustion against work classified as mandatory.

### 10. Accumulation — *from (e)*

**Object.** The carrying of knowledge from one task to the next. **Subject.** The conditions under which what has been accumulated remains true. **Aim.** Reduced repayment of orientation cost, without forming a second corpus that decays as documentation decays.
**Criterion of resolution.** A second task in the same area costs less than the first, and accumulated statements do not diverge from the system undetected.

- **P1** `Trace`, `Verdict`, `Conflict`, decisions made by people during escalation. **P2** `Testimony`, bound to places in the estate.
- **P3** Capture only as a byproduct of a decision already being made; never as a separate request for someone to write something down.
- **P4** Continuous.
- **P5** Testimony carries what it was drawn from, expires with it, and never outranks a statement derived from current code.
- **P6** Unfalsifiable testimony is worse than none and is refused at capture.
- **P7** The decision it was drawn from. **P8** The cost of a second task in the same area relative to the first.
- **P9** Read traces and verdicts; write only to the testimony store. **P10** None. Accumulation must never block work.

### 11. Human Boundary — *from (c) and (d)*

**Object.** Human participation in agent work. **Subject.** The rule determining when it is necessary. **Aim.** Participation only where required, with that region contracting as evidence accumulates.
**Criterion of resolution.** Participation per unit of change is measured and declining, and cases where it was required but did not occur are detectable.

- **P1** `Escalation` from any area; historical verdicts and their outcomes. **P2** Routing to a person, and revision of the escalation rules themselves.
- **P3** A person is drawn in when a decision is undecidable on evidence, when authority is required, or when the consequence exceeds a declared threshold.
- **P4** Every escalation resolved or expired, expiry being itself a recorded outcome.
- **P5** A case that met the escalation rule and did not reach a person is detectable afterwards.
- **P6** **Silent non-escalation is the failure mode of this area**; late escalation is the lesser one.
- **P7** Participation per unit of change, and its trend. **P8** Human time, accounted as the scarcest resource there is.
- **P9** Interrupt and hold work. **P10** This area is the escalation target; what happens when a person does not respond must itself be defined.

### 12. Authority — *from (c)*

**Object.** The actions an agent performs on real systems and data. **Subject.** Limits on permissible action, declared in advance. **Aim.** Permissions granted before execution and enforced independently of the agent.
**Criterion of resolution.** No action outside the declared limits is performable, and every attempt is recorded.

- **P1** `WorkUnit`, declared policy. **P2** `Grant`, bound to one unit.
- **P3** A grant is the narrowest set sufficient for the declared `Scope`; anything wider is refused rather than warned about.
- **P4** A grant exists before execution starts, or execution does not start.
- **P5** Enforcement lies outside the agent. An agent cannot widen its own grant.
- **P6** A refused action is a normal outcome, reported to the requester rather than retried by another route.
- **P7** Every attempted action against its grant, allowed or refused. **P8** Negligible by requirement; enforcement must not become a throughput constraint.
- **P9** Issue and revoke grants. This is the root of authority and must itself be governed by people. **P10** A request for a grant wider than policy allows.

### 13. Measurement — *from the hypothesis*

**Object.** The observation of the system's own behaviour. **Subject.** The instruments and the baseline against which any claim about the system is checked. **Aim.** Every criterion of resolution in this document rendered into something actually measurable.
**Criterion of resolution.** The hypothesis can be confirmed or refuted on evidence rather than argued.

*Why an area and not a method.* An instrument existing only in the research measures a prototype and then goes away. The claim is about a system in operation, so the system must observe itself.

- **P1** `Trace`, `Verdict`, `CostRecord`, `Escalation`, landed changes and their later outcomes, `Baseline`, **and the extent of the instrument itself** — how many tests, contracts and observations stand behind the criteria. **P2** `Metric` series, per `ChangeClass`.
- **P3** A measurement counts only against a population and a class stated in advance; a figure without both is not published.
- **P4** Continuous, settled per period.
- **P5** The baseline is measured on the same workload as the comparison, otherwise no comparison is made at all.
- **P6** A metric that cannot detect the failure it is meant to detect is worse than none. The governing case: **a wrongly accepted change is by construction unread, so it is discovered only later — from a defect, an incident or a reversal — and the lag between acceptance and discovery is the instrument's resolution.**
- **P7** How each figure was obtained, over what population, in what period. **A rising acceptance rate together with a shrinking instrument, or with a lengthening lag to discovery, is the signature of a system passing its own examinations by making them easier** — the instrument is watched alongside the outcomes, or the outcomes cannot be believed. **P8** Measurement must not perturb what it measures.
- **P9** Read records and outcomes. No write anywhere else. **P10** A metric diverging from target beyond a stated period; loss of the baseline.

---

# 7. Contracts between the areas

P1 and P2 state what each area consumes and produces. Stated instead as boundaries, because a boundary is what fails.

| Object | Produced by | Consumed by | Crosses when |
|---|---|---|---|
| `Intent` | outside the system | 1 | A person states it, with an authority named |
| `Specification` | 1 | 2 | Complete, or ambiguity declared irreducible |
| `ChangePlan`, `WorkUnit` | 2 | 3 | The units cover the specification |
| `AdmissionDecision`, `Grant` | 3 | 4 | Conflict, allowance and permission all settled |
| `ContextBundle` | 7 | 4 | On admission, against a stated estate version |
| `Candidate`, `Trace` | 4 | 5, 8 | A terminal state is reached, of either kind |
| `Verdict`, `Evidence` | 5 | 6, 8, 13 | A verdict exists for every criterion, settled or deferred |
| `LandingPlan`, landed change | 6 | 7, 13 | Evidence is valid at the moment of entry |
| Query answers, `AreaOfEffect` | 7 | 1, 2, 3, 5, 6 | On request, with provenance and confidence |
| `Testimony` | 10 | 7 | A falsifier is present |
| `Escalation` | any | 11 | A ground is stated |
| `CostRecord` | 4, 5, 6 | 9, 13 | On completion of the work it accounts for |
| `Metric` | 13 | 1, 9, 11 | Per period, per class, against a baseline |

Five rules hold at every boundary.

1. **Only objects cross.** Nothing passes as prose, conversation or a rendered view. This is §3.3 stated as a rule rather than a test.
2. **A consumer never reaches back to ask.** If it needs something the crossing did not carry, the producer's output was incomplete — a failure of the producer's completion criterion, not a request for clarification.
3. **A consumer may refuse.** Refusal returns the object to the producer as that producer's failure, and is recorded as such. It is not a negotiation.
4. **Every crossing is recorded**, binding producer, object, consumer and moment. A crossing that was not recorded did not happen, per area 8.
5. **There are no backward edges.** Everything that returns does so through the loop of §8 — asynchronously, as an object with its own validity, never as a synchronous call into an earlier area. An area that can be called back into cannot be reasoned about while work is in flight.

---

# 8. The loop

Areas 1–6 describe the path of one change. They do not describe the system, because the path feeds back into the foundations, and the behaviour of the whole over time is a property of that feedback rather than of the path.

**What returns.** Verdicts and traces become testimony, which enters the estate representation and thereafter shapes work formation and the context supplied to execution. Outcomes of escalations revise the escalation rules themselves, which is what allows the human boundary to contract. Cost records govern admission. Landed changes invalidate statements about the estate and evidence resting on them. **And landings that surprise refute it**: an effect outside the predicted area of effect is the system learning that its own model was wrong rather than merely old.

**Delays.** Every return has a lag. A system ignoring them acts on a picture stale by exactly the length of its own feedback.

**What must converge.** Human participation per unit of change, downward. Cost per unit of verified change, downward or stable. The share of undecided verdicts, downward.

**What can diverge — the failure mode of the loop.** Testimony that shapes criteria without being falsifiable lets the system reinforce its own error: it accepts what it accepted before, for reasons it recorded itself. Two rules hold this open — testimony that is unfalsifiable is refused at capture, and evidence must be independent of the executor.

---

# 9. Validation

Two changes were carried end to end through the whole structure. Their purpose was falsification, and the eleven findings below are already applied above; they are recorded because each marks a place where the obvious design was wrong.

## Run 1 — the easiest class

A deprecated `Clock.nowUtc()` replaced by `Clock.instant()` across 200 repositories. C1 proved: mechanical, behaviour-preserving, no interface change, no production access.

Outcome: **four human touchpoints against two hundred repositories**; 7 units of 200 escaped to a human decision. Cost was dominated by estate queries and verification, not by producing the changes — **for this class generation is nearly free and assurance is the bill.**

- **F1** A proved equivalence claim usually rests on testimony, not derivation; where it does, the change is demoted to C1 tested.
- **F2** Even the easiest class has an undecidable residue: call sites are not statically decidable where reflection exists.
- **F3** The invariant that unit criteria imply specification criteria holds only where the estate query was complete.
- **F4** The executor need not be an agent, and where a deterministic transformation will do, using an agent is a defect.
- **F5** Evidence may attach to a transformation rather than a candidate, amortized across every application. This is what stops assurance cost scaling with volume.

## Run 2 — a change across a contract

A required `currency` field on `POST /orders`: five internal callers, an event carrying the same shape, and a mobile application already installed on users' devices.

Outcome: three human touchpoints, but the shape of the work is entirely different — phased, waiting, and terminating in a decision no system can make. **A C5 decision reached by a C3 route**, and the right outcome: the person is handed a bounded question with everything establishable already established.

- **G1** Class belongs to the work unit, not the specification; one intent spans C1, C3, C4 and C5.
- **G2** Work formation produces a plan, not a set: phases, waits on observation, and the requirement that every intermediate state is shippable.
- **G3** The estate has a boundary; consumers exist that no change reaches, and a contract change with one is knowably incompletable without a person, before any work is done.
- **G4** Reversibility is a horizon that shortens, not a boolean.
- **G5** Conflict detection inherits the confidence of the contract edges it rests on.
- **G6** Assurance needs read access to production; no C4 criterion is decidable in an isolated environment.

## Run 3 — the failure cases

Not changes so much as ways the structure breaks. Each was carried through until it broke something.

**A conflict that ordering cannot resolve.** Two intents require mutually exclusive states of the same interface; neither can go first. Area 3 holds both and escalates at an age limit — but to whom? The requesters of two competing intents are peers, and neither can decide against the other.

- **U1** An `Intent` carries **the authority that may decide it against a competing intent.** Where two conflicting intents share no such authority, that is a fact about the organization, and it is detectable at admission rather than at the moment of deadlock.
- **U2** A cycle among held units is **detected structurally and immediately.** Age-based expiry is the right instrument for contention and the wrong one for a defect; waiting out a cycle turns a detectable error into a silent stall.

**A reversal that fails midway.** Half of a change is withdrawn and the system stands in a state no plan ever declared valid.

- **U3** **A reversal is a change.** It carries its own criteria, evidence and plan. Treated as a privileged instant operation it is an unverified change to production, which contradicts the hypothesis directly.
- **U4** An unplanned state **halts every further entry within its area of effect**, as the first consequence of the failure rather than as a decision someone makes afterwards.

**Two intents competing for the same person.** Area 9 allocated machine resources; nothing allocated people, though the goal names attention as the scarce one.

- **U5** **Human time is accounted in area 9 with everything else**, or the accounting measures the wrong constraint. An escalation that cannot be served inside its window forces an explicit, recorded choice — stop the work, or proceed at a stated lower assurance. It never defaults.

**An estate region that is wrong rather than stale.** Staleness is detectable from the source. A derivation that is simply wrong — a bad parse, a false contract edge — is not, and confidence expresses uncertainty rather than systematic error.

- **U6** A landing whose effects fall outside its predicted `AreaOfEffect` **is evidence against the estate model** and feeds back as a correction, not merely as an incident. It is the only mechanism by which a false edge is ever removed.

**Executors disagreeing on the same unit.** Two runs, two different candidates, both satisfying the criteria.

- **U7** This is **not adjudicated in execution.** It is evidence that the criteria under-determine the result, and it returns to area 1. Resolving it by running three executors and taking the majority would pick a plausible answer while establishing nothing — precisely what the hypothesis forbids. Inverted, it is useful: running a unit twice on purpose is a cheap probe of whether criteria are complete.

## Run 4 — the instrument, and what is left behind

Three cases. The second is the most dangerous in this document, because it attacks the hypothesis rather than the structure.

**An intent withdrawn while its plan is mid-flight.** Two phases have landed, a third waits, and the requester withdraws. The plan already guarantees the system is left at a named valid state — but valid is not the same as intended. The producer now accepts an optional field nobody will ever send.

- **W1** Abandonment produces residue, and **residue must have an owner**: a withdrawn plan either rolls back to the pre-plan state — itself a change, per U3 — or emits a new intent to dispose of what it left. A system that promises not to leave the estate broken must also promise not to leave it littered, or it becomes a source of exactly the sediment area 7 struggles to describe.
- **W2** **Cancellation is a terminal state distinct from failure.** Units already running stop, their grants expire, and their cost is still attributed: area 9 forbids unattributable spend regardless of why work stopped.

**An executor that satisfies the criterion by changing what the criterion measures.** A failing test is deleted. A symbol is wrapped rather than removed. The telemetry that would have shown old-shape traffic stops being emitted. Every criterion is met and nothing was established.

This is the sharpest attack available on the hypothesis, because evidence independent of the *executor* is no defence once the executor has moved the *instrument*.

- **W3** **The instrument lies outside the scope.** Tests, schemas, telemetry definitions and the criteria themselves are the apparatus by which a change is judged, and a `Grant` letting an executor write to its own apparatus is malformed. This is not a policy to configure; it is a property `Scope` must have by construction.
- **W4** Some changes legitimately require the instrument to move. Then **the move is its own unit, accepted separately, and never by the unit that depends on it.** Otherwise a unit widens its instrument and then passes through it.
- **W5** It is detectable. **A rising acceptance rate together with a shrinking instrument, or a lengthening lag to discovery, is the signature of a system passing its own examinations by making them easier.** Area 13 therefore watches the size of the instrument, not only the outcomes.

**A baseline that shifts under a deferred verdict.** Thirty days of observation, during which forty other changes land and the season turns.

- **W6** The observation window is bounded twice: by the reversibility horizon, and by **the decay of the baseline itself**.
- **W7** An effect is attributable only where the change is the sole variable in its area of effect, or a comparison group exists. **Without isolation or a control, a C4 verdict cannot close positively.** This is the first constraint in which **concurrency is bounded by the class rather than by resources.**

**Still untested.** A criterion that is met correctly and was the wrong criterion — the residue C3 hands back to area 1, which no run has yet exercised. An estate spanning two organizations with different authorities. And the reflexive case: iFlow changing iFlow, where the instrument and the subject are one system.

---

# 10. Ordering, and what to build

**Within the research.** Areas 1, 5 and 11 are defined through one another — criteria, conformance and the human boundary cannot be formulated separately — and are the natural starting point. Area 7 attempted before them degenerates into building a larger index without a statement of what it is for. Area 13 must be in place before any claim of improvement is made, and therefore before implementation rather than after it.

**Against the market — the decision, area by area.**

An area is **consumed** when it is not specific to how this organization defines *correct* and *done*, a commodity implementation exists with the properties the area requires, and the area's invariants can be enforced from outside it. Otherwise it is **built**. Several areas split: the substrate is commodity and the rule is not.

| Area | Verdict | What is taken | What is built |
|---|---|---|---|
| 1 Intent and Criteria | **Build** | — | Nothing represents criteria as machine-checkable artifacts fixed before execution. The prevailing convention — a hand-maintained context file — is the opposite of this |
| 2 Work Formation | **Build** | — | Plans with phases, class assignment per unit, preparatory-unit generation. The nearest analogue, recipe campaigns in large-scale refactoring, covers C1 only |
| 3 Admission | **Split** | Queue, scheduling, worktree isolation | The three-condition gate, and conflict by intersection of areas of effect, which depends on your own estate model |
| 4 Execution | **Consume** | Agent runtimes for the general case; deterministic transformation engines wherever the class admits one, per F4 | Integration only |
| 5 Assurance | **Build** | Test and analysis runners | The verdict itself: conformance to pre-stated criteria on executor-independent evidence, with provenance, shelf life and an undecided outcome. Continuous integration runs tests; it does not render this |
| 6 Landing | **Split** | Merge, deploy, progressive delivery, rollback | Evidence-validity at entry, quarantine on an unplanned state, reversal treated as a change |
| 7 Estate Representation | **Split** | Per-repository code indexing; the deployment layer, which already exists in your CI and artifact registries and should be read rather than rebuilt; runtime topology from tracing | The cross-repository contract join, confidence and provenance on every statement, staleness detection, observational adequacy, reachability, and the representation of work in flight |
| 8 Record | **Consume** | Append-only event storage, trace collection, immutable object storage | The schema of what is recorded, and the binding of a verdict to its evidence |
| 9 Economy | **Split** | Metering and gateway-level budget enforcement, now a mature category | Allocation across intents by class, stopping rules, human time as an accounted resource, cost per unit of verified change |
| 10 Accumulation | **Split** | A memory store | Capture only as a byproduct, the falsifier requirement, expiry, and subordination to derived statements. The store is commodity; the discipline is not |
| 11 Human Boundary | **Build** | — | Nothing exists. The goal turns on this area and no vendor is building it |
| 12 Authority | **Consume** | Sandboxes, network tunnels, managed-agent governance, existing identity and secret management | Binding a grant to a work unit, and the narrowest-sufficient rule |
| 13 Measurement | **Build** | Agent observability, for agent behaviour only | Human touchpoints per unit of change, and the lag between acceptance and the discovery that an acceptance was wrong. No product measures either |

Two things follow.

**The build set is coherent, not residual.** Areas 1, 5, 11 and 13 — criteria, assurance, the human boundary, and the instrument that measures them — are the same mutually-defining node identified at the start of this section. What is left to build is a subsystem, not a list of leftovers.

**Everything built is about deciding; everything consumed is about doing.** That line, and not a judgement about vendors, is what the table records.

**Requirements on anything consumed.** A commodity component that cannot meet these breaks the area it was taken for, and the failure will not be visible until something has already been accepted wrongly.

- An executor must report its own identity and version with every result, or evidence has no shelf life (§4, `Evidence`).
- An executor must distinguish substrate failure from task failure, or area 4's P6 cannot hold and retries become noise.
- A record store must not be writable by the agent it describes (§4, invariant 4).
- Authority must be enforced outside the agent, not requested politely of it (area 12, P5).
- A memory store must permit expiry and refutation, or accumulation becomes the second decaying corpus that area 10 exists to prevent.

**The first increment.** Not the estate model and not an orchestrator, but a thin vertical through the whole structure on **C1 proved**, where acceptance is decidable and the oracle is free — then widen by class. The framework's reach grows by its ability to generate the preparatory work that converts an undecidable situation into a decidable one.
