> Product plan for flotilla-stream. The owner's ideas vault holds the editable
> original; this copy is updated whenever a release is formally re-scoped.
> Execution rules for agents: `../../CLAUDE.md`. Open slice: `../releases/`.

# Real-Time Location & Dispatch Event Platform — Project Plan

Oct 25, 2026 · @Abdo

## Summary

A delivery company's real-time backbone, built and measured at scale: up to 50,000 simulated drivers stream positions, the system tracks them live, raises zone events, assigns orders to the nearest available driver, and survives node and zone failures without losing or duplicating an assignment. It ships in eighteen thin releases in strict order, sized relatively (S < M < L; roughly 1 : 1.5 : 2.5), with R4 and R6 each split into two slices after the slicing review. Every release runs end to end and leaves a demo and its own measured data. Two cut lines — v0.4 after R4a and v0.5 after R4d — are already presentable if interviews take over.

| Release | Size | Thin slice | Demo at the end |
| --- | --- | --- | --- |
| R0 Walking skeleton | L | One driver's position travels from device to map through every layer | A dot moves on a map; one trace spans the whole path |
| R1 Load source and baseline | M | Thousands of simulated drivers on real roads; first throughput and lag numbers | 1,000 drivers moving; baseline numbers in the README |
| R2 Live dispatcher view | M | Viewport-based live fan-out to many dashboards | Ten dashboards watching different areas; position-to-screen latency measured |
| R3 Zones and geofence events | M | Zones, enter/exit and dwell events; state rebuilt by replay | Zone events firing live; a full rebuild from the log matches live state |
| R3b Public always-on demo | M | Hardened edge (TLS, short-lived tokens, read-only public view, rate limits), small fleet on a cheap cluster, cost metered per component | Public URL shows the live map; monthly cost on the dashboard |
| R4a Orders and single-writer dispatch | L | Orders in through the outbox, reservation + pinning, nearest available driver assigned, automatic acceptance | Orders assigned live; zero double assignments under a burst and a consumer restart |
| **v0.4 cut** | — | **First showable system: map, zones, dispatch, invariants 1–3** | **README with numbers; 60-second clip** |
| R4b Offer protocol | L | Offers with expiry, accept/decline, heartbeat loss releasing offers | Device declines and times out live; late accept rejected |
| R4c Multi-order routes | M | Several orders per offer, planner interface, routes across zones, lateness | A driver carries orders from two zones; one turns late |
| R4d Ops view | M | Console read model, lists and drawers, snapshots and client resync | 100,000 orders scroll; consoles converge after a gateway kill |
| **v0.5 cut** | — | **Showable system on a local orchestrated cluster** | **3-minute video and README** |
| R5 Trip history | S | Cold path: trails and events stored for history and analytics | Replay any driver's day; hot path unaffected under load |
| C1 Operations console | L | Duty and suspend, cancel, return to queue, review mode and proposals, alert processor and incidents, watch, Day overview, Day profile | A dispatcher works a lunch-peak outage incident in the console while the Day profile runs |
| C2 Integrations | M | Order import and export jobs, API keys, driver messages through a notification sink with pluggable channels | Re-uploaded import creates no duplicates; a redelivered message event never messages a driver twice |
| R6a Cloud environment | M | Same system on a managed 3-zone cluster from code; existing demo and checks pass unchanged | One command creates and destroys the environment; demo passes there |
| R6b Elasticity | L | Lag-based autoscaling, graceful drain, disruption budgets | A load spike scales consumers out and back with no lost events |
| R7 SLOs and alerting | S | Service-level objectives and burn-rate alerts | Dashboard showing SLOs; an injected slowdown fires the right alert |
| R8 Load proof | M | Step load to the breaking point; cost per unit of load | Published throughput, latency and cost curves |
| R9 Failure proof | M | Fault-injection scenarios with automated invariant checks | Published results: no lost or duplicated assignments, recovery times |
| R10 v1.0 | S | Design write-up, decision records, runbook, video | Tagged release |

## Goals, non-goals and success measures

The goal is evidence, not features: every claim the README makes about scale, latency or correctness is backed by a test anyone can rerun. Targets below are design targets to confirm in R1; the published numbers are whatever the tests actually show, including where the system breaks.

### Goals

1. Ingest a sustained stream of driver positions, keep the latest position of every driver queryable in milliseconds, and keep full history cheaply.
2. Push live positions to many dispatcher screens, each seeing only its own area.
3. Detect zone entry, exit and dwell on every update.
4. Assign orders to the nearest available driver with no double assignment, even under concurrency and failures.
5. Run on an orchestrated, multi-zone cloud platform, delivered declaratively, scaling on load.
6. Publish load results, failure results and cost.

### Non-goals

- A product for real customers (no billing, merchant onboarding, driver app store release).
- Route optimization (the planner is a pluggable function inside dispatch; v1 is nearest driver with greedy stop insertion; an optimizer can be plugged in later, decision 16).
- Turn-by-turn navigation, map tile hosting, a driver mobile app, or any console feature without a backing event. The console's scope is `CONSOLE-PRD.md`.
- Multi-region active-active (single region, multiple availability zones).

### Success measures

| Measure | Target | How it is measured |
| --- | --- | --- |
| Sustained ingest | 50,000 drivers at one update every 5 s = 10,000 events/s | Load generator plus event-log throughput metrics |
| Position-to-screen latency | p99 under 500 ms at target load | Timestamp at the simulated device, compared at the dashboard |
| Latest-position query | p99 under 20 ms | Live gateway metrics |
| Assignment latency | p99 under 1 s from order created to driver offered | Event timestamps across the dispatch flow |
| Correctness | Zero lost assignments, zero double assignments, zero out-of-order position regressions per driver | Automated invariant checker over the event log after every test |
| Recovery | Consumer lag back to normal within 2 minutes after losing one node or one availability zone | Lag metrics during fault injection |
| Elasticity | Consumers scale out within 1 minute of lag crossing threshold and back in without losing in-flight work | Autoscaler events plus invariant checker |
| Cost | Published cost per 1,000 drivers per month at target load | Cloud billing for the benchmark window, normalized |

## Domain and core concepts

The domain is deliberately small so that most of the difficulty sits in the distributed-systems behaviour, not in business rules. Terms are defined in `../../CONTEXT.md`.

| Entity | What it is | Owned by | Key properties |
| --- | --- | --- | --- |
| Fleet | A delivery company; the tenant (decision 15) | Seeded | id, name, settings (assignment mode, thresholds) |
| City | Where a fleet operates | Seeded | id, fleet id, timezone |
| Driver | A courier with a device | Fleet (seeded) | id, fleet id, home zone, vehicle type, capacity (planner input), shift window |
| Position | One location report from a driver's device | Ingest | driver id, device sequence number, device timestamp, coordinates, speed, heading |
| Driver state | The derived, current view of a driver | Location projection and dispatch | latest position, current zone, duty, activity (idle, offered, on route), connectivity (live, stale, lost) |
| Zone | A named polygon inside a city | Order service (zones and fleet settings live with the order API) | id, city id, shape, dwell threshold |
| Order | A pickup and a dropoff, with a promise, to be served | Order service | id, external reference, pickup and dropoff points, pickup-by and deliver-by, status |
| Offer | One or more orders from one pickup zone proposed to one driver, with their stops placed in the driver's route | Dispatch | offer id, driver id, order ids, expiry, outcome |
| Assignment | The binding of one order to one driver; a driver may hold many | Dispatch | order id, driver id, offer id |

