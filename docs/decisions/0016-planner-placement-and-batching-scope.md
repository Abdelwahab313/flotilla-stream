# 0016 — Planner placement and batching scope

- Status: proposed
- Introduced in: R4a

## Context
Drivers carry several orders at once (courier and prepared-food delivery): one offer may group several orders and place their pickup and dropoff stops into the driver's route. Orders are partitioned by pickup zone and each zone has a single writer (0006). An offer grouping orders from two zones would need two owners to commit one decision atomically.

## Options considered
- **Group across zones with a two-owner protocol.** Best batches; reintroduces the distributed coordination the single-writer design exists to avoid, and new failure modes for invariants 1 and 2.
- **City-level dispatch partitions.** One owner sees the whole city, so any orders can be grouped; far fewer writers, hot partitions at peak, longer rebuild on rebalance.
- **Group within one zone's queue; planner runs inside the zone owner.** Cross-zone work happens as successive offers inserted into the driver's existing route.

## Decision
The planner is a pure, pluggable function inside each zone owner: `Plan(zone queue, candidate drivers with their current routes) → offers` (or proposals in review mode). One offer contains orders from one pickup zone only. A driver has at most one open offer at a time; once it is accepted, another zone's owner may offer more orders, inserted into the same route. Dropoffs may be anywhere. v1 planner: nearest driver, ties broken by fewest active assignments, greedy stop insertion, straight-line ETA at average speed.

## Why
Every offer stays one decision by one writer, so "one order, one driver" and "one open offer per driver" hold by construction plus the existing reservation and fencing (0013). The planner can be replaced by a real optimizer without touching the concurrency model.

## Consequences / trade-off accepted
- Batches are less efficient near zone borders; zone size becomes a planner-quality knob (bigger zones: better batches, less parallelism).
- Cross-zone batching in one offer needs a new decision (coarser partitions or a two-owner protocol).
- The planner must be deterministic given its recorded inputs, so a test can re-run it from what `offer.sent` recorded; a rebuilt owner folds its past outputs instead of re-running it (invariant 6).
- A driver's route can hold stops from several zones' owners. Each owner emits `route.updated` for its own segment, keyed by driver in `driver.facts`; the location projector merges the segments into the driver's route in the hot store, and the planner reads it only while holding the driver's reservation (0013), so two owners never re-plan one driver at once. Activity (idle, offered, on route) is folded by the location projector from every owner's `driver.commitment_changed`, so no single zone decides a driver is idle while another zone still has stops.
- Open, decided in R4a with 0013: replay rebuilds an owner by folding its own outputs, not by re-running the planner; `offer.sent` records the inputs it was planned from.

## How it was tested
<!-- Burst into adjacent zones with border drivers: no driver ever has two open offers, no order in two offers; replay of the dispatch log reproduces the same offers. -->
