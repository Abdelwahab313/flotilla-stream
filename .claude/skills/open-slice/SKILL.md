---
name: open-slice
description: Open the next release slice — verify the previous one is closed and tagged, create docs/releases/Rn.md from the product plan with In/Out, exit-criteria checklist, ADRs due, demo script name and required numbers, and mark it open in the releases index. Use when the user says "open R3", "start the next slice", "next release".
---

# Open a slice

1. Confirm the previous slice is closed: its file has every checkbox ticked, the
   tag `v0.<n>` exists (`git tag --list`), and `docs/releases/README.md` shows it
   closed. If not, stop and say what is missing — never open a slice on top of an
   unfinished one.
2. Read the slice's section in `docs/product/PLAN.md` (In, Out, Demo, Exit) and the
   decisions it must record.
3. Create `docs/releases/Rn.md` using `docs/releases/R0.md` as the shape:
   title with size and `status: open`; In; Out; Sittings (planned, walking
   skeleton first); Exit criteria as checkboxes — always including the demo
   script, the numbers with their second derivation, the ADRs, the
   `docs/results/Rn.md` file, the sitting count and the tag; empty Deferred and
   Blockers lists.
4. Add the row's status `open` in `docs/releases/README.md`; add `demo/rN.sh` to
   `demo/README.md` if missing.
5. Hand off to the `slice-planner` agent for the first sitting plan.
