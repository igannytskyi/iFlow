# The process, drawn

The path a change travels, what crosses each boundary, and where a person touches it. Companion to [iflow.md](iflow.md); nothing here is new, and where the two disagree the research document is right.

---

## 1. The path and its artefacts

```mermaid
flowchart TD
    subgraph outside[" "]
        T["Ticket / epic / issue<br/><i>a rendering surface, not an object</i>"]
    end

    T --> I["<b>Intent</b><br/>what is to be achieved<br/>+ the authority that may decide it"]
    I --> S["<b>Specification</b><br/>criteria as scenarios · termination<br/>scope · arbiter · what the arbiter reads"]
    S --> P["<b>ChangePlan</b> + <b>WorkUnit</b>s<br/>class per unit · area of effect<br/>preparatory units where needed"]
    P --> A{"<b>Admission</b>"}
    A --> G["<b>Grant</b> + <b>EvidencePlan</b><br/>+ prior state captured"]
    G --> X["<b>Execution</b><br/>→ <b>Candidate</b> in candidates/<br/>→ <b>Trace</b>"]
    X --> V["<b>Assurance</b><br/>→ <b>Evidence</b> → <b>Verdict</b>"]
    V --> L["<b>Landing</b><br/>→ the change takes effect"]
    L -.->|"invalidates statements<br/>refutes the model when it surprises"| E[("Estate model")]
    E -.->|"area of effect<br/>observational adequacy<br/>reachability"| P
    E -.-> A

    A -->|"conflict · no allowance · no permission<br/>· no evidence plan"| H["<b>Held</b><br/>recorded, never silent"]
    X -->|"cannot satisfy the criteria"| F["<b>failed-task</b>"]
    V -->|"evidence unobtainable"| U["<b>undecided</b><br/><i>not a failure</i>"]
```

Three edges are worth reading twice. Landing **refutes** the estate model when a change reaches beyond its predicted area of effect — that is the only mechanism that removes a false contract edge. `undecided` is a verdict, not an absence, and nothing lands on one. And a held unit is written down: silence is the single unacceptable outcome.

---

## 2. Where a person actually touches it

```mermaid
flowchart LR
    subgraph HUMAN["A person"]
        direction TB
        H1["states the intent<br/>and its criteria"]
        H2["reviews before admission<br/><i>cheapest minute in the path</i>"]
        H3["answers an escalation"]
        H4["refuses at a boundary<br/><i>optional, at any time</i>"]
        H5["decides the residue<br/><i>what no arbiter can settle</i>"]
    end

    H1 --> S1["Specification"]
    H2 -.-> S1
    H3 -.-> ESC{{"Escalation<br/><i>the system asks</i>"}}
    H4 -.-> REF{{"Refusal<br/><i>the person acts</i>"}}
    H5 -.-> C5["C5 residue"]

    S1 --> AUTO["everything else runs<br/>without a person:<br/>planning · admission · execution<br/>assurance · landing"]
    AUTO --> ESC
    AUTO --> C5
    REF --> AUTO
```

**Escalation is the system pulling; refusal is the person pushing.** The first is a rule that fires — an undecided verdict, a conflict no ordering resolves, a grant wider than policy, a consequence past a threshold. The second is available at any boundary and costs nothing when unused: a person who looks and disagrees returns the object to its producer as *that producer's* failure, not as a request to try again.

The boundary contracts as evidence accumulates, but **toward a floor rather than to zero**: the correctness of an intent cannot be established from inside the system, and a system cannot arbitrate itself.

---

## 3. A defect whose cause is unknown

The case where a criterion and the evidence that settles it cannot be fixed at the same moment.

```mermaid
flowchart TD
    TK["A symptom is reported.<br/>Nobody knows the cause"] --> INT["<b>Intent</b>"]
    INT --> CR["<b>Criteria</b>, fixed now, never again<br/><i>in the vocabulary of the symptom:</i><br/><i>WHEN these conditions hold</i><br/><i>THEN the symptom does not occur</i><br/><i>AND everything in the area of effect</i><br/><i>behaves as before</i>"]
    CR --> D["<b>Diagnosis</b> — a unit of its own<br/>its acceptance is decidable:<br/>a reproduction that fails on the<br/>unmodified system, for the stated reason"]
    D --> AOE["<b>Area of effect</b><br/><i>computed from the estate,<br/>not guessed from the fix</i>"]
    AOE --> EP["<b>EvidencePlan</b>, fixed now<br/>what is observed · by what method<br/>what <i>as before</i> is measured against"]
    EP --> CAP["<b>Prior state captured</b><br/><i>the moment moves here: before<br/>the diagnosis there was nothing<br/>to capture, and <b>as before</b><br/>had no referent</i>"]
    CAP --> REP["<b>Repair</b>"]
    REP --> ASS["<b>Assurance</b> against the criteria,<br/>using the planned evidence"]

    style CR fill:#1f6f43,color:#fff
    style EP fill:#1f6f43,color:#fff
```

