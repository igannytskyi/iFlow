# Why the defects kept coming

From 11-21 to 11-23 the real-size bench (two tasks on this repository) was run about a dozen times, for about $6. Every run stopped on a defect of the framework, not of the method or the models, and nearly every run found a new one. This is why, and what is done about it.

## The defects, by class

| Class | What it is | Found |
|---|---|---|
| A. A fact about code read from its text | qualified names (P8, P10); P7 by file, not package; `relies_on` by a declaration's first line — a method by its type, then an interface's method; fuzz tests missed by a pattern; Python constants; the contract's imports | 8 |
| B. A tool's output read as prose | failed tests by `--- FAIL`; a panic's test; a path as the copy held it; a panic ending the run taken for tests not found | 4 |
| C. A handoff not typed | which test judges which criterion; a scope fault sent to the planner; a unit not waiting for what it reads; a criterion judged only at the end | 5 |
| D. A bound set by guess | P13 counting what a unit only reads; 250k tokens over all turns; compaction after eight messages; a turn of 48k | 4 |
| E. Feedback an agent cannot act on | one break told at a time; a refusal without its reason; a prompt whose format the author copied into a name | 3 |

## Why they kept coming

1. **The framework's tests test what I imagined, not what happens.** The units are carried in them by stub agents that write the files and the ledger exactly right, on a twenty-line estate whose tests never panic, whose names are never an interface's method, whose authors never copy a hint into a field. All of them pass, and say nothing about the space a real model and a real repository open. That space was first met on the paid bench.
2. **A run stops at its first defect.** Each paid run of twenty minutes showed one or two defects; the ones behind them stayed hidden until the next run. Discovery was serial and paid.
3. **Each defect was mended where it surfaced, not across its class.** Qualified names were mended for P8 and not for `relies_on`; `relies_on` was read by its text, then by its owner's text, then by Go's parser — three runs for one class. Every row of the table above was a class from the start.

## What is done

1. **Close each class, not each instance.** For each class, every place of that kind in the code is listed below and brought to the sound form at once.
2. **Replay** (done 11-23, `iflow bench --replay`). Every model answer of a bench run is recorded, by role, unit and attempt. `iflow bench --replay <run>` carries the task again against the framework as it now stands, with those answers, on the real repository and the real tests: free, deterministic, in a minute. Every run that ever failed becomes a test, run in CI. A paid run is made only once every recorded run replays; it is then a measure of the models, not a search for the framework's defects.
3. **The author is handed declarations as data.** What it may rely on is handed as `{file, name, kind, of, signature}`, the shape `relies_on` is written in, never as lines to transcribe.

## What is left of each class

| Class | Place | Sound form |
|---|---|---|
| A | `estate.Signature`, `declares`, `chosen`: a declaration by its first line, for languages other than Go | the estate's own syntax trees (tree-sitter), which it already holds for every language it reads |
| A | `testsIn`: tests found by a pattern for Python, JavaScript, Rust, Java | the same trees; Python by its own ast, as its declarations are |
| A | `goMissing`, `goUndefined`, `goNoMember`: the compiler's messages read by patterns | kept: `go build -json` carries the same `file:line:col: message` lines, and the message is the compiler's documented diagnostic, the only form it gives |
| B | `fault` per language: where a build fails, by `file:line:col` | kept: the one form every compiler shares |
| B | `noneRan`: a runner that writes no report says it ran nothing | kept, for cargo and a script of the estate's own, which report nothing else |
| B | `failing` in the harness: which lines of a check to show an agent | presentation to a model, not a fact the framework acts on; kept |
| E | the author's hint lists declarations as `file:line  signature` | done 11-23: handed as `relies_on` entries, read by go/ast |
