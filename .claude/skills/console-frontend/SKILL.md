---
name: console-frontend
description: How to build the Flotilla Console in TypeScript + React 19 + Vite — state tiers by update rate kept outside React, the transport worker, external stores read with useSyncExternalStore selectors, commands with pending and outcome, React Aria components, virtualized lists, URL state, Vitest and Playwright tests, performance budgets and security. Use when writing or reviewing anything under web/ or the Go code that serves it.
---

# Console frontend

Stack and state model are fixed by `docs/decisions/0017-console-frontend-stack.md`;
read its Decision section first. Visual rules come from the `design-system` skill.

## Layout (created in R2; keep it unless an ADR changes it)

```
web/
  src/
    app/        shell, routes, URL <-> state sync
    transport/  worker: socket, decode, sequence/resync; main-thread port
    store/      positions (typed arrays), entities (normalized + version), ui, commands; no React imports
    map/        MapLibre setup, layers by zoom, symbology, interpolation
    ui/         design-system components on React Aria Components + CSS Modules (no domain fetching)
    features/   orders, drivers, proposals, alerts, events, overview, imports, settings
    strings/    externalized strings
  tokens.css    copied from docs/design/console/tokens.css
  package.json, pnpm-lock.yaml, vite.config.ts, tsconfig.json, .nvmrc
cmd/console/    Go: serves web/dist with CSP and cache headers; no Node in the image
```

Dependencies point inward: `features` → `ui`, `store`; `ui` never imports `store` or
`transport`; only `transport` touches the socket (live gateway) and the HTTP APIs
(ops view for reads and snapshots, console API for commands); `store` and `transport` are plain
TypeScript with no React, so they are unit-tested without rendering.

## State, by rate

| Tier | Holds | Reactive? | Update path |
| --- | --- | --- | --- |
| Positions | lon/lat/heading plus device (session, seq) per driver slot in typed arrays; free list of slots | No | Worker posts transferable buffers; map reads in its frame loop |
| Entities | orders, drivers, offers, proposals, incidents keyed by id, each with version = (partition, offset) of its last applied event | Per view, once per second | Deltas accumulate; one flush per second publishes a new immutable snapshot per view; components read it with `useSyncExternalStore` + selector |
| UI | selection, panels, theme, density, filters | Yes | Selection and filters in the URL; panels, theme, density in a small store persisted to `localStorage`; cross-window via `BroadcastChannel` |
| Commands | command id → pending / outcome / race outcome | Yes | Resolved only by the event carrying the command id |

Rules:
- **Idempotent apply.** Drop an entity delta whose offset is not above the held one
  (offsets compare only within one topic and partition; each entity's events live in one
  topic, PLAN § Topics). Fencing of stale owners happens at the broker (decision 13); the
  client does not filter epochs. Drop a position whose (session, seq) is not newer within the session;
  a new session is accepted (decision 4). Test with duplicated and reordered streams.
- **Resync.** Connection sequences are per connection: a gap or any reconnect resyncs.
  Order: subscribe and buffer → snapshot stamped with per-partition cut offsets →
  replace the snapshot's scope (not merge, so deletions land) → replay buffered deltas
  above the cut. Same for newly subscribed cells after a pan. The UI shows "as of …"
  until done.
- **What may wait.** The once-per-second flush and the "n updates — show" hold apply to
  list order and membership only. Command outcomes, stale/lost connectivity and critical
  alerts apply at once.
- **Bounded memory.** Evict resolved entities no open view needs; return position slots
  to the free list when a driver leaves scope.
- **No optimistic final state.** A clicked action shows pending until the event with its
  command id arrives (outcome or rejection); open command outcomes come back in the
  snapshot after a reconnect. Commands are sent as `command.requested` with a
  client-generated command id (`crypto.randomUUID()`) and the subject version the
  dispatcher saw; a resend reuses both. `lost_race` rejections name the winning events.
- **Never put positions in React state**, context or a store snapshot; never call
  `setState` per message.
- **Selectors are stable.** `getSnapshot` returns the same reference until the next
  flush; selectors return primitives or values memoized per snapshot. A selector that
  builds a new array or object each call re-renders every subscriber every flush.
- **Narrow subscriptions.** A row subscribes to its own entity by id, not to the list;
  the list subscribes to ids and order. Context carries stable handles (store, map),
  never changing data.
