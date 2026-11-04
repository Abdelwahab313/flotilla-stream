# Flotilla Console — Product Requirements for Design

- Status: ready for design
- Date: 2026-10-25
- Owner: @Abdo
- Language: [`../../CONTEXT.md`](../../CONTEXT.md) is the glossary. Every capitalised domain word below (Driver, Offer, Route, Incident…) means exactly what it says there.
- Decisions behind this document: [0015 Tenancy](../decisions/0015-tenancy-model.md), [0016 Planner placement and batching](../decisions/0016-planner-placement-and-batching-scope.md), [0017 Console frontend stack](../decisions/0017-console-frontend-stack.md), and the system plan [`PLAN.md`](PLAN.md).

---

## 0. Brief for the design tool

> Design **Flotilla Console**, the web product a delivery company's dispatchers run their operations day from. It is map-first: a full-bleed live map of thousands of moving drivers inside a city, with collapsible panels for orders, drivers, proposals, events and alerts, and a context panel for whatever is selected. Drivers carry many orders at once along a route of pickup and dropoff stops (courier and prepared-food delivery). Dispatchers watch, approve, cancel, return orders to the queue, change a driver's duty, message drivers and work grouped alert incidents. It must stay calm and fast at scale: 10,000 drivers per city, 100,000 orders a day, thousands of events per second.
>
> Visual concept (decided from the Claude Design mockups, 2026-11-03): one design system, two modes. **Cargo** (dark, the dispatcher default — neutral charcoal, white ink, one indigo accent, bright data colours) and **Ops Grey** (light, the public-view default — a grey world where colour appears only when something needs attention). Values live in `docs/design/console/`.
>
> Deliver: the app shell; nine screens and six overlays (section 10) in both themes; every state listed per screen; the phone layout of the public view; a component sheet; and prototypes of flows 1, 3, 4 and 10 (section 8).

---

## 1. What this document is

A product requirements document for the console's UI and UX, written for a designer who has not seen the rest of the project. It defines who uses the console, the domain they work in, the features, the flows, every screen with its states, the map symbology, the visual direction and the performance budgets the design must respect.

It does not define colours as hex values, exact type sizes or pixel layouts. Those are the designer's. It does fix meanings: what each status is, which ones must be distinguishable, what is clickable, what must never be hidden.

---

## 2. Product summary

Flotilla is the real-time backbone of a delivery operation. Drivers' devices stream positions every few seconds; the platform tracks every driver live, detects zone entry and exit, and a dispatch engine groups waiting orders and offers them to nearby drivers. All traffic in this project is synthetic: a simulator generates drivers, positions and orders.

The console is the product surface. It has two audiences at once:

1. **In the product: dispatchers.** They run an operations day for one or more cities. They need to see where every driver is, which orders are waiting, late or stuck, and act in a few keystrokes.
2. **In reality: reviewers and interviewers.** The project exists to demonstrate large-scale infrastructure and architecture. The console is the first thing a reviewer sees, and it carries the system's evidence inside the product: live latency, event rates, an incident appearing when a fault is injected, the map continuing to move while a node is killed.

Design for the dispatcher. The reviewer is served by the same screens being visibly fast, honest about liveness and calm under load.

---

## 3. Goals and non-goals (console v1)

### Goals
- A dispatcher can run a whole operations day from the console without another tool.
- Every live element states how fresh it is; nothing stale pretends to be live.
- The console stays smooth at target scale (section 6) and its performance is measured, not asserted.
- Every action the console offers is backed by a real system event; nothing is a UI-only fiction.
- One design language across dispatcher, fleet admin, platform operator and public read-only view.

### Non-goals (v1)
- CRUD forms for every entity. Drivers, zones, cities and people are seeded. There is no driver editor, zone editor or user management UI.
- Manual assignment of an order to a chosen driver. The dispatch engine is the only thing that assigns; the dispatcher approves, rejects, cancels or returns to the queue.
- Turn-by-turn navigation, a driver mobile app, a customer-facing tracking page, geocoding of addresses, analytics beyond the operations-day overview.
- System metrics boards. Grafana is a separate product with its own UI; the console only links to it.
- Arabic/RTL implementation. Layout must be RTL-capable and every string externalized, but RTL ships after v1.0.

---

## 4. People and roles

| Role | Who | Scope | What they do in the console |
| --- | --- | --- | --- |
| **Dispatcher** | Runs the operations day | The cities they are assigned to, within one fleet | Watch the map and lists; approve or reject proposals; cancel; return to queue; mark delivered or failed; set duty; suspend; message drivers; work incidents; import and export orders |
| **Fleet admin** | Runs the fleet's configuration | All cities of one fleet | Everything a dispatcher can, plus fleet settings, API keys |
| **Platform operator** | Runs the platform itself, outside any fleet | Local environment only | Operator drawer: start and steer the Day profile, fire order bursts, inject faults, switch fleet |
| **Public viewer** | A reviewer on the public demo URL, often on a phone | One demo fleet, read-only | Watch only. On laptop every action is visible but disabled with a "Read-only demo" tooltip; on phone actions are hidden |

### Persona: Mona, dispatcher, Cairo
Eight-hour shift, two monitors, keyboard-heavy. Works alongside two other dispatchers on the same city. Cares about: orders that are late or about to be, drivers who went dark with parcels on board, proposals waiting for approval at peak. Hates: alerts that bury the map, lists that jump while they are reading, not knowing whether a click "took".

