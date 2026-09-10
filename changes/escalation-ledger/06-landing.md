# Landing — escalation-ledger

| Order | Unit | Verdict state | Evidence still valid | Entered at | Invalidated by this entry | Re-established before next |
|---|---|---|---|---|---|---|
| 1 | WU-007-01 | settled | yes | 2026-09-10 | none | none |
| 2 | WU-007-03 | settled | yes | 2026-09-10 | none | none |
| 3 | WU-007-04 | settled | yes | 2026-09-10 | none | none |
| 4 | WU-007-02 | settled | yes | 2026-09-10 | CR-003-01, whose arbiter scanned only the per-folder failure path | re-established by WU-007-04 |

WU-007-02 invalidated a verdict standing from an earlier change: R15 is emitted from the ledger rather than from the per-folder gate, and that change's arbiter took the per-folder path for a proxy of enforcement. **The invalidation was found after entry rather than before**, because the candidates were applied before every standing arbiter had been run. That is a defect of this run, recorded rather than smoothed over, and it is escalation 2.

## Unplanned state

| Occurred | Area of effect halted | Resolved by |
|---|---|---|
| no | | |
