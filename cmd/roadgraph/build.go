package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"

	"github.com/Abdelwahab313/flotilla-stream/simulator/roadgraph"
)

type overpassResponse struct {
	OSM3S struct {
		TimestampOSMBase string `json:"timestamp_osm_base"`
	} `json:"osm3s"`
	Remark   string    `json:"remark"`
	Elements []element `json:"elements"`
}

type element struct {
	Type     string            `json:"type"`
	ID       int64             `json:"id"`
	Nodes    []int64           `json:"nodes"`
	Geometry []*latLon         `json:"geometry"`
	Tags     map[string]string `json:"tags"`
}

type latLon struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

var drivableHighway = map[string]bool{
	"motorway": true, "trunk": true, "primary": true, "secondary": true, "tertiary": true,
	"unclassified": true, "residential": true, "living_street": true,
	"motorway_link": true, "trunk_link": true, "primary_link": true, "secondary_link": true, "tertiary_link": true,
	"service": true,
}

// isDrivable mirrors the tag filters of the committed query, so a raw file from
// elsewhere (or a hand-written fixture) gets the same treatment.
func isDrivable(t map[string]string) bool {
	switch {
	case !drivableHighway[t["highway"]]:
		return false
	case t["service"] == "parking_aisle" || t["service"] == "driveway":
		return false
	case t["access"] == "private" || t["access"] == "no":
		return false
	case t["motor_vehicle"] == "private" || t["motor_vehicle"] == "no":
		return false
	case t["vehicle"] == "private" || t["vehicle"] == "no":
		return false
	case t["area"] == "yes":
		return false
	}
	return true
}

type direction int

const (
	both direction = iota
	forward
	reverse
)

// wayDirection fails on oneway values it cannot honour (reversible, alternating, typos)
// rather than guessing a direction.
func wayDirection(t map[string]string) (direction, error) {
	switch t["oneway"] {
	case "yes", "true", "1":
		return forward, nil
	case "-1":
		return reverse, nil
	case "no", "false", "0":
		return both, nil
	case "":
	default:
		return both, fmt.Errorf("unsupported oneway=%q", t["oneway"])
	}
	if t["junction"] == "roundabout" {
		return forward, nil
	}
	return both, nil
}

const num = `\s*([-\d.]+)\s*`

var (
	bboxRe     = regexp.MustCompile(`way\(` + num + `,` + num + `,` + num + `,` + num + `\)`)
	districtRe = regexp.MustCompile(`(?m)^//\s*district` + num + `,` + num + `,` + num + `,` + num + `$`)
	dateRe     = regexp.MustCompile(`\[date:"([^"]+)"\]`)
	sourceRe   = regexp.MustCompile(`(?m)^//\s*district-source\s+(.+?)\s*$`)
)

func parseBBoxes(query []byte) (bbox, district roadgraph.BBox, err error) {
	if bbox, err = parseBBox(bboxRe, query, "way(S,W,N,E)"); err != nil {
		return
	}
	if district, err = parseBBox(districtRe, query, "// district S,W,N,E"); err != nil {
		return
	}
	if district.S < bbox.S || district.W < bbox.W || district.N > bbox.N || district.E > bbox.E {
		err = fmt.Errorf("district %+v is not inside the query bbox %+v", district, bbox)
	}
	return
}

func parseBBox(re *regexp.Regexp, query []byte, what string) (roadgraph.BBox, error) {
	m := re.FindSubmatch(query)
	if m == nil {
		return roadgraph.BBox{}, fmt.Errorf("query has no %s bbox", what)
	}
	var v [4]float64
	for i := range v {
		f, err := strconv.ParseFloat(string(m[i+1]), 64)
		if err != nil {
			return roadgraph.BBox{}, fmt.Errorf("query %s: %w", what, err)
		}
		v[i] = f
	}
	b := roadgraph.BBox{S: v[0], W: v[1], N: v[2], E: v[3]}
	if b.S >= b.N || b.W >= b.E {
		return roadgraph.BBox{}, fmt.Errorf("query %s %v is empty", what, v)
	}
	return b, nil
}

type piece struct {
	way     int64
	highway string
	oneway  bool
	reverse bool
	nodes   []int64
}

