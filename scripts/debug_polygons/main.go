// debug_polygons renders a single replay's scmapanalyzer base polygons to an
// SVG, with optional points (e.g. a Nexus build location) annotated. Used to
// sanity-check whether a build position falls inside a player's start polygon
// or a separate "natural" polygon.
//
// Usage:
//
//	go run ./scripts/debug_polygons /path/to/replay.rep tile:77,91 tile:55,86 > /tmp/bgh.svg
//
// Each "tile:X,Y" arg renders a labeled red dot at that map-tile position.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/marianogappa/scmapanalyzer/lib/scmapanalyzer"
	"github.com/marianogappa/scmapanalyzer/replaymap"
)

const (
	pxPerTile     = 32
	pxPerMinitile = 8
	svgScale      = 0.25 // 4096px → 1024px output
)

type point struct {
	X, Y float64
	Tag  string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: debug-polygons <replay-path> [tile:X,Y ...] [pixel:X,Y ...]")
		os.Exit(1)
	}
	replayPath := os.Args[1]

	var pts []point
	for _, arg := range os.Args[2:] {
		p, err := parsePoint(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bad point %q: %v\n", arg, err)
			os.Exit(1)
		}
		pts = append(pts, p)
	}

	client, err := scmapanalyzer.NewClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "scmapanalyzer client: %v\n", err)
		os.Exit(1)
	}
	result, err := client.Analyze(replayPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "analyze: %v\n", err)
		os.Exit(1)
	}

	renderSVG(os.Stdout, result, pts)
}

func parsePoint(arg string) (point, error) {
	parts := strings.SplitN(arg, ":", 2)
	if len(parts) != 2 {
		return point{}, fmt.Errorf("expected prefix:X,Y")
	}
	xy := strings.SplitN(parts[1], ",", 2)
	if len(xy) != 2 {
		return point{}, fmt.Errorf("expected X,Y")
	}
	x, err := strconv.ParseFloat(strings.TrimSpace(xy[0]), 64)
	if err != nil {
		return point{}, err
	}
	y, err := strconv.ParseFloat(strings.TrimSpace(xy[1]), 64)
	if err != nil {
		return point{}, err
	}
	switch parts[0] {
	case "tile":
		return point{X: x*pxPerTile + pxPerTile/2, Y: y*pxPerTile + pxPerTile/2, Tag: arg}, nil
	case "pixel":
		return point{X: x, Y: y, Tag: arg}, nil
	case "minitile":
		return point{X: x * pxPerMinitile, Y: y * pxPerMinitile, Tag: arg}, nil
	}
	return point{}, fmt.Errorf("unknown prefix %q (use tile:/pixel:/minitile:)", parts[0])
}

func renderSVG(w *os.File, result *replaymap.Result, pts []point) {
	// Map size from polygon extent (BGH = 4096x4096 px).
	maxPx := 0
	for _, b := range append(append([]replaymap.BasePolygon{}, result.Starts...), result.Expas...) {
		for _, v := range b.PolygonVertices {
			if v.X*pxPerMinitile > maxPx {
				maxPx = v.X * pxPerMinitile
			}
			if v.Y*pxPerMinitile > maxPx {
				maxPx = v.Y * pxPerMinitile
			}
		}
	}
	side := 4096
	if maxPx > side {
		side = maxPx
	}
	out := side * 1
	scale := svgScale

	fmt.Fprintf(w, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" style="background:#1b1b1b;font-family:monospace">`,
		out, out, int(float64(out)*scale), int(float64(out)*scale))

	// Tile grid (every 16 tiles).
	for i := 0; i <= side; i += 16 * pxPerTile {
		fmt.Fprintf(w, `<line x1="%d" y1="0" x2="%d" y2="%d" stroke="#333" stroke-width="2"/>`, i, i, side)
		fmt.Fprintf(w, `<line x1="0" y1="%d" x2="%d" y2="%d" stroke="#333" stroke-width="2"/>`, i, side, i)
	}

	drawBase := func(b replaymap.BasePolygon, fill, stroke string) {
		var pts []string
		for _, v := range b.PolygonVertices {
			pts = append(pts, fmt.Sprintf("%d,%d", v.X*pxPerMinitile, v.Y*pxPerMinitile))
		}
		fmt.Fprintf(w, `<polygon points="%s" fill="%s" fill-opacity="0.25" stroke="%s" stroke-width="6"/>`,
			strings.Join(pts, " "), fill, stroke)
		cx, cy := b.CenterTile.X*pxPerMinitile, b.CenterTile.Y*pxPerMinitile
		fmt.Fprintf(w, `<circle cx="%d" cy="%d" r="14" fill="%s"/>`, cx, cy, stroke)
		label := fmt.Sprintf("%s (clock %d, %s)", b.Name, b.Clock, b.Kind)
		fmt.Fprintf(w, `<text x="%d" y="%d" font-size="64" fill="white" stroke="black" stroke-width="2" paint-order="stroke">%s</text>`,
			cx+20, cy-20, label)
	}
	for _, b := range result.Starts {
		drawBase(b, "#ff7f50", "#ff3d00")
	}
	for _, b := range result.Expas {
		drawBase(b, "#5fa8ff", "#1565c0")
	}

	for _, p := range pts {
		fmt.Fprintf(w, `<circle cx="%f" cy="%f" r="40" fill="yellow" stroke="black" stroke-width="6"/>`, p.X, p.Y)
		fmt.Fprintf(w, `<text x="%f" y="%f" font-size="72" fill="yellow" stroke="black" stroke-width="3" paint-order="stroke">%s</text>`,
			p.X+50, p.Y+30, p.Tag)
	}

	fmt.Fprintln(w, "</svg>")

	// Also emit a stderr summary of which base each point lands in.
	for _, p := range pts {
		hit := classifyPoint(result, p.X, p.Y)
		fmt.Fprintf(os.Stderr, "%s pixel=(%.0f,%.0f) → %s\n", p.Tag, p.X, p.Y, hit)
	}
}

func classifyPoint(result *replaymap.Result, x, y float64) string {
	all := append(append([]replaymap.BasePolygon{}, result.Starts...), result.Expas...)
	var hits []string
	for _, b := range all {
		if pointInPoly(b.PolygonVertices, x, y) {
			hits = append(hits, fmt.Sprintf("%s(clock=%d,%s)", b.Name, b.Clock, b.Kind))
		}
	}
	if len(hits) == 0 {
		return "OUTSIDE all polygons"
	}
	return strings.Join(hits, " AND ")
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
