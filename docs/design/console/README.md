# Console design system

The visual basis for every console screen. `CONSOLE-PRD.md` fixes **meanings** (what a
status is, what must be distinguishable, what is clickable). This document and
[`tokens.css`](tokens.css) fix **values** (colour, type, shape, layout). Owner rule (2026-11-03): the Claude Design
mockups win on look and layout; the PRD is updated to match. The PRD still wins on
domain meaning and wording (statuses, lifecycle, permissions, glossary terms, time formats); a mismatch there is recorded in
[Open items](#open-items).

Source: the R2 design revision, six desktop bundles in `mockups/bundles-2026-11-03/`
(2026-11-03). `mockups/boards/<board>/index.html` holds the decoded markup of each board,
with its components (`TopBar.dc.html`, `NavRail.dc.html`, `Basemap.dc.html`) beside it;
read those rather than the bundles, which are gzip+base64 inside a JSON manifest. Serve
`mockups/boards/` over HTTP to render a board. The first revision (2026-10-25), including
the O7 phone boards, is in `mockups/archive/`. `mockups/` is a local asset folder, not
tracked in this repo.

## Boards

| Board | Covers | Folder in `mockups/boards/` |
| --- | --- | --- |
| S1 Live, peak 12:40, Cargo | App shell, map, dock, context panel (driver), watch rail, toasts, incident banner | `S1-Live-Cargo-peak-12-40-mode-toggle-Ops-Grey` |
| S1 Live, Ops Grey | The whole shell in the light mode | `S1-Live-Ops-Grey-mode` |
| S1 Live, collapsed regions | Context panel, bottom dock, watch rail, nav rail each collapsed, both modes | `S1-Live-{Cargo,Ops-Grey}-{context-panel,bottom-dock,watch-rail,nav-rail}-collapsed` |
| S1 Live, map focus mode | Map and top bar only, "Exit map focus `\`" pill, both modes | `S1-Live-{Cargo,Ops-Grey}-map-focus-mode` |
| S1 Live, 1280 × 800 and 1920 × 1080 | Shell and top bar at the outer breakpoints (Cargo) | `S1-Live-Cargo-1280x800`, `S1-Live-Cargo-1920x1080` |
| S1 Live, quiet morning 07:58 | Flow 1, empty Waiting tab, import info item, zero badges hidden | `S1-Live-quiet-morning-07-58-Flow-1` |
| S1 Live, connection lost | Reconnecting banner, everything frozen "as of 12:40:11", countdown paused | `S1-Live-live-connection-lost` |
| Top bar tiers | What never hides, collapse order, each tier at its width | `Top-bar-breakpoint-tiers` |
| R2 tokens and contrast | Final token list, changed tokens, contrast tables | `R2-tokens-and-contrast` |
| S2 Orders | Flow 5 race outcome, table, bulk bar | `S2-Orders-Flow-5-race-outcome` |
| S4 Proposals | Flow 3, before/after route, ETA impact, keyboard | `S4-Proposals-Flow-3-prototype` |
| S5 Alerts | Flow 4, incident grouping, claim, progress | `S5-Alerts-Flow-4-prototype` |
| S7 Day overview | KPI tiles, orders per hour, on-time by zone | `S7-Day-overview` |
| S0 Sign in | Sign in, city chooser | `S0-Sign-in-and-city-choice` |
| Tokens and symbology | Tokens, type scale, map symbols, zoom representations | `Tokens-type-and-map-symbology` |
| Component sheet | PRD §16 inventory, every state | `Component-sheet` |
| Top bar, Nav rail, Basemap | Shell components in isolation | `Component-Top-bar`, `Nav-rail`, `Component-Basemap-themed` |

Not yet designed (PRD §18): S3 Drivers full page, S6 Events full page, S8 Imports and
exports, S9 Settings, O3 command palette as a full overlay (the component sheet has it),
compact vs comfortable density side by side, Ops Grey renders of S2, S4, S5 and S7.
The O7 phone boards were not revised for R2 (`mockups/archive/`).

## Themes

One system, two modes. The CSS classes keep the PRD names; the mockup renamed the modes.

| Class | Mode name | Role | Character |
| --- | --- | --- | --- |
| `.t-bridge` | **Cargo** | Dark; default for signed-in dispatchers | Neutral charcoal, white ink, one indigo accent, bright data colours |
| `.t-chart` | **Ops Grey** | Light; public view and README screenshots | A grey world where colour appears only when something needs attention |

Every value is in [`tokens.css`](tokens.css). Roles:

- **Ground and ink:** `ground` → `surface` → `raised` → `overlay` (elevation order),
  `line`, `line-strong`, `ink`, `ink-2`, `ink-3`, `accent` (fills, rings, focus; never
  text), `accent-text` (accent as text), `accent-ink` (text on an accent fill),
  `accent-soft`.
- **Semantic:** `positive`, `attention`, `active`, `neutral`, `restricted`, `critical`,
  `info`, each with a `-soft` fill for pills and row tints (no `info-soft`);
  `critical-ink` is text on a `critical` fill.
- **Map:** `map-land`, `map-land-2`, `map-water`, `map-road`, `map-road-major`,
  `map-label`, `depth` (zone fill colour), `zone-1`…`zone-4` (zone fill steps: `depth`
  mixed into `map-land-2`; `zone-4` is the cap), `zone-line`, `route`, `halo`,
  `stale-fill` (fill opacity of a stale symbol).
- **Effects:** `scrim`, `shadow`, `hazard` (operator drawer edge only).

Cargo zones are neutral (`depth` = `ink-2`), so blue on-route dots never sit on an
indigo fill. In Ops Grey the selection `halo` is the accent, distinct from near-black
`active`. The `R2-tokens-and-contrast` board lists which tokens the R2 revision changed.

### Contrast

Verified 2026-11-03 with the WCAG 2 formula on the hex values in `tokens.css`, computed
independently of the designer's table on the `R2-tokens-and-contrast` board. All 122 ratios (66
text, 48 glyph, 8 text-on-fill) agree to 0.01, and all pass.

