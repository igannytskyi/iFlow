# Execution — <name>

| Unit | Executor | Version | Terminal state | Candidate | Trace |
|---|---|---|---|---|---|
| WU-<nnn>-01 | <deterministic transformation, or agent> | | candidate · terminated · exhausted · failed-substrate · failed-task · cancelled | CA-<nnn>-01 | trace/<id>.md |

Where the class admits a deterministic transformation, using an agent is a defect: it makes a reproducible result unreproducible and costs more.

`failed-substrate` says nothing about the work. `failed-task` says something. They are retried differently.

## Candidates

| Id | Unit | Stored at | Artefacts | Produced at |
|---|---|---|---|---|
| CA-<nnn>-01 | WU-<nnn>-01 | candidates/CA-<nnn>-01/ | <what is proposed, never applied here> | |

A candidate is written under `candidates/<id>/` and nowhere else. Execution that writes into the live tree leaves invariant 2 with nothing to enforce it.

## Arbiter acceptance

Required where a unit builds an arbiter: how that arbiter behaved before the repair, per criterion. A check that could not fail is not evidence of anything.

| Unit | Criterion | Before repair |
|---|---|---|
| WU-<nnn>-01 | CR-<nnn>-01 | failed · passed |
