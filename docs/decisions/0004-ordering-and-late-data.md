# 0004 — Ordering and late data

- Status: proposed
- Introduced in: R1

## Context
<!-- Why this decision is needed; what breaks without it. Written in R1. -->

## Options considered
<!-- At least two real alternatives and why they lose. -->

## Decision
(device session id, device sequence number) per driver; the projector keeps only the newest within a session and accepts a new session unconditionally; older reports go to history but never overwrite current state

## Why
Mobile networks reorder and replay; clocks drift; a device reboot resets its sequence and must not freeze the driver forever

## Consequences / trade-off accepted
History may contain out-of-order points; queries sort by (session, sequence)

## How it was tested
<!-- The test or scenario in docs/results/ that exercised this decision. -->
