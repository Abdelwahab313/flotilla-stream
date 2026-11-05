/*
  Drivable roads of the simulated district: Maadi, Cairo (Old Maadi, Degla, New Maadi).
  Source of simulator/roadgraph/maadi.json.gz; regenerate with `make roadgraph`.
  OSM has no administrative boundary for Maadi (checked 2026-11-05: Overpass name search,
  Nominatim search and reverse; the smallest enclosing boundary is Cairo, relation 5466227).
  The district bbox's south, north and east edges are the union of way 777522265 (El Maadi
  El Gadida, place=neighbourhood, 29.9610532..29.9928101 N, 31.2693515..31.3677671 E),
  node 768769535 (Maadi al-Khabiri, 29.9680141 N 31.2491432 E) and node 768769471
  (El Faroukiya, 29.9500000 N 31.2666670 E), rounded outward to 3 decimals. The west edge
  is hand-set to 31.230, the Nile Corniche, so Old Maadi (Sarayat) is inside the district.
  The way(S,W,N,E) bbox adds a 0.020° margin each side; the preprocessor clips to it and
  keeps the largest strongly connected component over it.
  [date:] pins the OSM data state, so a rerun reproduces the committed file.
  Data © OpenStreetMap contributors, ODbL 1.0.
*/
// district 29.950,31.230,29.993,31.368
// district-source no OSM administrative boundary; assembled from way 777522265, node 768769535, node 768769471; west edge hand-set to 31.230 (Nile Corniche) so Old Maadi (Sarayat) is inside
[out:json][timeout:180][date:"2026-11-05T16:23:05Z"];
way(29.930,31.210,30.013,31.388)
  [highway~"^(motorway|trunk|primary|secondary|tertiary|unclassified|residential|living_street|motorway_link|trunk_link|primary_link|secondary_link|tertiary_link|service)$"]
  [service!~"^(parking_aisle|driveway)$"]
  [access!~"^(private|no)$"]
  [motor_vehicle!~"^(private|no)$"]
  [vehicle!~"^(private|no)$"]
  [area!=yes];
out geom;
