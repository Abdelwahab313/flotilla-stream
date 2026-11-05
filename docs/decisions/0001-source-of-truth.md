# 0001 — Source of truth

- Status: accepted (R0, 2026-11-05)
- Introduced in: R0

## Context
R0 moves one driver's position from device to map. Several components need it. Today
that is the location projector (latest position and geo index in Redis) and the live
gateway (push to the map). Later come the zone processor, the history sink and the
checker. Without one ordered record for all of them, each reader would be fed by the
ingest gateway directly. The gateway's reply to the device would then depend on every
reader being up. A position lost from Redis could never be rebuilt, a new reader could
not see the past, and invariant 6 (replay equals live) would have nothing to replay.

Scope: the log is the record for movement and decisions. Data the order service owns
(R4a) is recorded in its own store and reaches the log through the transactional outbox
(decision 7).

What R0 built (verified in code, `abeb343`):
- The ingest gateway writes only to the log: one record per report on
  `position.reported`, key `fleet:driver`, `acks=all`
  (`services/ingest-gateway/main.go:80-84`, `:148`). It has no Redis client. It answers
  202 only after the broker acks, and 503 with `Retry-After` when it cannot
  (`services/ingest-gateway/internal/ingest/ingest.go:72-80`).
- Redis is written only by the projector, from the log. Hash, geo index and the pub/sub
  update all come from one Lua script per record
  (`services/location-projector/internal/projector/projector.go:18-27`).
- The live gateway reads the projector's pub/sub output, never the log or the ingest
  gateway.
- The projector commits offsets only after a whole polled batch is applied
  (`consumer.go:69-81`). A crash resumes from the last committed offset.
- Topic: 6 partitions, 1 replica, `retention.ms` 259200000 (3 days), auto-create off
  (`deploy/base/kafka/topic-position-reported.yaml`, `deploy/base/kafka/kafka.yaml`).

## Options considered
- **Dual write: the ingest gateway writes Redis and Kafka itself.** This is the obvious
  shortcut for R0, because the map would update without a consumer hop. It loses
  because the two writes are not atomic. If Redis succeeds and Kafka fails, the gateway
  has already shown the position, and the 202 or 503 no longer says whether it is
  recorded. The map then shows a position no other reader will ever see. The order
  service faces the same problem in R4a, which is why decision 7 picks an outbox there.
- **The hot store is the record (Redis first, changes published from it).** Loses here
  because Redis is on `emptyDir` in R0 and in memory by design. Making it the record means
  making it durable and ordered, which it is not built for. Every new reader (zones R3,
  history R5) would also need change capture from it.
- **The gateway fans out directly over pub/sub, no log.** Loses because R0's live
  fan-out already uses Redis pub/sub, and it drops messages during a re-subscribe (R0
  Deferred). Using the same mechanism as the record would make that loss permanent.
  There would be no offset to resume from and nothing to replay.

## Decision
An append-only, partitioned event log; stores are projections of it

## Why
Replay rebuilds any view; new consumers join without touching producers; failures become "resume from offset"

## Consequences / trade-off accepted
- Eventual consistency between views. One trace, single-sourced (`756fb689…`): 16.89 ms
  from `ingest.produce` start to `projector.apply` start (the log hop, with `acks=all`),
  and 18.51 ms from the ingest request to `live-gateway.ws_write`.
- Log retention must cover the longest rebuild. In R0 it is 3 days, a placeholder. A
  latest-position view needs only the newest record per driver. With time retention, a
  driver silent for more than 3 days drops out of any rebuild. A compacted companion
  topic, or a snapshot, is the candidate when R3 builds rebuilds.
- Not true yet in R0: a Redis restart does not rebuild hot state from the log, because
  offsets are already committed. A driver reappears only on its next report, and a
  driver that stopped reporting does not reappear (R0 Deferred). R0 accepts this
  decision for the write path only: every projected position comes from the log. The
  rebuild half of Why is R3's.
- With one replica and `min.insync.replicas` 1 locally, the record is only as durable as
  one broker's disk. Replication is R6a.

## How it was tested
- Only the log writes to the projection. This is structural, verified by reading the
  code, not by a test: the ingest gateway has no Redis client, and the only Redis writer
  is `projector.Store.Apply`.
- Redelivery converges: `TestConsumerFailureLeavesBatchUncommittedAndRerunConverges`
  (`services/location-projector/internal/projector/consumer_test.go:78`, `kfake` +
  miniredis). The store fails seq 3 of 5, and the consumer stops with nothing past seq 2
  committed. A second consumer in the same group re-reads the records and reaches seq 5.
  The updates published are exactly `[1 2 3 4 5]`, each once.
  - The test does not show a resume from a committed offset. All five records arrive in
    one poll, so nothing is committed. The rerun starts at the earliest offset, and the
    seq rule drops seq 1 and 2.
- No 202 without the log: `TestHandlerProduceFailure` and
  `TestHandlerAnswersWithinProduceTimeout` (`ingest_test.go:133`, `:169`). A failed or
  blocked produce answers 503 with `Retry-After` within the timeout, never 202.
  - The converse does not hold: after a 503 on timeout the record may still land
    (`services/ingest-gateway/main.go:152-154`). The projector's seq rule absorbs it.
- End to end on kind, Argo-synced at `abeb343` (2026-11-04): trace
  `756fb689faec7b3a65286061712e2c5e`, seven spans in one parent chain. `ingest.produce`
  → `projector.apply` is the log hop, and it ends at `live-gateway.ws_write`. Commands:
  `make gitops-local`, then the Tempo query in `demo/R0.sh`. Raw:
  `docs/results/raw/argo-trace-756fb689faec7b3a65286061712e2c5e.{json,tsv}`. Evidence
  in `docs/releases/R0.md`.
- Command: `go test -race -count=1 -tags integration ./...` (`.github/workflows/ci.yml:42`;
  CI run `41905692944` on `abeb343`, green).
- Not tested in R0: replay from offset zero into an empty store with a new group,
  resume from a committed offset, and a consumer joining late. Invariant 6 is asserted by
  the checker from R3. Until then the "replay rebuilds any view" part of Why is assumed,
  not shown.