| Check | Target | Cargo minimum | Ops Grey minimum |
| --- | --- | --- | --- |
| Text roles on `ground`, `surface`, `raised` | 4.5:1 | 4.60 (`active` on raised) | 4.61 (`attention` on ground) |
| Status glyphs on `map-land` | 3:1 | 4.66 (`active`) | 4.05 (`attention`) |
| Status glyphs on `map-land-2` | 3:1 | 4.32 (`active`) | 4.52 (`attention`) |
| Status glyphs on the busiest zone (`zone-4` blend: #3F3F43 / #B3B2D5) | 3:1 | 3.08 (`active`) | 3.15 (`attention`) |
| `accent-ink` on `accent`, `critical-ink` on `critical` | 4.5:1 | 4.63 / 5.45 | 6.64 / 6.47 |

Zone fill: opacity is capped (`zone-4`) rather than outlining every glyph; the 1.5 px
`map-land` outline on dots stays. The cap is tight in Cargo (3.08), so a new status colour
or a higher zone step must re-run the check. Selection halo against `active` is low
(1.21 Cargo, 1.72 Ops Grey). It works because of the 3 px `map-land` gap ring between halo and dot
and the hue difference. Contrast alone does not separate them.

## Type

| Style | Size / weight | Family | Use |
| --- | --- | --- | --- |
| Title | 20 / 600 | UI | Panel and incident titles |
| Heading | 14 / 600 | UI | Section headings |
| Body | 13 / 400 | UI | Prose, outcomes ("Not cancelled — accepted by D-118 at 14:02:11.") |
| Label | 12 / 500 | UI | Column heads, field labels; sentence case, quiet, never shouted |
| Mono | 13 / 500 | Mono | Ids, refs, timestamps, numeric columns |
| KPI | 24 / 600 | Mono | Headline numbers |
| Map label | caps | UI | Zone names and counts on the map only |

UI family: Plus Jakarta Sans throughout (no IBM Plex Sans). Mono: IBM Plex Mono. Plus
Jakarta Sans has the `tnum` feature, but its default figures are proportional, so
`tokens.css` switches tabular figures on at the theme root; live numbers must not
shimmer. Dense tables use 11–12 px in the boards. Map labels render from vector-tile
glyphs (Noto Sans in R0), not the UI font.

## Shape and layout

- Surfaces are rounded cards (18 px) floating on `ground` with 12 px gaps; controls are
  pills (10 px radius).
- Shell sizes in the boards: top bar 48 px high, nav rail 68 px wide, context panel
  372 px wide, toast stack 352 px wide at top-left of the map (12 px inset), legend
  300 px at bottom-left. Shell minimum height 760 px. Below 1440 px shell width the
  context panel is 320 px.
- Every region collapses (context panel, bottom dock, watch rail, nav rail) and the state
  persists per dispatcher. Map focus mode (`\`) leaves map and top bar with an "Exit map
  focus `\`" pill at the bottom centre of the map (PRD §9).
- Breakpoints are container queries on the component's own width, not the viewport.
- **Top bar** never hides: brand mark, city switcher, local time with timezone, health
  strip (state and latency), alerts with critical and warning counts, account. It
  collapses in this order:
  1. < 1800: fleet name, KPI sub-values, search label, date.
  2. < 1440: Available and On-time into a "+2" menu.
  3. < 1280: Waiting and Late into "Figures" (the late count stays on the button);
     wordmark text, event lag, mode toggle (moves to the account menu).
  4. < 1024: "Figures", the city name in the timezone, the "p99" label.
- **Dock status chips:** all eight at ≥ 1800; Waiting…Assigned plus "More" below; below
  1440 the zone and late filters fold into "Filters".
- Alert badges show one count everywhere (top bar, nav rail, dock) and hide at zero.
- The "n updates — show" pill sits in the table header strip, never over rows.
- Map overlays (banners, notes, toasts) never cover the layer chips, zone labels or
  scale bar.

## Connection lost

Nothing stale may look live. While the live connection is down the map shows "Live
updates paused — reconnecting · positions as of 12:40:11", and every live value freezes
with that stamp. This covers top-bar figures (dimmed, "as of"), the health strip ("Reconnecting…",
p99 and lag "—"), watch tiles ("as of" instead of "live"), connectivity badges, dock
lists (a "Frozen as of" strip, `.paused-stripes`), Events ("paused"). Countdowns stop and
say so ("0:21 left as of 12:40:11 — the real time left is unknown until the connection is
back"). The page returns to live on the first fresh update, not on socket open.
- Layout must mirror for RTL: use logical properties (`inline-start`, `padding-inline`),
  never `left`/`right` for flow layout.

## Map symbology

Every state is told apart by shape, fill or motion, never colour alone.

| State | Symbol | Colour role |
| --- | --- | --- |
| Available | Solid dot, glides | `positive` |
| Offered | Dot with outer ring; ring pulses | `attention` |
| On route | Dot with heading arrow | `active` |
| Finishing route | Outlined dot (fill `map-land`) with arrow | `active` |
| On break | Hatched dot, static | `neutral` |
| Off duty | Small hollow dot, hidden by default | `neutral` |
| Suspended | Double ring, static | `restricted` |
| Stale | Status ring at full strength, fill at `stale-fill` (0.35), no heading, no gliding | status colour |
| Lost | Dashed ghost ring with time since | `critical` |
| Waiting order | Hollow pin | `ink-2` |
| Late order | Pin with ring | `critical` |
| Pickup stop | Square with stop number | `route` |
| Dropoff stop | Circle (fill `map-land-2`, stroke `route`) with number | `route` |
| Next stop | Filled in `route` | `route` |
| Selected driver | Halo with a 3 px `map-land` gap ring | `halo` |

Every driver dot carries a 1.5 px `map-land` outline. A faded symbol cannot hold 3:1 on a
busy zone, so stale is a ring, not a fade. Zone fills step through `zone-1`…`zone-4`.

Zoom representation: city (z ≤ 11) hex density by driver count plus zone waiting counts;
district (z 12–14) clusters with count and a ring segmented by status mix (on route,
available, offered, other); street (z ≥ 14) individual drivers with heading, order pins
with late ring. Above ~5,000 drivers in view, use the representation one zoom out.

## Motion

Drivers glide between reports and never past the last reported point. The offer ring
(`fl-ring`, 1.6 s) is the only loop. Rows settle once a second after a batch. Skeletons
shimmer (`fl-shimmer`). With `prefers-reduced-motion`: positions step, the pulse becomes
a static ring, skeletons and spinners stop.

## Writing and time

| Format | Where |
| --- | --- |
| `14:02` | Lists |
| `14:02:11` | Timelines |
| `14:02:11.384` | Events stream |
| `40 s ago` | Freshness |
| `0:28` | Countdowns |
| `4 min` | ETAs and deltas; ETAs are labelled "est." |

Outcomes are specific: "Not cancelled — accepted by D-118 at 14:02:11. Return to queue
instead?", never "Something went wrong". Pending reads "Cancelling…", then after 5 s
"Still waiting for dispatch…". All strings are externalized.

## Components

The component sheet board renders the PRD §16 inventory. Implemented components carry
these names and live in one place (`web/src/ui/` when R2 creates it):

Buttons (primary, secondary, outline, ghost, destructive, pending, read-only), key cap ·
driver status pill · connectivity badge (`live · 2 s`, `stale · 40 s`, `lost · 4 m`) ·
order status pill · late ring · severity marker · countdown · stop list item · timeline
item with actor · message bubble with status chain · incident row with progress ·
proposal row with age · status tabs with counts · filter chips · bulk bar with result
line · "n updates — show" pill · toast (critical, watch, "+n more") · banners
(reconnecting, read-only, scenario running) · health strip and panel · empty, error,
skeleton, pending and race-outcome states · drop zone and job progress · API key
reveal-once card · confirm with reason · command palette · operator drawer (hazard edge) ·
watch tile (live, stale).

Rules that hold for every component:

1. Role tokens only. No hex, no `rgba()` outside `tokens.css`.
2. Every live value shows its freshness; nothing stale looks live.
3. Every action has pending, outcome, race-outcome and read-only states.
4. Keyboard first: every row action has a key from PRD §10.13; focus is visible
   (`2px solid var(--accent)`, 2 px offset).
5. Status is never colour alone: pills carry text, map symbols carry shape.

## Open items

Gaps and defects still to resolve. Theme concept, status palette, font, `info` sharing
the accent in Ops Grey, and hidden actions on the phone view were settled in favour of
the mockup on 2026-11-03 and the PRD was updated.

| # | PRD says | Mockup does | Status |
| --- | --- | --- | --- |
| 5 | §12.1, §18.2: every screen in both modes | R2 revision draws the whole S1 shell in Ops Grey (`.t-chart .t-bridge` selector stops Cargo leaking into an Ops Grey subtree). S2, S4, S5 and S7 still have no Ops Grey render; O7 has no Cargo render | Open for S2/S4/S5/S7 and O7 only (not needed for R2) |
| 6 | §11.3, F2: the next stop is emphasised (filled) | S4 legend uses the same filled `route` dot for "New stop"; S1 and the symbology board use it for "Next stop" | Open — one symbol, two meanings; give new stops a distinct mark (e.g. `+` badge on the number) |
| 8 | §12.7: lists use `14:02` for clock times, `4 min` for durations | "In status" column renders age as `11:42` / `0:02` (S1 dock, S2), indistinguishable from a clock time; S5 uses `6m`, S7 `1m 48s` | Open — recommend `11 min` (and `<1 min`) for ages |
| 9 | CONTEXT.md lists "service area" as an _Avoid_ word for City; "batch" for Import job | PRD §5.8 and the S1 quiet-morning board (still in the R2 revision) use "Pickup outside service area" and "Night batch finished…" | Open — reword to "Pickup outside the city's zones" and "Night import finished…", or amend the glossary |
| 10 | §12.2: AA for all text and status glyphs against their ground in both modes | R2 tokens pass every check (§ Contrast) | Closed 2026-11-03 |
