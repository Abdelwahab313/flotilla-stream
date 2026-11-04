---
name: console-map
description: How to build the console's live MapLibre map — layer order, zoom-dependent representation (hex, clusters, individual drivers), symbology from tokens, gliding interpolation that never passes the last report, area subscriptions, selection and incident footprint, and how to measure frame time. Use when writing or reviewing anything under web/src/map/ or the live gateway's subscription protocol.
---

# Console map

Meanings: CONSOLE-PRD §11. Values and symbols: `docs/design/console/README.md` § Map
symbology, and the `Tokens-type-and-map-symbology` and `Component-Basemap-themed` boards.

## Layer order (bottom to top)

basemap → zone fills (`depth` intensity = waiting orders / available drivers) → zone
outlines and labels with counts → driver hexes (z ≤ 11) → trails (selected and watched,
15 min, fading) → routes (selected driver; dashed before / solid after in Proposals) →
order pins and stop markers → drivers → selection halo and incident footprint mask.
Insert data layers below basemap labels only where the PRD says labels stay subdued.

## Representation by zoom

| Zoom | Drivers | Orders |
| --- | --- | --- |
| ≤ 11 | hex density by count | zone waiting counts |
| 12–14 | clusters with count, ring segmented by status mix | clustered pins |
| ≥ 14 | individual symbols with heading | individual pins with late ring |

Above ~5,000 drivers in view, use the next zoom-out representation. Decide the
representation from the server-side count for the viewport, not by rendering first.

## Theme

Build the MapLibre style from tokens: read computed CSS custom properties at load and on
theme switch, then `setPaintProperty` (do not reload the style). Both basemaps (Cargo,
Ops Grey) use `map-land`, `map-land-2`, `map-water`, `map-road`, `map-road-major`,
`map-label`.

## Drivers: data path

1. Walking skeleton: one GeoJSON source with `promoteId` on driver id; positions from
   the position store applied with `updateData({ update: [...] })` diffs at report rate
   (not per frame); symbols by `match` on a numeric status property; stale via
   feature-state: status ring at full strength, fill opacity `--stale-fill` (0.35). Dots step at report rate; the skeleton does
   not glide, and says so in the slice's Deferred list.
2. Measure at 5,000 moving points, on the reference laptop and at 4× CPU throttle:
   p95 frame time and receive-to-paint p99. `updateData` re-tiles in MapLibre's worker,
   so frame time can look healthy while receive-to-paint grows; watch both.
3. If either misses its budget (16.7 ms, 100 ms), or when gliding is in scope: a custom
   layer (`CustomLayerInterface`) that draws from the typed-array position store in one
   instanced draw call per symbol kind, interpolating on the GPU from
   (previous, current, t).

Never one DOM marker per driver. `maplibregl.Marker` is only for the selected subject's
callout, if at all.

## Gliding

Interpolate from the previous reported point to the latest over the report interval;
clamp at the latest (never extrapolate past it). A new report mid-glide starts from the
currently drawn point. A position with an older `(session, seq)` is dropped before it
reaches the map (invariant 4 holds on screen too). Stale drivers stop gliding. Reduced
motion: jump to the latest point.

## Subscriptions (decision 9)

The client subscribes to the cells covering the viewport plus one ring of margin;
debounce resubscription on `moveend` (not `move`); diff cell sets and send only
add/remove. The gateway coalesces to one update per driver per interval; the client must
tolerate a driver appearing, disappearing at a cell edge, and reappearing.

## Selection and footprint

One selection model (URL `?driver=D-118`): select from map, list, palette or events; the
map shows halo, route with numbered stops (square pickup, circle dropoff, filled next
stop), trail and open offer. Incident footprint dims everything outside it with a mask
layer above data layers, `scrim` colour.

## Measuring

`performance.mark('recv')` when the worker hands a frame over, `performance.mark('paint')`
in the next `requestAnimationFrame` after the source update (rAF runs before the frame
is drawn, so this approximates paint; say so wherever the number is published);
receive-to-paint is the difference, on one clock (worker and main thread aligned by
`performance.timeOrigin`), sampled and exported. Gateway send timestamps are a labelled
cross-check only: two machines' clocks skew by a large share of 100 ms. Frame time from `requestAnimationFrame` deltas in a
Playwright run (CPU throttling over CDP for the 4× case) with a fixed camera and a
fixed simulator seed.
