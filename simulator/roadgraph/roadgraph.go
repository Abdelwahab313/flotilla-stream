// Package roadgraph holds the directed road graphs of the simulated districts, each
// built by cmd/roadgraph from one committed Overpass query and embedded in the simulator.
// Road data © OpenStreetMap contributors, ODbL 1.0.
package roadgraph

import (
	"compress/gzip"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"math"
	"slices"
)

// MaxFileBytes is the hard limit on each embedded, gzipped graph file.
const MaxFileBytes = 1 << 20

// FormatVersion is the version of File written by cmd/roadgraph.
const FormatVersion = 1

// Default is the district a simulator run drives when none is chosen.
const Default = "maadi"

// files maps each district to its graph file, named after the query that built it.
var files = map[string]string{
	"downtown": "cairo-downtown.json.gz",
	"maadi":    "maadi.json.gz",
}

//go:embed *.json.gz
var embedded embed.FS

// Districts lists the embedded districts by name, sorted.
func Districts() []string {
	return slices.Sorted(maps.Keys(files))
}

// File is the on-disk form: undirected segments carry a oneway flag (already
// oriented along the direction of travel); Decode expands them to directed edges.
type File struct {
	Format   int       `json:"format"`
	Source   Source    `json:"source"`
	Build    Build     `json:"build"`
	Nodes    []float64 `json:"nodes"`
	Segments []Segment `json:"segments"`
}

// Source is the provenance of the graph. BBox is the fetched bbox the graph covers;
// District, inside it, is the bbox drivers are sent to. Roads in the margin between
// them stay in the graph so carriageways cut at the district edge still connect.
// OSMBase is the state of the OSM data: the query's [date:] pin.
type Source struct {
	Endpoint    string `json:"endpoint"`
	QuerySHA256 string `json:"query_sha256"`
	Query       string `json:"query"`
	BBox        BBox   `json:"bbox"`
	District    BBox   `json:"district"`
	// DistrictSource says where the district bbox comes from, when recorded.
	DistrictSource string `json:"district_source,omitempty"`
	OSMBase        string `json:"osm_base"`
	License        string `json:"license"`
}

// BBox is south, west, north, east in degrees.
type BBox struct {
	S float64 `json:"s"`
	W float64 `json:"w"`
	N float64 `json:"n"`
	E float64 `json:"e"`
}

// Contains reports whether p lies inside b, edges included.
func (b BBox) Contains(p LonLat) bool {
	return p.Lat >= b.S && p.Lat <= b.N && p.Lon >= b.W && p.Lon <= b.E
}

// Build records what the preprocessor saw before it kept the largest strongly
// connected component; the dropped part is not in the file.
type Build struct {
	WaysRaw              int                `json:"ways_raw"`
	WaysMatched          int                `json:"ways_matched"`
	WaysInBBox           int                `json:"ways_in_bbox"`
	OnewayWays           int                `json:"oneway_ways"`
	SCCs                 int                `json:"sccs"`
	NodesBeforeSCC       int                `json:"nodes_before_scc"`
	EdgesBeforeSCC       int                `json:"edges_before_scc"`
	OnewayEdgesBeforeSCC int                `json:"oneway_edges_before_scc"`
	KMBeforeSCC          float64            `json:"km_before_scc"`
	KMBeforeSCCByClass   map[string]float64 `json:"km_before_scc_by_class"`
	// Lengths between consecutive points that both lie in the district.
	DistrictKMBeforeSCCByClass map[string]float64 `json:"district_km_before_scc_by_class"`
}

// Segment is one street piece between two graph nodes. Shape holds the interior
// shape points as lon,lat pairs, from A to B.
type Segment struct {
	A       int32     `json:"a"`
	B       int32     `json:"b"`
	Oneway  bool      `json:"oneway,omitempty"`
	Way     int64     `json:"way"`
	Highway string    `json:"hw"`
	LengthM float64   `json:"m"`
	Shape   []float64 `json:"shape,omitempty"`
}

// LonLat is a WGS84 point in degrees.
type LonLat struct {
	Lon, Lat float64
}

// Edge is one direction of travel along a segment. Geom runs from Nodes[From] to
// Nodes[To] inclusive, through every shape point of the street.
type Edge struct {
	From, To int
	Oneway   bool
	Way      int64
	Highway  string
	LengthM  float64
	Geom     []LonLat
}

// Graph is the directed road graph: every node reaches every other.
type Graph struct {
	Source Source
	Build  Build
	Nodes  []LonLat
	Edges  []Edge
}

