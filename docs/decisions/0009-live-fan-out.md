# 0009 — Live fan-out

- Status: proposed
- Introduced in: R2

## Context
<!-- Why this decision is needed; what breaks without it. Written in R2. -->

## Options considered
<!-- At least two real alternatives and why they lose. -->

## Decision
Clients subscribe to map cells; the gateway forwards only updates for subscribed cells, coalescing to at most one update per driver per interval

## Why
Fan-out cost scales with what is watched, not with fleet size

## Consequences / trade-off accepted
Viewers may see a position up to one interval old

## How it was tested
<!-- The test or scenario in docs/results/ that exercised this decision. -->
