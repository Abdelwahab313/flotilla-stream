# 0003 — Delivery guarantee

- Status: proposed
- Introduced in: R0

## Context
In R0 a position crosses four hops: device → ingest gateway, ingest gateway → log,
log → projector → Redis, and Redis pub/sub → live gateway → socket. Each hop can fail
after it has done its work but before the sender knows. Three concrete cases:
- The ack misses ingest's 3 s deadline. Ingest answers 503, but the record may still
  land (`services/ingest-gateway/main.go:150-159`).
- The projector applies a batch to Redis and is killed before `CommitRecords`. The
  restart redelivers the whole batch (`consumer.go:69-81`).
- Redis pub/sub drops updates while the live gateway re-subscribes (R0 Deferred).

Without a stated guarantee, each of these is either silent loss or a duplicate that
moves a driver backwards on the map (invariant 4). In R0 the only duplicates are consumer
redeliveries: the simulator never retries, and the producer is idempotent. The plan also
gives the ingest gateway sequence-based dedupe. R0 does not build it; "dedupe beyond
newest-wins" is in R0's Out list.

## Options considered
- **Exactly-once via Kafka transactions (read-process-write).** Loses here because the
  projector's output is Redis, not Kafka. A Kafka transaction covers offsets and records
  written to Kafka, not a Lua script in Redis, so it would not prevent a double apply.
  The sink would still need its own rule. The projector already reads `read_committed`
  (`location-projector/main.go:92`), so transactional producers from R4a are safe to
  read.
- **Store the offset in Redis inside the same script (effectively-once per partition).**
  Loses because the newest-seq rule already makes a redelivered record a no-op, so the
  offset adds nothing for dedupe. It returns where a sink cannot compare versions: the
  ops view stores offsets in the same Postgres transaction as its rows (PLAN).
- **At-most-once (auto-commit before apply; fire-and-forget produce).** Cheapest. For
  positions alone, the next report heals a loss. Loses because the same rule must hold
  for orders and offers from R4a, where a lost event breaks invariant 3. One guarantee
  everywhere keeps the checker's rules uniform.

## Decision
At-least-once delivery with idempotent consumers, not exactly-once

## Why
Simpler, cheaper, honest about failures; duplicates are made harmless

## Consequences / trade-off accepted
Every consumer needs an idempotency rule (sequence numbers, upserts, dedupe keys). In R0
(verified in code, `abeb343`):

| Hop | Guarantee in R0 | Rule |
| --- | --- | --- |
| Device → ingest | At-most-once. The simulator drops a failed report and does not retry (`simulator/main.go:86-89`). | None needed yet. Event id `fleet:driver:seq` is a pure function of the report, so a future retry carries the same id. |
| Ingest → log | At-least-once from the 202, durable on one broker. `acks=all`, but with 1 replica and `min.insync.replicas` 1 that equals `acks=1`, on a PVC with `deleteClaim: true` (`deploy/base/kafka/kafka.yaml:15-19`, `nodepool.yaml:12-18`). The franz-go v1.22.2 idempotent producer (on by default) dedupes its own retries within one producer session; an ingest restart starts a new session. A 503 after a timeout may still be written. | The projector absorbs any duplicate by seq. It does not read `event_id`. |
| Log → projector → Redis | At-least-once. Offsets are committed after the batch is applied. | Newest device seq per driver wins, compared and written in one Lua script. Equal or older seq: no write, no publish. The result is the same in any order, including two members briefly racing during a rebalance. |
| Redis → live gateway → socket | At-most-once (pub/sub). A slow socket is disconnected, not buffered. | None in the gateway (`hub.go` and `subscriber.go` never compare seq). The socket sees newest-wins only because the script publishes strictly rising seqs per driver. A missed update heals on the driver's next report. A parked driver's last missed update stays missed until R2's snapshot on subscribe. |

- In R0, at-least-once holds from the ingest 202 to Redis, not end to end. The device hop
  and the live push are lossy by design until R1 (device retries, sessions) and R2
  (snapshot on subscribe).
- A Redis wipe (pod rescheduled, `emptyDir`) is not rebuilt in R0. Offsets are committed
  past every position, so a driver reappears on its next report and a parked driver
  does not reappear. No row of the table covers this; it is a decision 1 gap, and the
  rebuild is R3's.
- A record that fails the same way every time (for example WRONGTYPE in Redis) blocks
  every partition and crash-loops the projector. At-least-once allows no skip. A
  dead-letter topic is deferred.
- Invalid records are skipped and logged, not counted for the checker yet (deferred).

## How it was tested
- Real redelivery converges: `TestConsumerFailureLeavesBatchUncommittedAndRerunConverges`
  (`services/location-projector/internal/projector/consumer_test.go:78`, `kfake` +
  miniredis, one partition, one group member). Seq 1 and 2 are applied but never
  committed, and seq 3 fails. The rerun re-reads 1–5. Stored seq is 5. The published
  updates are exactly `[1 2 3 4 5]`: the redelivered 1 and 2 published nothing.
- The idempotency rule: `testNewestWins` (`projector_test.go:39`). It covers a
  duplicate, older-after-newer, a redelivered batch, skip-ahead, and a late arrival in
  the gap. After each step it checks the stored hash, the geo point and the publish
  count. It runs on miniredis (`TestApplyMiniredis`) and on real Redis 8.10.3
  (`TestApplyRealRedis`, build tag `integration`).
- No partial write: `testRefusedPointWritesNothing` (`projector_test.go:117`). When
  GEOADD refuses a point, the hash keeps the old seq.
- Event id is a pure function of fleet, driver and seq: `TestEventIDIsStableAcrossRetries`
  (`ingest_test.go:114`). It posts twice to a fake producer; it is not a device retry.
- Command: `go test -race -count=1 -tags integration ./...` with a `redis:8.10.3-alpine`
  service (`.github/workflows/ci.yml:42`).
  - CI run `41905692944` on `abeb343`: green (`gh run view`).
  - CI run `41905189571` on `f6bb3e4`: 67 tests passed and the 1 integration test was
    asserted (from that run's test job; single-sourced).
- Not tested in R0:
  - a 503 whose record still lands (kfake could cover it);
  - redelivery across several partitions or members;
  - a broker-side duplicate;
  - a projector kill on kind.

  Failure evidence starts with the checker in R2/R4a and the fault scenarios in R9.
