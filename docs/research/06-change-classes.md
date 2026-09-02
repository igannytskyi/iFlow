# iFlow — Change Classes

**Status:** approved, provisional
**Version:** 1.2 — 2026-09-02
**Object defined here:** `ChangeClass`, from [05-objects.md](./05-objects.md)

A change class is defined by **how the acceptance of a change is decided**, not by what the change touches, how large it is, or which team requested it. Two changes in the same file belong to different classes if one can be accepted on evidence and the other cannot.

A class is assigned per `WorkUnit`; the `Specification` carries only a default. One intent legitimately spans several classes — a contract change is C1 for the producer's existing callers, C3 per consumer, C4 while old-shape traffic drains, and C5 at the residue. It fixes, before any work begins, which evidence will be required, what budget profile applies, and what escalates by default. It is therefore the principal configuration lever of the whole system.

---

## 1. The oracle

Acceptance requires something that can pronounce on a result independently of whatever produced it. Call that an **oracle**. Classes are ordered by which oracle decides them, from the most independent and immediate to none at all.

| | Class | The claim being made | Oracle | Decidability |
|---|---|---|---|---|
| **C1** | Behaviour-preserving | Nothing observable changed | The prior system itself | Full, given observational adequacy |
| **C2** | Defect repair | This specific wrong behaviour is now right, and nothing else changed | A reproduction, plus C1's oracle for everything else | Full, given a reproduction exists |
| **C3** | Contract-bounded | The system now satisfies this stated contract | The contract — types, schema, interface, property, policy rule | Full within the contract |
| **C4** | Observable-effect | The deployed system behaves better against a measured quantity | Production observation against a baseline | Partial, and **delayed** |
| **C5** | Judgment-bound | This is what was wanted | A person | None, by construction |

---

## 2. The classes

### C1 — Behaviour-preserving

Dependency and version upgrades, framework migrations, mechanical refactoring, removal of dead code, moves and renames. The oracle is free and exact: the system as it stood before.

Two sub-modes, and the distinction is worth more than it appears:

- **Proved.** The transformation is behaviour-preserving by construction — a type-checked rename, a semantic-tree recipe. Acceptance costs nothing, because nothing needs to be run. This is the whole basis of industrial large-scale refactoring, and it is decidable in the strongest sense available anywhere in this document.

  **The proof rests on an equivalence claim, and that claim must be recorded with its provenance.** Where it is derived — the compiler establishes it, the transformation is total over the semantic tree — the class holds. Where it comes from documentation or a person it is `Testimony`, and **an equivalence claim resting on testimony demotes the change to C1 tested.** Without this rule the strongest guarantee in the catalogue rests on an unexamined assertion that no downstream oracle can catch, because the proof is what replaced the oracle.
- **Tested.** Behaviour is compared before and after. Acceptance costs a verification run and is only as good as what can be observed.

The binding difficulty in C1 is never the oracle. It is **observational adequacy** — whether current behaviour can be pinned down at all. See §3.

A second limit is inherent rather than circumstantial: where a language permits reflection or string-formed invocation, the set of call sites is not statically decidable. Criteria in this class are written to what is decidable, with the residue stated, never to what merely sounds complete.

### C2 — Defect repair

The claim has two halves and both need evidence: the reported behaviour is corrected, and nothing else moved. The second half is C1.

Decidability is conditional on a reproduction existing. Producing that reproduction is itself work, and its own acceptance is decidable — the test must fail on the unmodified system, and fail for the stated reason rather than incidentally. A defect without a reproduction is therefore not a C2 change; it is a request for a reproduction, followed by a C2 change.

### C3 — Contract-bounded

The criterion is an explicit statement the result must satisfy: a schema, an interface, a type, an invariant, a property, a policy or lint rule, a stated remediation. Security remediation against a named rule lives here; so does adding a field with a declared schema, or enforcing a validation rule.

What remains undecided is whether the contract was the right contract. That residue belongs to area 1, where the criterion was written, and must not be smuggled into area 5 as though assurance could settle it.

### C4 — Observable-effect

The claim concerns effect rather than form: fewer errors, lower latency, lower cost, more of some measured outcome. No pre-landing evidence can establish it, because the quantity does not exist until the change is exposed.

Acceptance therefore splits in two. Before landing, only safety is established — the change is behaviour-preserving where it must be, contract-satisfying where it can be. After landing, the effect is measured against a baseline, and the verdict is completed or reversed.

**A C4 change requires reversibility.** A delayed verdict is only tolerable if the change can be withdrawn while the verdict is outstanding. An irreversible change whose acceptance depends on observed effect is not C4 — it is C5, and requires a person before it lands.

### C5 — Judgment-bound

New user-facing behaviour, product decisions, interfaces meant to be used by people, anything whose criterion is desirability. No oracle exists and none can be built.

The system's task here is not to decide. It is to **reduce what must be judged**: establish everything establishable, present the person with a bounded decision rather than a diff, and record the decision as testimony so the next similar case starts further along. C5 does not become automatable. It becomes cheaper to judge.

---

## 3. Modifiers

These cut across the classes. They are not classes and must not be turned into any.

**Observational adequacy.** Whether current behaviour in a given region can be pinned down. In a brownfield estate this is unknown by default, and it determines whether C1 and C2 are actually decidable *here* rather than in principle. The same change is a different proposition in a well-covered region and in an uncovered one.

**Blast radius.** How far the effects of the change can travel before anything detects them. It governs admission, not acceptance.

**Reachability.** Whether every consumer of the changed contract can be changed at all. Shipped applications, third parties and anything already in someone else's hands are consumers that no repository change reaches. **A contract change with an unreachable consumer cannot complete without a human decision, and that is knowable at area 1, before any work is done.**

**Reversibility.** Whether the change can be withdrawn cheaply, and **for how long** — the horizon shortens as other work builds on the change. Data migrations, deletions and anything that leaves the system — messages sent, payments made, artifacts published — are irreversible regardless of class, and require a stricter gate than their class would otherwise imply.

---

## 4. Consequences for the rest of the corpus

Three things follow that the corpus does not yet account for.

**1. A verdict may be deferred.** `Verdict` as defined in `05` assumes a decision reached at assurance time. C4 requires a verdict that is opened before landing and closed after observation, with the change reversible for as long as it remains open. The object needs that state.

**2. The system must be able to produce work that makes other work decidable.** A reproduction for C2, characterization tests for C1 in a region of unknown coverage — these are preparatory units of work, generated by the system for itself, whose own acceptance is decidable. This is not a special case; it is how an undecidable situation is converted into a decidable one, and it is the main mechanism by which the framework's reach grows.

**3. Observational adequacy belongs in the estate model.** What can be observed, per region, determines what is decidable there. `EstateModel` as defined carries structure and provenance; it must also carry how well each region's behaviour can be pinned down, or area 1 cannot assign a class truthfully.

**Misclassification is a failure mode with a name.** Classifying a C5 change as C3 means accepting it against a contract that does not capture what was wanted — the result passes and is wrong. This failure belongs to area 1, is invisible to area 5 by construction, and can only be caught by area 13 as a lag between acceptance and later discovery.

---

## 5. Order of work

The classes give the order in which capability is built, because each one's oracle is weaker than the last:

**C1 proved → C1 tested → C2 → C3 → C4 → C5.**

The first is where the strongest guarantees and the existing industrial practice are. The last never arrives, and should not be aimed at: the goal for C5 is a smaller human decision, not an absent one.

The reference scenario should be run on **C1 proved**, since a decomposition that cannot carry the easiest class end to end will not carry any of the others.
