# 0006 — Dispatch concurrency

- Status: proposed
- Introduced in: R4a

## Context
<!-- Why this decision is needed; what breaks without it. Written in R4a. -->

## Options considered
<!-- At least two real alternatives and why they lose. -->

## Decision
Single writer per zone partition: one consumer owns all decisions for a zone at a time

## Why
One order is never matched twice (invariant 2) by construction; no coordination between zones for order state; easy to reason about and test

## Consequences / trade-off accepted
A zone's throughput is bounded by one consumer; ownership moves on rebalance, so state must be rebuildable; does not by itself protect the driver side (see 0013)

## How it was tested
<!-- The test or scenario in docs/results/ that exercised this decision. -->
