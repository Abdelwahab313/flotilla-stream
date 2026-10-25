# 0010 — Schema evolution

- Status: proposed
- Introduced in: R3

## Context
<!-- Why this decision is needed; what breaks without it. Written in R3. -->

## Options considered
<!-- At least two real alternatives and why they lose. -->

## Decision
Versioned event schemas with backward-compatible changes only; a compatibility check in the delivery pipeline

## Why
Producers and consumers deploy independently

## Consequences / trade-off accepted
Breaking changes need a new event type and a migration window

## How it was tested
<!-- The test or scenario in docs/results/ that exercised this decision. -->
