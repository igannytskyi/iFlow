# Glossary

What every word in the method means, in a sentence or two. Grouped by kind, alphabetical inside each group, because a person learning this reads by kind and a person checking a word looks it up.

`conventions.md` is the companion to this file and a different thing: it lists the **values** a tool validates against. This lists what the **words** mean. The gate reads the first; a person reads this one.

---

## The path a change travels

**Intent** — what a person wants achieved, in one or two sentences, in the language of the problem rather than of the code. The only thing here that starts outside the method.

**Specification** — the intent together with the criteria that would make a result acceptable, what would make work stop without acceptance, and the region it may touch. Fixed from the moment a candidate exists; before that, correcting it is free and recorded.

**Plan** — the intent turned into units of work, each with its own criteria, class and area of effect. Where the work cannot all land at once, it also carries the order, the waits, and the requirement that each intermediate state is a system that works.

**Admission** — the single gate where conflict, allowance and permission are settled together, before any resources are committed. Work that could not have been accepted is never started.

**Execution** — an executor produces a *proposed* change and a record of how. It never applies anything.

**Assurance** — evidence is gathered and a verdict rendered per criterion. This is where a change is accepted or not, on evidence rather than on anyone having read it.

**Landing** — an accepted change enters the live system, while the evidence supporting it still holds.

---

## The things that pass between them

**AcceptanceCriteria** — the set of criteria for one specification.

**AdmissionDecision** — the decision to commit resources to a unit, or not, and which of the three conditions decided it.

**AreaOfEffect** — the region a change can affect, computed from the estate rather than guessed. Short-lived: any landing that touches it invalidates it.

**Baseline** — the present way of working, measured on the same workload a later claim will be measured on. Without it no comparison means anything.

**Budget** — the allowance work is admitted against. Exceeding it blocks before the spend rather than being reported after.

**Candidate** — a proposed change, held apart from the live system until landing. Never applied by whatever produced it.

**ChangeClass** — what kind of change this is, defined by *how its acceptance is decided* rather than by what it touches. See the classes below.

**ChangePlan** — the ordered phases realising one specification where they cannot all land at once, with the waits between them and a rollback position for each.

**Conflict** — two units whose areas of effect intersect in a way that would invalidate the evidence of either. Found before work starts, not at merge. Derived from what unfinished work already holds rather than asserted: two units declaring the same region collide weakly, one reaching into what the other holds collides more strongly.

**In flight** — planned and not yet entered. An estate described only by what has landed describes the past, and a gate reading it decides against a world that has moved.

**ContextBundle** — the knowledge handed to an executor for one unit. Assembled for that run and not reused.

**CostRecord** — what one unit actually consumed: machine resources and human time both.

**Criterion** — one condition on an acceptable result, written as a scenario: *when* these conditions hold, *then* this must be true. In the vocabulary of the intent, never of the implementation.

**Escalation** — a request for a person, with the ground that raised it. Raised by the system; see Refusal for the opposite direction.

**EstateModel** — what the organization knows about its own software, derived from artefacts rather than from documents. Never wholly fresh or wholly stale: freshness is measured per region.

**Evidence** — an artefact supporting one claim, carrying who produced it, on what ground it is independent of the executor, and whether the run behind it can be produced again.

**EvidencePlan** — what will be observed to settle one criterion, and how. Written once the area of effect is known and never from the change itself, because the change does not exist yet.

**Grant** — the narrowest set of permissions sufficient for a unit's declared region. Anything wider is refused rather than warned about.

**LandingPlan** — the order in which accepted candidates enter, and what each invalidates.

**Metric** — one measured figure, always with the population and the class it was measured over.

**Refusal** — a person returns work to its producer at any boundary, as *that producer's* failure rather than as a request to try again. Raised by a person; see Escalation for the opposite direction.

**Scope** — the region a change may touch. Disjoint by construction from whatever will judge it, and from everything that judgement reads.

**Statement** — one assertion about the estate: the atom the estate model is made of, carrying where it came from and how far it is to be trusted.

**TerminationCondition** — what makes work stop without being accepted. Stated with the criteria, because a criterion that says only when to succeed leaves an agent circling.

**Testimony** — something held true on a person's word rather than derived from an artefact. It expires, it must be refutable, and it never outranks something derived from the code as it stands.

**Trace** — the record of one run: what happened, by which executor, at which version. Append-only, and not writable by what it describes.

**Verdict** — the acceptance decision on a candidate, per criterion: met, failed, or **undecided**. Undecided is an honest answer and not a failure; nothing lands on one.

**WorkUnit** — a bounded, executable piece of work with its own criteria, class and area of effect.

---

## The ideas

**Arbiter** — whatever settles that a result conforms, independently of whatever produced it: the prior system, a reproduction, a contract, an observation in production, or a person. A change may never modify its own arbiter, nor anything that arbiter reads.

