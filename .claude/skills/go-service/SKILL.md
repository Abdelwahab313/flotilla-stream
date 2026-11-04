---
name: go-service
description: How to write Go services, the simulator and the checker in flotilla-stream — layout, context and shutdown, at-least-once consumers with written idempotency rules, offsets committed after effects, injected clocks for replay, bounded queues and backpressure, WebSocket fan-out, Redis and outbox patterns, OpenTelemetry, testing with -race and fuzzing, profiling. Use when writing or reviewing Go under services/, simulator/, checker/ or cmd/.
---

# Go services

Rules of the system: `CLAUDE.md` (rules 4–6), `docs/product/PLAN.md` § Invariants and
§ Core distributed-systems decisions. The `invariant-auditor` agent checks the
distributed side; this skill is how the code is written.

## Layout

```
services/<name>/        one deployable per scaling profile (PLAN: split only where scaling differs)
  main.go               wiring only: config, deps, run, shutdown
  internal/...          the service's packages; nothing imported across services
internal/               shared, small, stable: event envelope, ids, clock, telemetry setup
cmd/<tool>/             dev and build tools (console build, load profiles)
```

Packages are named for what they hold (`projector`, `zones`), never `utils`/`common`.
Accept interfaces at the consumer, return concrete types.

## Process shape

- `main` builds a root `context` from `signal.NotifyContext(SIGINT, SIGTERM)`; every
  goroutine takes it and returns when it is done. Use `errgroup` to run and to stop.
- Shutdown order: stop accepting (readiness false) → drain in-flight work → commit
  offsets for completed work → close clients. A consumer that is scaled in must leave no
  uncommitted completed work and no committed incomplete work (decision 11).
- Config from environment, parsed once into a struct, validated at start; fail fast.
- `/healthz` (process alive) and `/readyz` (dependencies reachable, partitions
  assigned) are different endpoints.

## Consumers: at-least-once, idempotent

Every consumer states its idempotency rule in a comment at the apply function — the
one place a comment is required:
- position projection: newest `(device session, seq)` wins, compare-and-set in Redis
  (Lua or `WATCH`), never read-then-write;
- order/offer state: transition only from the expected prior state, keyed by event id;
- history sink: upsert keyed by event id.

Then:
- Commit an offset only after its side effects are durable. Batch commits; never
  auto-commit on poll.
- Redelivery after a partial side effect must converge to the same state; test it by
  applying the same batch twice and crashing between effect and commit.
- No ordering assumption across partition keys (driver vs zone). An accept and a
  heartbeat-loss are never assumed ordered.
- Owners write through a transactional producer per owned partition; readers use
  `read_committed`. No reader-side epoch filtering (decision 13).
- Command owners (PLAN Event catalog § Commands): on partition assignment, read to the
  high-water mark, including your own earlier outputs, before answering; dedupe by
  command id; check expected version, fleet and zone; answer each command id exactly
  once with the outcome or `command.rejected`; only the owner emits the fact.
- Producers key with the tenancy-scoped key (`fleet:zone`, `fleet:driver`) and the
  shared partitioner; an entity's events go to its one topic (PLAN § Topics).

## Kafka

Apache Kafka 4.x in KRaft mode (R0: one combined node; from R6a: separate controller
and broker pools over three zones with rack awareness, through Strimzi).

- **Client.** The Go client chosen in R0 must support the idempotent producer,
  transactions, `read_committed`, incremental rebalancing and assign/revoke callbacks.
  Transactions rule some pure-Go clients out; check before picking.
- **Producers.** `acks=all` with idempotence on. Key with the tenancy-scoped key and the
  shared partitioner. Batch with `linger` tuned by measurement, not guessed.
- **Topics.** Created declaratively (Strimzi `KafkaTopic`), never auto-created. One
  topic per key and retention class (PLAN § Topics). Partition count is chosen up front
  from the capacity estimate (decision 11), because adding partitions remaps keys and
  breaks per-key order and the versions the console relies on. In the cloud:
  replication factor 3, `min.insync.replicas=2`, unclean leader election off.
- **Consumers.** Manual commit after side effects. On revoke: finish or abandon
  in-flight work, commit what completed, drop the partition's in-memory state. On
  assign: rebuild state from the log or checkpoint, catch up to the high-water mark,
  then act. The new rebalance protocol (KIP-848, `group.protocol=consumer`) is GA in
  Kafka 4.0 but off by default; adopt it only with a rebalance-time measurement.
- **Owners that emit decisions** (dispatch, alerts, duty) use a transactional producer
  per owned partition, so offsets and outputs commit together and a stale owner is
  fenced by the broker; downstream readers use `read_committed`. Final choice and its
  latency cost: decision 13, R4a.
- **Lag** per consumer group and partition is the scaling signal (KEDA, decision 11) and
  a published metric.

## Time and replay

No `time.Now()` on a decision path. Time-driven facts (`driver.connectivity_changed`,
`offer.expired`, `order.late`, alert windows) are events written to the log and consumed
on replay (invariant 6). Timers run on event time (the newest ingest timestamp seen on
the partition) and arm only after catching up to the high-water mark, so a restart does
not mark everything stale. Events derived from fleet config carry the config version
used. Wall time is fine for metrics and logs.

## Backpressure

Bounded channels at every hop; when full, block the reader (consumers) or shed with an
explicit retry-after (gateways). Never spawn a goroutine per message. Size worker pools
from configuration and measure them.

## Live gateway (WebSocket)

One reader and one writer goroutine per connection; the writer drains a small bounded
queue that coalesces to the latest update per driver; a client that cannot keep up gets
coalesced, then disconnected, never an unbounded buffer. Ping/pong with read deadlines.
Authenticate on upgrade; filter by fleet and allowed cities before enqueue (invariant 7).
Messages carry a per-stream sequence so the client can detect gaps and resync.

## Writes across a database and the log

Business row and outbox row in one transaction; the relay publishes and marks sent; the
relay is idempotent because consumers dedupe by event id (decision 7).

## Errors and logs

Wrap with `%w` and context (`fmt.Errorf("project %s: %w", driverID, err)`); compare with
`errors.Is/As`. `log/slog` with JSON output; log fields, not formatted strings; no logs
per position on the hot path (sample or count instead). Never log tokens or phone
numbers.

## Telemetry

OpenTelemetry trace context travels inside the event envelope from simulator through the
log to the WebSocket push. Metrics for every bounded queue (depth, drops), consumer lag,
and end-to-end latency as a histogram. Metric names are stable once published in
`docs/results/`.

## Performance

Measure before changing: `go test -bench . -benchmem`, `pprof` (CPU, heap, goroutines)
on a load-profile run. On the hot path: reuse buffers, avoid `fmt` and reflection-based
JSON per message (the event codec is chosen per decision 10), preallocate slices, batch
Redis calls with pipelines. `sync.Pool` only with a benchmark showing it helps.

## Tests

- Table-driven unit tests; `go test -race ./...` always.
- Fuzz every decoder of external input (device reports, socket messages).
- Integration tests against the `docker compose` stack (Kafka, Redis) behind a build
  tag; adversarial inputs from the simulator: duplicates, reordering, reboots.
- A test passes only if the checker passes after quiescing (rule 4).
- `go vet` and `staticcheck` clean.
