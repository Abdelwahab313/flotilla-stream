# 0003 — Delivery guarantee

- Status: proposed
- Introduced in: R0

## Context
<!-- Why this decision is needed; what breaks without it. Written in R0. -->

## Options considered
<!-- At least two real alternatives and why they lose. -->

## Decision
At-least-once delivery with idempotent consumers, not exactly-once

## Why
Simpler, cheaper, honest about failures; duplicates are made harmless

## Consequences / trade-off accepted
Every consumer needs an idempotency rule (sequence numbers, upserts, dedupe keys)

## How it was tested
<!-- The test or scenario in docs/results/ that exercised this decision. -->
