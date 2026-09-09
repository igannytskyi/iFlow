# Intent — clock-instant

| Field | Value |
|---|---|
| Id | INT-001 |
| Stated by | platform team |
| Deciding authority | head of platform |
| Priority | normal |
| Deadline | before the library's next major version |
| Statement | No service reads the wall clock through the deprecated call; every place that asks for the current instant does so through the supported one. |
| Status | open |
