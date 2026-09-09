# Assurance — placeholder-filter

## Evidence

| Id | Subject | Kind | Producer | Independence | How established | Obtained at | Valid until |
|---|---|---|---|---|---|---|---|
| EV-002-01-01 | candidate | test-run | tests/test_row_filter.py | independent-by-precommitment | authored under WU-002-01 and its failure on the unmodified gate recorded before WU-002-02 existed; it could not have been shaped to fit a repair that did not yet exist | 2026-09-09 | next change to tools/check.py |
| EV-002-01-02 | candidate | test-run | tests/test_row_filter.py, over templates/ | independent-by-executor | the templates are outside the scope of both units, so neither executor could alter what it is measured against | 2026-09-09 | next change to templates/ |
| EV-002-02-01 | candidate | test-run | the gate at HEAD, run against the existing changes | not-independent | the gate is the artefact under change; running it to attest that its own behaviour did not move is circular | 2026-09-09 | — |
| EV-002-02-02 | candidate | test-run | baseline/gate-row-filter.txt | independent-by-precommitment | the prior gate is fixed in version control and predates the candidate; the comparison isolates the row-filter repair as the only variable between the two runs | 2026-09-09 | next change to tools/check.py |

## Verdicts

| Id | Unit | Criterion | Outcome | Evidence | State | Window | Baseline |
|---|---|---|---|---|---|---|---|
| VE-002-01 | WU-002-01 | CR-002-01 | met | EV-002-01-01 | settled | | |
| VE-002-02 | WU-002-01 | CR-002-02 | met | EV-002-01-02 | settled | | |
| VE-002-03 | WU-002-02 | CR-002-03 | met | EV-002-02-02 | settled | | |

## Escalations

| # | Standing | Ground | Raised against | To |
|---|---|---|---|---|
| 1 | closed | CR-002-03 cannot be decided: the only evidence available is produced by the artefact under change. A recorded output from the unmodified gate would have decided it, and none was captured before the repair | area 5 | Illia Gannytskyi |
| 2 | closed | Invariant 3 says evidence must not be produced by the agent that produced the candidate. Here one agent produced both, and independence rests instead on pre-commitment — the arbiter was written, and its failure recorded, before the repair existed. If that counts, invariant 3 is worded too narrowly. Deciding this is a change to the framework, arbitrated outside iFlow under R9 | framework | Illia Gannytskyi |
| 3 | closed | Execution wrote into the working tree because this increment has no candidate store, so invariant 2 is unenforced and R2's stated check does not exist in the gate | framework | Illia Gannytskyi |
| 4 | closed | WU-002-02 landed while one of its criteria was `undecided`, and the gate allowed it: nothing in the research document forbids entry on an undecided verdict, and the gate only blocks `failed`. But an undecided criterion means acceptance was not established, which is what the hypothesis requires. Raised during landing | framework | Illia Gannytskyi |

All four were arbitrated by the Licensor under R9 and applied to the research document as Y1-Y4. CR-002-03 became decidable once the prior gate was recovered from version control, which is why it now reads `met` rather than `undecided`.
