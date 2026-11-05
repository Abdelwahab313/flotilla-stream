# Simulator
Device simulator and load profiles. Any city from an OpenStreetMap extract; adversarial by default (reorder, duplicates, reboot resets sequence, reconnect storms, clock skew). Lives here until its contract to the ingest gateway is stable, then moves to flotilla-sim.

## Road data
`roadgraph/cairo-downtown.json.gz` is derived from OpenStreetMap data, © OpenStreetMap contributors,
available under the Open Database License (ODbL) 1.0: https://www.openstreetmap.org/copyright.
It is built by `make roadgraph` from the query in `cmd/roadgraph/cairo-downtown.overpassql`; the
file records that query, its SHA-256, the fetched bbox (the district plus a margin, so one-way
carriageways reconnect outside it), the district bbox drivers are sent to, and the Overpass
`osm_base` timestamp of the data.
