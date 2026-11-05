/*
  Drivable roads of the simulated district: downtown Cairo.
  Source of simulator/roadgraph/cairo-downtown.json.gz; regenerate with `make roadgraph`.
  The way(S,W,N,E) bbox is the district plus a 0.010° margin each side; the preprocessor
  clips to it, keeps the largest strongly connected component over it, and records the
  district line below as the bbox drivers are sent to. [date:] pins the OSM data state,
  so a rerun reproduces the committed file.
  Data © OpenStreetMap contributors, ODbL 1.0.
*/
// district 30.030,31.225,30.060,31.255
[out:json][timeout:180][date:"2026-11-05T15:38:21Z"];
way(30.020,31.215,30.070,31.265)
  [highway~"^(motorway|trunk|primary|secondary|tertiary|unclassified|residential|living_street|motorway_link|trunk_link|primary_link|secondary_link|tertiary_link|service)$"]
  [service!~"^(parking_aisle|driveway)$"]
  [access!~"^(private|no)$"]
  [motor_vehicle!~"^(private|no)$"]
  [vehicle!~"^(private|no)$"]
  [area!=yes];
out geom;
