# iFlow — Second Run: a change across a contract

**Status:** approved, provisional
**Version:** 1.0 — 2026-09-02
**Follows:** [07-reference-scenario.md](./07-reference-scenario.md), which exercised the most favourable case
**Classes exercised:** C3 contract-bounded, with C1, C4 and preparatory work inside one intent

The first run took the easiest change in the catalogue and broke five things. This run takes the first genuinely hard one: a change that crosses a contract between repositories, cannot be landed at a single moment, and reaches consumers that cannot be changed at all. It broke six more, all applied in this version.

---

## 1. The change

> `orders-api` must require a `currency` field on `POST /orders`. Three services in the estate call the endpoint. The same shape is carried by an `order.created` event consumed by two more services. A mobile application in the field also calls it, on versions already installed on users' devices.

---

## 2. The run

### Area 7 — Estate Representation *(consulted first)*

Static analysis proposes the call sites; runtime telemetry from the gateway confirms which of them actually call the endpoint, and discovers one the static pass missed — a client whose URL is assembled from configuration. Five internal callers are established with high confidence.

The mobile client is different in kind. It is a real consumer, it is visible in telemetry, and **it is not in the estate**: no change to any repository reaches the copies already installed. *(Finding G3.)*

### Area 1 — Intent and Criteria

`AcceptanceCriteria`: the field is required by the producer's schema; every reachable consumer sends it; **no traffic in the old shape is observed for a stated period**; existing behaviour is otherwise unchanged.

The third criterion cannot be decided before exposure. The first can be decided against a schema. The fourth is behaviour-preserving. **One intent, three different oracles.** Assigning a single `ChangeClass` to the specification, as the corpus required, is not possible here. *(Finding G1.)*

### Area 2 — Work Formation

The units cannot all be landed together, and their intermediate states must each be a system that works:

1. Producer accepts the field as optional. Reversible; C1 with respect to existing callers.
2. Each reachable consumer starts sending it. C3 per consumer. One consumer repository has no usable tests, so a **preparatory unit** — characterization of its current request behaviour — is emitted first and the consumer unit depends on it.
3. Observation: no old-shape traffic for the stated period. C4, a deferred verdict.
4. Producer makes the field required. C3, and it is what breaks any straggler.

This is not a set of units. It is an ordered plan with waits between phases, and with a criterion that belongs to no unit — *every intermediate state is a valid, shippable system*. The corpus had `WorkUnit.dependencies` and nothing that could carry this. *(Finding G2.)*

### Area 3 — Admission

Two of the consumer units are in different repositories and touch no common file, yet they conflict: another in-flight change is altering the same event schema. Their areas of effect intersect only through a **contract edge**, and that edge is the kind established by matching a route to a client call and confirmed by telemetry — that is, it carries a confidence below one.

An admission decision resting on a probabilistic edge is itself probabilistic. The gate must carry that, and a low-confidence edge must either be confirmed at runtime before admission or raise the human involvement for that unit. *(Finding G5.)*

Phase 4 is not admitted at all yet: its precondition is the closing of a verdict that does not exist until phase 3 has run.

### Area 4 — Execution

Consumer changes are ordinary code edits with no mechanical transformation available, so the executor here **is** an agent — the case F4 said to avoid where a deterministic option exists, and here there is none.

One agent exhausts its step budget on the repository with no tests, because it keeps trying to establish for itself whether its change is safe. That work belonged to the preparatory unit and had already been done; the context bundle did not include its result. A defect of context supply presenting as a budget failure.

### Area 5 — Assurance

Phases 1, 2 and 4 are decided against the schema and the characterization tests — evidence produced independently of the executors.

Phase 3 is decided against **production telemetry**: the absence of old-shape traffic over the window. That evidence is not producible in an isolated environment, and area 5's authority in the corpus is *tests and analyses in isolated environments, no production access*. Deciding a C4 criterion requires reading production, and the corpus forbade it. *(Finding G6.)*

### Area 6 — Landing

Phase 1 lands. Phase 2 lands per consumer as each is accepted. Phase 3 lands nothing; it waits.

Then the difficulty. The observation window is thirty days. The deferred verdict holds phase 1 reversible for its duration — but a schema that has been live for thirty days, with consumers built against it, is not reversible in the sense the invariant assumed. **Reversibility is not a property a change has or lacks; it is a horizon that shortens as the system moves on.** The window must fit inside that horizon, and where it does not, a person must decide at the moment the horizon expires rather than the verdict being quietly abandoned. *(Finding G4.)*

Phase 4 lands only after the deferred verdict closes. The mobile client's traffic has not reached zero, so it does not close on its own.

### Area 11 — Human Boundary

The decision that arrives is exactly the one the system cannot make: old-shape traffic has fallen to a small residue from an application nobody can update. Continuing means breaking those users; waiting means never finishing.

This is a C5 decision reached by a C3 route, and it is the right outcome — the person is handed a bounded question with everything establishable already established, rather than a diff. It is also unavoidable: **a contract change with an unreachable consumer can never complete without a human, and that is knowable at area 1, before any work is done.**

### Area 13 — Measurement

Human touchpoints: stating the intent, one low-confidence contract edge, and the terminal decision on the residue. Three. Against run 1's four across two hundred repositories, this is the more expensive shape of work per unit of change, and the measurement must be reported per class or it will average the two into a meaningless figure.

---

## 3. What the run changed

**G1 — Class belongs to the work unit, not the specification.** One intent legitimately spans C1, C3, C4 and C5. The specification carries a *default* class; each unit carries its own, and assurance uses the unit's.

**G2 — Work formation produces a plan, not a set.** Where phases must be separated in time, there is an object above the units: an ordered plan with wait conditions, a criterion that every intermediate state is valid and shippable, and a rollback position per phase. `WorkUnit.dependencies` cannot express a wait on an observation.

**G3 — The estate has a boundary, and reach is not coextensive with it.** Consumers exist that no repository change reaches: shipped applications, third parties, anything already in someone else's hands. They must be modelled, because their existence changes what class a change can attain and guarantees a human decision at the end. A contract change with an unreachable consumer is knowably incompletable at area 1.

**G4 — Reversibility is a horizon, not a boolean.** It shortens as other work builds on a change. A deferred verdict is admissible only while its observation window fits inside that horizon; where the horizon expires first, a person decides at that moment. The invariant binding deferred verdicts to reversibility was true but under-specified.

**G5 — Conflict detection inherits the confidence of the estate edges it rests on.** Cross-repository conflict is found through contract edges, and those are matched and confirmed rather than derived. An admission decision therefore carries a confidence, and a low-confidence edge either gets runtime confirmation before admission or raises human involvement for that unit.

**G6 — Assurance needs read access to production.** A C4 criterion is decided on observed behaviour, which no isolated environment can produce. Area 5's authority was written as isolated-only. It now reads production observations, and writes nothing anywhere.

**And one that is not a corpus change but a rule.** The agent that burned its budget re-establishing what a preparatory unit had already established was failing at context supply, not at execution. **A budget exhaustion should be diagnosed before it is retried**, or the system will pay repeatedly for a missing input.

---

## 4. What is still untested

A conflict that ordering cannot resolve. A reversal that fails midway. Two intents competing for the same scarce human. An estate region whose representation is wrong rather than stale. Agent executors disagreeing on the same unit.
