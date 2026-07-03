package worldstate

import (
	"math"
	"testing"
)

func approxEq(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestDist_KnownTriangle(t *testing.T) {
	if got := dist(0, 0, 3, 4); !approxEq(got, 5) {
		t.Fatalf("dist(0,0,3,4)=%v want 5", got)
	}
	if got := dist(1, 1, 1, 1); got != 0 {
		t.Fatalf("dist to self=%v want 0", got)
	}
}

func TestNearestBase(t *testing.T) {
	bases := []base{
		{CenterX: 0, CenterY: 0},
		{CenterX: 100, CenterY: 100},
		{CenterX: 10, CenterY: 0},
	}
	if got := nearestBase(9, 0, bases); got != 2 {
		t.Fatalf("nearestBase near (10,0) got %d want 2", got)
	}
	if got := nearestBase(-1, -1, bases); got != 0 {
		t.Fatalf("nearestBase near origin got %d want 0", got)
	}
	if got := nearestBase(1, 1, nil); got != -1 {
		t.Fatalf("nearestBase on empty got %d want -1", got)
	}
}

func TestCentroid(t *testing.T) {
	pts := []point{{X: 0, Y: 0}, {X: 4, Y: 0}, {X: 4, Y: 4}, {X: 0, Y: 4}}
	c := centroid(pts, []int{0, 1, 2, 3})
	if !approxEq(c[0], 2) || !approxEq(c[1], 2) {
		t.Fatalf("centroid=%v want [2,2]", c)
	}
	c2 := centroid(pts, []int{1})
	if !approxEq(c2[0], 4) || !approxEq(c2[1], 0) {
		t.Fatalf("single-member centroid=%v want [4,0]", c2)
	}
}

func TestAverageDistanceToPoint(t *testing.T) {
	pts := []point{{X: 0, Y: 0}, {X: 6, Y: 0}}
	got := averageDistanceToPoint(pts, []int{0, 1}, 3, 0)
	if !approxEq(got, 3) {
		t.Fatalf("avg dist=%v want 3", got)
	}
	if got := averageDistanceToPoint(pts, nil, 0, 0); got != math.MaxFloat64 {
		t.Fatalf("empty members avg=%v want MaxFloat64", got)
	}
}

func TestClusterSizes(t *testing.T) {
	sizes := clusterSizes([]int{0, 0, 1, 2, 2, 2}, 3)
	want := []int{2, 1, 3}
	if len(sizes) != 3 {
		t.Fatalf("len=%d want 3", len(sizes))
	}
	for i := range want {
		if sizes[i] != want[i] {
			t.Fatalf("sizes=%v want %v", sizes, want)
		}
	}
}

func TestMaxInt(t *testing.T) {
	if maxInt(3, 7) != 7 || maxInt(7, 3) != 7 || maxInt(5, 5) != 5 {
		t.Fatal("maxInt wrong")
	}
}

func TestPercentile(t *testing.T) {
	vals := []float64{10, 20, 30, 40}
	if got := percentile(vals, 0); !approxEq(got, 10) {
		t.Fatalf("p0=%v want 10", got)
	}
	if got := percentile(vals, 1); !approxEq(got, 40) {
		t.Fatalf("p100=%v want 40", got)
	}
	// pos = 0.5*3 = 1.5 -> interp between 20 and 30 => 25
	if got := percentile(vals, 0.5); !approxEq(got, 25) {
		t.Fatalf("p50=%v want 25", got)
	}
	if got := percentile(nil, 0.5); got != 0 {
		t.Fatalf("empty percentile=%v want 0", got)
	}
	// exact index (no interpolation branch): pos=1.0 -> x[1]=20
	if got := percentile(vals, 1.0/3.0); !approxEq(got, 20) {
		t.Fatalf("p=1/3=%v want 20", got)
	}
}

func TestKthNeighborDistances(t *testing.T) {
	// Collinear points at 0,1,3.
	pts := []point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 3, Y: 0}}
	// k=1 (nearest neighbor): p0->1, p1->1 (to p0), p2->2 (to p1).
	got := kthNeighborDistances(pts, 1)
	want := []float64{1, 1, 2}
	for i := range want {
		if !approxEq(got[i], want[i]) {
			t.Fatalf("k=1 got %v want %v", got, want)
		}
	}
	// k clamped up to n-1 = 2 (farthest). p0->3, p1->2, p2->3.
	got2 := kthNeighborDistances(pts, 99)
	want2 := []float64{3, 2, 3}
	for i := range want2 {
		if !approxEq(got2[i], want2[i]) {
			t.Fatalf("k>=n got %v want %v", got2, want2)
		}
	}
	// k<1 clamps to 1.
	got3 := kthNeighborDistances(pts, 0)
	for i := range want {
		if !approxEq(got3[i], want[i]) {
			t.Fatalf("k<1 got %v want %v", got3, want)
		}
	}
	if len(kthNeighborDistances(nil, 1)) != 0 {
		t.Fatal("empty should give empty")
	}
}

