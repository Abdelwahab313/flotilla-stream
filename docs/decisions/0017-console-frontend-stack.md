# 0017 — Console frontend stack

- Status: proposed
- Introduced in: R2

## Context
Go is the runtime for the backend and every resource-intensive workload (services, simulator, checker). The console is a product UI (CONSOLE-PRD, mockups in `mockups/boards/`): a live map with thousands of moving drivers, virtualized tables, drawers, a command palette, RTL-ready layouts, and performance budgets that are measured (frame time, receive-to-paint latency, heap over a soak). It needs a typed frontend stack with a large ecosystem of accessible components; the owner accepts Node as a frontend build and dev tool for that.

The state the console tracks has three very different rates: driver positions (up to ~5,000 in view, reports every few seconds, drawn at 60 fps), entity state (orders, drivers, offers, incidents; once per second for lists), and UI state (selection, panels, theme; on user input). A framework that pushes all three through one render path breaks the frame budget, whichever framework it is.

## Options considered
- **Vanilla TypeScript, no framework.** No dependency; every list, dialog and live update re-invented; hard to keep consistent across two modes and four roles.
- **Preact with signals, bundled by esbuild from Go.** About 4 KB, and fine-grained updates. Its main advantage, keeping Node out entirely, no longer matters once Node is allowed for the build. Its ecosystem is smaller, and React libraries run through `preact/compat` with occasional friction, so the virtualized table, palette and bottom sheet would be built in-house.
- **Next.js.** Server rendering is of no use for a live, signed-in SPA, and it would put a Node server in production next to the Go gateways.
- **React 19 + Vite.** Largest ecosystem: React Aria Components (accessible dialogs, tabs, combobox, menus, with RTL through `I18nProvider`), TanStack Virtual, Testing Library; the mockups are React prototypes, so design and code share a vocabulary. React re-renders coarsely, which matters only on paths this design keeps out of React.

## Decision
TypeScript, React 19, MapLibre GL. Vite 8 builds and serves in development; Node (pinned LTS, 20.19+ or 22.12+ as Vite 8 requires) is used only at build and dev time. The build output is static files served by a Go binary, so production runs no Node. Dependencies: React Aria Components for interactive primitives, styled with CSS Modules over `tokens.css`; TanStack Virtual for lists; nothing else without a reason in review. pnpm with a frozen lockfile. Type checking with `tsc --noEmit` (TypeScript 7, the native compiler), `strict`. Tests with Vitest 5 (unit, plus browser mode through Playwright for components) and Playwright for flows and performance runs.

Client state is split by rate, and React sees only the slow tiers:

1. **Transport.** One WebSocket per window, opened and decoded in a Web Worker. A connection's message sequence is local to that connection, so it detects gaps only within it; every reconnect (including failover to another gateway replica) resyncs.
2. **Versions and idempotent apply.** Each entity's events live in one topic (PLAN Event catalog § Topics). An entity's version is the offset of the last event applied to it within that topic and partition, which the live gateway forwards with every event; a delta whose offset is not above the held one is dropped. The live gateway reads with `read_committed`, so a fenced old owner's writes never reach the console (decision 13). Positions use the device (session, seq) from decision 4, stored beside each slot, so a replayed report never moves a dot backwards.
3. **Resync.** Subscribe first and buffer deltas; fetch a snapshot stamped with its cut offset per partition; apply the snapshot as a replacement of its scope (city, cells or list), not a merge, so deletions take effect; then replay buffered deltas above the cut. A viewport pan does the same for newly subscribed cells. The gateway sends an explicit "left" when a driver leaves the subscribed cells. The UI shows "as of" until resync completes.
4. **Position store.** Driver positions live in typed arrays (struct of arrays, one slot per driver, slots returned to a free list when a driver leaves scope), outside React. The map layer reads them in its render loop; no component renders per position.
5. **Entity store.** Normalized maps keyed by id, outside React. Deltas accumulate and flush once per second into a new immutable snapshot per view; components read it through `useSyncExternalStore` with a selector, so a component renders only when its slice changed. The hover/scroll hold behind "n updates — show" applies only to list order and membership. Command outcomes, connectivity (stale, lost) and critical alerts apply immediately. Resolved entities outside every open view are evicted.
6. **UI state.** Selection and filters in the URL; panels, theme and density in a small store persisted per dispatcher; selection, city and filters stay in sync across windows of one session over `BroadcastChannel` (PRD §9).
7. **Commands.** Each dispatcher action gets a client-generated command id and a pending entry, POSTed to the console API, which writes a `command.requested` carrying the subject version the dispatcher saw (PLAN Event catalog § Commands); the answer arrives on the socket like any other event. Lists, drawers and snapshots are read from the ops view's query API. It resolves only on the event carrying that id, either the outcome or a `command.rejected` naming the winning event ("Not cancelled — accepted by D-118 at 14:02:11"), never optimistically. Pending turns to "Still waiting for dispatch…" after 5 s. Open commands' outcomes are part of the snapshot, so an outcome that arrived during a disconnect is not lost; a resent command is harmless because its owner dedupes by command id.

The map is plain MapLibre created once and held in a ref, never re-created by a render. It starts with a GeoJSON source fed by `updateData` diffs at report rate (walking skeleton), which does not interpolate: dots step at report rate. It moves to a custom WebGL layer over the position store, which also brings gliding, if either p95 frame time (16.7 ms) or receive-to-paint p99 (100 ms) misses its budget at 5,000 points, on the reference laptop or the 4× CPU-throttled run. `updateData` work runs in MapLibre's worker, so frame time alone can look healthy while receive-to-paint grows.

## Why
Go keeps the work that needs throughput, memory control and the distributed guarantees; TypeScript and React give the console the largest pool of accessible, maintained components and of engineers who can read the code. The cost of React's coarse rendering is bounded by construction of the state tiers: positions never enter React, and entity changes reach it once per second through selectors.

## Consequences / trade-off accepted
- Node is a build dependency: its version is pinned (`engines` and `.nvmrc`), CI installs with the frozen lockfile, and dependencies are audited on update. Production images carry only the Go binary and static files.
- React plus React DOM costs more bundle than Preact; the budget (app code < 250 KB gzipped, excluding MapLibre) is checked on every build from the Vite build report.
- Selectors must return stable values (memoized per snapshot), or every flush re-renders every subscriber; the reviewer checks this.
- One socket per window multiplies gateway connections by open windows; the load proof counts windows per dispatcher. A shared socket across windows (SharedWorker) is the fallback if that count matters.
- Budgets in the console PRD (frame time, receive-to-paint, heap, bundle size) become exit criteria for console slices.

## How it was tested
<!-- 1. Convergence (the client's version of invariant 6): Playwright run with a fake gateway that duplicates, reorders, drops a range and kills the connection mid-stream with a cancel in flight; assert client state equals a fresh snapshot and the command resolves once. 2. Reference laptop and 4× throttle at 5k moving points: p95 frame time and receive-to-paint p99 for the updateData path and, if triggered, the custom layer. Receive-to-paint measured in the browser on one clock (worker and main thread aligned by timeOrigin); gateway send timestamps only as a labelled cross-check. 3. Heap across the 2 h Day-profile soak. 4. Bundle size from the Vite build report. 5. React render counts per flush under the Day profile (React Profiler) to prove selectors hold. -->
