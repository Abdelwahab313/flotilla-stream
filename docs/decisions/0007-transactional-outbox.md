# 0007 — Writes that cross a database and the log

- Status: proposed
- Introduced in: R4a

## Context
<!-- Why this decision is needed; what breaks without it. Written in R4a. -->

## Options considered
<!-- At least two real alternatives and why they lose. -->

## Decision
Transactional outbox: business row and outbox row in one transaction; a relay publishes

## Why
No dual-write inconsistency between the order store and the log

## Consequences / trade-off accepted
Publishing is slightly delayed; the relay must be idempotent

## How it was tested
<!-- The test or scenario in docs/results/ that exercised this decision. -->
