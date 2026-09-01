# iFlow — Areas to Describe and Solve

**Status:** approved, provisional
**Version:** 1.0 — 2026-09-02
**Derived from:** [01-goal.md](./01-goal.md)

## Method

Every area below is derived from a clause of the goal. Nothing is included because it is common practice, and nothing is included that no clause requires. The goal, restated in clauses for reference:

- **(a)** the development and maintenance of software is carried out at any scale by autonomous agents;
- **(b)** a person contributes the intent and the criteria of an acceptable result;
- **(c)** that person is free not to take part in how the result is achieved;
- **(d)** that person is able to verify it at will;
- **(e)** the volume of such work is limited only by the resources one is willing to spend.

## A note on naming

These are not layers and not stages. Areas 1–5 are sequential: they are the path a single change travels. Areas 6–11 are not steps at any point on that path; they hold across all of it at once. The accurate term is *areas that must be described and solved*, each becoming its own document.

---

## Part I — The path a change travels

### 1. Intent and Criteria — *from (b)*

In what form desire and acceptability are expressed, such that no further dialogue is required. How a criterion becomes checkable rather than merely stated. What happens to criteria when a task is divided into parts. Criteria must cover **termination as well as acceptance**: without a stated condition for stopping, an agent facing ambiguity does not stop.

### 2. Work Formation — *from (a), at any scale*

At scale, intent does not map onto one place. How bounded units of work are derived from an intent and from knowledge of the estate, each carrying its own criteria and a known area of effect. The size of a unit is **a reliability parameter, not only an organizational one**: success probability compounds across steps, so unit size determines how often the whole completes.

### 3. Execution — *from (a), by autonomous agents*

What an agent receives, in what isolation it works, what it produces. The executing substrate must be treated as unreliable and non-stationary rather than as a working black box: tools fail silently, capacity limits interrupt, and the model beneath changes over time. Two consequences follow and belong here — **results are not reproducible across executor versions, and evidence therefore has a shelf life.**

### 4. Assurance — *from (b) and (c)*

If a person does not read the result, conformance to criteria is established by evidence instead. What can be established mechanically, what cannot be established at all, and what evidence must accompany a result for it to count as accepted.

### 5. Landing — *from (a), since scale means simultaneity*

How many separately correct changes enter a live system without turning out to be jointly incorrect. Interference between concurrent changes is semantic and is not visible to version control.

---

## Part II — Foundations

### 6. Estate Representation — *from (a), autonomy*

An agent has no accumulated familiarity with the system, and the system's own description is unreliable. Knowledge must therefore come from somewhere: what counts as an evidential source, in what form knowledge is delivered, and how staleness is detected rather than silently tolerated. The distinction between what is derivable from artifacts and what exists only in people belongs here.

### 7. Record — *from (d)*

Only what was recorded can be verified; evidence about a step cannot be produced after the fact. What is written always, independently of whether anyone will look at it.

### 8. Economy — *from (e)*

If resources are the only limit, they must operate as a control mechanism rather than as reporting: allocation, priority, stopping rules, and the cost of a unit of verified change.

### 9. Accumulation — *from (e)*

If every task pays the full cost of orientation again, resources are spent on repetition. What carries across tasks, what expires, and how this avoids becoming a second corpus that decays the way documentation decays.

### 10. Human Boundary — *from (c) and (d)*

Participation becomes the exception, so the exception needs a rule. Who determines that a human is required, on what evidence, and how that boundary contracts as evidence accumulates.

### 11. Authority — *from (c)*

An autonomous agent acts on real systems while no person is present to authorize each act, so permission must be declared in advance rather than granted interactively. What an agent may touch, with what data, credentials, environments and operations, and how that limit is enforced and audited.

---

## Ordering

Areas 1, 4 and 10 are defined through one another — criteria, conformance and the human boundary cannot be formulated separately — and are the natural starting point. Area 6 attempted before them degenerates into building a larger index without a statement of what it is for.
