import base64, json, math, re, statistics, sys, os
from datetime import datetime, timezone, timedelta

R = sys.argv[1]
def p(*a): print(*a)
def ts(s): return datetime.strptime(s, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)
def secs(a, b): return int((b - a).total_seconds())

def pods_from_json(path):
    out = []
    for it in json.load(open(path))["items"]:
        m = it["metadata"]; conds = it["status"].get("conditions", [])
        r = next((c["lastTransitionTime"] for c in conds if c["type"] == "Ready" and c["status"] == "True"), None)
        img = ",".join(c["image"] for c in it["spec"]["containers"])
        out.append((r, m["creationTimestamp"], m["namespace"] + "/" + m["name"], img))
    return out

def pods_from_log(path):
    out, on = [], False
    for line in open(path):
        if "Pod Ready times" in line: on = True; continue
        if on:
            parts = line.split()
            if len(parts) == 3 and parts[0].endswith("Z"): out.append((parts[0], parts[1], parts[2], ""))
            elif line.startswith("=="): on = False
    return out

def summarize(label, pods):
    first_created = min(ts(c) for _, c, _, _ in pods)
    last_ready_t, last_ready_name = max((ts(r), n) for r, _, n, _ in pods if r)
    p(f"[{label}] pods={len(pods)} first_created={first_created:%H:%M:%SZ} last_ready={last_ready_t:%H:%M:%SZ} ({last_ready_name}) span_s={secs(first_created, last_ready_t)}")
    svc = [(ts(r), n) for r, _, n, _ in pods if r and re.match(r"flotilla/(ingest-gateway|live-gateway|location-projector|simulator)-", n)]
    lr = max(svc); p(f"[{label}]   last of the four services Ready: {lr[1]} {lr[0]:%H:%M:%SZ}")
    for r, c, n, _ in pods:
        if n.endswith("flotilla-combined-0"):
            p(f"[{label}]   kafka pod created {c} -> Ready {r} = {secs(ts(c), ts(r))} s")
        if "/strimzi-cluster-operator" in n:
            p(f"[{label}]   strimzi operator created {c} -> Ready {r} = {secs(ts(c), ts(r))} s")
    tags = sorted({i.split(':')[-1] for *_, i in pods if 'flotilla-stream' in i})
    if tags: p(f"[{label}]   service image tags: {tags}")
    return first_created, last_ready_t

def done_s(path):
    return int(re.search(r"Done in (\d+)s", open(path).read()).group(1))

p("== 1. time to demo")
for name in ["cold", "cold2", "cold3", "warm1", "warm2", "warm3"]:
    path = f"{R}/R0-run-{name}.log"
    txt = open(path).read()
    tag = re.search(r"building \S+:(\S+)", txt).group(1)
    ws = re.search(r"== \[\s*(\d+)s\] The dot moves.*?\n(\d\d:\d\d:\d\d\.\d+) driver", txt, re.S)
    p(f"run {name}: Done in {done_s(path)} s; image tag {tag}; wsprobe step at {ws.group(1)} s, first frame local {ws.group(2)}")
p("cold Done-in values:", [done_s(f"{R}/R0-run-{n}.log") for n in ["cold", "cold2", "cold3"]])
p("warm Done-in values:", [done_s(f"{R}/R0-run-{n}.log") for n in ["warm1", "warm2", "warm3"]])

