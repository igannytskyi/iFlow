# Specification — greet-the-world

| Field | Value |
|---|---|
| Id | SPEC-001 |
| Intent | INT-001 |
| Default class | C2 |

Asked to greet nobody, the greeting today says "Hello, !". A defect repair: the wrong behaviour is made right and nothing else moves, so a reproduction is written first and must fail against the greeting as it stands.

## Acceptance criteria

| Id | Criterion | When | Then | Plan | Procedure | Required evidence | Threshold |
|---|---|---|---|---|---|---|---|
| CR-001-01 | Nobody named, the world is greeted | a greeting is asked for with no name | it reads "Hello, world!" | required | machine | test-run | exactly that text |
| CR-001-02 | A person named is still greeted by name | a greeting is asked for with a name | it reads "Hello, " then the name then "!" | not-required | machine | test-run | exactly that text |
| CR-001-03 | Nothing else in the package moves | the whole suite is run | it stands as it did before | not-required | machine | test-run | identical outcome |

## Termination

| Condition | Action |
|---|---|
| greeting the world needs a change to how a name is passed in | stop |

## Scope

| Included | Excluded |
|---|---|
| greeting.go | greeting_test.go |

## Arbiter

| Path | What it arbitrates |
|---|---|
| greeting_world_test.go | CR-001-01 |
| greeting_test.go | CR-001-02, CR-001-03 |

## Arbiter inputs

| Input | In scope? |
|---|---|
| greeting.go | yes — the reproduction judges the greeting, which is what changes |
