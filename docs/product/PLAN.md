> Product plan for flotilla-stream. The owner's ideas vault holds the editable
> original; this copy is updated whenever a release is formally re-scoped.
> Execution rules for agents: `../../CLAUDE.md`. Open slice: `../releases/`.

# Real-Time Location & Dispatch Event Platform — Project Plan

Oct 25, 2026 · @Abdo

## Summary

A delivery company's real-time backbone, built and measured at scale: up to 50,000 simulated drivers stream positions, the system tracks them live, raises zone events, assigns orders to the nearest available driver, and survives node and zone failures without losing or duplicating an assignment. It ships in fourteen thin releases in strict order, sized relatively (S < M < L; roughly 1 : 1.5 : 2.5), with R4 and R6 each split into two slices after the slicing review. Every release runs end to end and leaves a demo and its own measured data. Two cut lines — v0.4 after R4a and v0.5 after R4b — are already presentable if interviews take over.

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
| **v0.5 cut** | — | **Showable system on a local orchestrated cluster** | **3-minute video and README** |
| R5 Trip history | S | Cold path: trails and events stored for history and analytics | Replay any driver's day; hot path unaffected under load |
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
- Route optimization (the dispatch decision is nearest-available with simple rules; an optimizer can be added later).
- Turn-by-turn navigation, map tile hosting, or a polished UI beyond a live map and a dashboard.
- Multi-region active-active (single region, multiple availability zones).

### Success measures

| Measure | Target | How it is measured |
| --- | --- | --- |
| Sustained ingest | 50,000 drivers at one update every 5 s = 10,000 events/s | Load generator plus event-log throughput metrics |
| Position-to-screen latency | p99 under 500 ms at target load | Timestamp at the simulated device, compared at the dashboard |
| Latest-position query | p99 under 20 ms | Query service metrics |
| Assignment latency | p99 under 1 s from order created to driver offered | Event timestamps across the dispatch flow |
| Correctness | Zero lost assignments, zero double assignments, zero out-of-order position regressions per driver | Automated invariant checker over the event log after every test |
| Recovery | Consumer lag back to normal within 2 minutes after losing one node or one availability zone | Lag metrics during fault injection |
| Elasticity | Consumers scale out within 1 minute of lag crossing threshold and back in without losing in-flight work | Autoscaler events plus invariant checker |
| Cost | Published cost per 1,000 drivers per month at target load | Cloud billing for the benchmark window, normalized |

## Domain and core concepts

The domain is deliberately small (six entities) so that all the difficulty sits in the distributed-systems behaviour, not in business rules.

| Entity | What it is | Owned by | Key properties |
| --- | --- | --- | --- |
| Driver | A courier with a device | Fleet (seeded) | id, home zone, vehicle capacity, shift window |
| Position | One location report from a driver's device | Ingest | driver id, device sequence number, device timestamp, coordinates, speed, heading |
| Driver state | The derived, current view of a driver | Location projection | latest position, status (offline, available, offered, busy), current zone |
| Zone | A named polygon a dispatcher cares about | Zone service | id, shape, dwell threshold |
| Order | A pickup and a dropoff to be served | Order service | id, pickup point, dropoff point, created time, status |
| Assignment | The binding of one order to one driver | Dispatch | order id, driver id, offer expiry, outcome |

### Invariants (the checker asserts these after every test)

1. **One driver, one active assignment.** A driver is never offered or holding two orders at once.
2. **One order, one driver.** An order is never assigned to two drivers.
3. **Every order resolves.** Every created order ends assigned, expired or cancelled; none is lost.
4. **Positions never go backwards.** For each driver, the projected latest position never moves to an older device sequence number.
5. **Zone events pair up.** Every enter has at most one matching exit; no exit without an enter.
6. **Replay equals live.** Rebuilding any projection from the event log yields the same state as the live projection, compared per driver. Time-driven transitions (`driver.heartbeat_lost`, `offer.expired`) are events in the log and are consumed on replay, never re-derived from the wall clock.

The checker runs only after quiescing: load stopped, every consumer drained to the log's high-water marks, then asserted. It dedupes by event id itself. Invariant 1 is asserted over `offer.sent` as well as assignments.

### Order and driver lifecycle (dispatch view)

- Order: created → awaiting driver → offered → assigned → completed; or offered → declined/expired → awaiting driver again; or cancelled from any state before assigned.
- Driver: offline → available → offered → busy → available; a missing heartbeat beyond a timeout moves the driver to offline and releases any open offer.
- Arbiter: the dispatch owner is the sole arbiter of an offer. `offer.accepted` references an offer id; an accept arriving after the offer was released or expired is answered with `offer.expired`, and the device must handle "accepted but gone". Accept and heartbeat-loss travel on different partition keys and are never assumed ordered.

