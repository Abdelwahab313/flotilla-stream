---
name: design-system
description: The Flotilla Console design system — tokens by role, the Cargo and Ops Grey modes, type scale, shape, map symbology, motion, writing rules, component inventory and screen states. Use whenever a console screen, component, map layer or UI string is designed, built or reviewed, or the user asks "what should this look like".
---

# Console design system

Sources, in order of authority for **values**:
1. `docs/design/console/README.md` — the system; its Open items table lists PRD conflicts.
2. `docs/design/console/tokens.css` — every colour, font and shell size.
3. `mockups/boards/<board>/index.html` — decoded mockup markup; read the board for the
   screen at hand before proposing layout. Ignore `{{…}}`, `<sc-if>` and `<sc-for>`:
   they are the design tool's templating, not a component API.

For **meanings** (statuses, lifecycle, permissions, flows, budgets):
`docs/product/CONSOLE-PRD.md` and `CONTEXT.md`. Use the glossary's words in UI copy and
code; never its _Avoid_ words.

## Rules that every UI change follows

- **Tokens by role only.** `var(--critical)`, never `#F94E4C`. No new token without a
  role name and a value in both `.t-bridge` (Cargo) and `.t-chart` (Ops Grey).
- **Both modes.** Every screen, state and map symbol works in Cargo and Ops Grey.
- **Never colour alone.** Status pills carry text; map symbols differ in shape, fill or
  motion (symbology table in the design-system doc).
- **Type.** UI family for prose and labels; mono for ids, refs, timestamps, numeric
  columns; tabular figures everywhere; labels in sentence case.
- **Every state designed.** Loading (skeleton for lists), empty (reason + next action),
  error (what failed + retry), connection lost ("as of 14:02:11"), read-only (visible,
  disabled, "Read-only demo" tooltip), pending ("Cancelling…", then "Still waiting for
  dispatch…" at 5 s), race outcome ("Not cancelled — accepted by D-118 at 14:02:11").
- **Lists never jump.** Live rows update once per second; while scrolling or hovering,
  updates wait behind "n updates — show".
- **Freshness visible.** Every live surface shows its age; stale and lost drivers are
  marked in every row and symbol.
- **Keyboard.** Row actions map to PRD §10.13 keys; focus ring `2px solid var(--accent)`.
- **Motion.** Only gliding, the offer pulse, row settle, panel slide. Reduced motion:
  step, static ring, no shimmer.
- **RTL-ready.** Logical CSS properties; no string concatenation for sentences; all
  strings externalized.
- **Toasts.** At most 3; critical persists and sounds once; warnings never toast; toasts
  never cover the selected subject or the context panel's actions.

## When the mockup and the PRD disagree

The mockup wins on look and layout (owner rule, 2026-11-03): build it and update
the PRD to match. The PRD wins on domain meaning and wording (statuses, lifecycle, permissions, glossary terms, time formats); if
the mockup contradicts one, add a row to the Open items table in
`docs/design/console/README.md` and ask the owner.

## When a screen has no board

S3 Drivers, S6 Events, S8 Imports/exports and S9 Settings have no board yet. Compose
them from existing components and the closest board (dock tables in S1, S2 Orders
table), state that the layout is derived, and list the decisions made so the owner can
review them.
