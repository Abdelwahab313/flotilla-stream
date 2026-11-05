# Second derivation of the road-graph stats from the raw Overpass response, independent
# of cmd/roadgraph. Counts are before the largest-SCC cut, which jq does not do. Pass
# the query's way(S,W,N,E) bbox (district plus margin) for the graph totals, or the
# district bbox (30.030 31.225 30.060 31.255) for the district's pre-SCC km by class.
# Lengths use the raw 7-decimal coordinates; the graph's use 6, so km differ by metres.
# jq -r --argjson s 30.020 --argjson w 31.215 --argjson n 30.070 --argjson e 31.265 \
#   -f cmd/roadgraph/derive.jq /tmp/flotilla-osm/cairo-downtown.json
def inb: .lat >= $s and .lat <= $n and .lon >= $w and .lon <= $e;
def rad: . * 3.141592653589793 / 180;
def hav($a; $b):
  (($b.lat - $a.lat) | rad) as $dp | (($b.lon - $a.lon) | rad) as $dl
  | (($dp / 2 | sin) * ($dp / 2 | sin) + ($a.lat | rad | cos) * ($b.lat | rad | cos) * ($dl / 2 | sin) * ($dl / 2 | sin)) as $h
  | 2 * 6371008.8 * ($h | sqrt | asin);
def drivable:
  ((.highway // "") | test("^(motorway|trunk|primary|secondary|tertiary|unclassified|residential|living_street|motorway_link|trunk_link|primary_link|secondary_link|tertiary_link|service)$"))
  and ((.service // "") | test("^(parking_aisle|driveway)$") | not)
  and ((.access // "") | test("^(private|no)$") | not)
  and ((.motor_vehicle // "") | test("^(private|no)$") | not)
  and ((.vehicle // "") | test("^(private|no)$") | not)
  and (.area != "yes");
def oneway: (.oneway // "") as $o
  | ($o | test("^(yes|true|1|-1)$")) or (.junction == "roundabout" and ($o | test("^(no|false|0)$") | not));
# runs of consecutive in-bbox node ids with at least 2 distinct nodes
def runs: . as $way
  | reduce range(0; $way.nodes | length) as $i ({cur: [], out: []};
      if ($way.geometry[$i] | inb) then
        (if (.cur | length) > 0 and .cur[-1] == $way.nodes[$i] then . else .cur += [$way.nodes[$i]] end)
      else
        (if (.cur | length) >= 2 then .out += [.cur] else . end) | .cur = []
      end)
  | if (.cur | length) >= 2 then .out + [.cur] else .out end;
def km: add / 1000 * 1000 | round / 1000;

[.elements[] | select(.type == "way")] as $raw
| [$raw[] | select(.tags | drivable)] as $m
| [$m[] | (.tags | oneway) as $o | runs[] | {o: $o, r: .}] as $pieces
| (reduce ($pieces[].r[] | tostring) as $id ({}; .[$id] += 1)) as $seen
| (reduce ($pieces[] | .r[0], .r[-1] | tostring) as $id ({}; .[$id] = true)) as $end
| [$seen | keys[] | select($end[.] or $seen[.] >= 2)] as $nodes
| [$pieces[] | .o as $o | [.r[1:][] | tostring | select($end[.] or $seen[.] >= 2)] | length | {o: $o, segs: .}] as $segs
| [$m[] | .tags.highway as $hw | .geometry as $g | range(1; $g | length) | select(($g[. - 1] | inb) and ($g[.] | inb)) | {hw: $hw, m: hav($g[. - 1]; $g[.])}] as $metres
| "ways            raw \($raw | length), matched \($m | length), in bbox \([$m[] | select((runs | length) > 0)] | length), one-way \([$m[] | select(.tags | oneway)] | length)",
  "nodes           \($nodes | length)",
  "directed edges  \([$segs[] | if .o then .segs else 2 * .segs end] | add)",
  "one-way edges   \([$segs[] | select(.o) | .segs] | add)",
  "drivable km     \([$metres[].m] | km)",
  "km by class     \($metres | group_by(.hw) | map({(.[0].hw): (map(.m) | km)}) | add)"