for logname, jname in [("cold", None), ("cold2", "R0-pods-cold2.json"), ("cold3", "R0-pods-cold3.json")]:
    lp = pods_from_log(f"{R}/R0-run-{logname}.log")
    fc, lr = summarize(f"{logname} log table", lp)
    if jname:
        jp = pods_from_json(f"{R}/{jname}")
        summarize(f"{logname} {jname}", jp)
        same = sorted((r, c, n) for r, c, n, _ in lp) == sorted((r, c, n) for r, c, n, _ in jp if r)
        p(f"[{logname}] log table == {jname} (Ready, created, name for every pod): {same}")
    wall = done_s(f"{R}/R0-run-{logname}.log")
    p(f"[{logname}] wall - pod span = {wall - secs(fc, lr)} s")
    txt = open(f"{R}/R0-run-{logname}.log").read()
    ws = re.search(r"== \[\s*(\d+)s\] The dot moves.*?\n(\d\d:\d\d:\d\d\.\d+) driver", txt, re.S)
    frame = datetime.strptime("2026-11-04 " + ws.group(2), "%Y-%m-%d %H:%M:%S.%f").replace(tzinfo=timezone(timedelta(hours=2)))
    t0 = frame - timedelta(seconds=int(ws.group(1))); end = t0 + timedelta(seconds=wall)
    p(f"[{logname}] approx T0 (first wsprobe frame, EET, minus its step offset) {t0.astimezone(timezone.utc):%H:%M:%S}Z; T0 -> first pod {(fc - t0).total_seconds():.1f} s; last Ready -> end {(end - lr).total_seconds():.1f} s")
    op = next((r, c) for r, c, n, _ in lp if 'strimzi-cluster-operator' in n)
    ka = next((r, c) for r, c, n, _ in lp if n.endswith('flotilla-combined-0'))
    eo = next((r, c) for r, c, n, _ in lp if 'flotilla-entity-operator' in n)
    p(f"[{logname}] chain: operator created -> kafka pod created {secs(ts(op[1]), ts(ka[1]))} s; kafka create -> Ready {secs(ts(ka[1]), ts(ka[0]))} s; kafka Ready -> entity-operator Ready {secs(ts(ka[0]), ts(eo[0]))} s; operator created -> entity-operator Ready {secs(ts(op[1]), ts(eo[0]))} s")
summarize("R0-pods.json", pods_from_json(f"{R}/R0-pods.json"))
for w in ["warm1", "warm2", "warm3"]:
    same = sorted(pods_from_log(f"{R}/R0-run-{w}.log")) == sorted(pods_from_log(f"{R}/R0-run-cold.log"))
    p(f"{w} pod table identical to cold (run 1) table: {same}")
summarize("argo cluster now (argo-pods-abeb343.json)", pods_from_json(f"{R}/argo-pods-abeb343.json"))

p("\n== 2. argo bootstrap")
ft = {l.split()[0]: dict(kv.split("=") for kv in l.split()[1:]) for l in open(f"{R}/raw-file-times-abeb343.txt")}
born, last = ft["gitops-local-abeb343.log"]["birth"], ft["gitops-local-abeb343.log"]["modified"]
p(f"gitops-local log birth {born} -> last write {last} =", secs(ts(born), ts(last)), "s")
app = json.load(open(f"{R}/argo-app-abeb343.json"))
h = app["status"]["history"][0]
nsdoc = json.loads(open(f"{R}/argo-kube-system-ns-abeb343.txt").read().split("\n", 1)[1])
ns = nsdoc["metadata"]["creationTimestamp"]
p("kube-system ns created", ns, "-> history[0].deployedAt", h["deployedAt"], "=", secs(ts(ns), ts(h["deployedAt"])), "s")
p("log birth -> kube-system ns", secs(ts(born), ts(ns)), "s; deployedAt -> health lastTransitionTime",
  secs(ts(h["deployedAt"]), ts(app["status"]["health"]["lastTransitionTime"])), "s")
p("operation", app["status"]["operationState"]["startedAt"], "->", app["status"]["operationState"]["finishedAt"], "=",
  secs(ts(app["status"]["operationState"]["startedAt"]), ts(app["status"]["operationState"]["finishedAt"])), "s")
p("app created", app["metadata"]["creationTimestamp"], "health lastTransitionTime", app["status"]["health"].get("lastTransitionTime"))
waves = {}
for line in open(f"{R}/argo-waves-abeb343.tsv"):
    w, t, nsn, kind = line.rstrip("\n").split("\t")
    waves.setdefault(w, []).append(t)
for w in sorted(waves, key=int):
    p(f"wave {w}: {len(waves[w])} resources, created {min(waves[w])} .. {max(waves[w])}")

p("\n== 4. trace")
doc = json.load(open(f"{R}/argo-trace-756fb689faec7b3a65286061712e2c5e.json"))
def hx(b): return base64.b64decode(b).hex() if b else "-"
spans = []
for rs in doc["trace"]["resourceSpans"]:
    svc = next(a["value"]["stringValue"] for a in rs["resource"]["attributes"] if a["key"] == "service.name")
    for ss in rs["scopeSpans"]:
        for s in ss["spans"]:
            spans.append((int(s["startTimeUnixNano"]), int(s["endTimeUnixNano"]), svc, s["name"], hx(s["spanId"]), hx(s.get("parentSpanId")), hx(s["traceId"])))
