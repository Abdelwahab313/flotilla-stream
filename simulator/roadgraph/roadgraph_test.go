package roadgraph

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"os"
	"testing"
)

func TestEmbeddedFileWithinLimit(t *testing.T) {
	if n := len(cairoDowntown); n == 0 || n > MaxFileBytes {
		t.Fatalf("embedded graph is %d bytes, want 1..%d", n, MaxFileBytes)
	}
}

func TestEmbeddedGraphMatchesCommittedQuery(t *testing.T) {
	g, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	q, err := os.ReadFile("../../cmd/roadgraph/cairo-downtown.overpassql")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(q)
	if g.Source.Query != string(q) || g.Source.QuerySHA256 != hex.EncodeToString(sum[:]) {
		t.Fatal("embedded graph was not built from the committed query; run make roadgraph")
	}
	if g.Source.OSMBase == "" {
		t.Fatal("no osm_base timestamp")
	}
	if g.Stats().DistrictNodes == 0 {
		t.Fatalf("no graph node inside the district %+v", g.Source.District)
	}
}

func TestEmbeddedGraphIsStronglyConnected(t *testing.T) {
	g, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	fwd, rev := make([][]int, len(g.Nodes)), make([][]int, len(g.Nodes))
	for _, e := range g.Edges {
		fwd[e.From] = append(fwd[e.From], e.To)
		rev[e.To] = append(rev[e.To], e.From)
	}
	for name, adj := range map[string][][]int{"forward": fwd, "reverse": rev} {
		if n := reach(adj); n != len(g.Nodes) {
			t.Fatalf("%s reach from node 0: %d of %d nodes", name, n, len(g.Nodes))
		}
	}
}

func TestEmbeddedEdgesFollowTheirGeometry(t *testing.T) {
	g, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for i, e := range g.Edges {
		if e.Geom[0] != g.Nodes[e.From] || e.Geom[len(e.Geom)-1] != g.Nodes[e.To] {
			t.Fatalf("edge %d (way %d): geometry does not start and end at its nodes", i, e.Way)
		}
		var m float64
		for j, p := range e.Geom {
			if !g.Source.BBox.Contains(p) {
				t.Fatalf("edge %d (way %d): point %v outside the bbox", i, e.Way, p)
			}
			if j > 0 {
				d := DistanceM(e.Geom[j-1], p)
				if d <= 0 {
					t.Fatalf("edge %d (way %d): zero-length piece at %v", i, e.Way, p)
				}
				m += d
			}
		}
		if math.Abs(m-e.LengthM) > 0.01 {
			t.Fatalf("edge %d (way %d): length %.2f m, geometry %.2f m", i, e.Way, e.LengthM, m)
		}
	}
}

func TestDistanceM(t *testing.T) {
	// One degree of latitude on a 6,371,008.8 m sphere is 2πR/360.
	got := DistanceM(LonLat{Lon: 31.24, Lat: 30}, LonLat{Lon: 31.24, Lat: 31})
	if want := 2 * math.Pi * 6371008.8 / 360; math.Abs(got-want) > 1e-6 {
		t.Fatalf("got %.6f m, want %.6f", got, want)
	}
}

func reach(adj [][]int) int {
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
	return n
}
