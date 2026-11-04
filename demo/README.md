# Demos

One script per release, runnable from a clean checkout. Each prints what it is
about to show, runs it, and ends with the checker.

| Script | Shows | Status |
| --- | --- | --- |
| `r0.sh` | One command → dot moves on the map → trace link | todo |
| `r1.sh` | 1,000 drivers; ingest rate, lag, latency board | todo |
| `r2.sh` | Ten dashboards, each its own area; kill a gateway replica | todo |
| `r3.sh` | Zones light up; rebuild from log equals live | todo |
| `r3b.sh` | Public URL live; rejected unauthenticated write; token expiry; cost panel | todo |
| `r4a.sh` | Orders assigned live; burst, restart, border test; checker | todo |
| `r4b.sh` | Offers declined and expired; late accept rejected | todo |
| `r5.sh` | Replay a driver's day | todo |
| `r6a.sh` | Create GKE env, run r4b there, destroy | todo |
| `r6b.sh` | Load spike → scale out and back; checker green | todo |
| `r7.sh` | Inject slowdown; alert fires; runbook resolves | todo |
| `r8.sh` | Step load 1k → 50k; publish curves | todo |
| `r9.sh` | Kill node and zone under load; recovery table | todo |

Video links are added at v0.5 and v1.0.

## R0 without Kubernetes (sitting 1)

```sh
docker compose up -d --wait
go build -o bin/ ./simulator ./services/ingest-gateway ./services/location-projector ./services/live-gateway
bin/ingest-gateway & bin/location-projector & bin/live-gateway &
bin/simulator
```

| What | Where |
| --- | --- |
| Map | http://127.0.0.1:8080/ |
| Traces (Grafana, Explore → Tempo) | http://127.0.0.1:3000/explore |
| Tempo API | http://127.0.0.1:3200/api/search?q=%7Bresource.service.name%3D%22live-gateway%22%7D |
| Ingest | `POST http://127.0.0.1:8081/v1/positions`, `Authorization: Bearer dev-token` |

Tests: `go test -race ./...`; against the compose Redis: `go test -race -tags integration ./services/location-projector/...`.
Stop: `docker compose down`.
