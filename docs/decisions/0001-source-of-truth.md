# 0001 — Source of truth

- Status: proposed
- Introduced in: R0

## Context
<!-- Why this decision is needed; what breaks without it. Written in R0. -->

## Options considered
<!-- At least two real alternatives and why they lose. -->

## Decision
An append-only, partitioned event log; stores are projections of it

## Why
Replay rebuilds any view; new consumers join without touching producers; failures become "resume from offset"

## Consequences / trade-off accepted
Eventual consistency between views; log retention must cover the longest rebuild

## How it was tested
<!-- The test or scenario in docs/results/ that exercised this decision. -->
