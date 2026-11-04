# 0015 — Tenancy model

- Status: proposed
- Introduced in: R3b (token-enforced); key prefix exists from R0.

## Context
The console becomes the product a delivery company runs its operations day from, so more than one company must be able to use one deployment. The plan previously ruled multi-tenancy out ("one fleet"), keeping only a fleet-id key prefix. Without an enforced boundary, a dispatcher panning the map over another company's city would receive its drivers, and an imported order could land in the wrong fleet.

## Options considered
- **Single fleet (status quo).** Simplest; but the console cannot be presented as a product, and the fleet prefix stays an untested claim.
- **Physical isolation: partitions, consumers and stores per fleet.** Strong noisy-neighbour protection; multiplies partition count and consumer groups by fleet count, and the dispatch single-writer design (0006) would need per-fleet ownership. Cost and operational weight out of proportion for a synthetic fleet.
- **Logical isolation on shared infrastructure.** `fleet_id` on every entity, event, key and query; tokens carry `fleet_id`; ingest and live gateways enforce it; fan-out cells keyed by (fleet, cell); rate limits per fleet.

## Decision
Tenant = **Fleet** (one delivery company). Logical isolation on shared partitions, consumers and stores. Every event carries `fleet_id`; every token is scoped to exactly one fleet; both gateways reject or drop anything outside the token's fleet. A seventh invariant is added: **no cross-fleet visibility**.

## Why
It turns the existing fleet prefix into a boundary that is enforced and asserted by the checker, which is what makes it evidence rather than decoration. Shared infrastructure keeps partition count and cost unchanged.

## Consequences / trade-off accepted
- A hot fleet can slow others on shared partitions; mitigated only by per-fleet rate limits. Per-tenant quotas, tenant self-signup and billing are out of scope.
- Partition keys stay driver id and zone id; zone ids and driver ids must be globally unique (or fleet-qualified) so a partition never mixes identities.
- One role inside a fleet in v1: dispatcher.

## How it was tested
<!-- Invariant 7 in the checker: across a two-fleet run, no event, live push or query answer crosses fleets. Scenario: fleet A's dispatcher subscribes to cells covering fleet B's city and must receive zero B updates. -->
