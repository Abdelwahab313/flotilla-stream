// Command roadgraph builds the simulator's directed road graph from one committed
// Overpass query: drivable ways clipped to the query's bbox, split at intersections
// with every shape node kept, reduced to the largest strongly connected component.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Abdelwahab313/flotilla-stream/simulator/roadgraph"
)

func main() {
	query := flag.String("query", "cmd/roadgraph/cairo-downtown.overpassql", "Overpass QL file, the committed source")
	in := flag.String("in", "", "raw Overpass JSON to read instead of fetching")
	raw := flag.String("raw", "/tmp/flotilla-osm/cairo-downtown.json", "where a fetched raw response is saved (never committed)")
	out := flag.String("out", "simulator/roadgraph/cairo-downtown.json.gz", "graph file to write")
	endpoint := flag.String("endpoint", "https://overpass-api.de/api/interpreter", "Overpass API endpoint")
	stats := flag.String("stats", "", "print the stats of this graph file and exit")
	flag.Parse()

	if *stats != "" {
		if err := printStats(os.Stdout, *stats); err != nil {
			fail(err)
		}
		return
	}
	if err := run(*query, *in, *raw, *out, *endpoint); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "roadgraph:", err)
	os.Exit(1)
}

func run(queryPath, in, rawPath, out, endpoint string) error {
	q, err := os.ReadFile(queryPath)
	if err != nil {
		return err
	}
	var raw []byte
	if in != "" {
		if raw, err = os.ReadFile(in); err != nil {
			return err
		}
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if raw, err = fetch(ctx, endpoint, q); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(rawPath), 0o755); err != nil {
			return err
		}
		if err := writeFile(rawPath, raw); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "roadgraph: raw response %d bytes -> %s\n", len(raw), rawPath)
	}
	f, err := build(q, raw, endpoint)
	if err != nil {
		return err
	}
	b, err := encode(f)
	if err != nil {
		return err
	}
	if err := checkSize(len(b), roadgraph.MaxFileBytes); err != nil {
		return err
	}
	if err := writeFile(out, b); err != nil {
		return err
	}
	return printStats(os.Stdout, out)
}

// writeFile replaces path only once the new bytes are complete, so an interrupted
// run never leaves a truncated graph for the embed or a truncated raw response.
func writeFile(path string, b []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func fetch(ctx context.Context, endpoint string, query []byte) ([]byte, error) {
	form := url.Values{"data": {string(query)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "flotilla-stream-roadgraph/1 (+https://github.com/Abdelwahab313/flotilla-stream)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("overpass %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("overpass %s: read: %w", endpoint, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("overpass %s: %s: %.300s", endpoint, resp.Status, b)
	}
	return b, nil
}

func checkSize(n, limit int) error {
	if n > limit {
		return fmt.Errorf("graph is %d bytes gzipped, over the %d-byte limit; shrink the bbox or the classes", n, limit)
	}
	return nil
}

func printStats(w io.Writer, path string) error {
	fh, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fh.Close()
	fi, err := fh.Stat()
	if err != nil {
		return err
	}
	g, err := roadgraph.Decode(fh)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	st, b, s := g.Stats(), g.Build, g.Source
	fmt.Fprintf(w, "file            %s (%d bytes, limit %d)\n", path, fi.Size(), roadgraph.MaxFileBytes)
	fmt.Fprintf(w, "osm_base        %s\n", s.OSMBase)
	fmt.Fprintf(w, "query_sha256    %s\n", s.QuerySHA256)
	fmt.Fprintf(w, "bbox            S %g W %g N %g E %g\n", s.BBox.S, s.BBox.W, s.BBox.N, s.BBox.E)
	fmt.Fprintf(w, "district        S %g W %g N %g E %g (%d nodes)\n", s.District.S, s.District.W, s.District.N, s.District.E, st.DistrictNodes)
	fmt.Fprintf(w, "ways            raw %d, matched %d, in bbox %d, one-way %d\n", b.WaysRaw, b.WaysMatched, b.WaysInBBox, b.OnewayWays)
	fmt.Fprintf(w, "sccs            %d total, 1 kept, %d dropped\n", b.SCCs, b.SCCs-1)
	fmt.Fprintf(w, "                kept     before SCC  dropped\n")
	fmt.Fprintf(w, "nodes           %-8d %-11d %d\n", st.Nodes, b.NodesBeforeSCC, b.NodesBeforeSCC-st.Nodes)
	fmt.Fprintf(w, "directed edges  %-8d %-11d %d\n", st.DirectedEdges, b.EdgesBeforeSCC, b.EdgesBeforeSCC-st.DirectedEdges)
	fmt.Fprintf(w, "one-way edges   %-8d %-11d %d\n", st.OnewayEdges, b.OnewayEdgesBeforeSCC, b.OnewayEdgesBeforeSCC-st.OnewayEdges)
	fmt.Fprintf(w, "drivable km     %-8.3f %-11.3f %.3f\n", st.KM, b.KMBeforeSCC, b.KMBeforeSCC-st.KM)
	fmt.Fprintf(w, "shape points    %d\n", st.ShapePoints)
	kept, district := map[string]float64{}, map[string]float64{}
	for _, e := range g.Edges {
		share := 1.0
		if !e.Oneway {
			share = 0.5
		}
		kept[e.Highway] += share * e.LengthM / 1000
		for j := 1; j < len(e.Geom); j++ {
			if s.District.Contains(e.Geom[j-1]) && s.District.Contains(e.Geom[j]) {
				district[e.Highway] += share * roadgraph.DistanceM(e.Geom[j-1], e.Geom[j]) / 1000
			}
		}
	}
	classTable(w, "km by class, bbox", kept, b.KMBeforeSCCByClass)
	classTable(w, "km by class, district", district, b.DistrictKMBeforeSCCByClass)
	return nil
}

func classTable(w io.Writer, title string, kept, before map[string]float64) {
	fmt.Fprintf(w, "%-25s kept     before SCC  kept %%\n", title)
	var k, bf float64
	for _, c := range slices.Sorted(maps.Keys(before)) {
		kc := round(kept[c], 3) // before is stored to the metre; compare like with like
		k, bf = k+kc, bf+before[c]
		fmt.Fprintf(w, "  %-23s %-8.3f %-11.3f %.1f\n", c, kc, before[c], 100*kc/before[c])
	}
	fmt.Fprintf(w, "  %-23s %-8.3f %-11.3f %.1f\n", "total", k, bf, 100*k/bf)
}
