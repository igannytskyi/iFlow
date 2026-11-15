# Specification — trim-name-whitespace

| Field | Value |
|---|---|
| Id | SPEC-002 |
| Intent | INT-002 |
| Default class | C3 |

Greet currently uses the name exactly as passed, so a name padded with spaces (e.g. " Ada ") is greeted with the spaces intact, and a name made only of spaces is treated as present rather than as absent. The fix trims surrounding whitespace from the name before deciding whether it is empty and before using it in the greeting, so padded names are greeted cleanly and whitespace-only names fall back to the "world" greeting, matching how an empty name already behaves.

## Acceptance criteria

| Id | Criterion | When | Then | Plan | Procedure | Required evidence | Threshold |
|---|---|---|---|---|---|---|---|
| CR-002-01 | A name padded with leading or trailing whitespace is greeted using the name with that whitespace removed | Greet is called with a name that has one or more spaces before and/or after the meaningful name text | the returned greeting contains the name with the surrounding whitespace stripped, in the form "Hello, <trimmed name>!" | required | machine | test-run | exact string match for several padded inputs, including names with internal spaces (e.g. "  Grace Hopper  ") |
| CR-002-02 | A name consisting only of whitespace is treated as absent | Greet is called with a name made up entirely of one or more whitespace characters | the returned greeting is "Hello, world!", the same as for an empty name | required | machine | test-run | exact string match for several whitespace-only inputs of varying length and composition (spaces, tabs) |
| CR-002-03 | nothing else moves | the existing suite is run after this change | every existing test still passes with its original pass/fail outcome | not-required | machine | test-run | test.sh run with no argument exits zero |

## Termination

| Condition | Action |
|---|---|
| test.sh passes with the new and existing tests in place | stop |
| test.sh still fails after a reasonable, directly-related fix to greeting.go has been tried | escalate |

## Scope

| Included | Excluded |
|---|---|
| greeting.go | greeting_trim_test.go |
|  | greeting_test.go |
|  | greeting_world_test.go |

## Arbiter

| Path | What it arbitrates |
|---|---|
| greeting_trim_test.go | CR-002-01, CR-002-02 |
| greeting_test.go | CR-002-03 |
| greeting_world_test.go | CR-002-03 |

## Arbiter inputs

| Input | In scope? |
|---|---|
| greeting.go | yes — what the tests judge is what changes |
