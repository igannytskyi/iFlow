# iFlow — Research Framework and Areas to Solve

**Status:** approved, provisional
**Version:** 2.0 — 2026-09-02
**Derived from:** [01-goal.md](./01-goal.md)

---

## Part 0 — Research framework

**Object of study.** The process by which an organization creates and changes software systems whose scale and heterogeneity exceed what any individual participant can comprehend.

**Subject of study.** The conditions and mechanisms under which that process can be carried out by autonomous agents while intent, criteria and the possibility of verification remain with a person.

**Aim.** As fixed in [01-goal.md](./01-goal.md).

**Hypothesis.** A change can be accepted without a person who understands the system if and only if the criteria of acceptability are stated before execution and conformance to them is established by evidence produced independently of the executor. Each of the three conditions is load-bearing: criteria stated after the fact describe the result rather than judge it, and evidence produced by an executor about its own work establishes nothing.

**Methods.** Derivation of requirements from the stated aim; comparative analysis of existing systems and of the practice of large-scale automated change; testing the derived structure against documented industry failure modes; a thin vertical experiment on a narrow class of changes whose acceptance is decidable.

**Claim to novelty.** Three positions, none of which is current practice. First, acceptance without a reader is treated as the governing constraint rather than as a downstream quality concern. Second, knowledge of the system is divided by *derivability from artifacts* rather than by document type, and the non-derivable remainder is treated as harvestable only as a byproduct of human decisions already being made. Third, the boundary of necessary human participation is treated as a computed and contracting quantity rather than as a fixed configuration.

**Practical significance.** The framework identifies which parts of the problem are being commoditized by platform vendors and must therefore be consumed rather than built, and which parts remain specific to how an organization defines *correct* and *done* and must therefore be built.

**Structure of the areas.** Areas 1–5 are sequential — the path a single change travels. Areas 6–11 are not steps at any point on that path; they hold across all of it at once. Each area is stated with its own object, subject, aim, tasks and criterion of resolution, and is derived from a clause of the aim:

- **(a)** work is carried out at any scale by autonomous agents;
- **(b)** a person contributes the intent and the criteria of an acceptable result;
- **(c)** that person is free not to take part in how the result is achieved;
- **(d)** that person is able to verify it at will;
- **(e)** the volume of work is limited only by the resources one is willing to spend.

---

## Part I — The path a change travels

### 1. Intent and Criteria — *from (b)*

- **Object.** The act of stating what is to be changed and what result would be acceptable.
- **Subject.** The form and completeness of that statement, sufficient for execution and for judgement without further participation by its author.
- **Aim.** A representation of intent and criteria fit for machine execution and machine judgement.
- **Tasks.** Determine the composition of a specification; determine how a criterion becomes checkable rather than merely stated; determine how criteria are inherited when a task is divided; determine the condition for stopping, not only for acceptance.
- **Criterion of resolution.** An executor raises no questions against the specification, and a judge reaches a decision without consulting its author.

### 2. Work Formation — *from (a), at any scale*

- **Object.** The conversion of an intent into units of work within a system spanning many components.
- **Subject.** The dependence of a unit's executability and reliability on its boundaries and its size.
- **Aim.** A rule by which units of work with a known area of effect are derived from an intent and from knowledge of the estate.
- **Tasks.** Determine what bounds a unit; determine how its area of effect is computed rather than estimated; relate unit size to the probability of completion, since success compounds across steps; determine how criteria descend to the parts.
- **Criterion of resolution.** For a typical intent the set of units is obtained reproducibly without a person, and the units together cover the intent.

### 3. Execution — *from (a), by autonomous agents*

- **Object.** The work of an autonomous agent on one unit of work.
- **Subject.** The behavior of execution on a substrate that is unreliable and non-stationary.
- **Aim.** Execution whose failures are detectable and whose results remain comparable across versions of the executor.
- **Tasks.** Determine the isolation in which an agent works; classify failure modes, including silent tool failure, capacity exhaustion and drift in the behavior of the underlying model; determine the shelf life of evidence; determine what reproducibility is required and what is unattainable.
- **Criterion of resolution.** A failure of execution is distinguishable from a failure of the intent, and every result carries what produced it and when.

### 4. Assurance — *from (b) and (c)*

