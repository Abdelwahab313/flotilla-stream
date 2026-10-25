# 0011 — Elasticity

- Status: proposed
- Introduced in: R6b

## Context
<!-- Why this decision is needed; what breaks without it. Written in R6b. -->

## Options considered
<!-- At least two real alternatives and why they lose. -->

## Decision
Consumers autoscale on log lag, not CPU; scale-in drains in-flight work and commits offsets before stopping

## Why
Lag is the real signal for a stream; a naive scale-in loses or duplicates work

## Consequences / trade-off accepted
Scaling is bounded by partition count; partition count is chosen up front for the target load

## How it was tested
<!-- The test or scenario in docs/results/ that exercised this decision. -->
