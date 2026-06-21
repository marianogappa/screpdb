package unittags

import (
	"math"
	"sort"

	"github.com/icza/screp/rep"
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// Mutalisk hit-and-run harass detection (issue #194).
//
// BW replays carry no unit-tag → combat-unit-type mapping (tags appear only in
// Select/Hotkey commands; Train/Morph names the produced unit but never its
// resulting tag), so a hotkey group can never be *proven* to hold Mutalisks.
// Detection is therefore behavioral + production-corroborated.
//
// What we detect is hit-n-run specifically (NOT "muta attacks"): the player
// micros a single hotkeyed flock in a tight, fast, OSCILLATING rhythm — dart in
// to shoot, pull back, dart in again — repeated many times. Measured against
// human-labeled replays, the discriminators that separate real hit-n-run from
// a-move attacks / macro are:
//
//   - One STABLE hotkey group drives a sustained burst of spatial commands
//     (right-click / move / follow / attack), continuously selected. An all-in
//     attack cycles several groups (1 a-move each) into the same point — that
//     never forms a single-group burst.
//   - High command density (~2-4 cmds/sec) sustained for seconds.
//   - OSCILLATION: successive repositions reverse direction (dart in ↔ pull
//     back), so consecutive significant-move vectors point opposite ways. An
//     attack marches monotonically toward the target; an engage drifts one way.
//
// Game-level gates keep this to plausible muta games: Zerg, a Spire built, real
// Mutalisk production, and only activity after the first Mutalisk popped.
const (
	// mutaMinGroupSize: a flock is ≥3 units. Early harass runs as few as 3-4
	// mutas, so the floor is low; single-unit selections (scouts, taps) are out.
	mutaMinGroupSize = 3
	// mutaMaxGroupSize caps the ball — a huge selection is a main-army a-move.
	mutaMaxGroupSize = 16
	// mutaMinMutaMorphs: minimum lifetime Mutalisk count to treat the player as
	// a muta player at all (morphs spawn pairs, so 6 ≈ 3 morphs).
	mutaMinMutaMorphs = 6

	// episodeGapSec: spatial commands on the same hotkey group more than this
	// many seconds apart belong to different control episodes.
	episodeGapSec = 3.0
	// moveEpsPx: repositions shorter than this are jitter (re-clicking the same
	// spot) and are ignored when measuring oscillation.
	moveEpsPx = 50.0
	// mergeGapSec: qualifying episodes within this gap are one harass window.
	mergeGapSec = 12.0

	// Hit-n-run thresholds (tuned on labeled true/false windows).
	hnrMinCmds     = 8    // sustained burst, not a few orders
	hnrMinDurSec   = 2.5  // lasts at least a couple of seconds
	hnrMinDensity  = 1.3  // commands per second
	hnrMinSigMoves = 5    // enough real repositions to show a rhythm
	hnrMinReversal = 0.34 // fraction of consecutive move-vectors that reverse

	// A merged harass window must be a sustained campaign: enough total volleys
	// over enough time. Isolated tight pokes (a single muta engage) fall short.
	// (Spatial compactness was tried as a discriminator and rejected — a tight
	// muta attack is indistinguishable from hit-n-run by cloud radius, and on
	// some games it is anti-correlated. Conservative surfacing leans on the
	// per-game-player confidence bar instead; see worldstate's accessor.)
	minWindowReversals = 10
	minWindowDurSec    = 6
)

// HarassPoint is one coordinate waypoint on the reconstructed harass path
// (pixel space), stamped with the second it was issued.
type HarassPoint struct {
	Sec int `json:"s"`
	X   int `json:"x"`
	Y   int `json:"y"`
}

// MutaHarassEpisode is one detected hit-and-run harass run by a Zerg player.
// Cycles is the number of dart-in/pull-back reversals observed (≈ volleys).
type MutaHarassEpisode struct {
	PlayerID  byte
	StartSec  int
	EndSec    int
	Cycles    int
	GroupSize int // median selection size across the window
	Path      []HarassPoint
}

// spatialCmd is one repositioning/attack order issued while a hotkey group was
// the active selection. sec is fractional (frame-derived) for cadence math.
type spatialCmd struct {
	sec     float64
	x, y    int
	grp     int
	selSize int
}

type harassAcc struct {
	cur        []uint16
	groups     map[byte][]uint16
	activeGrp  int
	spireBuilt bool
	firstMuta  float64
	hasMuta    bool
	mutaMorphs int
	cmds       []spatialCmd
}

// DetectMutaHarass walks the raw command stream and returns Mutalisk hit-and-run
// harass episodes per Zerg player. Players supplies race per replay PlayerID.
func DetectMutaHarass(r *rep.Replay, players []*models.Player) []MutaHarassEpisode {
	if r == nil || r.Commands == nil {
		return nil
	}
	isZerg := map[byte]bool{}
	for _, p := range players {
		if p != nil && p.Race == "Zerg" {
			isZerg[p.PlayerID] = true
		}
	}

	accs := map[byte]*harassAcc{}
	get := func(pid byte) *harassAcc {
		if accs[pid] == nil {
			accs[pid] = &harassAcc{groups: map[byte][]uint16{}, activeGrp: -1}
		}
		return accs[pid]
	}

	for _, c := range r.Commands.Cmds {
		b := c.BaseCmd()
		if b == nil || b.Type == nil || !isZerg[b.PlayerID] {
			continue
		}
		a := get(b.PlayerID)
		sec := b.Frame.Seconds()

		switch b.Type.ID {
		case repcmd.TypeIDSelect, repcmd.TypeIDSelect121:
			if sc, ok := c.(*repcmd.SelectCmd); ok {
				a.cur = tagsOf(sc.UnitTags)
				a.activeGrp = -1
			}
		case repcmd.TypeIDSelectAdd, repcmd.TypeIDSelectAdd121:
			if sc, ok := c.(*repcmd.SelectCmd); ok {
				a.cur = unionTags(a.cur, tagsOf(sc.UnitTags))
			}
		case repcmd.TypeIDSelectRemove, repcmd.TypeIDSelectRemove121:
			if sc, ok := c.(*repcmd.SelectCmd); ok {
				a.cur = removeTags(a.cur, tagsOf(sc.UnitTags))
			}
		case repcmd.TypeIDHotkey:
			if hc, ok := c.(*repcmd.HotkeyCmd); ok && hc.HotkeyType != nil {
				switch hc.HotkeyType.Name {
				case "Assign":
					a.groups[hc.Group] = append([]uint16(nil), a.cur...)
					// A reassign re-selects the group's units, so it stays active.
					a.activeGrp = int(hc.Group)
				case "Select":
					a.cur = append([]uint16(nil), a.groups[hc.Group]...)
					a.activeGrp = int(hc.Group)
				case "Add":
					a.cur = unionTags(a.cur, a.groups[hc.Group])
				}
			}
		case repcmd.TypeIDBuild:
			if bc, ok := c.(*repcmd.BuildCmd); ok && bc.Unit != nil && bc.Unit.Name == "Spire" {
				a.spireBuilt = true
			}
		case repcmd.TypeIDTrain, repcmd.TypeIDUnitMorph:
			if tc, ok := c.(*repcmd.TrainCmd); ok && tc.Unit != nil && tc.Unit.Name == "Mutalisk" {
				a.mutaMorphs += 2 // a morph spawns a pair
				if !a.hasMuta {
					a.firstMuta = sec
					a.hasMuta = true
				}
			}
		default:
			recordSpatial(a, c, b, sec)
		}
	}

	var out []MutaHarassEpisode
	for pid, a := range accs {
		if !a.spireBuilt || !a.hasMuta || a.mutaMorphs < mutaMinMutaMorphs {
			continue
		}
		out = append(out, episodesFor(pid, a)...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].StartSec != out[j].StartSec {
			return out[i].StartSec < out[j].StartSec
		}
		return out[i].PlayerID < out[j].PlayerID
	})
	return out
}

