# iFlow — Reference Scenario

**Status:** approved, provisional
**Version:** 1.0 — 2026-09-02
**Class exercised:** C1 proved, from [06-change-classes.md](./06-change-classes.md)

One change carried end to end through all thirteen areas, object by object. Its purpose is falsification: a decomposition that cannot carry the easiest class will not carry any of the others. The run below changed five things in the corpus; those are stated in §3 and have been applied.

---

## 1. The change

> A platform library deprecates `Clock.nowUtc()` in favour of `Clock.instant()`, documented as returning the same value. The call is to be removed everywhere before the library's next major version drops it.

Chosen because it is the most favourable case available: mechanical, behaviour-preserving by construction, spanning the whole estate, with no interface change and no production access required. **200 repositories are in scope in this scenario.**

---

## 2. The run

### Area 1 — Intent and Criteria

`Intent` arrives from a person: remove the deprecated call, estate-wide, before the next major version.

The class is assigned **C1 proved** — on the ground that the two calls are documented as equivalent. That ground is not derived from anything; it is a claim from the library's documentation, affirmed by a person. It is recorded as `Testimony`, with its source, and it carries the whole guarantee of the change. *(See finding F1.)*

`AcceptanceCriteria`: no statically resolvable call site of the old symbol remains within `Scope`; every replaced site type-checks against the new symbol; the transformation applied is the declared one and no other; the build passes.

`TerminationCondition`: a call site the transformation cannot resolve mechanically — a reflective or string-formed invocation. Work stops on it; it is never hand-edited under this class.

`Scope`: the repositories in which the symbol is reachable, from area 7.

### Area 7 — Estate Representation *(consulted)*

The query returns `Statement`s locating call sites, provenance *derived from source index*, confidence high but **not total**: reflective and string-formed invocations are not statically resolvable in any language that permits them.

The criterion is therefore written as *no statically resolvable call site remains*, with a residual risk stated explicitly rather than absorbed into a confident-sounding total. **Even the easiest class has an undecidable residue, and the system's job is to name it, not to hide it.**

### Area 2 — Work Formation

One `WorkUnit` per repository — the repository is chosen as the unit because it is the unit of build, verification and landing, not because it is the unit of the change.

Each unit inherits *no remaining call site in this repository*. `AreaOfEffect` per unit: the touched files and their dependents within the repository. No preparatory units are needed — the mark of C1 proved, and the reason it is the cheapest class in the catalogue.

### Area 3 — Admission

Conflict: three units intersect the area of effect of unfinished work in the same files. They are held and ordered behind it. Allocation exists. `Grant` per unit: read and write to a working copy, permission to open a proposal — no deployment, no secrets, no production. **197 admitted, 3 held.**

### Area 4 — Execution

197 executors run in parallel. Each applies the declared transformation and produces a `Candidate` and a `Trace`.

The executor here is a deterministic transformation, not a language model. Nothing in the class requires an agent, and much argues against one: a deterministic executor has no drift, no substrate non-stationarity, and reproduces exactly. *(See finding F4.)*

Four units meet a call site the transformation cannot resolve. This is task failure, not substrate failure, and the termination condition fires as written.

### Area 5 — Assurance

Two criteria are checked per candidate: the symbol no longer resolves anywhere in the repository, and the result type-checks.

The third criterion — that the transformation preserves behaviour — is **not checked per candidate at all.** It was established once, about the transformation itself, independently of any run of it. That is precisely why this class is cheap: the expensive evidence is produced once and amortized across 197 applications. *(See finding F5.)*

197 `Verdict`s, settled, not deferred: there is no observable effect to wait for.

### Area 6 — Landing

Candidates enter. No cross-repository ordering is required, because a behaviour-preserving change publishes no new interface.

One candidate's evidence is invalidated before entry: an unrelated change landed a new call site in the same repository while this one was waiting. The evidence is re-established rather than the candidate dropped — the criterion is about the repository's end state, not about the diff.

### Area 8 — Record

197 traces, 197 verdicts, the transformation's own provenance, and the testimony carrying the equivalence claim. All append-only, none writable by what they describe.

### Area 9 — Economy

Cost is dominated by the estate query and the verification runs, not by producing the changes. **For this class, generation is nearly free and assurance is the bill** — the reverse of the usual assumption, and a direct consequence of the executor being deterministic.

### Area 10 — Accumulation

Captured as a byproduct of decisions already being made: the four unresolvable call sites and why each resisted; the source and standing of the equivalence claim. Nothing was solicited from anyone.

### Area 11 — Human Boundary

A person is involved four times: stating the intent, affirming the equivalence claim, deciding what to do about the four escaped call sites, and resolving the three held conflicts if ordering does not settle them.

**Four touchpoints against two hundred repositories.** That number, and not agent throughput, is what this scenario demonstrates.

### Area 12 — Authority

No grant in this scenario touches production, secrets or deployment. Authority requirements track the class: a behaviour-preserving change needs none of them, and a class that needs them says so before any work starts.

### Area 13 — Measurement

Baseline: the same change performed the present way, on the same 200 repositories. Instruments: human touchpoints per estate-wide change; cost per accepted unit; the share of units escaping to a human decision — here 7 of 200, or 3.5%; and the lag between acceptance and any later discovery that an accepted change was wrong.

---

## 3. What the run changed

Five findings. All are applied in the corpus as of this version.

**F1 — C1 proved rests on a claim that is usually testimony, not derivation.** The guarantee of a behaviour-preserving transformation is only as strong as the equivalence claim underneath it, and that claim typically comes from documentation or a person. The class must record the ground of equivalence with its provenance, and **an equivalence claim that is testimony rather than derived demotes the change from C1 proved to C1 tested.** Otherwise the strongest guarantee in the catalogue rests on an unexamined assertion, and no oracle downstream can catch it, because the oracle was replaced by the proof.

**F2 — Even the easiest class has an undecidable residue.** "All call sites" is not derivable with certainty wherever reflection or string-formed invocation is possible. Criteria must be written to what is decidable, with the residue stated, rather than to what sounds complete.

**F3 — The invariant that unit criteria imply specification criteria is conditional.** It holds only if the estate query that produced the scope was complete. Stated unconditionally, as it was, it is false. It now carries its condition.

**F4 — The executor need not be an agent.** Area 4 assumed an autonomous agent. The cheapest and most reliable class does not want one: a deterministic transformation has no drift and reproduces exactly. The executor is whatever satisfies the class, and **where a class permits a deterministic executor, using an agent instead is a defect** — it converts a reproducible result into an unreproducible one and pays more for it.

**F5 — Evidence may attach to a transformation rather than to a candidate.** `Evidence` was defined as supporting a claim about a candidate. Establishing a property of the transformation once, and amortizing it across every application, is the mechanism by which assurance cost stops scaling with the number of changes. It is the single most important economic fact in this scenario and the object model did not permit it.

---

## 4. What the run did not test

This scenario exercises the most favourable case. It says nothing yet about: a change that spans a contract between repositories; a deferred verdict and its reversibility window; a region of unknown observational adequacy requiring preparatory work; conflicting changes that ordering cannot separate; or an agent executor with its own failure modes. Each needs its own run, and each is expected to break something further.
