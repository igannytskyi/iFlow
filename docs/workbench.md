# The workbench

`iflow desk`: where a person states intents, reads what agents wrote, decides, and watches the work go along the path.

## Goals, in the order they win

1. **Instant.** Every action is answered in the next frame, and a frame takes well under a millisecond. There is no compromise on this and no exception for any view.
   - A frame does no input or output and no heavy work. It lays out what is already in memory.
   - What the window shows from disk is read once and kept until something changes it — a project read again, a run's step, a check's answer — and only then read again.
   - Anything slow — reading a project, asking the gate, reading the estate, a model's answer — runs beside the window, never in it, and asks for a frame when it is done.
   - Whatever a person does — a click, a key, a word typed or dictated — asks for the next frame at once, rather than waiting for the next event that happens to come.
   - A test holds every view of a real project under its budget, and fails the build when one is not.
2. **The work, not a dashboard.** A task is its graph, then its specification, then the metadata of the area chosen on the graph. Nothing is shown that the person cannot act on or does not need to decide.
3. **Everything in view that costs.** Money, tokens and what each agent was handed are always beside the task.
4. **The method where it applies.** Its words appear where they are needed, never as a page to read first.

## Layout

- Projects on the left; under the chosen one, a new intent, the tasks still active, and the finished ones folded.
- A project chosen shows the project: its figures, its tasks, what is next, and areas 7 to 13.
- A task chosen shows its graph, its specification — the object a person reads, changes or sends back — and the metadata of the area chosen on the graph.
- Beside it, what the task and the project cost, and what each agent was handed.
- At the bottom, always, the box for the next task.
