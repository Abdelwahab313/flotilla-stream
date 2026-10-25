# 0002 — Partition keys

- Status: proposed
- Introduced in: R0, R4a

## Context
<!-- Why this decision is needed; what breaks without it. Written in R0, R4a. -->

## Options considered
<!-- At least two real alternatives and why they lose. -->

## Decision
Positions by driver; orders, offers, assignments and zone events by zone

## Why
Per-driver ordering for movement; one place per zone where dispatch decisions are made

## Consequences / trade-off accepted
Hot zones become hot partitions; mitigated by splitting large zones into cells

## How it was tested
<!-- The test or scenario in docs/results/ that exercised this decision. -->
