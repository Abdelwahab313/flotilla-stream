# 0014 — Cost as a measured, pluggable signal

- Status: proposed
- Introduced in: R3b, R8

## Context
<!-- Why this decision is needed; what breaks without it. Written in R3b. -->

## Options considered
<!-- At least two real alternatives and why they lose. -->

## Decision
Per-component cost is a first-class metric: an in-cluster cost agent (OpenCost-style) allocates node and store cost to namespaces and workloads from a provider billing adapter; the adapter is pluggable per cloud (GKE first), and cost per 1,000 drivers is a dashboard panel, not a spreadsheet.

## Why
Cost claims in the write-up must be rerunnable like latency claims; the demo environment needs a budget alert.

## Consequences / trade-off accepted
Allocation is approximate (shared nodes, network egress estimated); accuracy is stated with the number.

## How it was tested
<!-- The test or scenario in docs/results/ that exercised this decision. -->
