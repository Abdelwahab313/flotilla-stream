# 0013 — Driver-side exclusivity and owner fencing

- Status: proposed
- Introduced in: R4a

## Context
<!-- Why this decision is needed; what breaks without it. Written in R4a. -->

## Options considered
<!-- At least two real alternatives and why they lose. -->

## Decision
A driver is pinned to the zone owner of its projected current zone; before offering, the owner takes a per-driver reservation (SET NX with TTL in the hot store — a lock, named honestly); owners write through a transactional producer per owned partition, so outputs and offsets commit together and the broker rejects a stale owner's writes; readers use `read_committed`. The consumer-group generation alone does not fence an old owner that can still reach the brokers, and reader-side epoch filtering fails on topics with several producers (`driver.facts`). Command answers are fenced the same way (PLAN Event catalog § Commands).

## Why
A border driver read as available by two zone owners within projector lag would otherwise get two offers (invariant 1); a partitioned-but-alive old owner would otherwise keep offering during rebalance

## Consequences / trade-off accepted
One extra round-trip per offer; reservation TTL must exceed offer expiry; the lock's failure modes are tested in R9. Transaction commit latency is measured against the assignment p99.

Open, decided in R4a: the reservation lives only in the hot store, not in the log. If the hot store restarts while offers are open, the keys are gone and a second zone can offer to the same driver (invariant 1); and a rebuilt owner cannot re-run the planner deterministically, because the reservation outcome and the hot-store reads it used are not in the log. Candidates: make the reservation a driver-keyed command judged by the location projector (driver owner), so it is in the log; or keep `SET NX` and rebuild reservations from open offers before any owner acts. Either way, an owner rebuilds by folding its own past outputs, never by re-running the planner, and `offer.sent` records the inputs it was planned from.

## How it was tested
<!-- The test or scenario in docs/results/ that exercised this decision. -->
