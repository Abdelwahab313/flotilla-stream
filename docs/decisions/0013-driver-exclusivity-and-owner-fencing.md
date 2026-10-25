# 0013 — Driver-side exclusivity and owner fencing

- Status: proposed
- Introduced in: R4a

## Context
<!-- Why this decision is needed; what breaks without it. Written in R4a. -->

## Options considered
<!-- At least two real alternatives and why they lose. -->

## Decision
A driver is pinned to the zone owner of its projected current zone; before offering, the owner takes a per-driver reservation (SET NX with TTL in the hot store — a lock, named honestly); every dispatch event carries the owner's epoch (consumer-group generation) and consumers reject lower epochs

## Why
A border driver read as available by two zone owners within projector lag would otherwise get two offers (invariant 1); a partitioned-but-alive old owner would otherwise keep offering during rebalance

## Consequences / trade-off accepted
One extra round-trip per offer; reservation TTL must exceed offer expiry; the lock's failure modes are tested in R9

## How it was tested
<!-- The test or scenario in docs/results/ that exercised this decision. -->