- **Derived data** is computed once per flush in the store, or with `useMemo` keyed on
  the snapshot, not mirrored into state by `useEffect`.
- **Effects clean up.** Every subscription, timer, `ResizeObserver`, worker port and map
  listener is disposed in the effect's cleanup; the 2 h soak's flat-heap budget depends
  on it. No `useEffect` for things that are not synchronisation with something outside
  React.
- **Map outside renders.** The MapLibre instance is created once in a ref and driven
  imperatively; React never re-creates it and never renders per driver.
- **Server owns counts.** Tab counts and totals come from the server; never count a
  partial client list and present it as a total.
- **Tenancy.** Fleet and city come from the token; a city switch tears down
  subscriptions and stores for the old city before opening the new ones.

## Lists

Virtualize every list that can exceed a screen (fixed row height per density, overscan
of a few rows). Server-side paging and sorting at 100,000 orders. Row keys are entity
ids, never indexes. While the pointer is over the list or it is scrolling, hold
changes to row order and membership; release on "show" or when idle. Values inside
visible rows (status, connectivity, outcomes) still update.

## Events stream

Render at most ~50 lines/s; show the true rate and "showing 1 in N" when sampling.
Rendering never blocks input: append in `requestAnimationFrame`, cap the DOM rows.

## Performance budgets (PRD §14; exit criteria for console slices)

Map p95 frame < 16.7 ms at 5,000 moving points (also at 4× CPU throttle) ·
receive-to-paint p99 < 100 ms · 60 fps scroll at 100,000 orders · heap flat ±10% over
the 2 h soak · app code < 250 KB gzipped excluding MapLibre · interactive < 2 s.
Measure, don't assert: receive-to-paint in the browser on one clock (align worker and
main thread by `performance.timeOrigin`; gateway timestamps only as a labelled
cross-check, since two machines' clocks skew), the Vite build report for size,
Playwright runs (with CPU throttling over CDP) for frame time, and the React Profiler for
render counts per flush. Numbers go to
`docs/results/` through `bench-runner`.

## Build and test

- Vite 8 with `@vitejs/plugin-react`; Node pinned in `.nvmrc` and `engines` (Vite 8
  needs 20.19+ or 22.12+); pnpm with `--frozen-lockfile` in CI. Pin versions exactly;
  audit on update; a new dependency needs a reason in review.
- `build.target` pinned; vendor chunk for React and MapLibre so app code size is visible
  on its own; source maps in dev.
- Type check with `tsc --noEmit` (TypeScript 7), `strict: true`,
  `noUncheckedIndexedAccess: true`, `exactOptionalPropertyTypes: true`. Vite does not
  type check; a green build is not a type check.
- Unit tests in Vitest 5 for `store`, `transport`, formatters (no DOM needed). Component
  tests in Vitest browser mode (`@vitest/browser-playwright`) with Testing Library
  queries by role and name.
- Flows in Playwright against the built page with a fake gateway that can duplicate,
  reorder, drop a range and kill the connection: pending → outcome, race outcome,
  reconnect. The convergence test asserts client state equals a fresh snapshot
  afterwards; it is the client's version of invariant 6.
- ESLint with `eslint-plugin-react-hooks` (rules of hooks and exhaustive deps) as errors.

## Security

- Server strings render as text only (React escapes). No `dangerouslySetInnerHTML`, no
  `innerHTML`, no `href` built from server strings without a scheme allowlist.
- Content-Security-Policy from the Go server: `default-src 'self'`, no inline script,
  `connect-src` limited to the gateway origin; map tiles origin listed explicitly.
- Tokens: short-lived access token in memory, refresh via an httpOnly cookie; never in
  `localStorage`. The socket authenticates on connect and is closed on fleet or city
  change.
- Read-only (public) view: actions render disabled and the client never sends them;
  the gateway still rejects them (the client is not the boundary).

## Accessibility

Interactive primitives (dialog, tabs, menu, combobox, listbox, table selection) come
from React Aria Components, which supply roles, keyboard behaviour and focus
management; wrap them in `ui/` with tokens, never re-implement them. Locale and
direction through React Aria's `I18nProvider`, with `lang` and `dir` on the root.
Keyboard reachability for every action; visible focus; `aria-live="polite"` for
toasts and pending/outcome lines (assertive only for critical); roles on tabs, grids and
dialogs; focus trapped in dialogs and returned on close; reduced motion honoured.
