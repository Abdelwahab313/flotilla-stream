---
name: record-decision
description: Fill in a decision record in docs/decisions/ — Context, Options considered, Decision, Why, Consequences, How it was tested — from the current slice's work, then have design-critic review it. Use when the user says "write ADR 7", "record the decision", "document why we chose X".
---

# Record a decision

1. Open the stub `docs/decisions/00NN-*.md`. Decision, Why and Consequences are
   pre-filled from the product plan; keep them unless the slice changed them, and
   if it did, say what changed and why in Context.
2. Write Context from what actually happened in the slice: what broke or would
   break without the decision, with the concrete sequence of events.
3. Write at least two real alternatives under Options considered and the reason
   each loses here (not in general).
4. Write How it was tested: the scenario in `docs/results/Rn.md` that exercised
   it, and the result.
5. Run the `design-critic` agent on the file; apply ACCEPT-WITH-CHANGES items;
   leave RESHAPE findings as an open question at the bottom and tell the owner.
6. Status stays `proposed` until the slice closes.