func TestPrimMST_LineGraph(t *testing.T) {
	pts := []point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 2, Y: 0}, {X: 3, Y: 0}}
	edges := primMST(pts)
	if len(edges) != len(pts)-1 {
		t.Fatalf("edges=%d want %d", len(edges), len(pts)-1)
	}
	total := 0.0
	for _, e := range edges {
		total += e.W
	}
	// MST of a unit-spaced line is total length 3.
	if !approxEq(total, 3) {
		t.Fatalf("MST weight=%v want 3", total)
	}
	if len(primMST([]point{{X: 0, Y: 0}})) != 0 {
		t.Fatal("single point MST must be empty")
	}
	if len(primMST(nil)) != 0 {
		t.Fatal("empty MST must be empty")
	}
}

func TestUnionFind(t *testing.T) {
	uf := newUnionFind(5)
	for i := 0; i < 5; i++ {
		if uf.find(i) != i {
			t.Fatalf("initial find(%d)!=%d", i, i)
		}
	}
	uf.union(0, 1)
	uf.union(1, 2)
	uf.union(3, 4)
	if uf.find(0) != uf.find(2) {
		t.Fatal("0 and 2 should share a root")
	}
	if uf.find(0) == uf.find(3) {
		t.Fatal("0 and 3 should be in different sets")
	}
	// union of already-joined is a no-op.
	before := uf.find(0)
	uf.union(0, 2)
	if uf.find(0) != before {
		t.Fatal("redundant union changed root")
	}
	// Merge the two components; all five now share a root.
	uf.union(2, 4)
	root := uf.find(0)
	for i := 1; i < 5; i++ {
		if uf.find(i) != root {
			t.Fatalf("element %d not merged into common root", i)
		}
	}
}

func TestSilhouetteScore_WellSeparatedIsHigh(t *testing.T) {
	// Two tight clusters far apart -> silhouette close to 1.
	pts := []point{
		{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: 1},
		{X: 100, Y: 100}, {X: 101, Y: 100}, {X: 100, Y: 101},
	}
	labels := []int{0, 0, 0, 1, 1, 1}
	s := silhouetteScore(pts, labels, 2)
	if s < 0.9 {
		t.Fatalf("well-separated silhouette=%v want >=0.9", s)
	}
	if silhouetteScore(nil, nil, 0) != 0 {
		t.Fatal("empty silhouette must be 0")
	}
}

func TestSilhouetteScore_BadClusteringIsLow(t *testing.T) {
	// Same physical clusters but labels interleaved -> low/negative silhouette.
	pts := []point{
		{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: 1},
		{X: 100, Y: 100}, {X: 101, Y: 100}, {X: 100, Y: 101},
	}
	good := silhouetteScore(pts, []int{0, 0, 0, 1, 1, 1}, 2)
	bad := silhouetteScore(pts, []int{0, 1, 0, 1, 0, 1}, 2)
	if bad >= good {
		t.Fatalf("interleaved labels silhouette=%v should be < good=%v", bad, good)
	}
}

func TestLabelsFromMSTCuts_Degenerate(t *testing.T) {
	if labels, k := labelsFromMSTCuts(nil, 3, 1.9, 2.3); k != 0 || len(labels) != 0 {
		t.Fatalf("empty got k=%d labels=%v", k, labels)
	}
	labels, k := labelsFromMSTCuts([]point{{X: 5, Y: 5}}, 3, 1.9, 2.3)
	if k != 1 || len(labels) != 1 || labels[0] != 0 {
		t.Fatalf("single point got k=%d labels=%v", k, labels)
	}
}

func clusterBlob(cx, cy float64, n int) []point {
	out := make([]point, 0, n)
	for i := 0; i < n; i++ {
		dx := float64(i%3) * 2
		dy := float64(i/3) * 2
		out = append(out, point{X: cx + dx, Y: cy + dy})
	}
	return out
}

func TestLabelsFromMSTCuts_SeparatesFarBlobs(t *testing.T) {
	var pts []point
	pts = append(pts, clusterBlob(0, 0, 8)...)
	pts = append(pts, clusterBlob(1000, 0, 8)...)
	pts = append(pts, clusterBlob(0, 1000, 8)...)
	labels, k := labelsFromMSTCuts(pts, 3, 1.9, 2.3)
	if k < 2 {
		t.Fatalf("expected the far blobs to split into >=2 clusters, got k=%d", k)
	}
	if len(labels) != len(pts) {
		t.Fatalf("labels len=%d want %d", len(labels), len(pts))
	}
	// Points inside a single blob should land in the same label.
	if labels[0] != labels[1] {
		t.Fatalf("points from the same blob got different labels: %d vs %d", labels[0], labels[1])
	}
}

