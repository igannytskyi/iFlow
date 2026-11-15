# Plan — <name>

| Field | Value |
|---|---|
| Specification | SPEC-<nnn> |
| Phases | <1 if everything can land at once> |
| Intermediate states valid | yes · no — every phase boundary must leave a shippable system |
| Residue on abandonment | <rollback, or the intent that disposes of it> |

Planned by <who divided the work>.

## Approach

### How
<the change built as a whole, before it is divided>

### In order
- <the steps>

### What could go wrong
- <risks of the approach>

## Work units

| Id | Phase | Class | Scope | Area of effect | Criteria | Depends on | Preparatory for |
|---|---|---|---|---|---|---|---|
| WU-<nnn>-01 | 1 | C1P | | <how it was computed, and at what confidence> | CR-<nnn>-01 | | |

A preparatory unit — a reproduction, a characterization of behaviour in an inadequately observed region — carries its own decidable criteria and names the unit that depends on it.

## The units, described

Each unit as a ticket its agent can carry out having read nothing else.

### WU-<nnn>-01 — <title>
<objective>

Serves: <FR-n, CR-nnn-nn>

Relies on:
- <what exists, or what an earlier unit provides>

Tasks:
1. <in order>

Provides:
- <names and signatures, exactly>

Constraints:
- <what not to touch>

Edge cases:
- <each with what happens>

Done when:
- <the check, and what it shows>

## Coverage

Two questions, answered in a sentence rather than a table because nothing reads them and a table made them look like data.

Do the units together cover the specification? And was the estate query that produced the scope complete — because where it was not, the residue belongs on the specification rather than being assumed away.
