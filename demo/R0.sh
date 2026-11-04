#!/usr/bin/env bash
# R0 demo: clean checkout -> kind cluster -> a moving dot -> one trace device to screen.
set -euo pipefail
cd "$(dirname "$0")/.."

CTX=kind-flotilla
GRAFANA=http://127.0.0.1:13000
TEMPO="$GRAFANA/api/datasources/proxy/uid/tempo"
RAW=docs/results/raw
step() { printf '\n== [%4ss] %s\n' "$SECONDS" "$1"; }

step "Building images, creating the kind cluster, applying deploy/overlays/local (Strimzi Kafka, Redis, Tempo, Grafana, four services)"
make up-local

step "Building wsprobe"
go build -o bin/wsprobe ./cmd/wsprobe

step "The dot moves: two strictly rising seqs for D-118 over the live gateway WebSocket"
bin/wsprobe -url http://127.0.0.1:18080/ws -n 2 -timeout 60s

step "One trace spans simulator -> ingest -> log -> projector -> live gateway WebSocket write"
query='{ resource.service.name = "simulator" } && { name = "live-gateway.ws_write" }'
trace_id=""
for _ in $(seq 1 30); do
  trace_id=$(curl -fsS -G "$TEMPO/api/search" --data-urlencode "q=$query" --data-urlencode limit=1 |
    python3 -c 'import json,sys; t=json.load(sys.stdin).get("traces") or []; print(t[0]["traceID"] if t else "")')
  [ -n "$trace_id" ] && break
  sleep 2
done
[ -n "$trace_id" ] || { echo "no trace with simulator.report and live-gateway.ws_write in Tempo"; exit 1; }
mkdir -p "$RAW"
for _ in $(seq 1 10); do
  curl -fsS "$TEMPO/api/v2/traces/$trace_id" -o "$RAW/R0-trace-$trace_id.json"
  if python3 - "$RAW/R0-trace-$trace_id.json" <<'EOF'
import json, sys
doc = json.load(open(sys.argv[1]))
services, names = set(), set()
for rs in doc["trace"]["resourceSpans"]:
    for a in rs["resource"]["attributes"]:
        if a["key"] == "service.name":
            services.add(a["value"]["stringValue"])
    for ss in rs["scopeSpans"]:
        names.update(s["name"] for s in ss["spans"])
want = {"simulator", "ingest-gateway", "location-projector", "live-gateway"}
print("services:", ", ".join(sorted(services)))
print("spans:   ", ", ".join(sorted(names)))
sys.exit(0 if want <= services and {"simulator.report", "live-gateway.ws_write"} <= names else 1)
EOF
  then break; fi
  sleep 2
done
python3 -c 'import json,sys; json.load(open(sys.argv[1]))' "$RAW/R0-trace-$trace_id.json"
echo "trace $trace_id saved to $RAW/R0-trace-$trace_id.json"

step "Secondary: the projector's newest seq for D-118 in Redis rises"
key='fleet:{fleet-demo}:driver:D-118'
a=$(kubectl --context $CTX exec -n flotilla deploy/redis -- redis-cli HGET "$key" seq)
sleep 2
b=$(kubectl --context $CTX exec -n flotilla deploy/redis -- redis-cli HGET "$key" seq)
echo "seq $a -> $b"
[ "$b" -gt "$a" ]

step "Pod Ready times"
kubectl --context $CTX get pods -A -o json > "$RAW/R0-pods.json"
python3 - "$RAW/R0-pods.json" <<'EOF' | sort
import json, sys
for p in json.load(open(sys.argv[1]))["items"]:
    m, conds = p["metadata"], p["status"].get("conditions", [])
    ready = next((c["lastTransitionTime"] for c in conds if c["type"] == "Ready" and c["status"] == "True"), "-")
    print(ready, m["creationTimestamp"], m["namespace"] + "/" + m["name"])
EOF

step "Done in ${SECONDS}s"
echo "Map:    http://127.0.0.1:18080/"
echo "Trace:  $GRAFANA/explore (Tempo, trace id $trace_id)"