spans.sort()
t0 = spans[0][0]; byid = {s[4]: s[3] for s in spans}
p("traceId", {s[6] for s in spans})
p("root start", datetime.fromtimestamp(t0 / 1e9, timezone.utc).isoformat(timespec="microseconds"))
p("| service | span | span id | parent | start offset (ms) | duration (ms) |")
for st, en, svc, name, sid, pid, _ in spans:
    p(f"| {svc} | `{name}` | `{sid}` | {('`'+byid.get(pid, pid)+'`') if pid != '-' else '(root)'} | {(st - t0)/1e6:.3f} | {(en - st)/1e6:.3f} |")
end_last = max(s[1] for s in spans)
p("root start -> last span end ms", (end_last - t0) / 1e6)
ws = [s for s in spans if s[3] == "live-gateway.ws_write"][0]
p("ws_write end UTC", datetime.fromtimestamp(ws[1] / 1e9, timezone.utc).isoformat(timespec="microseconds"))
for line in open(f"{R}/argo-wsprobe-abeb343.log"):
    if "seq=1793826636605" in line:
        fr = datetime.strptime("2026-11-04 " + line.split()[0], "%Y-%m-%d %H:%M:%S.%f").replace(tzinfo=timezone(timedelta(hours=2)))
        p("wsprobe frame seq 1793826636605 at", fr.astimezone(timezone.utc).isoformat(timespec="milliseconds"),
          "; minus ws_write end =", round((fr - datetime.fromtimestamp(ws[1] / 1e9, timezone.utc)).total_seconds() * 1000, 3), "ms")
for line in open(f"{R}/kafka-p4-o83-abeb343.txt"):
    m = re.search(r"traceparent:00-([0-9a-f]{32})-([0-9a-f]{16})-", line)
    if m: p("kafka p4 o83 traceparent trace", m.group(1), "parent span", m.group(2), "== ingest.produce:", m.group(2) == [s for s in spans if s[3] == "ingest.produce"][0][4])
prod = [s for s in spans if s[3] == "ingest.produce"][0]
p("ingest.produce span id", prod[4])

p("\n== 6. hot-path write cost")
b0, b1 = map(int, open(f"{R}/redis-commandstats-window-abeb343.bounds").read().split())
p("bounds", datetime.fromtimestamp(b0, timezone.utc).isoformat(), datetime.fromtimestamp(b1, timezone.utc).isoformat(), "=", b1 - b0, "s")
for line in open(f"{R}/redis-commandstats-window-abeb343.txt"):
    m = re.match(r"cmdstat_(\S+):calls=(\d+),usec=(\d+)", line)
    if m: p(f"redis {m.group(1)} calls={m.group(2)} usec={m.group(3)} mean_us={int(m.group(3))/int(m.group(2)):.2f}")
tq = json.load(open(f"{R}/tempo-projector-apply-window-abeb343.json"))
d = []; starts = []
for t in tq["traces"]:
    for s in t["spanSet"]["spans"]:
        if s["name"] == "projector.apply":
            d.append(int(s["durationNanos"]) / 1e6); starts.append(int(s["startTimeUnixNano"]))
d.sort()
def nearest_rank(q): return d[max(0, math.ceil(q * len(d)) - 1)]
def upper_index(q): return d[int(q * len(d))]
p(f"  upper-index d[int(q*n)]: p50={upper_index(0.5):.4f} p90={upper_index(0.9):.4f} p99={upper_index(0.99):.4f}; nearest-rank p90={nearest_rank(0.9):.4f}")
p(f"tempo projector.apply n={len(d)} traces={len(tq['traces'])} unique_traceIDs={len({t['traceID'] for t in tq['traces']})}")
p(f"  mean={statistics.mean(d):.3f} ms median={statistics.median(d):.3f} p50(nearest-rank)={nearest_rank(0.5):.3f} p99(nearest-rank)={nearest_rank(0.99):.3f} max={d[-1]:.3f} min={d[0]:.3f}")
p(f"  top 3 = {d[-3:]}")
p("  first span start", datetime.fromtimestamp(min(starts)/1e9, timezone.utc).isoformat(timespec="milliseconds"),
  "last", datetime.fromtimestamp(max(starts)/1e9, timezone.utc).isoformat(timespec="milliseconds"))
