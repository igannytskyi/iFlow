# iFlow — Descriptive Schema

**Status:** approved, provisional
**Version:** 1.0 — 2026-09-02
**Derived from:** [01-goal.md](./01-goal.md)
**Applied in:** [04-areas-specified.md](./04-areas-specified.md)

This document defines *how* an area is described. It contains no description of any area.

Two things are specified: the requirements every object must satisfy in order to pass between areas, and the parameters every area must define in order to operate unattended.

---

## Part A — Requirements on any object

An object is anything that passes from one area to another. Without every one of the following, an object cannot be handed over mechanically.

| # | Requirement | What it means | Why |
|---|---|---|---|
| **O1** | **Identity** | A stable identifier that survives re-creation of the object and can be cited from a record or a decision. | Without it a decision cannot be tied to what it was made about. |
| **O2** | **Schema** | A fixed structure, not free text. | An area whose output is prose does not compose with the next one. |
| **O3** | **Provenance** | What produced it: derived from an artifact, asserted by a person, produced by an agent. | Distinguishes what is established from what is supposed. |
| **O4** | **Confidence** | A stated degree of certainty — required of every object, not only of inferred relations. | A supposition presented as a fact silently corrupts every decision downstream of it. |
| **O5** | **Validity** | Until when it is held true, and what invalidates it. | The substrate is non-stationary; evidence has a shelf life. |
| **O6** | **Ownership** | The place in the estate the object pertains to. | So that it surfaces where it applies and expires with what it describes. |

---

## Part B — Parameters of any area

Ten parameters. Each is derived from a clause of the aim, and without any one of them an area cannot run unattended.

| # | Parameter | What must be stated | Derived from |
|---|---|---|---|
| **P1** | **Inputs** | Which objects are consumed, and from where. | *(a), (c)* — an unnamed input is supplied by a person. |
| **P2** | **Outputs** | Which objects are produced, and in what form. | *(a)* — an output that is not an object cannot be consumed by the next area. |
| **P3** | **Decision rule** | What decision is made here, and by what rule. | *(c)* — a decision without a rule requires a person; the rule is what makes participation unnecessary. |
| **P4** | **Completion** | When the area's work on an item is finished, including when it stops without success. | *(b)* — criteria must cover termination, not only acceptance. |
| **P5** | **Invariant** | What must hold true throughout, regardless of outcome. | This is what is checked, as distinct from what is declared. |
| **P6** | **Failure semantics** | How the area fails, and how failure is distinguished from a wrong result. | *(a)* — automation without defined failure produces silent corruption instead of an error. |
| **P7** | **Evidence emitted** | What the area leaves behind so that its own work can be checked later. | *(d)* — verification at will is possible only over what was recorded. |
| **P8** | **Cost and stopping** | What is consumed, how it is measured, when it is cut off. | *(e)* — resources are the only limit, so they must be a mechanism. |
| **P9** | **Authority required** | Which permissions the area's actions need. | *(c)* — no person is present to authorize each act. |
| **P10** | **Escalation** | Under what circumstances the area must draw in a person. | *(c), (d)* — participation is the exception, and the exception needs a rule. |

---

## Part C — The test

There is one test of whether an area has been described adequately:

> **The output of an area is consumed by the next area with no person in between.**

If a person is needed to carry a result across a boundary, the output is not an object with a schema, and no amount of automation inside the areas repairs that.
