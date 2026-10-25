# 0012 — Failure domains

- Status: proposed
- Introduced in: R6a, R6b

## Context
<!-- Why this decision is needed; what breaks without it. Written in R6a, R6b. -->

## Options considered
<!-- At least two real alternatives and why they lose. -->

## Decision
Every stateful component replicated across availability zones; disruption budgets keep a minimum running during maintenance

## Why
Survive losing a node or a zone, the scenarios R9 tests

## Consequences / trade-off accepted
Cross-zone traffic cost; published with the cost results

## How it was tested
<!-- The test or scenario in docs/results/ that exercised this decision. -->
