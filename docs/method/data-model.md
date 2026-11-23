# The data model

Every document a change keeps about itself has one type, in [`internal/meta`](../../internal/meta/meta.go). The type is the definition: every field has a name, a type and a description, every code is one of the values [conventions.md](conventions.md) defines, and every identifier has the form conventions.md gives it. The JSON Schema in [`schema/`](schema/) is generated from the types and a test fails when the two differ.

## What is kept, and where

| Document | Kept as | Type | Schema |
|---|---|---|---|
| The intent | `00-intent.json` | `meta.Intent` | [00-intent](schema/00-intent.schema.json) |
| How the change is judged | `01-specification.json` | `meta.Specification` | [01-specification](schema/01-specification.schema.json) |
| The units of work | `02-plan.json` | `meta.Plan` | [02-plan](schema/02-plan.schema.json) |
| What was admitted, on what | `03-admission.json` | `meta.Admission` | [03-admission](schema/03-admission.schema.json) |
| Each unit carried out | `04-execution.json` | `meta.Execution` | [04-execution](schema/04-execution.schema.json) |
| Evidence and verdicts | `05-assurance.json` | `meta.Assurance` | [05-assurance](schema/05-assurance.schema.json) |
| What entered the estate | `06-landing.json` | `meta.Landing` | [06-landing](schema/06-landing.schema.json) |
| Every crossing | `record.jsonl`, one a line | `meta.Crossing` | [record](schema/record.schema.json) |
| What each piece of work cost | `costs.jsonl`, one a line | `artifact.Cost` | — |

The JSON is the record. Beside each document its Markdown — `00-intent.md` and the rest, `record.md` — is rendered from it for a person to read, and is never read back: editing it changes nothing the gate or the workbench sees. The objects the method hands on — the specification described in `change.md`, the plan in `plan.md`, the tickets in `units/`, the candidates, the evidence's output — are documents for people and agents, described in [objects.md](objects.md); this is their record.

## Writing

A document is written with `artifact.Save`, which checks it against the model first and refuses it whole if it breaks the model: a code that is not one of its values, an identifier not in its form, a schema version this program does not write. Then its JSON and its Markdown are each replaced at once, so that no reader sees half of either.

The record is appended to with `artifact.AppendRecord`: each crossing is numbered after the last as it is written, and `record.md` is rendered anew from the whole record. Nothing rewrites the record.

## Reading

`artifact.Load` reads a change into `Change.Meta`, one field per document. JSON that does not decode — a field the model does not have, a value of the wrong type — is an error, not a variation. The gate's rules and the workbench read `Change.Meta` and nothing else.

What the gate judges is what the documents mean: whether every criterion has an arbiter, whether what entered was met. Whether a document has the shape of the model is checked first, by the same validation `Save` applies, and reported under `CONVENTIONS`; a document that cannot be read whole is a `B1` finding.

## Versions and migration

Every document carries `schema`, the version of the model it was written in. A document's version is its own: a plan is version 2 — a unit says what it writes (`kind`: contract, tests, code), its contract is named entries (`provides`: file, name, kind, signature) and its dependencies are names (`uses`) — and every other document is version 1. A document of an older version is read into the current one whole, nothing dropped, what the older model left unsaid kept as unknown (a provision version 1 wrote in prose, with no one file it could be in, is `described`); `iflow migrate` writes it so, keeps the older JSON under `legacy/`, and says so in the record. Raising one document's version leaves the others — and with them the specification's digest — as they were. A change written before the model kept its documents as Markdown only. `iflow migrate [--dry] <changes>` reads such a change into the model once:

- every table and section of every document lands in a field, or, where the model has no field for it — the population and hypothesis of a study's specification — in the document's `sections`, kept as written;
- what the Markdown says that the document read from it does not — a cell, a line of prose — stops the migration of that change, which is left as it is and named;
- the Markdown is kept, word for word, under `legacy/`, the JSON written, the Markdown rendered anew, and the record says so with a `migrated` crossing that fixes the digest of the specification as it now is.

R1 holds the specification fixed from admission: the digest taken at admission is of `legacy/01-specification.md` for a change admitted before the model, and the `migrated` crossing carries it on to the JSON. Until it is migrated, a change is read through the same decoder, so the gate and the workbench see it in the model either way.

## Changing the model

A field is added to the type with its `json` name and its `doc` description; a field a document may leave out is `omitempty`, any other is required. A new code is added to `meta.Codes` and to conventions.md together. Then the schema is written anew — `IFLOW_WRITE_SCHEMA=1 go test ./internal/meta` — and a change of shape or meaning raises that document's version (`meta.Version`) and teaches the reader (`artifact.upgrade`) and `Migrate` to carry the documents of the version before it.
