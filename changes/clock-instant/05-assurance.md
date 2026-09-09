# Assurance — clock-instant

## Evidence

| Id | Subject | Kind | Producer | Independence | How established | Obtained at | Valid until |
|---|---|---|---|---|---|---|---|
| EV-001-01-01 | candidate | static-analysis | source index, run by the gate | independent-by-executor | produced by the index, not by the executor that made the change | 2026-09-09 | next landing touching src/billing |
| EV-001-01-02 | candidate | test-run | CI, on tests/ which the grant excludes | independent-by-executor | the arbiter is outside the scope, so the executor could not have altered it | 2026-09-09 | next landing touching src/billing |

## Verdicts

| Id | Unit | Criterion | Outcome | Evidence | State | Window | Baseline |
|---|---|---|---|---|---|---|---|
| VE-001-01 | WU-001-01 | CR-001-01 | met | EV-001-01-01 | settled | | |
| VE-001-02 | WU-001-01 | CR-001-02 | met | EV-001-01-02 | settled | | |
