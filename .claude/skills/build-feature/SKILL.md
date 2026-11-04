---
name: build-feature
description: Run the feature team on one item of the open slice — ux-designer specs any UI, implementer builds it, code-reviewer (plus invariant-auditor for services/checker/simulator) reviews, fixes loop at most twice. Use when the user says "build X", "implement the next slice item", "ship this feature", or "have the team do Y".
---

# Build a feature with the team

Subagents cannot call each other; this session runs the steps and passes outputs along
verbatim.

1. **Scope.** Read the open `docs/releases/Rn.md`. Name the In-list item being built.
   If the request is not in the In list, stop: add it to Deferred and tell the user.
2. **Design (only if the item touches the console).** Run `ux-designer` with the item,
   the slice file path and the PRD sections. Keep its UI spec. If it raises open items
   the owner must decide, ask the user before building.
3. **Plan.** Run `slice-planner` if the slice has no sitting plan yet; otherwise skip.
4. **Build.** Run `implementer` with: the item, the slice path, the UI spec (verbatim),
   and the planner step if any.
5. **Review, in parallel.** Run `code-reviewer` on the diff. If the diff touches
   `services/`, `checker/` or `simulator/`, also run `invariant-auditor`. If it touches
   the console UI, also run `ux-designer` in review mode against the built page.
6. **Fix loop.** Send blockers and majors back to `implementer` verbatim. Re-review
   only the changed files. At most two rounds; then stop and report what is still open.
7. **Report** to the user: the item, files changed, tests run with results, review
   verdicts, open items and deferrals. Do not commit unless the user asks. Numbers go
   through `bench-runner`, not this skill.