// recordSpatial captures repositioning/attack orders issued while a hotkey
// group of plausible flock size is selected. Macro orders (rally) and
// non-spatial toggles are excluded.
func recordSpatial(a *harassAcc, c repcmd.Cmd, b *repcmd.Base, sec float64) {
	if a.activeGrp < 0 || len(a.cur) < mutaMinGroupSize || len(a.cur) > mutaMaxGroupSize {
		return
	}
	var x, y int
	switch b.Type.ID {
	case repcmd.TypeIDRightClick, repcmd.TypeIDRightClick121:
		rc, ok := c.(*repcmd.RightClickCmd)
		if !ok {
			return
		}
		x, y = int(rc.Pos.X), int(rc.Pos.Y)
	case repcmd.TypeIDTargetedOrder, repcmd.TypeIDTargetedOrder121:
		to, ok := c.(*repcmd.TargetedOrderCmd)
		if !ok || to.Order == nil {
			return
		}
		switch to.Order.Name {
		case "Move", "Attack Move", "AttackMove", "Patrol", "Follow",
			"Attack1", "Attack2", "Attack Unit", "AttackUnit":
		default:
			return // rally points, stop, casts, etc. are not flock micro
		}
		x, y = int(to.Pos.X), int(to.Pos.Y)
	default:
		return
	}
	a.cmds = append(a.cmds, spatialCmd{sec: sec, x: x, y: y, grp: a.activeGrp, selSize: len(a.cur)})
}