### Invariants (the checker asserts these after every test)

1. **One open offer per driver.** A driver never has two open offers at once. A driver may hold many accepted assignments; an offer may carry several orders.
2. **One order, one driver.** An order is never in two open offers or assigned to two drivers.
3. **Every order resolves.** After quiescing (order creation stopped, routes finished), every created order is delivered, failed or cancelled; none is lost.
4. **Positions never go backwards.** For each driver, the projected latest position never moves to an older device sequence number.
5. **Zone events pair up.** Every enter has at most one matching exit; no exit without an enter.
6. **Replay equals live.** Rebuilding any projection from the event log yields the same state as the live projection, compared per driver. Time-driven transitions (`driver.connectivity_changed`, `offer.expired`, `order.late`) are events in the log and are consumed on replay, never re-derived from the wall clock.
7. **No cross-fleet visibility.** No event, live-gateway push or query answer ever reaches a token of another fleet, or a city outside the token's allowed cities (decision 15).

The checker runs only after quiescing: load stopped, routes finished, orders still waiting cancelled through commands from C1 on (their count reported, since waiting orders never expire; before C1, scenarios end with enough drivers on duty that none is left waiting, and any left is reported as a failure), every consumer drained to the log's high-water marks, then asserted. It dedupes by event id itself. Invariant 1 is asserted over `offer.sent` and each offer's outcome (accepted, declined, expired, released).

### Order and driver lifecycle (dispatch view)

- Order: created → waiting → (review mode: proposed → approved) → offered → assigned → picked up → delivered or delivery failed; offered → declined/expired → waiting; proposed → rejected → waiting; assigned and not yet picked up → returned to queue → waiting; cancelled from waiting, proposed, offered, or assigned and not yet picked up. Waiting orders never expire; waiting past a threshold raises an alert.
- Driver: duty (on duty, on break, off duty, suspended) is set by the dispatcher or the device; activity (idle, offered, on route) is set by dispatch only; connectivity (live, stale, lost) is derived from reports. Eligible for an offer = on duty, live, no open offer. Lost releases an open offer only, never accepted orders. Going off duty releases an open offer and lets the route finish.
- Dispatcher actions (cancel, return to queue, approve or reject proposal, mark delivered or failed, set duty, claim or acknowledge an incident) and device reports that can race them are commands, never direct writes: a `command.requested` in the subject's topic, judged by the subject's owner in log order, answered exactly once by the outcome or `command.rejected` (Event catalog § Commands). Every action event carries its actor.
- Arbiter: the dispatch owner is the sole arbiter of an offer. `offer.accepted` references an offer id; an accept arriving after the offer was released or expired is answered with `offer.expired`, and the device must handle "accepted but gone". Accept and heartbeat-loss travel on different partition keys and are never assumed ordered.

## Master architecture

Every write enters through a gateway, lands in a partitioned, replayable event log, and is turned into state by independent processors that each own one job; every read is served from a store shaped for that read. Nothing reads another component's database.

[embedded content: master architecture · technology-neutral]