func build(query, raw []byte, endpoint string) (*roadgraph.File, error) {
	bbox, district, err := parseBBoxes(query)
	if err != nil {
		return nil, err
	}
	var resp overpassResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode overpass response: %w", err)
	}
	// Overpass answers 200 with a remark and partial data when a query times out or runs out of memory.
	if resp.Remark != "" {
		return nil, fmt.Errorf("overpass remark: %s", resp.Remark)
	}
	if resp.OSM3S.TimestampOSMBase == "" {
		return nil, errors.New("overpass response has no osm3s.timestamp_osm_base")
	}
	// The file records the query's [date:] pin, not the server's osm_base, so the same
	// query gives the same bytes on every fetch; a server behind the pin cannot answer it.
	m := dateRe.FindSubmatch(query)
	if m == nil {
		return nil, errors.New(`query has no [date:"..."] pin`)
	}
	date := string(m[1])
	if resp.OSM3S.TimestampOSMBase < date {
		return nil, fmt.Errorf("overpass osm_base %s is older than the query's date %s", resp.OSM3S.TimestampOSMBase, date)
	}
	var districtSource string
	if m := sourceRe.FindSubmatch(query); m != nil {
		districtSource = string(m[1])
	}
	sum := sha256.Sum256(query)
	f := &roadgraph.File{
		Format: roadgraph.FormatVersion,
		Source: roadgraph.Source{
			Endpoint:       endpoint,
			QuerySHA256:    hex.EncodeToString(sum[:]),
			Query:          string(query),
			BBox:           bbox,
			District:       district,
			DistrictSource: districtSource,
			OSMBase:        date,
			License:        "© OpenStreetMap contributors, ODbL 1.0",
		},
	}

	coord := map[int64]roadgraph.LonLat{}
	// Consecutive nodes that round to one point are merged rather than one dropped: the
	// dropped one may be another way's intersection, and dropping it would cut the junction.
	merged := map[int64]int64{}
	find := func(id int64) int64 {
		for {
			to, ok := merged[id]
			if !ok {
				return id
			}
			id = to
		}
	}
	var pieces []piece
	for _, el := range resp.Elements {
		if el.Type != "way" {
			continue
		}
		f.Build.WaysRaw++
		if !isDrivable(el.Tags) {
			continue
		}
		f.Build.WaysMatched++
		dir, err := wayDirection(el.Tags)
		if err != nil {
			return nil, fmt.Errorf("way %d: %w", el.ID, err)
		}
		if dir != both {
			f.Build.OnewayWays++
		}
		if len(el.Nodes) != len(el.Geometry) {
			return nil, fmt.Errorf("way %d: %d nodes but %d coordinates; the query must end in `out geom;`", el.ID, len(el.Nodes), len(el.Geometry))
		}
		var run []int64
		flush := func() {
			if len(run) >= 2 {
				pieces = append(pieces, piece{way: el.ID, highway: el.Tags["highway"], oneway: dir != both, reverse: dir == reverse, nodes: run})
			}
			run = nil
		}
		for i, id := range el.Nodes {
			g := el.Geometry[i]
			if g == nil {
				return nil, fmt.Errorf("way %d: node %d has no coordinates", el.ID, id)
			}
			// Clip on the raw coordinate: rounding first would pull points just outside onto the edge.
			if !bbox.Contains(roadgraph.LonLat{Lon: g.Lon, Lat: g.Lat}) {
				flush()
				continue
			}
			p := roadgraph.LonLat{Lon: round(g.Lon, 6), Lat: round(g.Lat, 6)}
			coord[id] = p
			if len(run) > 0 {
				prev := find(run[len(run)-1])
				if prev == find(id) {
					continue
				}
				if coord[prev] == p {
					merged[find(id)] = prev
					continue
				}
			}
			run = append(run, id)
		}
		flush()
	}

	ways := map[int64]bool{}
	resolved := pieces[:0]
	for _, p := range pieces {
		var ids []int64
		for _, id := range p.nodes {
			if id = find(id); len(ids) == 0 || ids[len(ids)-1] != id {
				ids = append(ids, id)
			}
		}
		if len(ids) < 2 {
			continue
		}
		if p.reverse {
			slices.Reverse(ids)
		}
		p.nodes = ids
		resolved = append(resolved, p)
		ways[p.way] = true
	}
	pieces = resolved
	f.Build.WaysInBBox = len(ways)
	if len(pieces) == 0 {
		return nil, errors.New("no drivable road inside the bbox")
	}

	// A node is a graph node where a piece ends or where pieces (or one piece twice) pass through it.
	seen := map[int64]int{}
	isEnd := map[int64]bool{}
	for _, p := range pieces {
		for _, id := range p.nodes {
			seen[id]++
		}
		isEnd[p.nodes[0]], isEnd[p.nodes[len(p.nodes)-1]] = true, true
	}
	isNode := func(id int64) bool { return isEnd[id] || seen[id] >= 2 }
	index := map[int64]int{}
	var nodes []int64
	for _, p := range pieces {
		for _, id := range p.nodes {
			if _, ok := index[id]; !ok && isNode(id) {
				index[id] = len(nodes)
				nodes = append(nodes, id)
			}
		}
	}

	type segment struct {
		a, b      int
		oneway    bool
		way       int64
		highway   string
		lengthM   float64
		districtM float64
		shape     []int64
	}
	var segs []segment
	out := make([][]int, len(nodes))
	for _, p := range pieces {
		start, lengthM, districtM := 0, 0.0, 0.0
		for j := 1; j < len(p.nodes); j++ {
			a, b := coord[p.nodes[j-1]], coord[p.nodes[j]]
			d := roadgraph.DistanceM(a, b)
			lengthM += d
			if district.Contains(a) && district.Contains(b) {
				districtM += d
			}
			if !isNode(p.nodes[j]) {
				continue
			}
			s := segment{a: index[p.nodes[start]], b: index[p.nodes[j]], oneway: p.oneway, way: p.way, highway: p.highway, lengthM: lengthM, districtM: districtM, shape: p.nodes[start+1 : j]}
			segs = append(segs, s)
			out[s.a] = append(out[s.a], s.b)
			if !s.oneway {
				out[s.b] = append(out[s.b], s.a)
			}
			start, lengthM, districtM = j, 0, 0
		}
	}

	comp, count := components(out)
	largest := largestComponent(comp, count)
	f.Build.SCCs = count
	f.Build.NodesBeforeSCC = len(nodes)

	kept := make([]int32, len(nodes))
	for i, id := range nodes {
		kept[i] = -1
		if comp[i] == largest {
			kept[i] = int32(len(f.Nodes) / 2)
			p := coord[id]
			f.Nodes = append(f.Nodes, p.Lon, p.Lat)
		}
	}
	var kmBefore float64
	byClass, districtByClass := map[string]float64{}, map[string]float64{}
	for _, s := range segs {
		kmBefore += s.lengthM / 1000
		byClass[s.highway] += s.lengthM / 1000
		districtByClass[s.highway] += s.districtM / 1000
		if s.oneway {
			f.Build.EdgesBeforeSCC++
			f.Build.OnewayEdgesBeforeSCC++
		} else {
			f.Build.EdgesBeforeSCC += 2
		}
		if kept[s.a] < 0 || kept[s.b] < 0 {
			continue
		}
		var shape []float64
		for _, id := range s.shape {
			p := coord[id]
			shape = append(shape, p.Lon, p.Lat)
		}
		f.Segments = append(f.Segments, roadgraph.Segment{
			A: kept[s.a], B: kept[s.b], Oneway: s.oneway, Way: s.way,
			Highway: s.highway, LengthM: round(s.lengthM, 2), Shape: shape,
		})
	}
	f.Build.KMBeforeSCC = round(kmBefore, 3)
	for c, km := range byClass {
		byClass[c] = round(km, 3)
		districtByClass[c] = round(districtByClass[c], 3)
	}
	f.Build.KMBeforeSCCByClass = byClass
	f.Build.DistrictKMBeforeSCCByClass = districtByClass
	return f, nil
}

