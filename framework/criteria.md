# Arriving at a criterion

The method checks that a criterion is decidable and written in the vocabulary of the intent. It does not help anyone write one, and that is the gap with the largest consequence: acceptance establishes conformance and never correctness of intent, so a bad criterion is accepted correctly and nothing downstream can catch it.

What follows is not a menu of postures. Each probe names one way a criterion goes wrong, and each can be answered **before any work exists** — a question answerable only afterwards is a review, not a technique.

## The probes

**Could this be satisfied by editing the thing it names?** If the criterion mentions a file, a function, a flag or a table that the change may touch, it can be met by changing that thing and nothing else. Say what must be observably true instead. *This one is checked mechanically, crudely — the crudeness is enough, because the mistake is not subtle.*

**Can you describe two materially different results that both satisfy it?** If you can, the criterion under-determines the outcome, and which of the two you get is left to whoever executes. Two executors disagreeing on the same unit is the same defect discovered late; asking now is cheaper.

**Does it name a solution rather than an outcome?** A criterion that forecloses every approach but one is a design decision wearing a criterion's clothes. It may be the right decision — but it belongs in the plan, where it can be argued with, not in the criteria, where it cannot.

**What observation would show it unmet?** If nothing would, it is not a criterion. This is the same test the method applies to accumulated knowledge, which is refused at capture when nothing could refute it.

**What must stay true that this does not mention?** Most criteria say what should change and forget to say what should not. The second half is where defect repair fails: the symptom stops and something else quietly breaks.

**What would settle it, and does that exist?** If the evidence must be built before the criterion can be judged, that is preparatory work, and knowing it now is much cheaper than discovering it at admission.

**Suppose it passed and the intent was still unserved. How?** The only probe aimed at the irreducible failure — the right answer to the wrong question. It cannot be removed, and asking the question is the only thing that lowers its rate.

## What is checked and what is not

Only the first probe is mechanical, and only in its crudest form: a criterion whose text names a path the change may touch is reported. The rest are questions for whoever writes or reviews the criteria, and the method says so rather than pretending otherwise — a rule claiming an enforcement it lacks is worse than an acknowledged gap, because it is relied upon.

The cheapest moment to ask all seven is before admission, when nothing has been spent and the criteria are still free to change. Whether anyone did is recorded, so that whether it helps can eventually be answered rather than assumed.
