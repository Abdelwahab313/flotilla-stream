# Releases

Fourteen thin vertical slices in strict order, sized relative to each other
(S < M < L, roughly 1 : 1.5 : 2.5). No dates: the build is AI-paired and the
only clock is the interview calendar. A slice closes when every exit criterion
in its file is ticked and the tag exists. One slice is `open` at a time.

| Release | Size | Slice | Demo at the end | Status |
| --- | :-: | --- | --- | --- |
| [R0](R0.md) Walking skeleton | L | One driver's position travels device → log → projector → store → map | Dot moves; one trace spans the path | open |
| R1 Load source and baseline | M | Thousands of simulated drivers on real roads; first throughput and lag numbers | 1,000 drivers moving; baseline in README | — |
| R2 Live dispatcher view | M | Cell-based live fan-out to many dashboards | Ten dashboards, each its own area; position-to-screen p99 | — |
| R3 Zones and geofence events | M | Enter/exit/dwell; state rebuilt by replay | Zones light up; rebuild equals live | — |
| R3b Public always-on demo | M | Hardened edge (TLS, short-lived tokens, read-only view, rate limits), small fleet on a cheap cluster, cost metered | Public URL live from a phone; cost panel shows spend | — |
| R4a Orders and single-writer dispatch | L | Outbox, reservation + pinning + fencing, nearest-available, auto-accept | Orders assigned live; zero double assignment under burst, restart, border test | — |
| **v0.4 cut** | — | First showable system | README numbers; 60 s clip | — |
| R4b Offer protocol | L | Offers with expiry, accept/decline, heartbeat loss | Declines and timeouts live; late accept rejected | — |
| **v0.5 cut** | — | Showable on local cluster | 3-minute video; README | — |
| R5 Trip history | S | Cold path to columnar history | Replay a driver's day; hot path unaffected | — |
| R6a Cloud environment | M | Same system on a 3-zone GKE cluster from code | One command creates/destroys; demo passes there | — |
| R6b Elasticity | L | Lag-based autoscaling, drain, disruption budgets | Spike scales out and back, checker green | — |
| R7 SLOs and alerting | S | Three SLOs, burn-rate alerts, runbooks | Injected slowdown fires the right alert | — |
| R8 Load proof | M | Step load to the breaking point; cost per 1k drivers | Published curves | — |
| R9 Failure proof | M | Fault-injection scenarios with invariant checks | Kill node and zone; map keeps moving | — |
| R10 v1.0 | S | Design write-up, ADRs, runbook, video | Tagged release | — |

Full product plan (goals, invariants, fourteen decisions, event catalog, evidence
plan): `../product/PLAN.md`. The design write-up `../design/README.md` grows per
release and is complete at v1.0.
