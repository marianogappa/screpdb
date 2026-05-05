//go:build ignore

// dump_starts prints the replay's StartLocations and which scmapanalyzer
// polygon contains each one. Used to confirm whether the engine's
// startBaseByPID assignment lands in the player's "main" polygon or
// somewhere unexpected.
package main

import (
	"fmt"
	"os"

	"github.com/icza/screp/repparser"
	"github.com/marianogappa/scmapanalyzer/lib/scmapanalyzer"
	"github.com/marianogappa/scmapanalyzer/replaymap"
)

const pxPerMinitile = 8

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: dump_starts <replay-path>")
		os.Exit(1)
	}
	path := os.Args[1]

	rep, err := repparser.ParseFileConfig(path, repparser.Config{Commands: false, MapData: true})
	if err == nil {
		rep.Compute()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse: %v\n", err)
		os.Exit(1)
	}

	client, err := scmapanalyzer.NewClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "client: %v\n", err)
		os.Exit(1)
	}
	result, err := client.Analyze(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "analyze: %v\n", err)
		os.Exit(1)
	}

	all := append(append([]replaymap.BasePolygon{}, result.Starts...), result.Expas...)

	fmt.Printf("Map: %s\n", rep.Header.Map)
	fmt.Printf("Map size: %dx%d tiles (%dx%d px)\n",
		rep.Header.MapWidth, rep.Header.MapHeight,
		int(rep.Header.MapWidth)*32, int(rep.Header.MapHeight)*32)
	fmt.Println()

	fmt.Println("Header.Players (slot → name):")
	for _, p := range rep.Header.Players {
		if p == nil {
			continue
		}
		fmt.Printf("  slot=%d id=%d name=%s race=%s\n", p.SlotID, p.ID, p.Name, p.Race.String())
	}
	fmt.Println()

	if rep.Computed != nil {
		fmt.Println("Computed.PlayerDescs[i].StartLocation:")
		for i, pd := range rep.Computed.PlayerDescs {
			if pd == nil || pd.StartLocation == nil {
				continue
			}
			x, y := int(pd.StartLocation.X), int(pd.StartLocation.Y)
			poly := classify(all, float64(x), float64(y))
			tx, ty := x/32, y/32
			fmt.Printf("  i=%d pid=? pixel=(%d,%d) tile=(%d,%d) → %s\n", i, x, y, tx, ty, poly)
		}
		fmt.Println()
	}

	if rep.MapData != nil {
		fmt.Println("MapData.StartLocations:")
		for _, sl := range rep.MapData.StartLocations {
			x, y := int(sl.X), int(sl.Y)
			poly := classify(all, float64(x), float64(y))
			tx, ty := x/32, y/32
			fmt.Printf("  slot=%d pixel=(%d,%d) tile=(%d,%d) → %s\n", sl.SlotID, x, y, tx, ty, poly)
		}
	}
}

func classify(bases []replaymap.BasePolygon, x, y float64) string {
	var hits []string
	for _, b := range bases {
		if pointInPoly(b.PolygonVertices, x, y) {
			hits = append(hits, fmt.Sprintf("%s (clock=%d %s)", b.Name, b.Clock, b.Kind))
		}
	}
	if len(hits) == 0 {
		return "OUTSIDE all polygons"
	}
	out := ""
	for i, h := range hits {
		if i > 0 {
			out += " AND "
		}
		out += h
	}
	return out
}

func pointInPoly(verts []replaymap.TilePoint, x, y float64) bool {
	inside := false
	n := len(verts)
	if n < 3 {
		return false
	}
	j := n - 1
	for i := 0; i < n; i++ {
		xi := float64(verts[i].X * pxPerMinitile)
		yi := float64(verts[i].Y * pxPerMinitile)
		xj := float64(verts[j].X * pxPerMinitile)
		yj := float64(verts[j].Y * pxPerMinitile)
		if (yi > y) != (yj > y) {
			t := (x-xi)*(yj-yi) - (y-yi)*(xj-xi)
			if (yj > yi) == (t > 0) {
				inside = !inside
			}
		}
		j = i
	}
	return inside
}