The dispatch processor also reads the hot state store to find nearby available drivers, and the live gateway answers latest-position queries from it (the only reads of another component's store, all of the hot store, all latency-bound; the live gateway also reads trails there). The per-driver reservation (decision 13) is a write to the hot store by the dispatch processor; see the open item in decision 13; those read paths are left out of the drawing for clarity.

| Component | Responsibility | State | Scales by |
| --- | --- | --- | --- |
| Device simulator | Generates realistic driver movement on a real road network at a configurable rate, for any city given an OpenStreetMap extract; adversarial by default: reordering, duplicates, reboots that reset sequence numbers, reconnect storms, clock skew; also simulates accept/decline, late accepts and device disconnects; a control API for the operator drawer (Day profile, order bursts, fault injection), deployed in the local overlay only | None | Number of simulator instances |
| Ingest gateway | Authenticates devices, validates and normalizes reports, drops duplicates by device sequence, writes reports to the log keyed by driver, and writes device reports that can race a dispatcher as commands | None | Connections and request rate |
| Order service | Order API and import jobs, zone definitions and fleet settings; writes each change and an outbox record in one transaction, a relay publishes them; order status for API reads is served by the ops view, so the order store holds only what this service writes | Order store (its own) | Request rate (low) |
| Event log | Durable, ordered per partition, replayable, retained for days; the system's source of truth for movement and decisions | Partitions | Partition count |
| Location projector | Driver owner: keeps each driver's latest position (newest device sequence wins), the geo index, connectivity (live, stale, lost) and a capped 15-minute trail; folds the per-zone commitments of every zone owner into one activity (idle, offered, on route) and emits it; emits `driver.lost_in_zone` for a lost driver carrying picked-up orders; owns duty commands | Hot state store | Partitions consumed |
| Zone processor | Tests each position against zones; emits enter, exit and dwell events; keeps per-driver zone membership rebuildable from the log | Local, rebuildable | Partitions consumed |
| Dispatch processor | Zone owner: a single writer per zone matches waiting orders to the nearest available driver, sends and releases offers, judges every zone-keyed command, and emits the time-driven facts of its zone (order late, late to assign, offer expiry) and, per driver, its commitment and its route segment | In-memory zone state rebuilt from the log, with a checkpoint | Zone partitions; reads `driver.facts` as a broadcast reader, not as a group member |
| History sink | Batches positions and events into the history store with idempotent writes and retention tiers | History store | Batch size and partitions |
| Alert processor | Derives alerts from events, at most one open per kind and subject; groups them into incidents per kind, zone and window; owns claim and acknowledge commands; resolves when the condition clears | Local, rebuildable from the log | Zone partitions |
| Ops view | Projects dispatch, driver, alert and zone-membership events into the console's read model (orders, drivers with current zone and last seen from connectivity events, proposals, incidents, timelines with actor, counters) and stores each partition's offset in the same transaction as the rows; serves lists with server-side paging, counts and search, drawers, Day overview KPIs, and snapshots stamped with their cut offsets; runs export jobs to object storage | Read model (its own Postgres), object storage for exports | Partitions consumed (projector); request rate (query API) |
| Console API | The console's write entry: checks role, fleet and city (PRD §15) and writes `command.requested`; the console reads from the ops view directly | None | Request rate |
| Identity service | Seeded users, roles and allowed cities; fleet-scoped API keys (hashed, shown once); issues short-lived signed tokens that gateways verify locally | Its own store | Request rate (low) |
| Notification sink | Delivers driver messages through pluggable channels; dedupes by command id | Dedupe set, rebuildable | Partitions consumed |
| Live gateway | Holds dashboard connections; forwards events by map cell, by watched driver id (up to 10) and as a per-city event stream with its true rate and 1-in-N sampling; forwards every event with its topic, partition and offset; filters by fleet and city before sending; serves latest-position and 15-minute trail queries from the hot store | Connection state | Connections |
| Hot state store | Latest position and status per driver; geo index for nearest queries | In memory, persisted for restart | Memory and shards |
| History store | Columnar, compressed, cheap; trails, events, analytics queries | Disk | Storage |

## Core distributed-systems decisions

Seventeen decisions carry the design; each becomes a short decision record in the repository, written in the release that introduces it, and each is a ready-made answer to a system-design interview question.

| # | Decision | Choice | Why | Trade-off accepted | Release |
| --- | --- | --- | --- | --- | --- |
| 1 | Source of truth | An append-only, partitioned event log; stores are projections of it | Replay rebuilds any view; new consumers join without touching producers; failures become "resume from offset" | Eventual consistency between views; log retention must cover the longest rebuild | R0 |
| 2 | Partition keys | Positions by driver; orders, offers, assignments and zone events by zone | Per-driver ordering for movement; one place per zone where dispatch decisions are made | Hot zones become hot partitions; mitigated by splitting large zones into cells | R0, R4 |
| 3 | Delivery guarantee | At-least-once delivery with idempotent consumers, not exactly-once | Simpler, cheaper, and honest about failures; duplicates are made harmless | Every consumer needs an idempotency rule (sequence numbers, upserts, dedupe keys) | R0 |
| 4 | Ordering and late data | (device session id, device sequence number) per driver; the projector keeps only the newest within a session and accepts a new session unconditionally; older reports go to history but never overwrite current state | Mobile networks reorder and replay; clocks drift; a device reboot resets its sequence and must not freeze the driver forever | History may contain out-of-order points; queries sort by (session, sequence) | R1 |
| 5 | Hot path vs cold path | Latest state in an in-memory store with a geo index; history in a columnar store via batched writes | The hot path needs milliseconds; history needs cheap storage and scans | Two stores to operate; history lags the hot path by seconds | R0, R5 |
| 6 | Dispatch concurrency | Single writer per zone partition: one consumer owns all decisions for a zone at a time | One order is never matched twice (invariant 2) by construction; no coordination between zones for order state; easy to reason about and to test | A zone's throughput is bounded by one consumer; ownership moves on rebalance, so state must be rebuildable; **does not by itself protect the driver side (see 13)** | R4a |
| 7 | Writes that cross a database and the log | Transactional outbox: business row and outbox row in one transaction; a relay publishes | No dual-write inconsistency between the order store and the log | Publishing is slightly delayed; the relay must be idempotent | R4 |
| 8 | Backpressure | Bounded queues at every hop; the gateway sheds load with an explicit retry-after rather than accepting what it cannot write; consumers pause on lag | Fail visibly instead of falling over; the simulator honours retry-after like a real device | Some position reports are delayed or dropped under overload, by design and measured | R1, R8 |
| 9 | Live fan-out | Clients subscribe to map areas (cells); the gateway forwards only updates for subscribed cells, coalescing to at most one update per driver per interval | Fan-out cost scales with what is watched, not with fleet size | Viewers may see a position up to one interval old | R2 |
| 10 | Schema evolution | Versioned event schemas with backward-compatible changes only; a compatibility check in the delivery pipeline | Producers and consumers deploy independently | Breaking changes need a new event type and a migration window | R3 |
| 11 | Elasticity | Consumers autoscale on log lag, not CPU; scale-in drains in-flight work and commits offsets before stopping | Lag is the real signal for a stream; a naive scale-in loses or duplicates work | Scaling is bounded by partition count; partition count is chosen up front for the target load | R6 |
| 12 | Failure domains | Every stateful component replicated across availability zones; disruption budgets keep a minimum running during maintenance | Survive losing a node or a zone, the scenarios R9 tests | Cross-zone traffic cost; published with the cost results | R6 |
| 14 | Cost as a measured, pluggable signal | Per-component cost is a first-class metric: an in-cluster cost agent (OpenCost-style) allocates node and store cost to namespaces and workloads from a provider billing adapter; the adapter is pluggable per cloud (GKE first), and cost per 1,000 drivers is a dashboard panel, not a spreadsheet | Cost claims in the write-up must be rerunnable like latency claims; the demo environment needs a budget alert | Allocation is approximate (shared nodes, network egress estimated); accuracy stated with the number | R3b, R8 |
| 13 | Driver-side exclusivity and owner fencing | A driver is pinned to the zone owner of its *projected* current zone; before offering, the owner takes a per-driver reservation (`SET NX` with TTL in the hot store — a lock, named honestly); owners write through a transactional producer per owned partition, so the broker fences a stale owner and readers use `read_committed` (Event catalog § Commands) | A border driver read as available by two zone owners within projector lag would otherwise get two offers (invariant 1); a partitioned-but-alive old owner would otherwise keep offering during rebalance | One extra round-trip per offer; reservation TTL must exceed offer expiry; the lock's failure modes are tested in R9 | R4a |
| 15 | Tenancy | Fleet (a delivery company) is the tenant; logical isolation on shared partitions, consumers and stores; tokens carry fleet and allowed cities; gateways enforce | The console is a product several companies can use; the fleet prefix becomes an enforced, checker-asserted boundary (invariant 7) | A hot fleet can slow others; mitigated only by per-fleet rate limits | R3b |
| 16 | Planner placement and batching scope | The planner is a pure, pluggable function inside each zone owner; one offer groups orders from one pickup zone only; one open offer per driver | Every offer stays one decision by one writer; a real optimizer can replace the v1 planner without touching concurrency | Weaker batches near zone borders; cross-zone batching needs a new decision | R4a |
| 17 | Console frontend stack | TypeScript, React 19, MapLibre; built with Vite (Node at build and dev time only); static output served by Go; client state split by rate so positions and per-second updates stay outside React | Go keeps the backend and resource-intensive work; the console gets the largest ecosystem of accessible components | Node as a build dependency; React re-render cost bounded only by the state tiers | R2 |

One more decision is about restraint: services are split only where scaling or failure behaviour differs (ingest, fan-out, each processor). The order service and the dispatch processor share a domain but not a scaling profile, so they are separate; the zone definitions and the order API share both, so they live in one service. Recording why is part of the deliverable.

## Event catalog

Two partition keys (driver and zone); every event carries an event id (for dedupe), a schema version, a trace context, the fleet id, the actor for action events, the command id for events that answer a command, and the time it happened at the source.

### Topics

Retention is set per topic, so events are grouped into one topic per partition key and retention class. Each entity's events live in exactly one topic, which is what lets a reader version an entity by the offset of the last event applied to it within that topic and partition; the live gateway forwards every event with its topic, partition and offset (decision 17). Adding an event that changes an entity's fields from another topic breaks this rule and needs decision 17 revisited.

| Topic | Key | Holds | Retention |
| --- | --- | --- | --- |
| `position.reported` | driver id | position reports | 3 days |
| `driver.facts` | driver id | connectivity, per-zone commitments, activity, route segments, duty, messages; driver-keyed commands and rejections | 30 days |
| `zone.dispatch` | zone id | orders, offers, assignments, proposals, pickups and deliveries, lateness; zone-keyed dispatch commands and rejections | 30 days |
| `zone.alerts` | zone id; city id for city-scoped info items (import results) | alerts and incidents; their commands and rejections | 7 days |
| `zone.membership` | zone id | enter, exit, dwell | 7 days |
| `fleet.config` | `fleet:zone` for zones, `fleet` for settings | zone definitions and fleet settings, each change with a version number; every processor reads it to the end before starting, and every event derived from config carries the config version it used, so replay uses the same one | forever, not compacted (small) |

Every producer uses the same partitioner, the same partition count per key, and the tenancy-scoped key (`fleet:zone`, `fleet:driver`, decision 15). `offer.sent` carries the pickup zone so the device's reply is keyed to the same zone partition.

### Commands

Command owners: the dispatch processor (orders, offers, proposals, assignments, pickups and deliveries, in `zone.dispatch`), the alert processor (incidents, in `zone.alerts`), the location projector (duty, in `driver.facts`). Only the owner emits the resulting fact.

- **Entry.** Dispatcher commands enter through the console API, which checks role, fleet and city (PRD §15); API-key commands from the order service enter via its outbox. Device reports that can race a dispatcher (picked up, delivered, failed, duty) are commands too, written by the ingest gateway with the device as actor. Offer accepts and declines were already requests judged by the owner (the arbiter rule above).
- **Expected version.** A dispatcher command carries the subject's version the dispatcher saw. If the subject changed since, the owner rejects with `lost_race` and names the events that won ("Not cancelled — accepted by D-118 at 14:02:11"). Device commands carry no version and are judged on the subject's state.
- **Exactly one answer per command id.** The outcome event carrying the command id, or `command.rejected` with a reason: `lost_race`, `invalid_state`, `not_eligible` (for example, driver not live), `not_permitted`, `not_found`.
- **Dedupe and handoff.** Owners dedupe by command id over a window checkpointed with their committed offsets. On taking ownership of a partition, an owner reads to its high-water mark, including its own earlier outputs, before answering anything, so a command answered just before a crash is not answered again. A resend after the window or after the outcome is harmless, because its expected version or the subject's state no longer allows it.
- **Fencing.** A partitioned-but-alive old owner must not have its writes count. Fencing is on write: each owner writes through a transactional producer per owned input partition, so outputs and offsets commit together and the broker rejects a fenced producer; readers use `read_committed`. Reader-side epoch filtering is rejected: topics with several producers (`driver.facts` gets events from every zone owner) mix epochs, and a valid owner's event would be dropped. The latency cost is measured in decision 13 (R4a).
- **Tenancy.** The owner checks that the subject's fleet and zone match the command's fleet and key; a mismatch is rejected as `not_found` with no subject data. A rejection never carries another fleet's data (invariant 7).
- **Cancel inside an open offer.** Cancelling one order of an open multi-order offer releases the whole offer (`offer.released`, the other orders back to waiting) before `order.cancelled`; the driver may receive a new offer.
- **Messages** cannot race, so `driver.message_requested` is itself the command; the notification sink dedupes it by command id.

### Time

Time-driven facts (`driver.connectivity_changed`, `offer.expired`, `order.late`, `order.late_to_assign`, alert windows) run on event time, the newest ingest timestamp seen on the partition, never the wall clock, and timers arm only after the owner has caught up to the high-water mark. A projector that was down for 45 s therefore does not mark every driver stale on restart, and replay produces the same facts.

Event names in rows marked R4a–C2 below the original thirteen are proposed and fixed when their slice opens. The ops view and the history sink consume every event in `driver.facts`, `zone.dispatch` and `zone.alerts`; they are left out of the Consumers column.

| Event | Partition key | Producer | Consumers | Topic | Release |
| --- | --- | --- | --- | --- | --- |
| position.reported | driver id | Ingest gateway | Location projector, zone processor, history sink | `position.reported` | R0 |
| zone.entered / zone.exited | zone id | Zone processor | Live gateway, dispatch processor, history sink | `zone.membership` | R3 |
| zone.dwell_exceeded | zone id | Zone processor | Live gateway (alert), alert processor, history sink | `zone.membership` | R3 |
| order.created | zone id (of pickup) | Order service via outbox | Dispatch processor, live gateway, history sink | `zone.dispatch` | R4 |
| order.cancelled | zone id (of pickup) | Dispatch processor (on a cancel command) | Order service, live gateway, alert processor, history sink | `zone.dispatch` | C1 |
| offer.sent | zone id (of pickup) | Dispatch processor | Simulator (device), live gateway, history sink | `zone.dispatch` | R4 |
| offer.accepted / offer.declined | zone id (of pickup, from `offer.sent`) | Ingest gateway (from device) | Dispatch processor | `zone.dispatch` | R4 |
| offer.expired | zone id | Dispatch processor | History sink, live gateway | `zone.dispatch` | R4 |
| offer.released | zone id | Dispatch processor (driver lost, off duty, or a cancel of one of its orders) | Simulator (device), live gateway, history sink | `zone.dispatch` | R4 |
| assignment.confirmed | zone id | Dispatch processor | Live gateway, history sink | `zone.dispatch` | R4 |
| order.picked_up / order.delivered / order.delivery_failed | zone id (of pickup) | Dispatch processor (on a device report or a dispatcher override, both commands) | Order service, live gateway, alert processor, history sink | `zone.dispatch` | R4a |
| route.updated (one zone's segment of a driver's route: stops, planned times, ETAs) | driver id | Dispatch processor (each zone owner, for its own stops) | Live gateway, location projector (merged route for the planner) | `driver.facts` | R4c |
| order.late / order.late_to_assign | zone id (of pickup) | Dispatch processor, on its clock (stop time past promise; waiting past the fleet threshold) | Alert processor, live gateway | `zone.dispatch` | R4c (late), C1 (late to assign) |
| driver.connectivity_changed (live, stale, lost) | driver id | Location projector (stale after 30 s and lost after 10 min without a report, fleet settings) | Dispatch processor (lost releases an open offer), live gateway, alert processor | `driver.facts` | R2 (live, stale), R4b (lost) |
| driver.activity_changed (idle, offered, on route with stop count) | driver id | Location projector, folding every zone's `driver.commitment_changed` | Live gateway, dispatch processor | `driver.facts` | R4c |
| driver.commitment_changed (this zone's open offer, assigned and picked-up orders for the driver) | driver id | Dispatch processor (each zone owner, for its own orders) | Location projector | `driver.facts` | R4c |
| driver.duty_changed | driver id | Location projector (on a set-duty command from a dispatcher or the device) | Dispatch processor, live gateway | `driver.facts` | C1 |
| proposal.created / proposal.approved / proposal.rejected | zone id | Dispatch processor (created; approved and rejected on an approve or reject command) | Dispatch processor, live gateway, history sink | `zone.dispatch` | C1 |
| proposal.withdrawn | zone id | Dispatch processor (its inputs changed: driver no longer eligible) | Live gateway | `zone.dispatch` | C1 |
| assignment.released (return to queue) | zone id | Dispatch processor (on a return-to-queue command) | Dispatch processor, order service, live gateway, history sink | `zone.dispatch` | C1 |
| alert.raised / alert.claimed / alert.acknowledged / alert.resolved | zone id | Alert processor (raised, resolved; claimed and acknowledged on a claim or acknowledge command) | Live gateway, history sink | `zone.alerts` | C1 |
| driver.lost_in_zone (picked-up order count) | zone id (driver's last zone) | Location projector, when a driver carrying picked-up orders goes lost | Alert processor | `zone.alerts` | C1 |
| driver.message_requested / driver.message_delivered / driver.message_failed | driver id | Console API (requested; it is the command), notification sink (delivered, failed) | Notification sink, live gateway, history sink | `driver.facts` | C2 |
| driver.message_read | driver id | Ingest gateway (from device) | Live gateway | `driver.facts` | C2 |
| zone.defined / fleet.settings_changed | `fleet:zone` / `fleet` | Seed job (R3); order service via outbox (from R4a) | Every processor, live gateway | `fleet.config` | R3 (zones), C1 (settings) |
| import.finished (created, skipped, rejected counts) | city id (an import job belongs to one city) | Order service via outbox | Alert processor (info item), live gateway | `zone.alerts` | C2 |
| command.requested | the subject's key | Console API (dispatcher); ingest gateway (device reports that can race); order service via outbox (API-key cancels) | The subject's owner | The subject's topic | R4a (device reports), C1 (dispatcher) |
| command.rejected | the command's key | The command's owner | Live gateway, history sink | The command's topic | R4a, C1 |

Retention is long enough to rebuild every projection from scratch; history beyond that lives in the history store.

## Platform and operations

The same declarative deployment runs in all three environments from release 0, so moving to the cloud in R6 changes the target, not the system.

### Environments

| Environment | Purpose | When it exists | Cost posture |
| --- | --- | --- | --- |
| Local orchestrated cluster | Daily development; every release demo up to v0.5 | From R0 | Free |
| Low-cost remote cluster | Always-on public demo with a small simulated fleet; a second Argo CD target that every later slice also deploys to | From R3b | A few dollars a month, metered by the cost agent and shown on the public dashboard |
| Managed cloud cluster, three availability zones | Autoscaling, load proof and failure proof | R6 to R9, then torn down | Pay for days, not months; interruptible capacity where safe |

### Delivery pipeline

1. Every change: build, unit tests, contract tests on event schemas (compatibility check), integration tests against real dependencies in containers.
2. Main branch: images published, deployment manifests updated, a declarative delivery controller syncs the cluster to the repository.
3. Infrastructure (network, cluster, managed stores) defined as code; one command creates and destroys the cloud environment.
4. Every release tag runs the release's demo script and its invariant checks automatically.

### Observability

- **Traces** propagated through every event, so one position can be followed from device to screen across asynchronous hops.
- **Metrics** per component: rate, errors, duration; per consumer: lag and processing time; per gateway: connections and shed load.
- **Logs** structured, correlated by trace id.
- **Dashboards:** one golden-signals board per component, plus one end-to-end board for the SLOs.

### Service-level objectives (introduced in R7)

| SLO | Objective | Window |
| --- | --- | --- |
| Position freshness on screen | 99% of updates visible within 1 s | the 2-hour soak run |
| Ingest availability | 99.9% of valid reports accepted | the 2-hour soak run |
| Assignment timeliness | 99% of orders offered within 5 s when a driver is available | the 2-hour soak run |

Alerts fire on error-budget burn rate, not on single spikes. The window is the soak run because that is how long the system actually runs; the write-up states what the 28-day equivalent would be and why it was not measured.

### Security baseline

- Device and console authentication with short-lived tokens carrying the fleet id and allowed cities; every key, topic record and query is scoped by fleet id; both gateways enforce it (decision 15, invariant 7). Integrations use fleet-scoped, revocable API keys.
- Secrets from the platform's secret store, never in the repository.
- Rate limits per device and per client at the gateways.

Tenancy is logical isolation on shared infrastructure, asserted by the checker. Physical per-tenant isolation (partitions, consumers or stores per fleet), tenant self-signup and billing are out of scope.

## Release plan

Each release is a vertical slice through the whole system, sized S, M or L relative to the others, and ends with a demo script, its own measured data in the README and a decision record. Nothing starts until the previous release's exit criteria pass.

[embedded content: release roadmap · target dates]

### R0 Walking skeleton (L)

- **In:** one simulated driver; ingest gateway; event log with one topic; location projector; hot state store; live gateway pushing to a bare map; declarative deployment to the local orchestrated cluster; delivery pipeline; one trace spanning device to screen.
- **Out:** zones, orders, history, scaling, auth beyond a static token.
- **Demo:** start everything with one command; a dot moves on the map; open the trace.
- **Exit:** clean checkout to running demo in under 10 minutes; pipeline green; decisions 1, 3 and 5 recorded.

### R1 Load source and baseline (M)

- **In:** simulator moves thousands of drivers along a real city road network with realistic speeds, stops and connection drops; device sequence numbers; out-of-order and duplicate injection; gateway dedupe and bounded queues with retry-after; throughput, lag and latency metrics.
- **Out:** cloud, autoscaling.
- **Demo:** 1,000 drivers moving; dashboard of ingest rate, lag and latency.
- **Exit:** baseline numbers in the README, including a step load to the local machine's limit; invariant 4 (positions never go backwards) passes with injected reordering; decisions 4 and 8 recorded.

### R2 Live dispatcher view (M)

- **In:** cell-based area subscriptions; per-driver coalescing; driver-state changes on screen; latency measured from device timestamp to the browser's paint; several gateway replicas behind a balancer; the console shell from `CONSOLE-PRD.md` (Cargo and Ops Grey modes, live map, health strip).
- **Out:** alerts, assignment overlays.
- **Demo:** ten dashboards watching different areas, each receiving only its own drivers.
- **Exit:** position-to-screen p99 recorded at 1,000 drivers; fan-out messages scale with watched cells, not fleet size (measured); killing one gateway replica moves its viewers to another within seconds; decision 9 recorded.

### R3 Zones and geofence events (M)

- **In:** zone definitions published as `zone.defined` on `fleet.config` by a seed job (the order service takes over in R4a); enter, exit and dwell detection; the live gateway's per-city event stream (true rate, 1-in-N sampling); zone events on the map; schema versioning and the compatibility check; a rebuild command that replays the log into fresh state.
- **Out:** zone editing UI (zones are seeded).
- **Demo:** drivers crossing zones light them up; run the rebuild and compare.
- **Exit:** invariants 5 (zone events pair up) and 6 (replay equals live) pass; decision 10 recorded.

### R3b Public always-on demo (M)

- **In:** a cheap always-on cluster (smallest GKE regional footprint on spot nodes, or a single-node k3s host if cheaper — chosen by measured monthly cost); the same Kustomize base with a `demo` overlay; Argo CD syncing it as a second target; TLS via cert-manager; short-lived signed tokens for devices and dashboards, issued by the identity service (seeded users, roles, allowed cities) and verified locally by the gateways; the public dashboard is read-only (no order or zone writes exposed); rate limits per device and per client; secrets from the platform's secret store; the cost agent and a budget alert; a 100-driver fleet in a rotating city.
- **Also in:** tenancy (decision 15): tokens carry fleet and allowed cities; two seeded fleets; invariant 7 asserted; seeded users with dispatcher and fleet-admin roles; the console's public view and phone layout.
- **Out:** user management UI, public write APIs, SSO, physical tenant isolation, tenant self-signup.
- **Demo:** open the public URL from a phone; the map is live; the cost panel shows today's spend.
- **Exit:** public URL up; an unauthenticated write attempt is rejected and logged; token expiry is enforced (tested); monthly cost projected from 24 h of metering and published; decision 14 recorded; every later slice's demo script also runs against the `demo` overlay.

### R4 Orders and dispatch (split into R4a, R4b, R4c, R4d)

Split twice after review: once to separate the offer protocol (R4b), then again after the whole-product architecture audit (2026-10-26), when multi-order routes and the console read side made R4a about two L slices plus an M.

#### R4a Orders and single-writer dispatch (L)

- **In:** order service with transactional outbox; zone-partitioned dispatch with a single writer per zone; driver pinning to the projected zone, per-driver reservation and owner fencing through transactional producers (decision 13); nearest-available matching from the geo index; one order per offer, accepted automatically; pickup and delivery reported by the simulator as device commands, so every order can resolve; the order service takes over `zone.defined`; assignments on the map.
- **Out:** offer expiry and declines (R4b), several orders per offer and routes (R4c), console lists and drawers (R4d).
- **Demo:** orders appear and get assigned live; a concurrency test fires bursts of orders into one zone.
- **Exit:** invariants 1, 2 and 3 pass under the burst test, under a consumer restart mid-burst, under an owner frozen past its session timeout (its writes fenced), and under a border-driver test (drivers oscillating across a zone boundary while both zones receive orders); assignment p99 recorded, with the transaction commit cost; decisions 2, 6, 7 and 13 recorded, including the reservation open item.

#### R4b Offer protocol (L)

- **In:** offers with expiry, accept and decline, late-accept rejection, `driver.connectivity_changed` lost releasing open offers (`offer.released`).

#### R4c Multi-order routes (M)

- **In:** drivers hold many assignments; one offer may carry several orders from one pickup zone; one open offer per driver; v1 planner behind an interface (decision 16); `driver.commitment_changed` and `route.updated` segments from each zone owner, activity folded by the location projector; `order.late` on event time; routes and late rings on the map.
- **Out:** route optimization, cross-zone batching in one offer.
- **Demo:** a driver carries orders from two zones; its route shows both segments; one order turns late.
- **Exit:** invariants 1–3 pass with multi-order offers under the burst and border tests; a driver with stops in two zones never shows idle while either zone has stops; decision 16 recorded.

#### R4d Ops view (M)

- **In:** the ops view (read model with per-partition offsets, server-side paging, counts and search, snapshots stamped with cut offsets); console orders and drivers lists, order and driver drawers with route and timeline; the 15-minute trail served by the live gateway; the client resync of decision 17.
- **Demo:** 100,000 orders in the list scroll smoothly; a gateway replica is killed and every open console converges.
- **Exit:** the convergence test of decision 17 passes (client state equals a fresh snapshot after duplicates, reordering, a dropped range and a replica kill); table scroll at 100,000 orders measured; rebuilding the read model from the log equals live (invariant 6 for the ops view).

### v0.4 cut line (after R4a)

- **Deliverable:** README with architecture, numbers so far and decision records 1–7, 10, 13; a 60-second clip of orders being assigned on the live map with zones lit.
- **Rule:** the first artifact that can go into an application or an interview. If R4b is not done when an interview needs it, this is what is shown.

### v0.5 cut line (after R4d)

- **Deliverable:** README with architecture, numbers so far and decision records; a 3-minute video; resume and profile updated.
- **Rule:** if interviews or an offer take priority, the project can stop here and still stand as a portfolio piece.

### R5 Trip history (S)

- **In:** history sink with batched, idempotent writes; retention tiers; replay of any driver's day on the map; a few analytics queries (busiest zones, average time to assignment).
- **Demo:** pick a driver, replay their day.
- **Exit:** hot-path latency unchanged with the sink running at full rate (measured); duplicate deliveries do not duplicate history rows.

### C1 Operations console (L)

- **In:** duty (on duty, on break, off duty, suspended) with release of open offers; dispatcher commands (cancel, return to queue, mark delivered or failed) as `command.requested` events processed by the zone owner, each answered by its outcome or `command.rejected`; assignment mode (auto, review) and proposals; the console API as the command entry with role, fleet and city checks; fleet settings on `fleet.config`; an alert processor deriving alerts, grouping them into incidents, with claim and acknowledge as commands; `order.late_to_assign`, `driver.lost_in_zone`, `proposal.withdrawn`; watch through driver-id subscriptions on the live gateway; Day overview from the ops view; the Day profile in the simulator; the operator drawer backed by the simulator's control API (local overlay only); command palette.
- **Demo:** Day profile at lunch peak; a cell outage drops 30 drivers; one incident appears, is claimed, and resolves as drivers return.
- **Exit:** invariants 1–3 and 7 pass over a full Day profile run including dispatcher commands racing device accepts; every command id is answered exactly once in the log by the current owner (one outcome or one rejection; fenced answers of an old owner do not count), including commands resent after a reconnect and commands in flight across an owner crash; alert processor idempotent under redelivery (measured: alerts per condition = 1).

### C2 Integrations (M)

- **In:** import jobs (CSV and API batch) idempotent on (fleet, external reference), owned by the order service, ending in `import.finished`; export jobs (CSV, NDJSON) run by the ops view into object storage; fleet-scoped API keys with scopes in the identity service; `driver.message_read` from the device; driver messages through a notification sink with a pluggable channel interface (device push first).
- **Demo:** upload a file, re-upload it, see zero duplicates; message a driver and see delivered, then read.
- **Exit:** re-import creates zero orders (checker count vs order store count); a redelivered message event produces one channel send (sink log vs channel log).

### R6 Cloud platform (M + L, split)

- **In:** infrastructure as code for network, cluster and managed stores across three availability zones; declarative delivery controller; lag-based autoscaling for consumers and connection-based scaling for gateways; graceful drain on scale-in and deploys; disruption budgets; secrets and network policies.
- **Demo:** a load spike scales consumers out and back in, live, with the invariant checker running.
- **Exit:** one command creates and destroys the environment; scale-out within 1 minute of the lag threshold; zero invariant violations during scale-in; decisions 11 and 12 recorded.

*Split after review:* **R6a (M)**: cloud environment from code, with the existing demo and invariant checks passing there unchanged. **R6b (L)**: lag-based autoscaling, graceful drain, disruption budgets.

### R7 SLOs and alerting (S)

- **In:** the three SLOs; burn-rate alerts; an end-to-end dashboard; a short runbook per alert.
- **Demo:** inject a slowdown in the zone processor; the right alert fires and the runbook resolves it.
- **Exit:** every alert has a runbook entry and was fired at least once on purpose.

### R8 Load proof (M)

- **In:** step load 1k → 5k → 10k → 25k → 50k drivers, holding each step; identify the first bottleneck and fix or document it; cost per 1,000 drivers.
- **Demo:** the published curves.
- **Exit:** results page with throughput, latency percentiles, lag, cost and the breaking point, plus what broke first and why.

### R9 Failure proof (M)

- **In:** fault-injection scenarios (next section) run under steady load with the invariant checker.
- **Demo:** kill a node and a zone while orders flow; the map keeps moving.
- **Exit:** results table with recovery time per scenario and zero invariant violations, or the violation found and fixed.

### R10 v1.0 (S)

- **In:** design write-up (architecture, the twelve decisions, capacity estimate, cost, results, what I would do next); final video; tagged release; cloud environment destroyed.
- **Exit:** a stranger can run the local demo from the README, and an interviewer can read the design in ten minutes.

## Slicing assessment

The plan is mostly well sliced: eight of eleven releases are true vertical slices that end in a demo, and the most impressive demo (live map with dispatch) lands by v0.5. The review found four weaknesses: two horizontal slices, one oversized slice, and evidence bunched at the end. All four are fixed below, and the release sections above have been updated to match.

### Criteria

- **Vertical:** touches the path from device or client to screen, not one layer.
- **Runs end to end:** the whole system works at the end of the slice, with no half-built paths.
- **Small:** M or smaller preferred; L only where splitting would leave a half-built path.
- **Leaves evidence:** a demo script plus a measured number or a passing invariant.
- **Cut-able:** stopping after this slice still leaves a coherent project.
- **Retires risk early:** the hardest unknowns come before the polish.

### Scorecard

| Release | Vertical | Runs end to end | Small | Leaves evidence | Cut-able | Assessment |
| --- | --- | --- | --- | --- | --- | --- |
| R0 Walking skeleton | Yes | Yes | L (includes pipeline and cluster) | Yes | Yes | Good; the end-to-end trace may slip to R1 without harm |
| R1 Load source, baseline | Yes | Yes | Yes | Yes | Yes | Good; it retires the biggest unknown (can the pipe take load?) in week 1 |
| R2 Live dispatcher view | Yes | Yes | Yes | Yes | Yes | Good |
| R3 Zones, geofence events | Yes | Yes | Yes | Yes | Yes | Good |
| R4 Orders and dispatch | Yes | Yes | **No: XL, five new concepts** | Yes | Yes | **Oversized; split** |
| R5 Trip history | Yes | Yes | Yes | Yes | Yes | Good |
| R6 Cloud platform | **No: platform only** | Yes | **Borderline: XL** | Yes | Yes | **Horizontal and the riskiest slice; split and de-risked** |
| R7 SLOs and alerting | **No: observability only** | Yes | Yes | Yes | Yes | Acceptable: small and time-boxed; per-release metrics already exist |
| R8 Load proof | Evidence slice | Yes | Yes | Yes | Yes | Intentional, but depends on R6 |
| R9 Failure proof | Evidence slice | Yes | Yes | Yes | Yes | Intentional, but depends on R6 |
| R10 v1.0 | Write-up | — | Yes | Yes | — | Fine because write-ups accumulate in every release |

### Fixes applied

| Weakness | Fix |
| --- | --- |
| R4 bundles outbox, single-writer matching, the offer protocol and heartbeat handling | Split into **R4a (L):** orders in through the outbox and single-writer matching with reservation, pinning, fencing and automatic acceptance, checking invariants 1–3; and **R4b (L):** offers, accept and decline, expiry, heartbeat loss. R4a alone is a complete, demoable dispatch system (the v0.4 cut) |
| R6 is a five-day platform slice where cloud surprises would land late | The same declarative deployment runs on the local orchestrated cluster from R0, so R6 changes the target, not the system. R6 is split into **R6a (M):** cloud environment from code plus the existing demo passing there unchanged; and **R6b (L):** lag-based autoscaling, graceful drain, disruption budgets |
| Load and failure evidence bunched in R8–R9, behind the riskiest slice | Smaller versions run earlier on the local cluster: a gateway replica killed in R2, a consumer restarted mid-burst in R4a, and a step load to the local machine's limit in R1. If R6 slips, v0.5 still carries real failure and load evidence |
| R7 is horizontal | Each release already ships its own metrics as part of its exit criteria; R7 only adds SLO definitions, burn-rate alerts and runbooks on top; sized S |

### What the slicing deliberately does not do

- **No microservice per entity.** Boundaries follow scaling and failure behaviour, and the decision records say why.
- **No UI polish slice.** The console grows inside the slices that produce its data (R2, R3, R3b, R4a, R4b, C1, C2); no console feature exists without a backing event.
- **No parallel tracks.** One person, one slice at a time; the gate between slices is the passing exit criteria.

## Evidence plan

Three kinds of evidence, each rerunnable from the repository: load results, failure results, and an invariant checker that judges both.

### Invariant checker

A standalone job that reads the event log and the stores after any test and asserts the seven invariants from the domain section. It runs in every release's exit criteria and after every load and failure scenario. A test passes only if the checker passes.

### Load profiles

| Profile | Shape | What it answers |
| --- | --- | --- |
| Step | 1k → 5k → 10k → 25k → 50k drivers, 10 minutes per step | Where latency and lag start to bend; the first bottleneck |
| Soak | Target load for 2 hours | Memory growth, store compaction, connection churn |
| Burst | Normal load plus 10× orders into one zone for 2 minutes | Hot partition behaviour; single-writer limits; backpressure |
| Reconnect storm | 50% of devices disconnect and reconnect within 30 s | Gateway admission control; duplicate and out-of-order handling |
| Fan-out | 200 dashboards watching overlapping areas | Live gateway cost per viewer |

### Failure scenarios

| Scenario | Expected behaviour | Measured |
| --- | --- | --- |
| Kill one ingest gateway replica | Devices reconnect to others; no accepted report lost | Reconnect time; reports dropped vs retried |
| Kill a consumer mid-batch | Another takes its partitions; work resumes from committed position; duplicates absorbed | Rebalance time; lag peak; invariant results |
| Kill the dispatch owner of a busy zone during a burst | New owner rebuilds zone state and continues; no double or lost assignment | Recovery time; invariants 1–3 |
| Lose one node of the event log | Writes continue on replicas | Write latency during failover |
| Lose a whole availability zone | Service continues in the remaining zones within SLO | Recovery time; SLO budget spent |
| Network delay and loss between processors and stores | Bounded queues fill; backpressure engages; nothing crashes | Lag, shed load, recovery |
| Hot-state store restart | State rebuilt from the log; live view resumes | Rebuild time at target fleet size |
| Scale-in during load | Drained consumers commit and leave; no duplicates or losses | Invariant results |

### Published write-up (R10)

1. Architecture overview and the twelve decision records.
2. Capacity estimate made before testing, compared with what was measured.
3. Load results: curves, percentiles, breaking point, first bottleneck and its fix.
4. Failure results: the table above, filled in.
5. Cost: total for the benchmark window and per 1,000 drivers per month.
6. What would change at 10× scale, and what was deliberately left out.

## Repository and deliverables

One repository, organized so a reviewer finds the design before the code.

| Path | Contents |
| --- | --- |
| README | One-paragraph pitch, the architecture picture, the headline numbers, one-command local start, links to everything below |
| docs/design | The design write-up |
| docs/decisions | One short record per decision, numbered as in this plan |
| docs/results | Load and failure results per release, with the commands that produced them |
| docs/runbook | One entry per alert |
| services/ | One folder per deployable component |
| simulator/ | Device simulator and load profiles |
| checker/ | Invariant checker |
| deploy/ | Declarative deployment, shared by every environment |
| infra/ | Infrastructure as code for the cloud environment |
| demo/ | Demo scripts per release, plus the video link |

Outside the repository: a short post per release (what was measured, what broke), the resume line once v0.5 exists, and the 3-minute video.

## Risks and open questions

| Risk | Mitigation |
| --- | --- |
| Platform work becomes a rabbit hole | Local orchestrated cluster from R0; R6 split; time-box each platform slice and record what was cut |
| The local machine cannot generate enough load | Measure the local ceiling in R1; run full-scale load only in the cloud (R8) |
| Cloud costs climb during benchmarks | One benchmark window; interruptible capacity for simulators and stateless parts; destroy the environment the same day; publish the bill |
| The road-network simulation takes longer than planned | Start with random walks between real road points; add realistic routing only if time allows |
| Interviews compress the calendar | The v0.4 and v0.5 cut lines; order of slices never changes, only how far it gets |
| Too many components to keep running locally | A single command starts everything; components that are not under change can run as one combined process locally |

Open questions:

- [x] City: the simulator takes any OpenStreetMap extract; demos rotate cities (Cairo, Riyadh, Barcelona, Berlin) to match the audience. The dashboard's strings are externalized and its layout RTL-capable in design; Arabic/RTL is implemented only after v1.0.
- [x] Public always-on demo: yes, as slice R3b, hardened before exposure, cost metered and shown.
- [x] Name: **Flotilla**. This plan is `flotilla-stream`; `flotilla-sim`, `flotilla-dispatch` (routing/optimization, a non-goal here), `flotilla-connect` (integrations) are reserved for later extraction.
- [x] Hours per week: not planned. The build is AI-paired; sizes are relative, gates are exit criteria, and the only clock is the interview calendar.

---

## Changelog

- 2026-10-25 — baseline copied verbatim from `ideas_portfolio_and_master_plan.md`.
- 2026-10-25 (re-scope after critique, see `assessment.md`) — dates replaced by relative sizes; v0.4 cut added after R4a; decision 13 (driver reservation + zone pinning + owner-epoch fencing) added and decision 6 corrected; decision 4 keyed on (session id, sequence); invariant 6 and the checker made explicit about time-driven events and quiescing; dispatch made sole arbiter of offers; SLO window set to the soak run; security baseline trimmed to one fleet; simulator made multi-city and adversarial; project named Flotilla. Scope otherwise unchanged — no slice was cut on time grounds.
- 2026-10-25 (owner decisions, second round) — R3b public always-on demo added as a slice with security hardening as its exit criteria; decision 14 (cost as a measured, pluggable signal) added; the demo cluster becomes a second Argo CD target for every later slice.
- 2026-10-25 (console re-scope, design grilling) — the console becomes the product UI (`CONSOLE-PRD.md`); fleet becomes an enforced tenant with cities (decision 15, invariant 7); drivers carry many orders, offers group orders from one pickup zone, one open offer per driver (decision 16; invariants 1–3 rewritten); duty, activity and connectivity split; pickup and delivery events, proposals, return to queue, alerts and incidents, messages added to the event catalog; console frontend stack (decision 17); slices C1 and C2 added after R5; R2, R3b and R4a re-scoped.
- 2026-10-26 (client state, decision 17) — commands become `command.requested` events with a command id, placed with their subject's events and answered once by the outcome or the new `command.rejected`; owners dedupe by command id; `order.cancelled` moves to C1 and is produced by the dispatch processor; console-produced rows now name their owner; envelope gains the command id; each entity's events live in one topic (new Topics table: `driver.facts`, `zone.dispatch`, `zone.alerts`, `zone.membership`, plus R0's `position.reported`), so clients version entities by offset within topic and partition; device reports that can race a dispatcher (picked up, delivered, failed, duty) become commands judged by the owner; commands carry the expected subject version; owners catch up to the high-water mark before answering and are fenced per partition; `offer.released` added to the catalog; the Retention column becomes Topic. Decision 17 changed with the owner: Go for the backend and resource-intensive work; the console is TypeScript + React 19 + MapLibre built with Vite, Node at build and dev time only, static output served by Go.
- 2026-10-26 (owner decision) — the event log is Apache Kafka 4.x in KRaft mode instead of Redpanda, so the project exercises Kafka's own replication (ISR, `min.insync.replicas`, leader election, rack awareness) in the failure tests; run through Strimzi on kind and GKE. The plan text stays technology-neutral; the tool is named in `README.md` and `docs/releases/R0.md`.
- 2026-10-26 (whole-product architecture audit against CONSOLE-PRD) — components added: ops view (console read model with per-partition offsets, queries, snapshots, exports), console API (command entry with role checks), identity service, alert processor, notification sink, operator control; the dispatch processor no longer lists the order store as its state (one writer per store); zones and fleet settings published by the order service on a compacted `fleet.config` topic; time-driven facts become events (`order.late`, `order.late_to_assign`, `route.updated`); `driver.state_changed` and `driver.heartbeat_lost` replaced by `driver.activity_changed` and `driver.connectivity_changed`; `proposal.withdrawn`, `driver.lost_with_orders`, `driver.message_read`, `import.finished`, `zone.defined`, `fleet.settings_changed` added; the live gateway gains watch subscriptions and a per-city event stream; release In-lists updated (R3, R3b, R4a, C1, C2). R4a grows by the ops view.
- 2026-10-26 (second architecture critique) — fencing is on write through transactional producers (reader-side epochs rejected: `driver.facts` has several producers); the location projector becomes the driver owner and alone emits activity, folding each zone's new `driver.commitment_changed`; `route.updated` becomes a per-zone segment keyed by driver; `driver.lost_with_orders` replaced by `driver.lost_in_zone` from the driver owner; time-driven facts run on event time and arm after catch-up; `fleet.config` kept forever with versions stamped on derived events; a seed job publishes zones in R3; operator control folded into the simulator's control API; the order service no longer mirrors dispatch outcomes; quiescing cancels still-waiting orders. Open for R4a: the reservation is not in the log (decision 13).
- 2026-10-26 (owner decision) — R4 split further: R4a single-writer dispatch with one order per offer (still the v0.4 cut), R4b offer protocol, R4c multi-order routes (M), R4d ops view (M); eighteen slices.