**What the criteria can say on the first day, and what they cannot.** They can say that the symptom must stop and that nothing else may move — both in the vocabulary of the symptom, both decidable once evidence exists, and neither ever revised. They cannot say **which** parts must be shown unchanged, **what** conditions are representative, **which** environment, how many observations, or what tolerance applies. All of that follows from a diagnosis that has not happened.

**Why this is not simply revising the requirement later.** The plan is derived from the area of effect, which is computed from the estate; the change does not exist yet, so the plan cannot be shaped around it. Revising a criterion after the work exists makes it describe the work rather than judge it, and that is the one thing the method forbids outright.

**Where this bites hardest.** A criterion about a measured quantity, settled by a single observation, rests on evidence that cannot be produced again. The conditions, the environment and the number of observations are not diligence — they are what makes such a criterion satisfiable at all.

## 4. What to do when something is wrong

The legal move depends on **what** is wrong and **whether a candidate exists yet**.

```mermaid
flowchart TD
    W{"What is wrong?"}
    W -->|"the intent itself"| I1["Withdraw it and state a new one.<br/>Units in flight are <b>cancelled</b> — a terminal<br/>state, still charged — and the residue<br/>gets an owner: a rollback, or a new intent"]
    W -->|"criteria imprecise<br/>or incomplete"| Q{"Has a candidate<br/>been produced?"}
    Q -->|"no"| E1["Edit and re-admit.<br/>The digest is taken again and<br/>the re-admission is recorded"]
    Q -->|"yes"| E2["Frozen. Add a <b>new specification</b><br/>under the same open intent —<br/>one ticket, several specifications"]
    W -->|"the evidence plan<br/>tests the wrong things"| Q2{"Has it been<br/>executed against?"}
    Q2 -->|"no"| E3["Amend and re-fix"]
    Q2 -->|"yes"| E4["The verdict rests on it:<br/>re-establish the evidence<br/>before anything else lands"]
    W -->|"the plan, the area of effect,<br/>an admission decision"| E5["Not immutable. Correct it<br/>and run the gate"]
    W -->|"the candidate itself"| E6["<b>Refuse</b> at the boundary.<br/>It returns as the producer's failure,<br/>not as a request to retry"]

    style E2 fill:#7a2020,color:#fff
    style E4 fill:#7a2020,color:#fff
```

**Why the criteria cannot simply be edited once a candidate exists.** Not strictness: criteria revised after the work exists describe the work rather than judging it, and the hypothesis fails at its first condition. Before a candidate exists that risk does not exist, which is why editing then is free and is the moment worth spending a minute on.

---

## 5. In enterprise vocabulary

| Their word | Here | Note |
|---|---|---|
| Epic | a group of intents | closes when every intent under it is satisfied |
| Issue / ticket | `Intent` | a rendering surface; one ticket may spawn several specifications |
| Requirements | `AcceptanceCriteria` | fixed at intake, in the vocabulary of the symptom, never edited after a candidate exists |
| **Definition of Ready** | the admission conditions | criteria complete · no conflict · an allowance exists · a grant no wider than scope can be issued · an evidence plan where one is needed |
| **Definition of Done** | verdict `met` on every criterion, then landed | on evidence produced independently of the executor, not on someone having read it |
| Test plan | `EvidencePlan` | written from the area of effect, after diagnosis, before the repair |
| Tests, QA | the **arbiter** | outside the scope by construction; a change may not edit what judges it |
| Code review | not the mechanism of acceptance | a person may **refuse**; approval by reading is not what makes a change accepted |
| Ticket closed | `Intent` → satisfied | a different event from any one change being accepted |

The two rows that will feel strangest to an enterprise reader are the last two, and they are the point. Acceptance rests on evidence rather than on review, because review does not scale and cannot be measured; and a ticket closing is not the same event as a change being accepted, because one ticket legitimately produces several.