// Stats are counted from the loaded graph.
type Stats struct {
	Nodes         int
	DistrictNodes int
	ShapePoints   int
	DirectedEdges int
	OnewayEdges   int
	KM            float64
}

// Load decodes the embedded graph of the named district.
func Load(district string) (*Graph, error) {
	name, ok := files[district]
	if !ok {
		return nil, fmt.Errorf("roadgraph: unknown district %q (have %v)", district, Districts())
	}
	fh, err := embedded.Open(name)
	if err != nil {
		return nil, fmt.Errorf("roadgraph: %s: %w", district, err)
	}
	defer fh.Close()
	g, err := Decode(fh)
	if err != nil {
		return nil, fmt.Errorf("roadgraph: %s: %w", district, err)
	}
	return g, nil
}

// Decode reads a gzipped File and expands it to a Graph.
func Decode(r io.Reader) (*Graph, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("roadgraph: gunzip: %w", err)
	}
	defer zr.Close()
	var f File
	if err := json.NewDecoder(zr).Decode(&f); err != nil {
		return nil, fmt.Errorf("roadgraph: decode: %w", err)
	}
	if f.Format != FormatVersion {
		return nil, fmt.Errorf("roadgraph: format %d, want %d", f.Format, FormatVersion)
	}
	if len(f.Nodes)%2 != 0 {
		return nil, fmt.Errorf("roadgraph: %d node coordinates, want lon,lat pairs", len(f.Nodes))
	}
	g := &Graph{Source: f.Source, Build: f.Build, Nodes: make([]LonLat, len(f.Nodes)/2)}
	for i := range g.Nodes {
		g.Nodes[i] = LonLat{Lon: f.Nodes[2*i], Lat: f.Nodes[2*i+1]}
	}
	g.Edges = make([]Edge, 0, 2*len(f.Segments))
	for i, s := range f.Segments {
		a, b := int(s.A), int(s.B)
		if a < 0 || a >= len(g.Nodes) || b < 0 || b >= len(g.Nodes) || len(s.Shape)%2 != 0 {
			return nil, fmt.Errorf("roadgraph: segment %d (way %d) is malformed", i, s.Way)
		}
		geom := make([]LonLat, 0, len(s.Shape)/2+2)
		geom = append(geom, g.Nodes[a])
		for j := 0; j < len(s.Shape); j += 2 {
			geom = append(geom, LonLat{Lon: s.Shape[j], Lat: s.Shape[j+1]})
		}
		geom = append(geom, g.Nodes[b])
		g.Edges = append(g.Edges, Edge{From: a, To: b, Oneway: s.Oneway, Way: s.Way, Highway: s.Highway, LengthM: s.LengthM, Geom: geom})
		if !s.Oneway {
			rev := make([]LonLat, len(geom))
			for j, p := range geom {
				rev[len(geom)-1-j] = p
			}
			g.Edges = append(g.Edges, Edge{From: b, To: a, Way: s.Way, Highway: s.Highway, LengthM: s.LengthM, Geom: rev})
		}
	}
	return g, nil
}

// InDistrict reports whether node i lies in the served district.
func (g *Graph) InDistrict(i int) bool {
	return g.Source.District.Contains(g.Nodes[i])
}

// Stats counts nodes, edges and road length; a two-way street counts once in
// ShapePoints and KM.
func (g *Graph) Stats() Stats {
	st := Stats{Nodes: len(g.Nodes), DirectedEdges: len(g.Edges)}
	for i := range g.Nodes {
		if g.InDistrict(i) {
			st.DistrictNodes++
		}
	}
	var twoWayShape int
	var onewayM, twoWayM float64
	for _, e := range g.Edges {
		if e.Oneway {
			st.OnewayEdges++
			st.ShapePoints += len(e.Geom) - 2
			onewayM += e.LengthM
		} else {
			twoWayShape += len(e.Geom) - 2
			twoWayM += e.LengthM
		}
	}
	st.ShapePoints += twoWayShape / 2
	st.KM = (onewayM + twoWayM/2) / 1000
	return st
}

// DistanceM is the haversine distance in metres on the mean Earth radius.
func DistanceM(a, b LonLat) float64 {
	const r = 6371008.8
	φ1, φ2 := a.Lat*math.Pi/180, b.Lat*math.Pi/180
	dφ, dλ := φ2-φ1, (b.Lon-a.Lon)*math.Pi/180
	h := math.Sin(dφ/2)*math.Sin(dφ/2) + math.Cos(φ1)*math.Cos(φ2)*math.Sin(dλ/2)*math.Sin(dλ/2)
	return 2 * r * math.Asin(math.Sqrt(h))
}
