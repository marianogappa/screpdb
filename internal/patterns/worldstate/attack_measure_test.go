package worldstate_test

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/marianogappa/screpdb/internal/parser"
	"github.com/marianogappa/screpdb/internal/patterns"
	"github.com/marianogappa/screpdb/internal/patterns/worldstate"
)

// TestAttackMeasure is an exploratory measurement harness (issue #186): it
// models attacks as bilateral space-time command clusters and prints their
// centroid, duration, inter-base-axis projection and drift direction, so we
// can compare against a human-curated attack list before rewriting detection.
//
// Run explicitly:
//
//	ATTACK_MEASURE=/abs/path/to.rep go test ./internal/patterns/worldstate/ -run TestAttackMeasure -v
const (
	clusterRadiusPx = 420 // commands within this of a live cluster join it
	clusterGapSec   = 16  // a cluster stays live this long after its last cmd
	bilateralMinCmd = 5   // each side needs >= this many cmds (FP-min, was 3)
	emitFloorSec    = 12  // bilateral cluster must last >= this to emit
	atBasePx        = 640 // centroid within this of a base centroid => "at" it
)

type measureCmd struct {
	sec  int
	pid  byte
	x, y float64
}

type measureCluster struct {
	startSec, lastSec int
	cntByPID          map[byte]int
	pts               []measureCmd
	sumX, sumY        float64
	n                 int
}

func (c *measureCluster) cx() float64 { return c.sumX / float64(c.n) }
func (c *measureCluster) cy() float64 { return c.sumY / float64(c.n) }