## Master architecture

Every write enters through a gateway, lands in a partitioned, replayable event log, and is turned into state by independent processors that each own one job; every read is served from a store shaped for that read. Nothing reads another component's database.

[embedded content: master architecture · technology-neutral]

The dispatch processor also reads the hot state store to find nearby available drivers, and the live gateway answers latest-position queries from it; those read paths are left out of the drawing for clarity.

| Component | Responsibility | State | Scales by |
| --- | --- | --- | --- |
| Device simulator | Generates realistic driver movement on a real road network at a configurable rate, for any city given an OpenStreetMap extract; adversarial by default: reordering, duplicates, reboots that reset sequence numbers, reconnect storms, clock skew; also simulates accept/decline, late accepts and device disconnects | None | Number of simulator instances |
| Ingest gateway | Authenticates devices, validates and normalizes reports, drops duplicates by device sequence, writes to the log keyed by driver | None | Connections and request rate |
| Order service | Accepts orders, writes the order and an outbox record in one transaction; a relay publishes outbox records to the log | Order store | Request rate (low) |
| Event log | Durable, ordered per partition, replayable, retained for days; the system's source of truth for movement and decisions | Partitions | Partition count |
| Location projector | Keeps each driver's latest position (newest device sequence wins), updates the geo index and status, emits driver-state changes | Hot state store | Partitions consumed |
| Zone processor | Tests each position against zones; emits enter, exit and dwell events; keeps per-driver zone membership rebuildable from the log | Local, rebuildable | Partitions consumed |
| Dispatch processor | For each zone, a single writer matches waiting orders to the nearest available driver, sends offers, handles accept, decline and expiry | Order store plus in-memory zone queues | Zone partitions |
| History sink | Batches positions and events into the history store with idempotent writes and retention tiers | History store | Batch size and partitions |
| Live gateway | Holds dashboard connections; each subscribes to map areas and receives only those updates; serves latest-position queries | Connection state | Connections |
| Hot state store | Latest position and status per driver; geo index for nearest queries | In memory, persisted for restart | Memory and shards |
| History store | Columnar, compressed, cheap; trails, events, analytics queries | Disk | Storage |

## Core distributed-systems decisions

Fourteen decisions carry the design; each becomes a short decision record in the repository, written in the release that introduces it, and each is a ready-made answer to a system-design interview question.

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
| 13 | Driver-side exclusivity and owner fencing | A driver is pinned to the zone owner of its *projected* current zone; before offering, the owner takes a per-driver reservation (`SET NX` with TTL in the hot store — a lock, named honestly); every dispatch event carries the owner's epoch (consumer-group generation) and consumers reject lower epochs | A border driver read as available by two zone owners within projector lag would otherwise get two offers (invariant 1); a partitioned-but-alive old owner would otherwise keep offering during rebalance | One extra round-trip per offer; reservation TTL must exceed offer expiry; the lock's failure modes are tested in R9 | R4a |

One more decision is about restraint: services are split only where scaling or failure behaviour differs (ingest, fan-out, each processor). The order service and the dispatch processor share a domain but not a scaling profile, so they are separate; the zone definitions and the order API share both, so they live in one service. Recording why is part of the deliverable.

## Event catalog

Thirteen event types, two partition keys (driver and zone); every event carries an event id (for dedupe), a schema version, a trace context and the time it happened at the source.

| Event | Partition key | Producer | Consumers | Retention | Release |
| --- | --- | --- | --- | --- | --- |
| position.reported | driver id | Ingest gateway | Location projector, zone processor, history sink | 3 days | R0 |
| driver.state_changed (available, offered, busy, offline) | driver id | Location projector, dispatch processor | Live gateway, dispatch processor, history sink | 7 days | R2 |
| driver.heartbeat_lost | driver id | Location projector | Dispatch processor, live gateway | 7 days | R4 |
| zone.entered / zone.exited | zone id | Zone processor | Live gateway, dispatch processor, history sink | 7 days | R3 |
| zone.dwell_exceeded | zone id | Zone processor | Live gateway (alert), history sink | 7 days | R3 |
| order.created / order.cancelled | zone id (of pickup) | Order service via outbox | Dispatch processor, live gateway, history sink | 30 days | R4 |
| offer.sent | zone id | Dispatch processor | Simulator (device), live gateway, history sink | 30 days | R4 |
| offer.accepted / offer.declined | zone id | Ingest gateway (from device) | Dispatch processor | 30 days | R4 |
| offer.expired | zone id | Dispatch processor | History sink, live gateway | 30 days | R4 |
| assignment.confirmed | zone id | Dispatch processor | Order service (updates the record), live gateway, history sink | 30 days | R4 |

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

