# Simulator
Device simulator and load profiles. Any city from an OpenStreetMap extract; adversarial by default (reorder, duplicates, reboot resets sequence, reconnect storms, clock skew). Lives here until its contract to the ingest gateway is stable, then moves to flotilla-sim.

## Road data
The district graphs `roadgraph/maadi.json.gz` (the default) and `roadgraph/cairo-downtown.json.gz`
are derived from OpenStreetMap data, © OpenStreetMap contributors, available under the Open
Database License (ODbL) 1.0: https://www.openstreetmap.org/copyright.
`make roadgraph` builds each from its query in `cmd/roadgraph/<name>.overpassql`; each file
records that query, its SHA-256, the fetched bbox (the district plus a margin, so one-way
carriageways reconnect outside it), the district bbox drivers are sent to, where that bbox comes
from when OSM has no boundary for it, and the `osm_base` state of the data (the query's date pin).
