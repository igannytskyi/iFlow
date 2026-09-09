# Execution — <name>

| Unit | Executor | Version | Terminal state | Candidate | Trace |
|---|---|---|---|---|---|
| WU-<nnn>-01 | <deterministic transformation, or agent> | | candidate · terminated · exhausted · failed-substrate · failed-task · cancelled | CA-<nnn>-01 | trace/<id>.md |

Where the class admits a deterministic transformation, using an agent is a defect: it makes a reproducible result unreproducible and costs more.

`failed-substrate` says nothing about the work. `failed-task` says something. They are retried differently.

## Candidates

| Id | Unit | Artefacts | Produced at |
|---|---|---|---|
| CA-<nnn>-01 | WU-<nnn>-01 | <what changed, never applied here> | |