- Device and dashboard authentication with short-lived tokens; every key, topic and query is prefixed by fleet id (one line of code now, so a second fleet never needs a migration).
- Secrets from the platform's secret store, never in the repository.
- Rate limits per device and per client at the gateways.

Network policies and full multi-tenant isolation are out of scope: there is one fleet, and claiming otherwise would read as decoration.

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

- **In:** cell-based area subscriptions; per-driver coalescing; driver-state changes on screen; latency measured from device timestamp to screen; several gateway replicas behind a balancer.
- **Out:** alerts, assignment overlays.
- **Demo:** ten dashboards watching different areas, each receiving only its own drivers.
- **Exit:** position-to-screen p99 recorded at 1,000 drivers; fan-out messages scale with watched cells, not fleet size (measured); killing one gateway replica moves its viewers to another within seconds; decision 9 recorded.

### R3 Zones and geofence events (M)

- **In:** zone definitions; enter, exit and dwell detection; zone events on the map; schema versioning and the compatibility check; a rebuild command that replays the log into fresh state.
- **Out:** zone editing UI (zones are seeded).
- **Demo:** drivers crossing zones light them up; run the rebuild and compare.
- **Exit:** invariants 5 (zone events pair up) and 6 (replay equals live) pass; decision 10 recorded.

### R3b Public always-on demo (M)

- **In:** a cheap always-on cluster (smallest GKE regional footprint on spot nodes, or a single-node k3s host if cheaper — chosen by measured monthly cost); the same Kustomize base with a `demo` overlay; Argo CD syncing it as a second target; TLS via cert-manager; short-lived signed tokens for devices and dashboards, issued by a small token service; the public dashboard is read-only (no order or zone writes exposed); rate limits per device and per client; secrets from the platform's secret store; the cost agent and a budget alert; a 100-driver fleet in a rotating city.
- **Out:** user accounts, public write APIs, SSO, multi-tenant isolation.
- **Demo:** open the public URL from a phone; the map is live; the cost panel shows today's spend.
- **Exit:** public URL up; an unauthenticated write attempt is rejected and logged; token expiry is enforced (tested); monthly cost projected from 24 h of metering and published; decision 14 recorded; every later slice's demo script also runs against the `demo` overlay.

### R4 Orders and dispatch (L + L, split)

- **In:** order service with transactional outbox; zone-partitioned dispatch with a single writer per zone; driver pinning to the projected zone, per-driver reservation and owner-epoch fencing (decision 13); nearest-available matching from the geo index; offers with expiry; simulator accepts or declines; heartbeat loss releases offers; assignments on the map.
- **Out:** route optimization, batching several orders per driver.
- **Demo:** orders appear and get assigned live; a concurrency test fires bursts of orders into one zone.
- **Exit:** invariants 1, 2 and 3 pass under the burst test, under a consumer restart mid-burst, and under a border-driver test (drivers oscillating across a zone boundary while both zones receive orders); assignment p99 recorded; decisions 2, 6, 7 and 13 recorded.

*Split after review:* **R4a (L)**: orders through the outbox, single-writer matching with reservation, pinning and fencing, automatic acceptance, invariants 1–3 under a burst, a consumer restart and the border-driver test. **R4b (L)**: offers with expiry, accept and decline, late-accept rejection, heartbeat loss releasing offers.

### v0.4 cut line (after R4a)

- **Deliverable:** README with architecture, numbers so far and decision records 1–7, 10, 13; a 60-second clip of orders being assigned on the live map with zones lit.
- **Rule:** the first artifact that can go into an application or an interview. If R4b is not done when an interview needs it, this is what is shown.

### v0.5 cut line (after R4b)

- **Deliverable:** README with architecture, numbers so far and decision records; a 3-minute video; resume and profile updated.
- **Rule:** if interviews or an offer take priority, the project can stop here and still stand as a portfolio piece.

### R5 Trip history (S)

- **In:** history sink with batched, idempotent writes; retention tiers; replay of any driver's day on the map; a few analytics queries (busiest zones, average time to assignment).
- **Demo:** pick a driver, replay their day.
- **Exit:** hot-path latency unchanged with the sink running at full rate (measured); duplicate deliveries do not duplicate history rows.

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
- **No UI polish slice.** The map exists to show the system working, not to be a product.
- **No parallel tracks.** One person, one slice at a time; the gate between slices is the passing exit criteria.

## Evidence plan

Three kinds of evidence, each rerunnable from the repository: load results, failure results, and an invariant checker that judges both.

### Invariant checker

A standalone job that reads the event log and the stores after any test and asserts the six invariants from the domain section. It runs in every release's exit criteria and after every load and failure scenario. A test passes only if the checker passes.

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