func TestChooseMSTLabels_ReturnsClustering(t *testing.T) {
	var pts []point
	// Four well-separated blobs of a healthy size so the k>=4 gate can pass.
	for _, c := range [][2]float64{{0, 0}, {2000, 0}, {0, 2000}, {2000, 2000}} {
		pts = append(pts, clusterBlob(c[0], c[1], 10)...)
	}
	alpha, beta, k, sil, labels := chooseMSTLabels(pts)
	if len(labels) != len(pts) {
		t.Fatalf("labels len=%d want %d", len(labels), len(pts))
	}
	if k < 1 {
		t.Fatalf("k=%d want >=1", k)
	}
	if alpha <= 0 || beta <= 0 {
		t.Fatalf("alpha/beta should be positive, got %v/%v", alpha, beta)
	}
	if sil > 1.0001 || sil < -1.0001 {
		t.Fatalf("silhouette out of range: %v", sil)
	}
}

func TestMakeBases_DropsSmallClustersAndComputesRadius(t *testing.T) {
	pts := []point{
		// cluster 0: 4 points spanning radius from centroid.
		{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}, {X: 0, Y: 10},
		// cluster 1: only 2 points -> dropped (<4).
		{X: 500, Y: 500}, {X: 501, Y: 500},
	}
	labels := []int{0, 0, 0, 0, 1, 1}
	bases := makeBases(pts, labels)
	if len(bases) != 1 {
		t.Fatalf("expected 1 base (small cluster dropped), got %d", len(bases))
	}
	b := bases[0]
	if !approxEq(b.CenterX, 5) || !approxEq(b.CenterY, 5) {
		t.Fatalf("center=(%v,%v) want (5,5)", b.CenterX, b.CenterY)
	}
	// Natural radius = max distance from centroid to a member = dist((5,5),(0,0)).
	if !approxEq(b.NaturalRadius, math.Sqrt(50)) {
		t.Fatalf("naturalRadius=%v want %v", b.NaturalRadius, math.Sqrt(50))
	}
}

func TestMakeBases_IgnoresNegativeLabels(t *testing.T) {
	pts := []point{
		{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: 1}, {X: 1, Y: 1},
		{X: 9, Y: 9},
	}
	labels := []int{0, 0, 0, 0, -1}
	bases := makeBases(pts, labels)
	if len(bases) != 1 {
		t.Fatalf("expected 1 base, got %d", len(bases))
	}
}

func TestAssignPerBaseRadii_CappedByHalfNeighborDistance(t *testing.T) {
	bases := []base{
		{CenterX: 0, CenterY: 0, NaturalRadius: 1000},
		{CenterX: 100, CenterY: 0, NaturalRadius: 1000},
	}
	assignPerBaseRadii(bases, 1.0)
	// Half-distance is 50; NaturalRadius huge -> capped at 50.
	if !approxEq(bases[0].GeoRadius, 50) || !approxEq(bases[1].GeoRadius, 50) {
		t.Fatalf("radii=%v,%v want 50,50", bases[0].GeoRadius, bases[1].GeoRadius)
	}
}

func TestAssignPerBaseRadii_NaturalRadiusUsedWhenSmall(t *testing.T) {
	bases := []base{
		{CenterX: 0, CenterY: 0, NaturalRadius: 10},
		{CenterX: 100, CenterY: 0, NaturalRadius: 10},
	}
	assignPerBaseRadii(bases, 1.0)
	if !approxEq(bases[0].GeoRadius, 10) {
		t.Fatalf("small natural radius should be used: got %v want 10", bases[0].GeoRadius)
	}
}

func TestAssignPerBaseRadii_SingleBaseUsesNaturalRadius(t *testing.T) {
	bases := []base{{CenterX: 0, CenterY: 0, NaturalRadius: 33}}
	assignPerBaseRadii(bases, 1.0)
	if !approxEq(bases[0].GeoRadius, 33) {
		t.Fatalf("single base radius=%v want 33", bases[0].GeoRadius)
	}
}

func TestEnlargeStartBaseRadii_GrowsStartBase(t *testing.T) {
	bases := []base{
		{CenterX: 0, CenterY: 0, GeoRadius: 10, StartCount: 1},
		{CenterX: 1000, CenterY: 0, GeoRadius: 10},
	}
	before := bases[0].GeoRadius
	enlargeStartBaseRadii(bases, 1.0)
	if bases[0].GeoRadius <= before {
		t.Fatalf("start base radius should grow: before=%v after=%v", before, bases[0].GeoRadius)
	}
	// Must not overlap the neighbor: r0 + r1 <= distance (safety 1.0).
	if bases[0].GeoRadius+bases[1].GeoRadius > 1000+1e-6 {
		t.Fatalf("start base grew past neighbor bound: %v + %v > 1000", bases[0].GeoRadius, bases[1].GeoRadius)
	}
}

func TestEnlargeStartBaseRadii_NoStartBasesIsNoop(t *testing.T) {
	bases := []base{
		{CenterX: 0, CenterY: 0, GeoRadius: 10},
		{CenterX: 1000, CenterY: 0, GeoRadius: 10},
	}
	enlargeStartBaseRadii(bases, 1.0)
	if bases[0].GeoRadius != 10 || bases[1].GeoRadius != 10 {
		t.Fatalf("radii changed with no start bases: %v %v", bases[0].GeoRadius, bases[1].GeoRadius)
	}
}
