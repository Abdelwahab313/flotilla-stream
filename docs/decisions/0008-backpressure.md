# 0008 — Backpressure

- Status: proposed
- Introduced in: R1, R8

## Context
<!-- Why this decision is needed; what breaks without it. Written in R1, R8. -->

## Options considered
<!-- At least two real alternatives and why they lose. -->

## Decision
Bounded queues at every hop; the gateway sheds load with an explicit retry-after rather than accepting what it cannot write; consumers pause on lag

## Why
Fail visibly instead of falling over; the simulator honours retry-after like a real device

## Consequences / trade-off accepted
Some position reports are delayed or dropped under overload, by design and measured

## How it was tested
<!-- The test or scenario in docs/results/ that exercised this decision. -->
