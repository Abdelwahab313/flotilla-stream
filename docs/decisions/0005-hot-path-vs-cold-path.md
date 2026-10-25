# 0005 — Hot path vs cold path

- Status: proposed
- Introduced in: R0, R5

## Context
<!-- Why this decision is needed; what breaks without it. Written in R0, R5. -->

## Options considered
<!-- At least two real alternatives and why they lose. -->

## Decision
Latest state in an in-memory store with a geo index; history in a columnar store via batched writes

## Why
The hot path needs milliseconds; history needs cheap storage and scans

## Consequences / trade-off accepted
Two stores to operate; history lags the hot path by seconds

## How it was tested
<!-- The test or scenario in docs/results/ that exercised this decision. -->
