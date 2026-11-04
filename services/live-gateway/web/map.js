import * as maplibregl from 'https://unpkg.com/maplibre-gl@6.16.0/dist/maplibre-gl.mjs';

const CAIRO = [31.2357, 30.0444];
const params = new URLSearchParams(location.search);
const wsURL = params.get('ws') || `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/ws`;

const $ = (id) => document.getElementById(id);
const token = (name) => getComputedStyle(document.body).getPropertyValue(`--${name}`).trim();
const clock = new Intl.DateTimeFormat('en-GB', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });

const MAJOR_ROADS = ['motorway', 'trunk', 'primary'];
const MINOR_ROADS = ['secondary', 'tertiary', 'minor', 'service'];

// [layer id, paint property, token]
const PAINT = [
  ['land', 'background-color', 'map-land'],
  ['landuse', 'fill-color', 'map-land-2'],
  ['park', 'fill-color', 'map-land-2'],
  ['water', 'fill-color', 'map-water'],
  ['waterway', 'line-color', 'map-water'],
  ['building', 'fill-color', 'map-land-2'],
  ['road-minor', 'line-color', 'map-road'],
  ['road-major', 'line-color', 'map-road-major'],
  ['place', 'text-color', 'map-label'],
  ['place', 'text-halo-color', 'map-land'],
  ['water-name', 'text-color', 'map-label'],
  ['water-name', 'text-halo-color', 'map-water'],
  ['drivers', 'circle-color', 'positive'],
  ['drivers', 'circle-stroke-color', 'map-land'],
  ['drivers-ring', 'circle-stroke-color', 'positive'],
  ['driver-label', 'text-color', 'ink-2'],
  ['driver-label', 'text-halo-color', 'map-land'],
];

function style() {
  const c = (name) => token(name);
  return {
    version: 8,
    glyphs: 'https://tiles.openfreemap.org/fonts/{fontstack}/{range}.pbf',
    sources: {
      basemap: { type: 'vector', url: 'https://tiles.openfreemap.org/planet' },
      drivers: { type: 'geojson', data: { type: 'FeatureCollection', features: [] }, promoteId: 'driver_id' },
    },
    layers: [
      { id: 'land', type: 'background', paint: { 'background-color': c('map-land') } },
      { id: 'landuse', type: 'fill', source: 'basemap', 'source-layer': 'landuse', filter: ['in', ['get', 'class'], ['literal', ['residential', 'commercial', 'industrial', 'retail']]], paint: { 'fill-color': c('map-land-2') } },
      { id: 'park', type: 'fill', source: 'basemap', 'source-layer': 'park', paint: { 'fill-color': c('map-land-2') } },
      { id: 'water', type: 'fill', source: 'basemap', 'source-layer': 'water', paint: { 'fill-color': c('map-water') } },
      { id: 'waterway', type: 'line', source: 'basemap', 'source-layer': 'waterway', paint: { 'line-color': c('map-water'), 'line-width': 1.5 } },
      { id: 'building', type: 'fill', source: 'basemap', 'source-layer': 'building', minzoom: 15, paint: { 'fill-color': c('map-land-2'), 'fill-opacity': 0.6 } },
      { id: 'road-minor', type: 'line', source: 'basemap', 'source-layer': 'transportation', filter: ['in', ['get', 'class'], ['literal', MINOR_ROADS]], layout: { 'line-cap': 'round', 'line-join': 'round' }, paint: { 'line-color': c('map-road'), 'line-width': ['interpolate', ['linear'], ['zoom'], 12, 0.5, 16, 3] } },
      { id: 'road-major', type: 'line', source: 'basemap', 'source-layer': 'transportation', filter: ['in', ['get', 'class'], ['literal', MAJOR_ROADS]], layout: { 'line-cap': 'round', 'line-join': 'round' }, paint: { 'line-color': c('map-road-major'), 'line-width': ['interpolate', ['linear'], ['zoom'], 10, 1, 16, 5] } },
      { id: 'water-name', type: 'symbol', source: 'basemap', 'source-layer': 'water_name', layout: { 'text-field': ['upcase', ['coalesce', ['get', 'name:en'], ['get', 'name']]], 'text-font': ['Noto Sans Italic'], 'text-size': 12, 'text-letter-spacing': 0.4 }, paint: { 'text-color': c('map-label'), 'text-halo-color': c('map-water'), 'text-halo-width': 1 } },
      { id: 'place', type: 'symbol', source: 'basemap', 'source-layer': 'place', filter: ['in', ['get', 'class'], ['literal', ['city', 'town', 'suburb', 'neighbourhood', 'quarter']]], layout: { 'text-field': ['upcase', ['coalesce', ['get', 'name:en'], ['get', 'name']]], 'text-font': ['Noto Sans Regular'], 'text-size': 13, 'text-letter-spacing': 0.12 }, paint: { 'text-color': c('map-label'), 'text-halo-color': c('map-land'), 'text-halo-width': 1.2 } },
      // Stale = status ring at full strength; the fill drops to --stale-fill and the dot stops moving.
      { id: 'drivers-ring', type: 'circle', source: 'drivers', paint: { 'circle-radius': 7.5, 'circle-opacity': 0, 'circle-stroke-width': ['case', ['boolean', ['feature-state', 'stale'], false], 2, 0], 'circle-stroke-color': c('positive') } },
      { id: 'drivers', type: 'circle', source: 'drivers', paint: { 'circle-radius': 6, 'circle-color': c('positive'), 'circle-opacity': ['case', ['boolean', ['feature-state', 'stale'], false], Number(c('stale-fill')), 1], 'circle-stroke-width': 1.5, 'circle-stroke-color': c('map-land') } },
      { id: 'driver-label', type: 'symbol', source: 'drivers', minzoom: 13, layout: { 'text-field': ['get', 'driver_id'], 'text-font': ['Noto Sans Regular'], 'text-size': 11, 'text-offset': [0, 1.4], 'text-anchor': 'top' }, paint: { 'text-color': c('ink-2'), 'text-halo-color': c('map-land'), 'text-halo-width': 1.2 } },
    ],
  };
}