func largestComponent(comp []int, count int) int {
	size := make([]int, count)
	for _, c := range comp {
		size[c]++
	}
	largest := 0
	for c := range size {
		if size[c] > size[largest] {
			largest = c
		}
	}
	return largest
}

func components(out [][]int) (comp []int, count int) {
	n := len(out)
	order, low := make([]int, n), make([]int, n)
	onStack := make([]bool, n)
	comp = make([]int, n)
	var stack []int
	next := 1
	var visit func(v int)
	visit = func(v int) {
		order[v], low[v] = next, next
		next++
		stack = append(stack, v)
		onStack[v] = true
		for _, w := range out[v] {
			if order[w] == 0 {
				visit(w)
				low[v] = min(low[v], low[w])
			} else if onStack[w] {
				low[v] = min(low[v], order[w])
			}
		}
		if low[v] == order[v] {
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				comp[w] = count
				if w == v {
					break
				}
			}
			count++
		}
	}
	for v := range n {
		if order[v] == 0 {
			visit(v)
		}
	}
	return comp, count
}

func round(x float64, decimals int) float64 {
	p := math.Pow10(decimals)
	return math.Round(x*p) / p
}

func encode(f *roadgraph.File) ([]byte, error) {
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	enc := json.NewEncoder(zw)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(f); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
