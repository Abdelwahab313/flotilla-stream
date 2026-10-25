# flotilla-stream

flotilla-stream is a portfolio project: the real-time backbone of a delivery company, built by me to show how I would run one. Simulated drivers stream GPS positions, the system tracks each driver live, raises an event when one crosses a geofence, and assigns each order to the nearest available driver with no double assignment, including while a node or a whole zone is down. All traffic is synthetic. There is no real fleet behind it, and `simulator/` is the generator that produces every driver, position and order you will see. Every number in this README comes from a test in this repo that you can rerun yourself, so if a figure is not backed by a test, it does not belong here.

**Status:** pre-R0. Nothing runs yet. The release plan is in `docs/releases/` and the decisions I have made so far, with the alternatives I rejected, are in `docs/decisions/`.

## Headline numbers

None yet. This table fills in per release, each row with the command that produced it.

| Measure | Target | Measured | Release |
| --- | --- | --- | --- |
| Sustained ingest | 50,000 drivers at one update / 5 s = 10,000 events/s | — | R1, R8 |
| Position-to-screen latency | p99 < 500 ms at target load | — | R2, R8 |
| Latest-position query | p99 < 20 ms | — | R1 |
| Assignment latency | p99 < 1 s from order created to driver offered | — | R4a |
| Correctness | zero lost / double assignments, zero position regressions | — | every release, via `checker/` |
| Recovery | consumer lag back to normal within 2 min after losing a node or a zone | — | R9 |
| Elasticity | scale out within 1 min of lag threshold; scale in without loss | — | R6b |
| Cost | per 1,000 drivers per month at target load | — | R8 |

## Run it locally

Not yet. From R0: `make demo-r0` takes a clean checkout to a moving dot on the map on a local kind cluster in under 10 minutes.

## Layout

| Path | Contents |
| --- | --- |
| `docs/design/` | The 10-minute design and results write-up |
| `docs/decisions/` | One record per design decision, 0001–0013 |
| `docs/releases/` | Release plan and the open slice's exit criteria |
| `docs/results/` | Measured numbers per release with the commands that produced them |
| `docs/runbook/` | One entry per alert |
| `services/` | One folder per deployable component (Go) |
| `simulator/` | Device simulator and load profiles |
| `checker/` | Invariant checker |
| `deploy/` | Kustomize manifests shared by every environment (`local`, `gke`) |
| `infra/` | Terraform for the GKE environment |
| `demo/` | One demo script per release |

## Flotilla

This is the first repo under the Flotilla umbrella. Reserved for later, once the boundaries here are proven: `flotilla-sim` (the simulator on its own), `flotilla-dispatch` (routing and optimization, a non-goal here), `flotilla-connect` (store and courier integrations).
