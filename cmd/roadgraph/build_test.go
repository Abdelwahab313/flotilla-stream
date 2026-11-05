package main

import (
	"bytes"
	"math"
	"math/rand/v2"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Abdelwahab313/flotilla-stream/simulator/roadgraph"
)

const fixtureQuery = "// district 30.000,31.000,30.006,31.010\n[out:json][date:\"2026-11-01T00:00:00Z\"];\nway(30.000,31.000,30.010,31.010)[highway];\nout geom;\n"

func fixture(t *testing.T) (*roadgraph.File, *roadgraph.Graph) {
	t.Helper()
	raw, err := os.ReadFile("testdata/overpass.json")
	if err != nil {
		t.Fatal(err)
	}
	f, err := build([]byte(fixtureQuery), raw, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	b, err := encode(f)
	if err != nil {
		t.Fatal(err)
	}
	g, err := roadgraph.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return f, g
}

func pt(lat, lon float64) roadgraph.LonLat { return roadgraph.LonLat{Lon: lon, Lat: lat} }

var (
	n1  = pt(30.001, 31.001)
	n11 = pt(30.001, 31.002)
	n12 = pt(30.001, 31.003)
	n2  = pt(30.001, 31.005)
	n3  = pt(30.005, 31.005)
	n4  = pt(30.005, 31.001)
	n20 = pt(30.006, 31.006)
	n21 = pt(30.007, 31.005)
	n22 = pt(30.006, 31.004)
	n30 = pt(30.001, 31.008)
	n40 = pt(30.0005, 31.0005)
	n41 = pt(30.006, 31.0005)
	n42 = pt(30.003, 31.003)
	n43 = pt(30.0005, 31.002)
	n44 = pt(30.0005, 31.006)
	n51 = pt(30.008, 31.001)
	n52 = pt(30.012, 31.001)
	n53 = pt(30.009, 31.001)
	n65 = pt(30.009, 31.003)
	n66 = pt(30.0085, 31.004)
)

func nodeAt(t *testing.T, g *roadgraph.Graph, p roadgraph.LonLat) int {
	t.Helper()
	for i, n := range g.Nodes {
		if n == p {
			return i
		}
	}
	t.Fatalf("no graph node at %v", p)
	return -1
}

func edges(g *roadgraph.Graph, from, to int) []roadgraph.Edge {
	var es []roadgraph.Edge
	for _, e := range g.Edges {
		if e.From == from && e.To == to {
			es = append(es, e)
		}
	}
	return es
}

func appears(g *roadgraph.Graph, p roadgraph.LonLat) bool {
	for _, e := range g.Edges {
		for _, q := range e.Geom {
			if q == p {
				return true
			}
		}
	}
	return false
}

func TestOnewayForwardOnly(t *testing.T) {
	_, g := fixture(t)
	a, b := nodeAt(t, g, n2), nodeAt(t, g, n3)
	if es := edges(g, a, b); len(es) != 1 || !es[0].Oneway || es[0].Way != 101 {
		t.Fatalf("2->3: %+v, want one one-way edge of way 101", es)
	}
	if es := edges(g, b, a); len(es) != 0 {
		t.Fatalf("3->2 against oneway=yes: %+v", es)
	}
}

func TestOnewayReverse(t *testing.T) {
	_, g := fixture(t)
	n3i, n4i := nodeAt(t, g, n3), nodeAt(t, g, n4)
	es := edges(g, n3i, n4i)
	if len(es) != 1 || !es[0].Oneway {
		t.Fatalf("3->4 (way 4->3 with oneway=-1): %+v, want one one-way edge", es)
	}
	if es[0].Geom[0] != n3 || es[0].Geom[len(es[0].Geom)-1] != n4 {
		t.Fatalf("geometry %v does not run 3->4", es[0].Geom)
	}
	if es := edges(g, n4i, n3i); len(es) != 0 {
		t.Fatalf("4->3 against oneway=-1: %+v", es)
	}
}

func TestRoundaboutIsOneway(t *testing.T) {
	_, g := fixture(t)
	n3i, n21i := nodeAt(t, g, n3), nodeAt(t, g, n21)
	out, back := edges(g, n3i, n21i), edges(g, n21i, n3i)
	if len(out) != 1 || len(back) != 1 {
		t.Fatalf("roundabout 3->21: %d edges, 21->3: %d edges; want 1 and 1", len(out), len(back))
	}
	if !out[0].Oneway || !slices.Equal(out[0].Geom, []roadgraph.LonLat{n3, n20, n21}) {
		t.Fatalf("3->21 = %+v, want one-way through 20", out[0])
	}
	if !back[0].Oneway || !slices.Equal(back[0].Geom, []roadgraph.LonLat{n21, n22, n3}) {
		t.Fatalf("21->3 = %+v, want one-way through 22 (not back through 20)", back[0])
	}
}

func TestExcludedWays(t *testing.T) {
	f, g := fixture(t)
	for name, p := range map[string]roadgraph.LonLat{
		"footway": n40, "access=private": n41, "service=driveway": n42,
		"motor_vehicle=no": n43, "vehicle=private": n44,
	} {
		if appears(g, p) {
			t.Errorf("%s way is in the graph", name)
		}
	}
	for _, n := range g.Nodes {
		if n == n12 {
			t.Error("an excluded driveway made shape node 12 an intersection")
		}
	}
	nodeAt(t, g, n53) // motor_vehicle=destination stays drivable
	if f.Build.WaysRaw != 17 || f.Build.WaysMatched != 12 {
		t.Errorf("ways raw %d matched %d, want 17 and 12", f.Build.WaysRaw, f.Build.WaysMatched)
	}
}

func TestClipsOnRawCoordinate(t *testing.T) {
	f, g := fixture(t)
	if appears(g, pt(30.010, 31.001)) {
		t.Fatal("point at lat 30.0100004, outside the bbox, kept after rounding onto its edge")
	}
	if f.Build.WaysInBBox != 11 {
		t.Fatalf("ways in bbox %d, want 11 (way 114 has one node inside)", f.Build.WaysInBBox)
	}
}

func TestMergesPointsThatRoundTogether(t *testing.T) {
	_, g := fixture(t)
	n51i := nodeAt(t, g, n51)
	for _, p := range []roadgraph.LonLat{n65, n66} {
		if es := edges(g, n51i, nodeAt(t, g, p)); len(es) != 1 || len(es[0].Geom) != 2 {
			t.Fatalf("51->%v: %+v, want one straight edge from the merged node", p, es)
		}
	}
	for _, e := range g.Edges {
		for j := 1; j < len(e.Geom); j++ {
			if roadgraph.DistanceM(e.Geom[j-1], e.Geom[j]) == 0 {
				t.Fatalf("edge of way %d has a zero-length piece at %v", e.Way, e.Geom[j])
			}
		}
	}
}

func TestDistrictInsideBBox(t *testing.T) {
	f, g := fixture(t)
	if f.Source.District != (roadgraph.BBox{S: 30.000, W: 31.000, N: 30.006, E: 31.010}) {
		t.Fatalf("district = %+v", f.Source.District)
	}
	for p, want := range map[roadgraph.LonLat]bool{n1: true, n4: true, n21: false, n51: false} {
		if got := g.InDistrict(nodeAt(t, g, p)); got != want {
			t.Errorf("InDistrict(%v) = %v, want %v", p, got, want)
		}
	}
	raw, err := os.ReadFile("testdata/overpass.json")
	if err != nil {
		t.Fatal(err)
	}
	outside := strings.Replace(fixtureQuery, "30.006,31.010", "30.006,31.011", 1)
	if _, err := build([]byte(outside), raw, "fixture"); err == nil {
		t.Fatal("district reaching outside the query bbox accepted")
	}
	noDistrict := strings.Replace(fixtureQuery, "// district 30.000,31.000,30.006,31.010\n", "", 1)
	if _, err := build([]byte(noDistrict), raw, "fixture"); err == nil {
		t.Fatal("query without a district line accepted")
	}
}

func TestSCCDropsOnewayDeadEnd(t *testing.T) {
	f, g := fixture(t)
	if appears(g, n30) {
		t.Fatal("one-way dead end 2->30 kept")
	}
	want := roadgraph.Build{WaysRaw: 17, WaysMatched: 12, WaysInBBox: 11, OnewayWays: 4,
		SCCs: 2, NodesBeforeSCC: 11, EdgesBeforeSCC: 19, OnewayEdgesBeforeSCC: 5}
	got := f.Build
	got.KMBeforeSCC, got.KMBeforeSCCByClass, got.DistrictKMBeforeSCCByClass = 0, nil, nil
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("build = %+v\nwant    %+v", got, want)
	}
	assertStronglyConnected(t, g)
}