### Persona: Sam, reviewer
Opens the public URL from a phone, then on a laptop. Has two minutes. Wants to see: it is live, it is big, it is fast, and something real happens when load or failure is applied.

---

## 5. Domain model the design must express

### 5.1 Hierarchy and tenancy
**Fleet** (a delivery company, the tenant) → **City** (has its own timezone and operations day) → **Zone** (a polygon inside a city).

- Fleets are totally isolated from each other: a user never sees another fleet's anything.
- A dispatcher's token carries their allowed cities; the city is a hard context in the UI, never just a filter. Switching city switches operations day, timezone, map, lists and KPIs.
- Every time shown is in the selected city's local time, with the timezone visible in the top bar.

### 5.2 Driver
Seeded. Fields shown: name, id, vehicle type, phone, home zone, current zone, capacity (planner input; not shown prominently), today's counts.

A driver's state has three independent parts:

| Part | Values | Set by |
| --- | --- | --- |
| **Duty** | on duty · on break · off duty · suspended | Dispatcher, or the driver's device |
| **Activity** | idle · offered · on route (n stops) | Dispatch engine only |
| **Connectivity** | live · stale (no report for 30 s) · lost (no report for 10 min) | Derived from the device's reports |

The console never shows duty and activity separately in compact places. It shows one **Driver status**, plus a connectivity badge beside it:

| Duty | Activity | Driver status shown |
| --- | --- | --- |
| on duty | idle | **Available** |
| on duty | offered | **Offered** |
| on duty | on route | **On route** · n stops |
| on break / off duty / suspended | on route | **Finishing route** (gets no new offers) |
| on break | idle | **On break** |
| off duty | idle | **Off duty** |
| suspended | idle | **Suspended** |

Rules the UI must reflect:
- Only Available drivers who are live receive offers. Stale and lost drivers get none.
- Losing connectivity never removes accepted orders from a driver. A lost driver with orders on board raises a critical alert.
- Going off duty while on route means "finish the route, then stop"; the UI says so.