- **Object.** The establishment of conformance between a result and its stated criteria.
- **Subject.** The composition and sufficiency of the evidence that replaces a person reading the result.
- **Aim.** An acceptance decision reached on evidence.
- **Tasks.** Separate what is mechanically establishable from what is not establishable at all; determine what makes evidence independent of the executor that produced the result; determine how to proceed where the adequacy of existing tests is itself unknown; determine the rule for escalation.
- **Criterion of resolution.** The share of changes accepted without human reading is measurable, and the share wrongly accepted is bounded and observable.

### 5. Landing — *from (a), since scale means simultaneity*

- **Object.** The entry of many simultaneous changes into a system in operation.
- **Subject.** Semantic interference between changes that are separately correct.
- **Aim.** An order of entry under which joint incorrectness is detected before the changes take effect.
- **Tasks.** Determine what constitutes a conflict beyond textual overlap; determine how conflict is detected before work starts rather than at merge; determine ordering, holding and reversal.
- **Criterion of resolution.** Conflict is detected before execution rather than at merge or in production.

---

## Part II — Foundations

### 6. Estate Representation — *from (a), autonomy*

- **Object.** An organization's knowledge of its own software systems.
- **Subject.** The provenance and trustworthiness of that knowledge where the system's self-description is unreliable.
- **Aim.** A representation that answers questions on demand, each answer carrying its provenance and its confidence.
- **Tasks.** Separate what is derivable from artifacts from what exists only in people; determine which sources carry evidential weight; determine how staleness is detected rather than tolerated; determine how changes still in flight are represented, since a representation of the past cannot govern concurrency.
- **Criterion of resolution.** The question *what does this change affect* is answered together with what confirms the answer.

### 7. Record — *from (d)*

- **Object.** The trace left by agent work.
- **Subject.** The composition of a record sufficient to reconstruct any step after the fact.
- **Aim.** An always-on record that makes verification possible at any moment.
- **Tasks.** Determine the unit of record and its contents; determine retention; determine how the record is bound to the acceptance decision it supports.
- **Criterion of resolution.** Any step of any run is reconstructible without recourse to the executor that performed it.

### 8. Economy — *from (e)*

- **Object.** The consumption of resources by agent work.
- **Subject.** The mechanism that bounds consumption against unbounded demand.
- **Aim.** Volume of work governed by declared limits rather than reported after the fact.
- **Tasks.** Determine the unit of accounting; determine allocation and priority across work of unequal value; determine the stopping rule for work that is not converging; determine the cost of a unit of verified change.
- **Criterion of resolution.** Exceeding a limit is prevented before the spend, not discovered after it.

### 9. Accumulation — *from (e)*

- **Object.** The carrying of knowledge from one task to the next.
- **Subject.** The conditions under which what has been accumulated remains true.
- **Aim.** Reduced repayment of orientation cost, without forming a second corpus that decays as documentation decays.
- **Tasks.** Determine what is worth carrying and what is correctly discarded; determine how an accumulated statement is expired or refuted; determine how it is bound to places in the system so that it surfaces where it applies.
- **Criterion of resolution.** A second task in the same area costs less than the first, and accumulated statements do not diverge from the system undetected.

### 10. Human Boundary — *from (c) and (d)*

- **Object.** Human participation in agent work.
- **Subject.** The rule that determines when that participation is necessary.
- **Aim.** Participation only where it is required, with that region contracting as evidence accumulates.
- **Tasks.** Determine the grounds on which a human is drawn in; determine who or what makes that determination; determine how the volume of participation is measured; determine the conditions under which the boundary may contract.
- **Criterion of resolution.** Participation per unit of change is measured and declining, and cases where participation was required but did not occur are detectable.

### 11. Authority — *from (c)*

- **Object.** The actions an agent performs on real systems and real data.
- **Subject.** Limits on permissible action, declared in advance of execution.
- **Aim.** Permissions granted before execution and enforced independently of the agent.
- **Tasks.** Determine the unit of permission; determine how permission is bound to a unit of work; determine enforcement and audit; determine the handling of credentials and of data an agent must not retain.
- **Criterion of resolution.** No action outside the declared limits is performable, and every attempt is recorded.

---

## Ordering

Areas 1, 4 and 10 are defined through one another — criteria, conformance and the human boundary cannot be formulated separately — and are the natural starting point. Area 6 attempted before them degenerates into building a larger index without a statement of what it is for.