func TestShapeNodesPreserved(t *testing.T) {
	_, g := fixture(t)
	a, b := nodeAt(t, g, n1), nodeAt(t, g, n2)
	fwd, rev := edges(g, a, b), edges(g, b, a)
	if len(fwd) != 1 || len(rev) != 1 {
		t.Fatalf("two-way 1<->2: %d and %d edges, want 1 and 1", len(fwd), len(rev))
	}
	if !slices.Equal(fwd[0].Geom, []roadgraph.LonLat{n1, n11, n12, n2}) {
		t.Fatalf("1->2 geometry %v", fwd[0].Geom)
	}
	if !slices.Equal(rev[0].Geom, []roadgraph.LonLat{n2, n12, n11, n1}) {
		t.Fatalf("2->1 geometry %v", rev[0].Geom)
	}
	want := roadgraph.DistanceM(n1, n11) + roadgraph.DistanceM(n11, n12) + roadgraph.DistanceM(n12, n2)
	if math.Abs(fwd[0].LengthM-want) > 0.01 {
		t.Fatalf("length %.2f m, want %.2f", fwd[0].LengthM, want)
	}
}

func TestClippedToBBox(t *testing.T) {
	f, g := fixture(t)
	nodeAt(t, g, n51)
	if appears(g, n52) {
		t.Fatal("point outside the bbox kept")
	}
	for _, e := range g.Edges {
		for _, p := range e.Geom {
			if !f.Source.BBox.Contains(p) {
				t.Fatalf("edge of way %d leaves the bbox at %v", e.Way, p)
			}
		}
	}
}