### 5.3 Order
Fields: external reference (the fleet's own id), pickup point and address, dropoff point and address, **promise** (pickup-by, deliver-by), recipient name and phone, notes; derived: fleet, city, pickup zone, current ETA for pickup and dropoff, **late** flag.

Lifecycle (italic = a dispatcher can trigger it):

```
created → waiting ──(review mode)──→ proposed ─approve→ offered ─accept→ assigned → picked up → delivered
             │                         └ reject → waiting       ├ decline / expire → waiting   └→ delivery failed
             └────────(auto mode)──────────────→ offered
assigned (not picked up) ── return to queue ──→ waiting
cancel: allowed from waiting, proposed, offered, assigned-not-picked-up
mark delivered / mark failed: dispatcher override with a reason
```

- **Waiting** orders never expire on their own. Waiting longer than 10 minutes (fleet setting) raises "Late to assign".
- **Late** = planned or actual stop time past the promise. Shown as a ring on the order everywhere it appears.
- Every transition is recorded with its **actor** (a dispatcher by name, the driver's device, dispatch, an API key, an import job) and is visible in the order's timeline.

### 5.4 Offer, Proposal, Assignment, Stop, Route
- An **Offer** proposes one or more orders from the same pickup zone to one driver, with their stops placed in the driver's route. The driver accepts or declines it as a whole, or it expires (default 30 s, fleet setting — assumed default).
- A driver has **at most one open offer at a time**. After accepting, the driver can receive further offers that insert more stops into the same route.
- In **review mode** (a fleet setting), the engine produces a **Proposal** instead of an offer; a dispatcher approves (it becomes an offer) or rejects (orders go back to waiting). In **auto mode** offers go straight out.
- An **Assignment** is one order confirmed to one driver. A driver may hold many.
- A **Route** is a driver's ordered list of upcoming **Stops**; each stop is one pickup or one dropoff of one order, with a planned time.
- The **Planner** decides grouping, stop order and ETAs. v1 is simple (nearest driver, ties to the one with fewer active orders, greedy stop insertion, straight-line ETAs). The UI must not imply optimisation it does not do: label ETAs as estimates.

### 5.5 Dispatcher actions are commands, not edits
Every action (cancel, return to queue, approve, set duty…) is sent to the dispatch engine, which decides in order. Two consequences for the design:
1. **No optimistic final state.** After a click, the subject shows a *pending* state ("Cancelling…") until the confirming event arrives, typically well under a second. Then it shows the outcome.
2. **Races have visible outcomes.** If a cancel and a driver's accept cross, whichever the engine processed first wins, and the UI says so plainly: "Not cancelled — driver accepted at 14:02:11. Return to queue instead?"

### 5.6 Alerts, Incidents, Severity, Claim
- An **Alert** is raised by the system from an event; never created by hand; at most one open per kind and subject.
- Alerts of one kind in one zone within a rolling 15-minute window are grouped into one **Incident**. Dispatchers work incidents, not individual alerts.
- **Severity**: critical (toast and sound; cannot be muted), warning (badge count), info (inbox only).
- A dispatcher can **claim** an incident (others see "Mona is on this"), acknowledge it, and it auto-resolves when its condition clears, showing progress ("29 / 30 drivers back live").

Alert kinds in v1:

| Kind | Severity | Subject | Clears when |
| --- | --- | --- | --- |
| Driver lost with picked-up orders | critical | driver | driver live again, or all its orders resolved |
| Delivery failed | critical | order | acknowledged |
| Zone has waiting orders and no eligible driver | critical | zone | an eligible driver appears or the queue empties |
| Driver lost (no picked-up orders) | warning | driver | driver live again |
| Late to assign | warning | order | order offered |
| Order late | warning | order | order delivered or cancelled |
| Offer declined three times | warning | order | order assigned |
| Dwell exceeded | info | driver in zone | driver leaves zone |
| Import finished with rejected rows | info | import job | acknowledged |

### 5.7 Message, Channel, Watch
- A **Message** is text from a dispatcher to one driver (free text or a quick template), sent through a **Channel** (device push now; SMS later). Status: sent → delivered → read, or failed.
- **Watch**: a dispatcher pins up to 10 drivers; every event for a watched driver surfaces to them (toast + feed line) even when the driver is off screen. Watch is personal and does not affect other dispatchers.

### 5.8 Import job, Export, API key
- **Import job**: one CSV upload (or API batch) of up to 10,000 orders into one city. Coordinates are required; address is display text. The fleet's external reference makes re-uploads harmless: rows already imported are reported as skipped, never duplicated. Result per row: created, skipped as duplicate, or rejected with a reason ("missing coordinates", "pickup outside service area").
- **Export**: orders for an operations day or a date range, filtered like the orders list, as CSV or NDJSON, in the import format plus status and lifecycle timestamps.
- **API key**: fleet-scoped, with scopes (`orders:write`, `orders:read`), shown once at creation, revocable. Managed by fleet admins.

---

## 6. Scale the design must hold

Design targets, to be confirmed by measurement; the published numbers will be whatever the tests show.

| Thing | Order of magnitude |
| --- | --- |
| Drivers per city | up to ~10,000 |
| Moving drivers inside one dispatcher's viewport | up to ~5,000 |
| Orders per busy city per operations day | ~100,000 |
| Events per second, one city | ~2,000 |
| Concurrent dispatchers per city | 3–10 |
| Open alerts during a cell-tower outage at peak | hundreds within minutes → must collapse to a handful of incidents |

Design implications:
- No screen may depend on showing "all" of anything. Lists are paged and virtualized; counts come from the server.
- Lists must not jump under the reader: live changes are batched (once per second) and, while the user is scrolling or hovering, held behind a "12 updates — show" pill.
- The map switches representation by zoom (section 11) instead of drawing 10,000 dots at city zoom.
- The events stream shows its true rate and samples visibly beyond what a human can read.

---

## 7. Feature inventory

Each feature lists what "done" means for the design.

| # | Feature | Acceptance (design) |
| --- | --- | --- |
| F1 | **Live map** | Drivers by Driver status and connectivity; zones with live counts (drivers, waiting orders); waiting and late order pins; layer toggles; zoom-dependent representation |
| F2 | **Driver focus on map** | Selecting a driver draws its route (numbered stops, planned times, pickup vs dropoff distinguishable), the last 15 minutes of trail, and its open offer if any |
| F3 | **Orders list** | Status tabs with server counts (Waiting · Proposed · Offered · Assigned · Picked up · Delivered · Failed · Cancelled); filters (zone, late, promise window, driver, import job); search by external ref, id, recipient phone; multi-select bulk cancel and return to queue; columns configurable |
| F4 | **Order detail** | Promise vs ETA, pickup and dropoff on the map, timeline with actor on every line, actions valid for the current state only |
| F5 | **Drivers list** | Driver status, connectivity badge, stops left, last seen, current zone, today's delivered count; set duty, suspend, message, watch |
| F6 | **Driver detail** | Route as an ordered stop list, timeline, message thread, actions, today's numbers |
| F7 | **Proposals** (review mode) | Queue sorted by age; each proposal shows driver, orders, the route before and after on the map, ETA and promise impact; approve / reject with keyboard; a proposal the engine can no longer honour shows "Stale — rejected by dispatch" |
| F8 | **Alerts and incidents** | Incidents grouped by kind and zone; severity; claim, acknowledge; auto-resolve progress; footprint highlighted on the map |
| F9 | **Watch rail** | Up to 10 pinned drivers with live status; their events as toasts and feed lines |
| F10 | **Events stream** | Live, filterable feed of every event in the city (by type, zone, driver, order, actor); true rate shown; visible sampling above the readable rate; pause on hover |
| F11 | **Day overview** | Operations-day KPIs from product data: orders by status, on-time %, median time to assign, active drivers, late now; trend through the day |
| F12 | **Imports and exports** | Upload with template download; job progress; row report with filters (created / skipped / rejected) and download of rejected rows; export with format and filters |
| F13 | **Settings** (fleet admin) | Assignment mode (auto / review); thresholds (stale, lost, late to assign, offer expiry); capacity defaults per vehicle type; API keys |
| F14 | **Command palette and shortcuts** | `⌘K` / `Ctrl K` finds any driver, order, zone or action; list navigation and actions from the keyboard (section 10.13) |
| F15 | **Live health strip** | Socket state, position-to-screen latency now, event lag; link to Grafana |
| F16 | **Operator drawer** | Day profile start / pause / speed with the scenario clock; order burst; fault injection; fleet switcher. Operator view only |
| F17 | **Day replay** | Time slider over the operations day; map replays. First feature after v1 |
| F18–F19 | Customer tracking link; planner tuning, analytics, geocoding, zone editor, user management | Later; not designed now |

---

## 8. Key flows

Each flow is a prototype candidate. Flows 1, 3, 4 and 10 are required prototypes.

### Flow 1 — Start of the operations day (required)
1. Mona signs in; lands on **Live** for Cairo (last-used city). Top bar shows "Tue 25 Oct · 07:58 Cairo".
2. The map shows drivers coming on duty (grey → green), zones with small waiting counts.
3. An import from the night batch finished with 14 rejected rows: an info item in the alerts inbox, not a toast.
4. Mona opens the import from the inbox, filters to rejected rows, downloads them to fix.
5. She checks Day overview: 1,240 orders already waiting for the 09:00 wave.

### Flow 2 — Lunch peak monitoring
1. 12:30. Waiting count rises in Downtown; its zone fill deepens.
2. Several orders cross "Late to assign"; one warning incident appears: "Late to assign · Downtown · 23 orders". Badge count rises; no toasts (warning).
3. Mona selects the incident: the map dims everything except the 23 orders and the eligible drivers nearby.
4. She sees there are few available drivers; she messages two drivers on break in the adjacent zone using a quick template.

### Flow 3 — Review-mode proposal approval (required)
1. Fleet runs in review mode. The Proposals tab shows 6 waiting, oldest 40 s.
2. Mona presses `J` to the first: driver D-118, 3 orders. The map shows D-118's route before (dashed) and after (solid), new stops numbered.
3. The panel shows the ETA impact: two existing deliveries shift by +4 min, both still within promise; one new order will be 2 min late against its promise (flagged).
4. She presses `A`. The proposal shows "Sending offer…", then "Offered to D-118 · expires in 0:28".
5. Next proposal: she presses `A`, but the engine answers that the driver went stale. The row shows "Stale — rejected by dispatch: driver not live". The orders return to waiting automatically; the engine will propose again.

### Flow 4 — Driver lost incident at scale (required)
1. 12:35, Nasr City. A cell outage: 120 drivers go stale (faded on map), 30 go lost.
2. One critical incident: "Driver lost · Nasr City · 30 drivers · 112 orders on board". One toast with sound; the toast budget holds — no flood.
3. Hassan (another dispatcher) has already claimed it: the incident shows "Hassan is on this". Mona sees it and moves on.
4. Hassan opens the incident: list of 30 drivers sorted by orders not yet picked up. He selects the 8 drivers carrying not-yet-picked-up orders and returns those orders to the queue. Picked-up orders stay with their drivers (the action is unavailable for them, with the reason shown).
5. Over 6 minutes drivers come back; the incident shows "24 / 30 back live", then auto-resolves.

### Flow 5 — Cancel races an accept
1. Customer calls to cancel O-552, which is offered to D-118.
2. Mona presses Cancel, picks a reason. The order shows "Cancelling…".
3. The driver accepted 300 ms earlier; the engine processed the accept first.
4. The order shows: "Not cancelled — accepted by D-118 at 14:02:11." Offered action: "Return to queue" (valid because it is not picked up yet).

### Flow 6 — Driver goes off duty mid-route
1. D-9 phones in sick. Mona sets duty to off duty from the driver panel.
2. If D-9 had an open offer, it is released and the orders return to waiting (visible in their timelines).
3. D-9 shows "Finishing route · 4 stops". Mona returns the 2 not-yet-picked-up orders to the queue; D-9 finishes the 2 dropoffs.
4. After the last stop D-9 shows "Off duty".

### Flow 7 — Re-uploading an import
1. Upload of `cairo-tuesday.csv` (2,400 rows) times out in the browser.
2. Mona uploads it again. The job reports: 1,700 skipped as already imported, 696 created, 4 rejected.
3. The map shows a wave of new waiting pins across zones.

### Flow 8 — Message a driver
1. From a driver panel or a watched driver: Message → template "Pickup moved to gate B" or free text.
2. Thread shows the message as sent, then delivered (seconds later), then read.
3. If the channel fails: "Failed · device unreachable" with retry.

### Flow 9 — Watch a driver
1. Mona pins D-118 (pin icon or `W`). It appears in the watch rail.
2. While she works another zone, D-118 picks up and delivers: each shows as a small toast and a rail update.
3. D-118 goes stale: the rail tile fades and shows "stale · 40 s".

### Flow 10 — Public viewer on a phone (required)
1. Sam opens the public URL. Ops Grey mode. Full-screen map of the demo city with moving drivers.
2. A compact top bar: city, local time, live badge with latency "230 ms".
3. A bottom sheet, peeking: KPI chips (drivers live, orders today, on-time %). Swipe up: tabs Drivers · Orders · Events.
4. Tapping a driver shows its route and status; actions are absent on phone (public is read-only).
5. A small "How this works" link opens a one-screen explainer with a link to the repository.

### Flow 11 — Platform operator demo
1. Operator opens the operator drawer: Day profile "Cairo weekday", speed 12×, start. A scenario clock appears beside the wall clock: "Scenario 11:20 · wall 16:05".
2. Fires an order burst into Downtown: 500 orders in 60 s. Waiting pins flood Downtown; proposals or offers stream; the events stream rate climbs and shows sampling.
3. Kills a consumer pod from fault injection. The health strip shows lag rising, then recovering; the map never stops moving.
4. Switches fleet to show the second tenant in Berlin: nothing of Cairo is visible.

### Flow 12 — Export the day
1. Orders list filtered to Delivered, today.
2. Export → CSV → "Preparing 38,410 orders…" → download.

---

## 9. Information architecture and app shell

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ TOP BAR  Fleet ▸ City ▾ · Tue 25 Oct 12:40 Cairo (UTC+3) · KPI chips ·       │
│          ⌘K search · Health ● live 230 ms lag 0.4 s · Alerts ▲3 · Theme · Me  │
├──┬──────────────────────────────────────────────────────────┬────────────────┤
│  │                                                          │ CONTEXT PANEL  │
│N │                                                          │ selected order │
│A │                 LIVE MAP (full bleed)                    │ / driver /     │
│V │                                                          │ proposal /     │
│  │                                                          │ incident       │
│R │                                                          │ (collapsible)  │
│A ├──────────────────────────────────────────────────────────┤                │
│I │ BOTTOM DOCK (resizable, collapsible; tabs)               │                │
│L │ Orders · Drivers · Proposals · Events · Alerts           │                │
├──┴──────────────────────────────────────────────────────────┴────────────────┤
│ WATCH RAIL (pinned drivers; collapsible)                                      │
└──────────────────────────────────────────────────────────────────────────────┘
```

- **Nav rail** (left, icons with labels on hover): Live, Orders, Drivers, Proposals (only in review mode), Alerts, Events, Day overview, Imports & exports, Settings (fleet admin). Operator view adds the operator drawer toggle.
- **Every region is collapsible**: context panel, dock, watch rail, nav rail. **Map focus mode** (`\`) hides all of them, leaving the map and the top bar. Sizes and collapse state persist per dispatcher.
- **One selection model.** Selecting an order, driver, proposal or incident anywhere (map, dock, full-page list, events stream, palette) opens the same context panel and highlights it on the map. Selection is in the URL (`/live?order=O-552`) so every view is linkable.
- **Multiple windows.** Any screen can open in a new window (two-monitor setups). Selection, city and filters stay in sync across windows of the same session.
- **City is a hard context**, switched in the top bar, never a filter chip.
- **Full-page screens** (Orders, Drivers, …) are the same content as the dock tabs with more room: wider tables, saved filters, bulk actions. The context panel works identically there, with a mini-map.

### Responsive targets
| Width | Target |
| --- | --- |
| ≥ 1280 px | Primary. All dispatcher work |
| 768–1279 px | Read-mostly: map + one panel at a time; actions available |
| < 768 px | Public view only: map + bottom sheet (Flow 10). Dispatchers on phone are out of scope |

---

## 10. Screens

Every screen must be designed for these common states: **loading** (skeletons, not spinners, for lists), **empty** (with the reason and the next action), **error** (what failed, retry), **live connection lost** (section 13), **read-only** (public view: actions disabled with tooltip), **pending action** and **race outcome** (section 5.5).

### 10.1 S0 — Sign in and city choice
- Purpose: get a seeded user into their city.
- Content: product mark, sign-in form, then (only if the user has several cities) a city chooser showing each city's local time and live driver count.
- Public view skips this screen entirely.

### 10.2 S1 — Live (default)
- Purpose: the dispatcher's home; situational awareness and the entry point to every action.
- Regions: top bar, map, context panel, bottom dock, watch rail (section 9).
- Map content: see section 11.
- Top bar KPI chips (clickable, each opens the filtered list): Waiting · Late · Drivers available / on duty · On-time % today.
- Dock default tab: Orders (Waiting). Dock rows are compact (one line each).
- States: quiet early morning (few drivers, an empty Waiting tab with "No waiting orders — 312 scheduled for 09:00 wave"); peak (hundreds of waiting pins; density mode); outage (incident footprint dimming the rest of the map).

### 10.3 S2 — Orders
- Purpose: find, triage and act on orders in bulk.
- Layout: status tabs with counts; filter bar (zone, late only, promise window, driver, import job, actor); search; virtualized table; context panel on selection.
- Columns (default): status, late ring, external ref, pickup zone, pickup-by, deliver-by, ETA, driver, age in status. Configurable.
- Bulk: select rows → Cancel (with reason) · Return to queue. Ineligible rows are skipped and reported: "18 returned, 3 skipped (already picked up)".
- Live behaviour: rows update in place once per second; while scrolling or hovering, new rows are held behind "12 new — show".
- Export action in the header (Flow 12).

### 10.4 S3 — Drivers
- Purpose: who is working, who is stuck, who is dark.
- Tabs with counts: All on duty · Available · On route · Finishing route · On break · Off duty · Suspended · Stale/Lost.
- Columns: driver status, connectivity badge with "last seen 40 s", name/id, vehicle, current zone, stops left, delivered today, open offer timer.
- Row actions: Set duty · Suspend / Unsuspend · Message · Watch.

### 10.5 S4 — Proposals (review mode only)
- Purpose: approve or reject the engine's proposals fast at peak.
- Layout: list on the left (oldest first, age timer per row), map on the right showing the selected proposal's route before (dashed) and after (solid), and a summary: driver, orders, stops added, ETA impact per affected order against its promise.
- Keyboard-first: `J/K` move, `A` approve, `R` reject (with optional reason), `Enter` open details.
- States: empty ("All caught up — the engine is proposing as orders arrive"), stale proposal, approval pending, approved (row slides out, offer timer appears on the driver).

### 10.6 S5 — Alerts
- Purpose: work incidents without drowning.
- Layout: tabs Open · Claimed by me · Acknowledged · Resolved; grouped by severity then kind; each incident row shows kind, zone, subject count, age, claimer, resolution progress.
- Incident detail (context panel): affected subjects list with bulk actions relevant to the kind (e.g. return not-yet-picked-up orders to the queue), timeline of the incident, Claim / Acknowledge buttons, and "Show on map" which applies the footprint highlight.
- Per-dispatcher mute by kind for 30 minutes (not for critical).

### 10.7 S6 — Events
- Purpose: the raw truth of what the system is doing, for the city.
- Layout: filter bar (event type, zone, driver, order, actor); live feed with columns time (ms precision), type, subject, zone, actor, summary; rate indicator "2,140 ev/s"; when sampling, "showing 1 in 40" is always visible.
- Behaviour: pause on hover or with `Space`; resume shows "skipped 3,412 events"; clicking a line selects its subject.
- Also a dock tab on Live, where it is filtered to the current map viewport by default.

### 10.8 S7 — Day overview
- Purpose: how the operations day is going, in business terms.
- Content: KPI tiles (orders created, delivered, failed, cancelled; on-time %; median time to assign; median delivery time; drivers active now / peak today); orders-per-hour chart through the day with the current hour marked; on-time % by zone (small multiples or a zone map); late orders now.
- All from product data. A small link "System metrics in Grafana ↗" — nothing else system-level here.

### 10.9 S8 — Imports and exports
- Purpose: get orders in and out.
- Import: drop zone; template download; city is fixed to the current city; job list with progress; job detail with created / skipped / rejected counts and a filterable row table; "Download rejected rows".
- Export: format (CSV / NDJSON), range (today / date range), filters copied from the Orders list; job list with download when ready.

### 10.10 S9 — Settings (fleet admin)
- Sections: Dispatch (assignment mode auto / review, with an explanation of each); Thresholds (stale, lost, late to assign, offer expiry); Capacity defaults per vehicle type; API keys (list with name, scopes, created, last used; Create shows the key once with a copy button and a warning; Revoke with confirmation).
- Changing assignment mode is a fleet-wide change: confirmation states its effect ("Offers will wait for dispatcher approval in all 3 cities").

### 10.11 O1 / O2 — Order drawer and Driver drawer (context panel)
- **Order**: header with status, late ring, external ref; promise vs ETA bar (pickup-by, deliver-by, planned times); pickup and dropoff addresses; assigned driver (link); recipient; notes; actions valid for the current state; timeline (newest first, each line: time, event, actor).
- **Driver**: header with Driver status, connectivity badge, name, vehicle; route as numbered stop list (pickup / dropoff icon, order ref, planned time, late ring); open offer with countdown; actions (Set duty, Suspend, Message, Watch, Return not-yet-picked-up orders to queue); message thread; timeline; today's numbers.

### 10.12 O3 — Command palette
- Opens with `⌘K` / `Ctrl K`. Finds drivers, orders (by external ref or id), zones, screens and actions ("cancel O-552", "message D-118", "switch to Berlin").
- Results grouped by type, with status shown inline.

### 10.13 Keyboard map (v1)
| Key | Action |
| --- | --- |
| `⌘K` / `Ctrl K` | Command palette |
| `J` / `K` | Next / previous row |
| `Enter` | Open selected |
| `Esc` | Close panel / clear selection |
| `A` / `R` | Approve / reject proposal |
| `C` | Cancel order (asks for reason) |
| `Q` | Return to queue |
| `W` | Watch / unwatch driver |
| `M` | Message driver |
| `\` | Map focus mode |
| `Space` | Pause / resume events stream |
| `G` then `O`, `D`, `P`, `A`, `E`, `L` | Go to Orders, Drivers, Proposals, Alerts, Events, Live |

### 10.14 O4 — Watch rail
- Up to 10 tiles: driver name, Driver status, connectivity, next stop and its planned time, mini progress (stops done / total).
- Click selects the driver; drag to reorder; `W` toggles.

### 10.15 O5 — Toasts
- Budget: at most 3 visible; extras collapse to "+12 more" which opens Alerts.
- Critical toasts persist until dismissed and play a sound once; warnings never toast; watch toasts are small and auto-dismiss.
- Toasts never cover the selected subject on the map or the context panel's actions.

### 10.16 O6 — Operator drawer (operator view only)
- Day profile: profile picker, start / pause / stop, speed (1× to 24×), **scenario clock** shown distinctly from wall clock everywhere while running.
- Order burst: zone, count, duration.
- Fault injection: kill a pod (by component), drop N drivers' connectivity, slow a consumer. Each shows what is happening and links to the trace or Grafana panel.
- Fleet switcher.
- Visual treatment: clearly a different layer (e.g. hazard-tape edge or distinct header) so it is never mistaken for dispatcher UI in a recording.

### 10.17 O7 — Public view, phone
- Map full screen, compact top bar (city, local time, live badge with latency), bottom sheet with KPI chips peeking; swipe up for Drivers · Orders · Events tabs; tap a driver or order for a read-only sheet; "How this works" explainer screen.

### 10.18 Health strip (component in the top bar)
- Three readings: connection (live / reconnecting / offline), position-to-screen latency now (rolling p50 or p99 — labelled), event lag. Colour moves to warning or critical with thresholds.
- Click opens a small panel: last 5 minutes sparkline of each, and links to Grafana and the trace of a recent position.

---

## 11. Map specification

### 11.1 Layers, bottom to top
1. Basemap (themed: Cargo or Ops Grey; desaturated; labels subdued so data dominates)
2. Zone fills (depth-tint; intensity = waiting orders relative to available drivers)
3. Zone outlines and labels with live counts
4. Driver density hexes (low zoom only)
5. Trails (selected or watched drivers; last 15 min; fading)
6. Routes (selected driver; or before/after in Proposals)
7. Order pins (waiting, late) and stop markers of the selected route
8. Drivers
9. Selection halo; incident footprint mask (dims everything outside the footprint)

### 11.2 Zoom behaviour
| Zoom | Drivers shown as | Orders shown as |
| --- | --- | --- |
| City (≤ 11) | Hex density coloured by count, plus zone counts | Zone waiting counts only |
| District (12–14) | Clustered dots with count, cluster ring segmented by status mix | Clustered pins |
| Street (≥ 14) | Individual drivers with heading and status | Individual pins with late ring |

Rule: above ~5,000 drivers in view the map always uses the representation for the zoom one level out.

### 11.3 Symbology (fixed meaning across both themes)
Every state is distinguishable by shape, fill or motion, not by colour alone.

| State | Shape / fill | Motion | Colour role |
| --- | --- | --- | --- |
| Available | Solid dot | Glides with position | Positive |
| Offered | Solid dot with outer ring | Ring pulses (stops if reduced motion is on; ring stays) | Attention |
| On route | Dot with heading arrow | Glides; arrow points heading | Active |
| Finishing route | Outlined dot with heading arrow | Glides | Active, muted |
| On break | Hatched dot | Static | Neutral |
| Off duty | Small hollow dot (hidden by default layer toggle) | Static | Neutral, faint |
| Suspended | Dot with double ring | Static | Restricted |
| Stale (any status) | Same symbol at reduced opacity | No gliding | — |
| Lost (any status) | Dashed ring "ghost" at last known point, with time since | None | Critical if carrying picked-up orders |
| Waiting order | Hollow pin | — | Neutral |
| Late order | Pin with ring | — | Critical |
| Pickup stop / dropoff stop | Two distinct shapes (e.g. square / circle) with stop number | — | Route colour |

- Movement between updates is interpolated so drivers glide rather than jump; a driver is never animated past its last reported point.
- Selected driver: halo; its route stops numbered in order; next stop emphasised.

---

## 12. Visual design direction

### 12.1 Concept: Cargo and Ops Grey
Decided from the Claude Design mockups (2026-11-03); values in `docs/design/console/README.md` and `tokens.css`, which win over this section on any value.
- **Cargo (dark)**: neutral charcoal ground, white ink, one indigo accent, a bright data palette (green, amber, blue, violet, red). Built for long shifts.
- **Ops Grey (light)**: a grey world where colour appears only when something needs attention.
- Surfaces are rounded cards floating on the ground; controls are pills.
- No decorative illustration; the product name carries the concept.

Defaults: Cargo for signed-in users; Ops Grey for the public view and for README screenshots. Users can switch; the choice persists.

### 12.2 Colour roles (designer assigns values)
Ground, surface, raised surface, ink (primary, secondary, faint), line, brand accent, and the semantic roles: positive, attention, active, neutral, restricted, critical, info. Each role exists in both themes. Map palettes are derived from the same roles. WCAG 2.2 AA contrast for all text and status glyphs against their ground in both themes.

### 12.3 Typography
- Plus Jakarta Sans for UI and headline numbers, IBM Plex Mono for ids, refs, timestamps and numeric columns (from the mockups).
- Monospace for ids, external references, timestamps and numeric columns.
- A condensed weight or tight tracking for dense tables.
- Everything uses tabular figures so live numbers do not shimmer.

### 12.4 Density
Two modes per user: **comfortable** (default on screens < 1600 px) and **compact** (dock rows, tables, panels). Both designed.

### 12.5 Motion
- Purposeful only: driver gliding, the offer pulse, rows settling after a batch update, panel slides.
- No decorative animation; nothing loops except the offer pulse.
- Respect reduced-motion: no gliding (positions step), no pulse (a static ring replaces it).

### 12.6 Iconography
One line-icon family, consistent stroke. Domain icons needed: pickup, dropoff, driver, vehicle types (bike, car, van), zone, proposal, offer, incident, claim, watch/pin, message, import, export, API key, connectivity (live/stale/lost).

### 12.7 Writing
- Plain, short, specific. "Not cancelled — accepted by D-118 at 14:02:11", not "Something went wrong".
- Times: local city time `14:02` in lists, `14:02:11` in timelines and events, relative ("40 s ago") for freshness; timezone shown once in the top bar.
- Durations: `0:28` for countdowns, `4 min` for ETAs and deltas.
- Ids: drivers `D-118`, orders by external ref first, internal id second.
- All strings externalized; layouts mirror-ready for RTL.

---

## 13. Real-time UX rules

1. **Freshness is always visible.** Every live surface knows its last update. If the socket drops: the health strip goes to "Reconnecting…", the map dims slightly with a "Live updates paused — reconnecting" banner, and lists show "as of 14:02:11". On reconnect, the console resyncs and says so briefly.
2. **No silent staleness on drivers.** Stale and lost are always shown on the driver symbol and in every list row.
3. **Commands show pending, then outcome** (section 5.5). Pending never lasts silently: after 5 s it reads "Still waiting for dispatch…".
4. **Lists never jump under the reader** (section 6).
5. **The events stream is honest about volume**: true rate always shown; sampling labelled.
6. **Actor everywhere.** Every timeline line names who or what caused it.
7. **Read-only is explicit.** In the public view on laptop, actions are visible but disabled with a "Read-only demo" tooltip, so reviewers see what a dispatcher could do. On phone (< 768 px) actions are hidden and the sheet says so.
8. **Two clocks never mix.** When a Day profile runs, the scenario clock is visibly separate from the wall clock; latency readings always use the wall clock.

---

## 14. Performance budgets

Design targets; each is measured and published with the command that produced it. The design must not require anything that breaks them (for example, per-driver DOM elements on the map, unbounded lists, or layouts that re-flow on every update).

| Concern | Budget |
| --- | --- |
| Map frame time with 5,000 moving drivers in view (reference laptop; plus a 4× CPU-throttled run) | p95 < 16.7 ms |
| Receive-to-paint latency (socket message to pixels) | p99 < 100 ms, so position-to-screen p99 stays < 500 ms end to end |
| Table scrolling at 100,000 orders | 60 fps; server paging, virtualized rows |
| Live list updates | Batched once per second |
| Events stream | Rendering capped (~50 lines/s); never blocks input |
| Memory over the 2-hour Day-profile soak | Heap flat within ±10% |
| First load | App code < 250 KB gzipped excluding the map library; interactive < 2 s |

Measured during load proof with 200 synthetic dashboard connections plus 3–5 real headless browsers running the console.

---

## 15. Permissions

| Action | Dispatcher | Fleet admin | Public view | Platform operator |
| --- | :-: | :-: | :-: | :-: |
| View Live, lists, events, overview | ✓ (own cities) | ✓ (all cities) | ✓ (demo fleet) | ✓ (via fleet switcher) |
| Approve / reject proposal | ✓ | ✓ | disabled | — |
| Cancel, return to queue, mark delivered / failed | ✓ | ✓ | disabled | — |
| Set duty, suspend, message driver | ✓ | ✓ | disabled | — |
| Claim / acknowledge incident | ✓ | ✓ | disabled | — |
| Watch drivers | ✓ | ✓ | ✓ (local only) | ✓ |
| Import, export | ✓ | ✓ | disabled | — |
| Fleet settings, API keys | — | ✓ | — | — |
| Operator drawer (Day profile, bursts, faults, fleet switch) | — | — | — | ✓ (local only) |

---

## 16. Component inventory

App shell (top bar, nav rail, dock, context panel, watch rail) · city switcher · health strip · KPI chip · status pill (Driver status) · connectivity badge · late ring · order status pill · severity marker · countdown timer · promise-vs-ETA bar · stop list item · timeline item (with actor) · message bubble with status · incident row with progress · proposal row with age · virtualized table (sortable header, row selection, bulk bar, "n new — show" pill) · status tabs with counts · filter bar and chips · search · command palette · toast (critical / watch) · banners (reconnecting, read-only, scenario running) · drawers · confirmation dialog with reason picker · empty / error / skeleton states · file drop zone with job progress · row report table · API key reveal-once card · settings sections · map controls (zoom, layers, theme, focus mode) · map legend · map symbol set (section 11.3) · operator drawer controls · phone bottom sheet.

---

## 17. Phasing (what gets built when)

The console grows with the system; nothing is a separate UI polish slice.

| Release | Console scope added |
| --- | --- |
| R2 Live dispatcher view | App shell, Cargo and Ops Grey modes, Live map with drivers, health strip, receive-to-paint metric |
| R3 Zones | Zone layers and counts; zone events in the events stream |
| R3b Public demo | Sign in, fleet + city scoping, public view and phone layout, read-only states, fleet switcher (operator) |
| R4a Orders and dispatch | Orders layer and assignments on the map |
| R4b Offer protocol | Offers on the map with countdown, declines and expiries in timelines, connectivity badges, lost-driver symbol |
| R4c Multi-order routes | Routes with numbered stops across zones, late ring |
| R4d Ops view | Orders and Drivers lists (dock and full page), order and driver drawers with route and timeline, 15-minute trail |
| C1 Operations | Duty and suspend, cancel, return to queue, mark delivered / failed, review mode and Proposals, alerts and incidents, watch rail, Day overview, Day profile and operator drawer, command palette, Settings (dispatch and thresholds) |
| C2 Integrations | Imports and exports, API keys, messages and thread |
| R8 Load proof | Real-browser measurements against section 14 |
| After v1 | Day replay first; then customer tracking link, planner tuning, analytics, geocoding, zone editor, user management |

---

## 18. What the designer delivers

1. App shell in both themes, all regions expanded, collapsed and in map focus mode.
2. Screens S0–S9 and overlays O1–O7 in Cargo and Ops Grey, at 1440 px; Live and Orders also at 1280 and 1920 px; compact and comfortable density for Orders and Drivers.
3. Every state listed in section 10 for each screen.
4. Map: all three zoom representations, the full symbol set, a selected driver with route, a proposal before/after, an incident footprint, the reconnecting state.
5. Public view on a 390 px phone: map, peeking sheet, expanded sheet, driver detail sheet, explainer.
6. Component sheet covering section 16, with tokens named by role (section 12.2).
7. Clickable prototypes of flows 1, 3, 4 and 10.

---

## 19. Open questions and assumptions

| Item | Status |
| --- | --- |
| Offer expiry default 30 s | Assumed; to be set when the offer protocol (R4b) is built |
| Stale 30 s, lost 10 min, late to assign 10 min | Agreed as defaults; fleet settings |
| Proposal ageing: does an unapproved proposal ever lapse on its own? | Open. Current design: the engine withdraws it if its inputs change (driver no longer eligible); otherwise it waits |
| Quick message templates: who writes them? | Assumed seeded per fleet in v1 |
| Delivery-failed reasons list | Open; designer can use a placeholder list (recipient absent, address wrong, refused, damaged, other) |
| Sound for critical toasts | Agreed; designer proposes the sound and a per-user toggle (on by default) |
| Brand mark | Not designed yet; designer proposes a wordmark within the concept rules (12.1) |
