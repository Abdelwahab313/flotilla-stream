# 0005 — Hot path vs cold path

- Status: proposed
- Introduced in: R0, R5

## Context
Two kinds of reads need positions.
- The map needs the latest position per driver on every report. From R4a,
  nearest-available dispatch also needs a radius query, once per dispatch decision. Both
  must answer within milliseconds.
- Trip history (R5) needs every position for a day, stored cheaply and scanned in bulk.

One store cannot serve both well at the plan's target of 10,000 events/s (PLAN Success
measures).

R0 builds only the hot half (verified in code, `abeb343`). For each record, the
projector runs one Lua script on Redis 8.10.3
(`services/location-projector/internal/projector/projector.go:18-27`). The script does
`HGET` seq, compares, then `GEOADD fleet:{f}:drivers:geo`,
`HSET fleet:{f}:driver:<id>` and `PUBLISH`. The `{fleet}` hash tag keeps a fleet's keys
in one slot for a later Redis Cluster. On kind, Redis runs on `emptyDir` with
`appendonly no` and default RDB snapshots (`redis-cli CONFIG GET`), so a pod restart
loses the hot state. Nothing in R0 writes history.

## Options considered
- **One store for both: Postgres with PostGIS.** The latest row would be upserted with
  `ON CONFLICT … WHERE seq <` and `NOTIFY`, and every point appended. Postgres can do the
  atomic compare-write-publish, so correctness is not the reason it loses. It loses on
  write volume at 10,000 events/s:
  - Every report changes an indexed geometry column. The update can never be a HOT
    update, so each report leaves a new index tuple and a dead tuple, and vacuum must keep
    up.
  - The history appends, 864M rows a day (10,000 × 86,400), go into the same instance.
  - Not measured; this is the expected cost, not a result.
- **The log is the only history (no columnar store; replay Kafka for a driver's day).**
  This is R0's de facto state, because history exists only as the 3-day topic. It loses
  for R5. `position.reported` is keyed by driver id over 6 partitions, so one driver's day
  means reading that partition's ~144M records to keep 17,280 (one report per 5 s), at
  target load. Retention also caps history at days. The log stays the rebuild source
  (ADR 0001), not the query store.
- **Hot state in the projector's process memory (no Redis).** Loses because the live
  gateway, and from R4a the dispatch processor, read latest state across all partitions.
  A per-projector map would need its own query API and a fan-in across members.

## Decision
Latest state in an in-memory store with a geo index; history in a columnar store via batched writes

## Why
The hot path needs milliseconds; history needs cheap storage and scans

## Consequences / trade-off accepted
- Two stores to operate; history lags the hot path by seconds. Only one exists in R0.
- The hot store is a projection, not a record (ADR 0001). R0 does not rebuild it after a
  Redis restart: a driver reappears on its next report, and a parked driver does not
  reappear (R0 Deferred). Rebuild is R3, and dispatch must not read a half-empty geo
  index before then (R4a).
- Redis memory grows with drivers, not with time: one hash and one geo member per driver.
  Sizing is R1.
- Accepted for the hot path at R0. The cold path stays proposed until R5, and R5 amends
  this record. If R5 chooses a different history store, a new record supersedes this
  one.
- R5's falsifiable test, fixed now: `projector.apply` p99 at R1 load with the history
  sink on and off. R5's exit criterion is "hot path unaffected under load".

## How it was tested
The tests show that the hot write is correct and fast at one driver. They do not show
that the split is right under load; the first load figure is R1's.
- Hot write correctness: `testNewestWins` and `testRefusedPointWritesNothing`
  (`projector_test.go:39`, `:117`). They run on miniredis and on real Redis
  (`TestApplyRealRedis`, `-tags integration`; CI run `41905692944` on `abeb343`, green).
  After every step the hash and the `GEOPOS` of the geo index agree.
- Hot write cost, two sources, over one window. Kind cluster at `abeb343`, 2026-11-04,
  21:21:00Z to 21:23:00Z. Load: one driver at one report per second; this is not a load
  figure. Commands: `redis-cli CONFIG RESETSTAT`, `sleep 120`, then
  `redis-cli INFO commandstats` (unfiltered). Tempo `GET /api/search` with
  `q={ name = "projector.apply" }`, `spss=1`, start and end set to the same window.
  - Store side: `evalsha` 121 calls, 232.5 µs mean per call on the server, 0 failed.
    Inside it, mean per call: `hget` 15.9, `geoadd` 32.1, `hset` 13.9, `publish` 15.3 µs.
    These sum to 77.3 µs; the remaining 155 µs is script execution overhead.
  - Application side: `projector.apply` span, n = 120: mean 1.18 ms, p50 0.96 ms, p90
    1.84 ms, p99 8.35 ms, max 9.39 ms. Percentiles are sample `⌊q·n⌋` of the sorted
    durations; the interpolated median is 0.950 ms (`docs/results/R0.md` §6).
  - Counts agree: 121 against 120, one record at the window edge.
  - Means compared: 0.23 ms in Redis against 1.18 ms in the client. The difference is
    the pod-to-pod hop through the `redis` Service, plus client work:
    - `json.Unmarshal` and `Validate` (`consumer.go:114-118`);
    - trace inject (`consumer.go:125-126`);
    - `json.Marshal` of the update (`projector.go:46`).
  - The p99 is 7× the p50 and is unexplained at n = 120 (p99 is the second-largest
    sample).
  - Raw: `docs/results/raw/redis-commandstats-window-abeb343.{txt,bounds}`,
    `docs/results/raw/tempo-projector-apply-window-abeb343.json`.
- End to end, one trace (`756fb689faec7b3a65286061712e2c5e`, single-sourced): report to
  socket write took 20.6 ms. Of that, 15.6 ms is `ingest.produce` with `acks=all`, not the
  hot store.
- Not tested in R0: radius queries (no reader uses `GEOSEARCH` before R4a), Redis restart
  recovery, and anything on the cold path (R5).
