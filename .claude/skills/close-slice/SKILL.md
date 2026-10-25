---
name: close-slice
description: Close the open release slice — run the release-gatekeeper audit, promote verified numbers to README, flip ADR statuses to accepted, mark the slice closed, tag v0.<n>. Use when the user says "close R2", "tag the release", "is this slice done".
---

# Close a slice

1. Run the `release-gatekeeper` agent on the open slice. If the verdict is NOT
   YET, report the missing list and stop. Do not tag.
2. Promote numbers: copy each verified number from `docs/results/Rn.md` into the
   README headline table with its release, keeping the command reference.
3. Flip the slice's ADRs from `proposed` to `accepted` in each file and in
   `docs/decisions/README.md`.
4. Set `status: closed` in `docs/releases/Rn.md`, record the sitting count, update
   `docs/releases/README.md` and `demo/README.md` status columns.
5. Commit with a message that names the release and its headline number, then
   `git tag v0.<n>`. Do not push unless asked.
6. Suggest the finding worth a post: one number, what broke, the fix.