ko = {}
for line in open(f"{R}/kafka-offsets-window-abeb343.txt"):
    if line.startswith("--time"): cur = line.split()[1]
    m = re.match(r"position\.reported:(\d+):(\d+)", line)
    if m: ko[cur] = int(m.group(2))
p("kafka offsets by time", ko, "records [21:21:00,21:23:00) =", ko["1793827380000"] - ko["1793827260000"],
  " [21:21:00,21:23:01) =", ko["1793827381000"] - ko["1793827260000"])

p("\n== 3. image sizes")
ins = {l.split()[0].split("/")[-1].split(":")[0]: int(l.split()[2]) for l in open(f"{R}/image-inspect-abeb343.txt") if l.startswith("ghcr")}
cri = {i["repoTags"][0].split("/")[-1].split(":")[0]: int(i["size"]) for i in json.load(open(f"{R}/crictl-images-abeb343.json"))["images"] if any("flotilla-stream" in t for t in i.get("repoTags") or [])}
sav = {l.split()[0]: int(l.split()[1]) for l in open(f"{R}/docker-save-bytes-abeb343.txt") if l.split()[0] in ins}
for k in ["simulator", "ingest-gateway", "live-gateway", "location-projector"]:
    p(f"{k}: inspect {ins[k]} B = {ins[k]/1e6:.1f} MB; crictl {cri[k]} B = {cri[k]/1e6:.1f} MB; crictl-inspect {cri[k]-ins[k]} B ({(cri[k]-ins[k])/ins[k]*100:.2f}%); docker save - crictl {sav[k]-cri[k]} B")

p("\n== 5. memory")
dk = [(l.split()[0], float(l.split()[2][:-3]) * 1024) for l in open(f"{R}/mem-docker-stats-abeb343.txt")]
kk = [(j["sampled"], j["node_memory"]["time"], j["node_memory"]["workingSetBytes"] / 1048576) for j in map(json.loads, open(f"{R}/mem-kubelet-summary-abeb343.jsonl"))]
for (t, a), (_, st, b) in zip(dk, kk):
    p(f"{t} docker={a:.1f} MiB kubelet node workingSet={b:.1f} MiB (stats time {st}) delta={a-b:+.1f} MiB ({(a-b)/b*100:+.2f}%)")
for name, xs in (("docker stats", [x for _, x in dk]), ("kubelet", [x for *_, x in kk])):
    p(f"{name}: n={len(xs)} min={min(xs):.1f} median={statistics.median(xs):.1f} mean={statistics.mean(xs):.1f} max={max(xs):.1f} MiB")
md, mk = statistics.median(x for _, x in dk), statistics.median(x for *_, x in kk)
p(f"median delta {md-mk:.1f} MiB = {(md-mk)/mk*100:.2f}%")
ag = [sum((q["ws"] or 0) for q in j["pods"] if q["ns"] == "argocd") / 1048576 for j in map(json.loads, open(f"{R}/mem-kubelet-summary-abeb343.jsonl"))]
p(f"argocd pods working set: min={min(ag):.1f} median={statistics.median(ag):.1f} max={max(ag):.1f} MiB")
p("per-pod working set, last sample:")
last = json.loads(open(f"{R}/mem-kubelet-summary-abeb343.jsonl").read().splitlines()[-1])
tot = 0
for pd in sorted(last["pods"], key=lambda x: -(x["ws"] or 0)):
    tot += pd["ws"] or 0
    p(f"  pod {pd['ns']}/{pd['name']}: {(pd['ws'] or 0)/1048576:.1f} MiB")
p(f"  sum of pod working sets {tot/1048576:.1f} MiB")
argo = sum((pd["ws"] or 0) for pd in last["pods"] if pd["ns"] == "argocd")
p(f"  argocd namespace pods {argo/1048576:.1f} MiB")