func TestStats(t *testing.T) {
	_, g := fixture(t)
	got := g.Stats()
	got.KM = 0
	want := roadgraph.Stats{Nodes: 10, DistrictNodes: 4, ShapePoints: 4, DirectedEdges: 18, OnewayEdges: 4}
	if got != want {
		t.Fatalf("stats = %+v, want %+v", got, want)
	}
}

func TestDeterministic(t *testing.T) {
	raw, err := os.ReadFile("testdata/overpass.json")
	if err != nil {
		t.Fatal(err)
	}
	var out [2][]byte
	for i := range out {
		f, err := build([]byte(fixtureQuery), raw, "fixture")
		if err != nil {
			t.Fatal(err)
		}
		if out[i], err = encode(f); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(out[0], out[1]) {
		t.Fatal("same input encoded to different bytes")
	}
}

func TestSizeGuard(t *testing.T) {
	f, _ := fixture(t)
	b, err := encode(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkSize(len(b), roadgraph.MaxFileBytes); err != nil {
		t.Fatal(err)
	}
	if err := checkSize(len(b), len(b)-1); err == nil {
		t.Fatal("file one byte over the limit passed")
	}
	if err := checkSize(roadgraph.MaxFileBytes+1, roadgraph.MaxFileBytes); err == nil {
		t.Fatal("513th KB passed")
	}
}

func TestRejectsBadResponses(t *testing.T) {
	for name, raw := range map[string]string{
		"remark":                       `{"osm3s":{"timestamp_osm_base":"2026-11-01T00:00:00Z"},"remark":"runtime error: Query timed out","elements":[]}`,
		"no osm_base":                  `{"elements":[]}`,
		"no geometry":                  `{"osm3s":{"timestamp_osm_base":"x"},"elements":[{"type":"way","id":1,"nodes":[1,2],"tags":{"highway":"primary"}}]}`,
		"null coord":                   `{"osm3s":{"timestamp_osm_base":"x"},"elements":[{"type":"way","id":1,"nodes":[1,2],"geometry":[null,{"lat":30.001,"lon":31.001}],"tags":{"highway":"primary"}}]}`,
		"empty":                        `{"osm3s":{"timestamp_osm_base":"x"},"elements":[]}`,
		"not json":                     `<html>429 Too Many Requests</html>`,
		"osm_base before the date pin": `{"osm3s":{"timestamp_osm_base":"2026-10-31T23:59:59Z"},"elements":[{"type":"way","id":1,"nodes":[1,2],"geometry":[{"lat":30.001,"lon":31.001},{"lat":30.002,"lon":31.001}],"tags":{"highway":"primary"}}]}`,
		"oneway=reversible":            `{"osm3s":{"timestamp_osm_base":"2026-11-01T00:00:00Z"},"elements":[{"type":"way","id":1,"nodes":[1,2],"geometry":[{"lat":30.001,"lon":31.001},{"lat":30.002,"lon":31.001}],"tags":{"highway":"primary","oneway":"reversible"}}]}`,
	} {
		if _, err := build([]byte(fixtureQuery), []byte(raw), "fixture"); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if _, err := build([]byte("way[highway];out geom;"), []byte(`{}`), "fixture"); err == nil || !strings.Contains(err.Error(), "bbox") {
		t.Errorf("query without bbox: %v", err)
	}
	raw, err := os.ReadFile("testdata/overpass.json")
	if err != nil {
		t.Fatal(err)
	}
	unpinned := strings.Replace(fixtureQuery, `[date:"2026-11-01T00:00:00Z"]`, "", 1)
	if _, err := build([]byte(unpinned), raw, "fixture"); err == nil || !strings.Contains(err.Error(), "date") {
		t.Errorf("query without a date pin: %v", err)
	}
}

func TestIsDrivable(t *testing.T) {
	for _, tc := range []struct {
		tags map[string]string
		want bool
	}{
		{map[string]string{"highway": "residential"}, true},
		{map[string]string{"highway": "living_street"}, true},
		{map[string]string{"highway": "motorway_link"}, true},
		{map[string]string{"highway": "footway"}, false},
		{map[string]string{"highway": "pedestrian"}, false},
		{map[string]string{"highway": "service"}, true},
		{map[string]string{"highway": "service", "service": "alley"}, true},
		{map[string]string{"highway": "service", "service": "parking_aisle"}, false},
		{map[string]string{"highway": "service", "service": "driveway"}, false},
		{map[string]string{"highway": "residential", "access": "permissive"}, true},
		{map[string]string{"highway": "residential", "access": "no"}, false},
		{map[string]string{"highway": "residential", "access": "private"}, false},
		{map[string]string{"highway": "residential", "motor_vehicle": "destination"}, true},
		{map[string]string{"highway": "residential", "motor_vehicle": "delivery"}, true},
		{map[string]string{"highway": "residential", "motor_vehicle": "no"}, false},
		{map[string]string{"highway": "residential", "motor_vehicle": "private"}, false},
		{map[string]string{"highway": "residential", "vehicle": "no"}, false},
		{map[string]string{"highway": "residential", "vehicle": "private"}, false},
		{map[string]string{"highway": "residential", "area": "yes"}, false},
		{map[string]string{"highway": "residential", "area": "no"}, true},
	} {
		if got := isDrivable(tc.tags); got != tc.want {
			t.Errorf("isDrivable(%v) = %v, want %v", tc.tags, got, tc.want)
		}
	}
}

func TestWayDirection(t *testing.T) {
	for _, tc := range []struct {
		tags    map[string]string
		want    direction
		wantErr bool
	}{
		{map[string]string{}, both, false},
		{map[string]string{"oneway": "yes"}, forward, false},
		{map[string]string{"oneway": "true"}, forward, false},
		{map[string]string{"oneway": "1"}, forward, false},
		{map[string]string{"oneway": "-1"}, reverse, false},
		{map[string]string{"oneway": "no"}, both, false},
		{map[string]string{"oneway": "false"}, both, false},
		{map[string]string{"oneway": "0"}, both, false},
		{map[string]string{"junction": "roundabout"}, forward, false},
		{map[string]string{"junction": "roundabout", "oneway": "no"}, both, false},
		{map[string]string{"junction": "roundabout", "oneway": "-1"}, reverse, false},
		{map[string]string{"oneway": "reversible"}, both, true},
		{map[string]string{"oneway": "alternating"}, both, true},
		{map[string]string{"oneway": "Yes"}, both, true},
	} {
		got, err := wayDirection(tc.tags)
		if (err != nil) != tc.wantErr || (err == nil && got != tc.want) {
			t.Errorf("wayDirection(%v) = %v, %v; want %v, error %v", tc.tags, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestComponentsMatchReachability(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for iter := range 2000 {
		n := 1 + rng.IntN(9)
		out := make([][]int, n)
		for u := range n {
			for v := range n {
				if rng.Float64() < 0.2 {
					out[u] = append(out[u], v)
				}
			}
		}
		reach := make([][]bool, n)
		for s := range n {
			reach[s] = make([]bool, n)
			reach[s][s] = true
			stack := []int{s}
			for len(stack) > 0 {
				u := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				for _, v := range out[u] {
					if !reach[s][v] {
						reach[s][v] = true
						stack = append(stack, v)
					}
				}
			}
		}
		comp, count := components(out)
		classes, maxSize := map[int]bool{}, 0
		for u := range n {
			size := 0
			for v := range n {
				mutual := reach[u][v] && reach[v][u]
				if mutual != (comp[u] == comp[v]) {
					t.Fatalf("graph %d %v: nodes %d,%d mutually reachable %v, same component %v", iter, out, u, v, mutual, comp[u] == comp[v])
				}
				if mutual {
					size++
				}
			}
			classes[comp[u]] = true
			maxSize = max(maxSize, size)
		}
		if count != len(classes) {
			t.Fatalf("graph %d %v: count %d, %d distinct components", iter, out, count, len(classes))
		}
		largest, size := largestComponent(comp, count), 0
		for u := range n {
			if comp[u] == largest {
				size++
			}
		}
		if size != maxSize {
			t.Fatalf("graph %d %v: kept component has %d nodes, largest has %d", iter, out, size, maxSize)
		}
	}
}

func assertStronglyConnected(t *testing.T, g *roadgraph.Graph) {
	t.Helper()
	fwd, rev := make([][]int, len(g.Nodes)), make([][]int, len(g.Nodes))
	for _, e := range g.Edges {
		fwd[e.From] = append(fwd[e.From], e.To)
		rev[e.To] = append(rev[e.To], e.From)
	}
	for name, adj := range map[string][][]int{"forward": fwd, "reverse": rev} {
		seen := make([]bool, len(adj))
		stack, n := []int{0}, 0
		seen[0] = true
		for len(stack) > 0 {
			v := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			n++
			for _, w := range adj[v] {
				if !seen[w] {
					seen[w] = true
					stack = append(stack, w)
				}
			}
		}
		if n != len(adj) {
			t.Fatalf("%s reach from node 0: %d of %d nodes", name, n, len(adj))
		}
	}
}