**Attested** — held true because someone said so. The opposite of derived, and always the weaker of the two.

**Ceremony** — a field people are asked to fill that nothing and nobody reads. The method hunts for it rather than accumulating it.

**Confidence** — how far a statement is to be trusted. Something attested can never carry the highest confidence, by definition.

**Derivable** — recoverable mechanically from code, configuration, history or runtime. What is not derivable exists only in people, and is the scarce half.

**Derived** — established mechanically from an artefact, and therefore re-establishable. Where derived and attested disagree, derived wins.

**Independence** — evidence is independent on one of two grounds: something other than the executor produced it, or it was fixed and accepted *before* the candidate existed. The second is the stronger, since a second executor may share the first's blind spots.

**Observational adequacy** — how well behaviour in a region can be pinned down at all. It decides which class a change in that region can be, and whether a criterion there is settleable. It has two grades and they are not interchangeable: a region something *names* is claimed, which is a guess; a region something *executes* is observed, which is a measurement.

**Predating** — an obligation added after a change was admitted does not apply to it. The gate distinguishes violating a rule from predating it and reports the second as a note.

**Preparatory unit** — work that makes other work decidable: a reproduction, or a characterization of behaviour in a region nobody can currently observe. How the method's reach grows.

**Proxy** — what an arbiter actually tests when it cannot test the criterion directly. Declared, because an undeclared proxy has broken three times while the rule it stood for was fine.

**Provenance** — where a statement came from: derived from an artefact, attested by a person, or produced by an agent.

**Contract edge** — a join between one repository offering something — a route, an event, a queue — and another consuming it. Matched by the key both sides name, raised to certainty only by seeing it happen, and never derived: the key is often not a literal in either source.

**Reachability** — whether every consumer of something can be changed at all. Shipped applications and third parties cannot, which is knowable before any work is done.

**Repeatable** — whether the run behind a piece of evidence can be produced again to the same result. A deterministic transformation can; an agent cannot. A claim to repeat must name what to re-run.

**Reversibility horizon** — how long a change can still be withdrawn. It shortens as other work builds on it, so a verdict left open must close inside it.

**Tenure** — the accumulated, largely unspoken familiarity a person builds with a system by working in it for years. What made large systems tractable before, what agents cannot inherit, and what this method exists to replace.

**Freshness** — whether what a region was derived from is still what last touched it. Measured per region, never per repository: an index is never wholly fresh or wholly stale, and treating it as either is how a model comes to be trusted about ground that moved under it.

**Unseen** — present and not read, because nothing here can parse it. Reported separately from what was looked at and found to be nothing, since a zero over unexamined ground reads exactly like a zero over covered ground and means the opposite.

**Validity** — until when a statement holds, and what ends it. Evidence has a shelf life because the ground it was obtained against moves.

---

## Classes of change

Ordered by what settles them, from the strongest to none at all.

**C1 proved** — nothing observable changes, and the transformation is behaviour-preserving by construction. Acceptance costs nothing because nothing need be run. If the claim of equivalence rests on documentation or a person rather than on derivation, it is not this class.

**C1 tested** — nothing observable changes, and this is established by comparing behaviour before and after.

**C2** — a defect is repaired: one wrong behaviour is now right and nothing else moved. Needs a reproduction that fails on the unmodified system for the stated reason; a defect without one is a request for a reproduction first.

**C3** — the result satisfies an explicitly stated contract: a schema, an interface, a type, a policy. What remains open is whether the contract was the right one, which belongs to whoever wrote the criterion.

**C4** — the deployed system behaves better against a measured quantity. Settled only after exposure, so the verdict is deferred and the change must stay reversible until it closes.

**C5** — only a person can say whether this is what was wanted. No arbiter exists and none can be built; the method's job is to make the decision smaller, not to remove it.

---

## What the gate is asked

```
estate.py affects <path>      what a change there reaches, and how far to trust it
estate.py observability <path> how well behaviour there can be pinned down
estate.py freshness           what the answers were derived from, and when
estate.py unknown             what the index cannot resolve at all
estate.py contracts <dir>     what crosses between repositories, and on what basis
estate.py observe <path> <cmd> run it and see which lines actually ran
estate.py inflight <dir>      what unfinished work already holds
estate.py conflicts <dir> <p> who already holds this ground, and on what footing
estate.py refresh             derive again only what moved
estate.py reachability <d> <r> who consumes this, and who no change reaches

check.py changes/<slug>     one change, against every rule
check.py --status [dir]     every change: how far it got, what holds it, what it owes
check.py --arbiters         run every arbiter in one action
check.py --mutate           remove each rule in turn and see whether anyone notices
check.py --repeat <slug>    run what claims it can be repeated, twice, and compare
check.py --escalations      what the method still owes itself, grouped by what it names
check.py --unused           columns nobody reads and codes nobody validates
```
