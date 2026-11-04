# flotilla-stream — working rules for agents

Real-time location and dispatch event platform. Portfolio project: the goal is
evidence, not features. Every claim in `README.md` is backed by a test in this
repo that anyone can rerun.

## Where the truth is
- `CONTEXT.md` — the glossary. Use its terms; its _Avoid_ words stay out of code and docs.
- `docs/product/PLAN.md` — the product plan: goals, invariants, seventeen decisions, event catalog, evidence plan.
- `docs/product/CONSOLE-PRD.md` — the console (product UI) requirements: roles, flows, screens, map, theme, budgets.
- `docs/design/console/` — the console design system (values: tokens, type, symbology) and its open conflicts with the PRD; `mockups/boards/` holds the decoded mockup markup it was taken from.
- `docs/releases/README.md` — the release plan (R0–R10 plus C1, C2; eighteen slices) with sizes and order.
- `docs/releases/Rn.md` — the **open slice**: In/Out, exit-criteria checklist,
  decision records due, demo script, numbers required. Only one is `open` at a time.
- `docs/decisions/` — ADRs 0001–0017, numbered as the design decisions. Written
  in the release that introduces them.
- `docs/results/` — measured numbers per release with the exact command that produced them.

## Rules
1. **Work only inside the open slice.** Read `docs/releases/Rn.md` first. Anything
   not in its In list is out, even if small. Write it in the slice's "Deferred" list instead.
2. **Walking skeleton first.** The new path exists end to end with the dumbest
   implementation before anything is made correct, then measured.
3. **Measured before declared.** A number goes in `README.md` only with the command
   that produced it, and only after a second independent derivation (application
   metric vs. log/store count). Disagreement is written down as a finding.
4. **The checker is the judge.** `checker/` asserts the seven invariants after every
   test, after quiescing (load stopped, consumers at high-water marks). A test
   passes only if the checker passes.
5. **At-least-once, idempotent everywhere.** Every consumer has a written
   idempotency rule (sequence, upsert, dedupe key). No exactly-once claims.
6. **Go for the backend, TypeScript for the console.** Go for services, simulator,
   checker and every resource-intensive workload; no second backend runtime without
   an ADR. The console is TypeScript + React + MapLibre under `web/`, built with Vite;
   Node is a build and dev tool only, and a Go binary serves the static output
   (ADR 0017). The R0 map is static HTML + MapLibre.
7. **Same manifests everywhere.** `deploy/` with Kustomize overlays `local` (kind)
   and `gke`. Argo CD syncs `main`. No hand `kubectl apply` after R0.
8. **ADR format** (one page): Context · Options · Decision · Consequences · How it
   was tested. Status: proposed → accepted → superseded.
9. **Slice close-out**: checklist ticked, `demo/Rn.sh` runs from clean checkout,
   numbers in README, ADRs merged, `docs/results/Rn.md` written, tag `v0.<n>`.
   If a slice cannot close, write the blocker in `docs/releases/Rn.md` — never
   open the next one.
10. **Comments**: none by default. Only for a non-obvious why.

## Agents and skills in `.claude/`
- `/open-slice` → creates `docs/releases/Rn.md` from the plan; `/close-slice` → gatekeeper audit, promote numbers, tag; `/record-decision` → fill an ADR and have it critiqued.
- `slice-planner` — sitting plan for the open slice (read-only).
- `release-gatekeeper` — can the slice close? Evidence-based pass/fail (read-only, runs demos).
- `invariant-auditor` — correctness review of any diff under `services/`, `checker/`, `simulator/`.
- `bench-runner` — runs scenarios, derives every number twice, writes `docs/results/Rn.md`.
- `platform-engineer` — `deploy/` and `infra/` work, time-boxed to one sitting per tooling problem.
- `design-critic` — adversarial review of ADRs and L-sized slice plans.

Feature team, run in order by `/build-feature`:
- `ux-designer` — specs any console change against the design system and mockups; reviews built UI; writes only `docs/design/console/`.
- `implementer` — builds one open-slice item end to end (Go or console), walking skeleton first, with tests.
- `code-reviewer` — correctness, consistency, scalability, performance, security, tests, scope; pairs with `invariant-auditor` on `services/`, `checker/`, `simulator/`.

Stack skills, preloaded by those agents: `design-system`, `console-frontend`, `console-map`, `go-service`.

## Commands (fill in as they exist)
- `make demo-r0` — clean checkout to moving dot.