func TestAttackMeasure(t *testing.T) {
	repPath := os.Getenv("ATTACK_MEASURE")
	if repPath == "" {
		t.Skip("set ATTACK_MEASURE=/path/to.rep to run the attack measurement harness")
	}

	info, err := os.Stat(repPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	replay := parser.CreateReplayFromFileInfo(repPath, filepath.Base(repPath), info.Size(), "")
	data, err := parser.ParseReplay(repPath, replay)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	orch, ok := data.PatternOrchestrator.(*patterns.Orchestrator)
	if !ok {
		t.Fatalf("no orchestrator")
	}
	eng := orch.WorldStateEngine()
	if eng == nil {
		t.Fatalf("no worldstate engine")
	}

	bases, startByPID, naturalByPID, _ := eng.DebugSnapshot()
	// Invert: base index -> owning pid, for main / natural identity labels.
	mainPidByBase := map[int]byte{}
	for pid, bi := range startByPID {
		mainPidByBase[bi] = pid
	}
	natPidByBase := map[int]byte{}
	for pid, bi := range naturalByPID {
		natPidByBase[bi] = pid
	}

	// Two human players + their colors, from the parsed replay.
	type pinfo struct {
		pid    byte
		name   string
		color  string
		sx, sy float64
	}
	var players []pinfo
	for _, p := range replay.Players {
		if p.IsObserver || p.Type != "Human" {
			continue
		}
		si, ok := startByPID[p.PlayerID]
		if !ok {
			continue
		}
		players = append(players, pinfo{p.PlayerID, p.Name, p.Color, bases[si].CenterX, bases[si].CenterY})
	}
	if len(players) != 2 {
		t.Fatalf("expected exactly 2 human players, got %d", len(players))
	}
	A, B := players[0], players[1] // axis A -> B
	colorByPID := map[byte]string{A.pid: A.color, B.pid: B.color}
	fmt.Printf("\n=== %s ===\n", filepath.Base(repPath))
	fmt.Printf("AXIS  A=%s(%s @%.0f,%.0f)  ->  B=%s(%s @%.0f,%.0f)\n",
		A.color, A.name, A.sx, A.sy, B.color, B.name, B.sx, B.sy)

	abx, aby := B.sx-A.sx, B.sy-A.sy
	ab2 := abx*abx + aby*aby
	proj := func(x, y float64) float64 {
		if ab2 == 0 {
			return 0.5
		}
		return ((x-A.sx)*abx + (y-A.sy)*aby) / ab2
	}
	band := func(t float64) string {
		switch {
		case t < 0.15:
			return A.color + "'s main"
		case t < 0.32:
			return "toward " + A.color + "'s natural"
		case t < 0.45:
			return A.color + "-side middle"
		case t <= 0.55:
			return "the middle"
		case t < 0.68:
			return B.color + "-side middle"
		case t < 0.85:
			return "toward " + B.color + "'s natural"
		default:
			return B.color + "'s main"
		}
	}

	// Aggressive commands only, in time order.
	var cmds []measureCmd
	for _, ec := range eng.EnrichedStream() {
		if ec.X == nil || ec.Y == nil {
			continue
		}
		if !worldstate.AttackOpeningPressure(ec) {
			continue
		}
		if ec.PlayerID != int64(A.pid) && ec.PlayerID != int64(B.pid) {
			continue
		}
		cmds = append(cmds, measureCmd{ec.Second, byte(ec.PlayerID), float64(*ec.X), float64(*ec.Y)})
	}
	sort.SliceStable(cmds, func(i, j int) bool { return cmds[i].sec < cmds[j].sec })

	// Sweep-line space-time clustering.
	var live, done []*measureCluster
	flush := func(nowSec int) {
		var keep []*measureCluster
		for _, c := range live {
			if nowSec-c.lastSec > clusterGapSec {
				done = append(done, c)
			} else {
				keep = append(keep, c)
			}
		}
		live = keep
	}
	for _, cm := range cmds {
		flush(cm.sec)
		var best *measureCluster
		bestD := math.MaxFloat64
		for _, c := range live {
			d := math.Hypot(c.cx()-cm.x, c.cy()-cm.y)
			if d <= clusterRadiusPx && d < bestD {
				best, bestD = c, d
			}
		}
		if best == nil {
			best = &measureCluster{startSec: cm.sec, cntByPID: map[byte]int{}}
			live = append(live, best)
		}
		best.lastSec = cm.sec
		best.cntByPID[cm.pid]++
		best.pts = append(best.pts, cm)
		best.sumX += cm.x
		best.sumY += cm.y
		best.n++
	}
	done = append(done, live...)

	// Post-merge: a single battle fragments into parallel clusters when
	// commands stray past the join radius. Iteratively fuse clusters whose
	// time ranges overlap (or nearly) AND whose centroids sit within radius.
	overlaps := func(a, b *measureCluster) bool {
		if a.startSec > b.lastSec+clusterGapSec || b.startSec > a.lastSec+clusterGapSec {
			return false
		}
		return math.Hypot(a.cx()-b.cx(), a.cy()-b.cy()) <= clusterRadiusPx
	}
	for {
		merged := false
		for i := 0; i < len(done); i++ {
			for j := i + 1; j < len(done); j++ {
				if !overlaps(done[i], done[j]) {
					continue
				}
				a, b := done[i], done[j]
				if b.startSec < a.startSec {
					a.startSec = b.startSec
				}
				if b.lastSec > a.lastSec {
					a.lastSec = b.lastSec
				}
				for pid, c := range b.cntByPID {
					a.cntByPID[pid] += c
				}
				a.pts = append(a.pts, b.pts...)
				a.sumX += b.sumX
				a.sumY += b.sumY
				a.n += b.n
				done = append(done[:j], done[j+1:]...)
				merged = true
				j--
			}
		}
		if !merged {
			break
		}
	}
	for _, c := range done {
		sort.SliceStable(c.pts, func(i, j int) bool { return c.pts[i].sec < c.pts[j].sec })
	}
	sort.SliceStable(done, func(i, j int) bool { return done[i].startSec < done[j].startSec })

	mmss := func(s int) string { return fmt.Sprintf("%d:%02d", s/60, s%60) }
	// Direction: mean projection of first third vs last third of points.
	thirdProj := func(c *measureCluster) (float64, float64) {
		k := len(c.pts) / 3
		if k < 1 {
			k = 1
		}
		var e0, eN float64
		for _, p := range c.pts[:k] {
			e0 += proj(p.x, p.y)
		}
		for _, p := range c.pts[len(c.pts)-k:] {
			eN += proj(p.x, p.y)
		}
		return e0 / float64(k), eN / float64(k)
	}
	// locate names the fight by the base its centroid sits in (real kind +
	// owner from the snapshot), falling back to inter-base-axis prose when the
	// centroid is in open field. NOTE: production must attribute via
	// point-in-polygon (the detector already has pointInPolyGeom); this
	// black-box prototype uses nearest-centroid, so it cannot disambiguate two
	// bases at the same o'clock (e.g. main vs adjacent expansion) — issue #186.
	locate := func(x, y, t float64) string {
		bi, bd := -1, math.MaxFloat64
		for i, b := range bases {
			d := math.Hypot(b.CenterX-x, b.CenterY-y)
			if d < bd {
				bi, bd = i, d
			}
		}
		if bi >= 0 && bd <= atBasePx {
			if pid, ok := mainPidByBase[bi]; ok {
				return fmt.Sprintf("%s's main (%doc)", colorByPID[pid], bases[bi].Clock)
			}
			if pid, ok := natPidByBase[bi]; ok {
				return fmt.Sprintf("%s's natural (%doc)", colorByPID[pid], bases[bi].Clock)
			}
			return fmt.Sprintf("the %doc expansion", bases[bi].Clock)
		}
		return band(t) // open field
	}

	fmt.Printf("\n(bilateral fights only; each side >= %d cmds, lasting >= %ds)\n", bilateralMinCmd, emitFloorSec)
	fmt.Printf("%-13s %4s %-7s %-24s %s\n", "time", "dur", "A/B", "location", "drift")
	fmt.Println("------------------------------------------------------------------------------------------")
	for _, c := range done {
		dur := c.lastSec - c.startSec
		a, b := c.cntByPID[A.pid], c.cntByPID[B.pid]
		bilat := a >= bilateralMinCmd && b >= bilateralMinCmd
		if !bilat || dur < emitFloorSec {
			continue // FP-min: drop one-sided, pokes, short skirmishes
		}
		t := proj(c.cx(), c.cy())
		e0, eN := thirdProj(c)
		drift := ""
		if d := eN - e0; math.Abs(d) >= 0.08 {
			if d > 0 {
				drift = "drifts → " + B.color
			} else {
				drift = "drifts → " + A.color
			}
		}
		fmt.Printf("%-13s %3ds  %d/%d  %-24s %-16s\n",
			fmt.Sprintf("%s-%s", mmss(c.startSec), mmss(c.lastSec)),
			dur, a, b,
			locate(c.cx(), c.cy(), t), drift)
	}
	fmt.Println()
}