const map = new maplibregl.Map({ container: 'map', style: style(), center: CAIRO, zoom: 13, attributionControl: { compact: true } });
map.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'top-right');

function applyTheme() {
  for (const [layer, prop, name] of PAINT) map.setPaintProperty(layer, prop, token(name));
  map.setPaintProperty('drivers', 'circle-opacity', ['case', ['boolean', ['feature-state', 'stale'], false], Number(token('stale-fill')), 1]);
}

const modeButton = $('mode');
function setMode(opsGrey) {
  document.body.classList.toggle('t-chart', opsGrey);
  document.body.classList.toggle('t-bridge', !opsGrey);
  modeButton.textContent = opsGrey ? 'Cargo' : 'Ops Grey';
  modeButton.setAttribute('aria-pressed', String(opsGrey));
  if (map.isStyleLoaded()) applyTheme();
}
modeButton.addEventListener('click', () => setMode(!document.body.classList.contains('t-chart')));
if (params.get('mode') === 'ops-grey') setMode(true);

// Newest device sequence wins on screen too: an older report never moves a dot back.
const latest = new Map();
let lastReportAt = null;
let paused = false;
let centred = false;

function toReport(m) {
  if (!m || typeof m !== 'object') return null;
  const lat = Number(m.lat), lon = Number(m.lon), seq = Number(m.seq);
  if (typeof m.driver_id !== 'string' || !Number.isFinite(lat) || !Number.isFinite(lon) || !Number.isFinite(seq)) return null;
  return { driver_id: m.driver_id, seq, lat, lon };
}

function apply(reports) {
  const add = [], update = [];
  for (const r of reports) {
    const prev = latest.get(r.driver_id);
    if (prev && prev.seq >= r.seq) continue;
    const geometry = { type: 'Point', coordinates: [r.lon, r.lat] };
    if (prev) update.push({ id: r.driver_id, newGeometry: geometry });
    else add.push({ type: 'Feature', geometry, properties: { driver_id: r.driver_id } });
    latest.set(r.driver_id, r);
  }
  if (!add.length && !update.length) return;
  map.getSource('drivers').updateData({ add, update });
  lastReportAt = new Date();
  if (!centred) {
    const first = add[0] || null;
    if (first) map.easeTo({ center: first.geometry.coordinates, zoom: 15 });
    centred = true;
  }
  renderDriverChip();
}

function renderDriverChip() {
  const chip = $('driver-chip');
  const n = latest.size;
  chip.hidden = n === 0;
  chip.textContent = n === 1 ? `${[...latest.keys()][0]} · seq ${[...latest.values()][0].seq}` : `${n} drivers`;
}

function setStale(stale) {
  for (const id of latest.keys()) map.setFeatureState({ source: 'drivers', id }, { stale });
}

function renderStatus() {
  const box = document.querySelector('.status');
  const asOf = lastReportAt ? clock.format(lastReportAt) : '—';
  if (paused) {
    box.dataset.state = 'paused';
    $('conn-text').textContent = 'Reconnecting…';
    $('conn-detail').textContent = `as of ${asOf}`;
    return;
  }
  box.dataset.state = lastReportAt ? 'live' : 'connecting';
  $('conn-text').textContent = lastReportAt ? 'live' : 'Connecting…';
  $('conn-detail').textContent = lastReportAt ? `${Math.max(0, Math.round((Date.now() - lastReportAt) / 1000))} s` : '';
}
setInterval(renderStatus, 1000);

let socket = null;
let attempt = 0;
let retryTimer = null;

function pause() {
  if (paused) return;
  paused = true;
  setStale(true);
  $('paused').hidden = false;
  $('paused-asof').textContent = lastReportAt ? clock.format(lastReportAt) : '—';
  renderStatus();
}

function resume() {
  paused = false;
  attempt = 0;
  setStale(false);
  $('paused').hidden = true;
  renderStatus();
}

function connect() {
  clearTimeout(retryTimer);
  attempt += 1;
  $('paused-attempt').textContent = String(attempt);
  const s = new WebSocket(wsURL);
  socket = s;
  // An open socket is not proof of fresh data; the map leaves the paused state on the first report.
  s.addEventListener('message', (e) => {
    let msg;
    try { msg = JSON.parse(e.data); } catch { return; }
    const reports = (Array.isArray(msg) ? msg : [msg]).map(toReport).filter(Boolean);
    if (!reports.length) return;
    if (paused) resume();
    apply(reports);
  });
  s.addEventListener('close', () => {
    if (s !== socket) return;
    pause();
    retryTimer = setTimeout(connect, Math.min(10000, 1000 * 2 ** Math.min(attempt - 1, 4)));
  });
}

$('retry').addEventListener('click', () => {
  if (socket && socket.readyState === WebSocket.CONNECTING) return;
  const old = socket;
  socket = null;
  if (old) old.close();
  connect();
});

map.on('load', () => {
  applyTheme();
  connect();
});