// episodesFor splits one player's flock commands into single-group control
// episodes, scores each for the hit-n-run signature, and merges qualifying
// episodes into harass windows.
func episodesFor(pid byte, a *harassAcc) []MutaHarassEpisode {
	var episodes []MutaHarassEpisode
	i := 0
	for i < len(a.cmds) {
		j := i + 1
		for j < len(a.cmds) &&
			a.cmds[j].grp == a.cmds[i].grp &&
			a.cmds[j].sec-a.cmds[j-1].sec <= episodeGapSec {
			j++
		}
		seg := a.cmds[i:j]
		i = j
		if ep, ok := scoreEpisode(pid, a, seg); ok {
			episodes = append(episodes, ep)
		}
	}
	return mergeEpisodes(episodes)
}

// scoreEpisode measures one single-group command burst and returns a harass
// episode when it matches the hit-n-run signature.
func scoreEpisode(pid byte, a *harassAcc, seg []spatialCmd) (MutaHarassEpisode, bool) {
	if len(seg) < hnrMinCmds || seg[0].sec < a.firstMuta {
		return MutaHarassEpisode{}, false
	}
	dur := seg[len(seg)-1].sec - seg[0].sec
	if dur < hnrMinDurSec {
		return MutaHarassEpisode{}, false
	}
	density := float64(len(seg)) / dur

	// Significant repositions (skip jitter), then count direction reversals.
	var moves [][2]float64 // displacement vectors between significant points
	var sigPts []spatialCmd
	last := seg[0]
	sigPts = append(sigPts, last)
	for _, s := range seg[1:] {
		if math.Hypot(float64(s.x-last.x), float64(s.y-last.y)) < moveEpsPx {
			continue
		}
		moves = append(moves, [2]float64{float64(s.x - last.x), float64(s.y - last.y)})
		sigPts = append(sigPts, s)
		last = s
	}
	reversals := 0
	for k := 1; k < len(moves); k++ {
		if moves[k-1][0]*moves[k][0]+moves[k-1][1]*moves[k][1] < 0 {
			reversals++
		}
	}
	revFrac := 0.0
	if len(moves) > 1 {
		revFrac = float64(reversals) / float64(len(moves)-1)
	}

	if len(moves) < hnrMinSigMoves || density < hnrMinDensity || revFrac < hnrMinReversal {
		return MutaHarassEpisode{}, false
	}

	path := make([]HarassPoint, 0, len(sigPts))
	for _, s := range sigPts {
		path = append(path, HarassPoint{Sec: int(s.sec), X: s.x, Y: s.y})
	}
	return MutaHarassEpisode{
		PlayerID:  pid,
		StartSec:  int(seg[0].sec),
		EndSec:    int(seg[len(seg)-1].sec),
		Cycles:    reversals,
		GroupSize: medianSize(seg),
		Path:      path,
	}, true
}

// mergeEpisodes folds qualifying episodes that are close in time into one
// harass window (the flock pauses to regroup between volleys).
func mergeEpisodes(eps []MutaHarassEpisode) []MutaHarassEpisode {
	if len(eps) == 0 {
		return nil
	}
	sort.SliceStable(eps, func(i, j int) bool { return eps[i].StartSec < eps[j].StartSec })
	out := []MutaHarassEpisode{eps[0]}
	for _, e := range eps[1:] {
		last := &out[len(out)-1]
		if float64(e.StartSec-last.EndSec) <= mergeGapSec {
			last.EndSec = e.EndSec
			last.Cycles += e.Cycles
			last.Path = append(last.Path, e.Path...)
			if e.GroupSize > last.GroupSize {
				last.GroupSize = e.GroupSize
			}
		} else {
			out = append(out, e)
		}
	}
	// Keep only sustained campaigns: enough total volleys over enough time.
	kept := out[:0]
	for _, w := range out {
		if w.Cycles >= minWindowReversals && w.EndSec-w.StartSec >= minWindowDurSec {
			kept = append(kept, w)
		}
	}
	return kept
}

func medianSize(seg []spatialCmd) int {
	v := make([]int, 0, len(seg))
	for _, s := range seg {
		v = append(v, s.selSize)
	}
	sort.Ints(v)
	return v[len(v)/2]
}
