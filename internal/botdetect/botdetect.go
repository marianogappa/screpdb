// Package botdetect flags players whose command stream no human hand can
// produce. Every threshold sits above the highest value measured on 51,520
// human player-games (1v1 ladder from MMR 512 up, plus 8-player BGH) and was
// checked against 1,105 cwal.gg-flagged bot games: 99.5% caught, zero humans.
// Each test alone is a verdict; one game is enough.
package botdetect

import (
	"math"
	"slices"

	"github.com/icza/screp/rep/repcmd"
)

type Signal uint8

const (
	// SignalSameFrame: most commands share a frame with the one before. Bots
	// issue select+order pairs in one frame; humans peak at 0.48.
	SignalSameFrame Signal = 1 << iota
	// SignalSelectOrderBursts: many frames hold three or more select→order
	// pairs. A lag spike can flush a human's buffered input into one frame, so
	// it is the volume, not a single burst, that no human reaches (max 26).
	SignalSelectOrderBursts
	// SignalFarJumps: coordinate commands 3 to 6 frames apart land far apart.
	// Humans need the camera there first; their median tops out at 355px.
	SignalFarJumps
	// SignalSustainedAPM: APM above 900 for five minutes or more after minute
	// ten (human max 686).
	SignalSustainedAPM
)

func (s Signal) Has(o Signal) bool { return s&o != 0 }

const (
	framesPerMinute = 1429

	minGameFrames = 4 * framesPerMinute
	minCommands   = 200

	sameFrameShare = 0.55

	burstCyclesPerFrame = 3
	burstFrames         = 30

	jumpMinGapFrames = 3
	jumpMaxGapFrames = 6
	jumpMinPairs     = 30
	// Pixels; build and land targets arrive in tiles and are scaled.
	jumpMedianPixels = 400

	apmFromMinute = 10
	apmMinMinutes = 5
	apmSustained  = 900
	pixelsPerTile = 32
)

// Detect returns the signals each player in cmds trips, keyed by replay player
// ID. Players who trip none are absent. cmds must be in frame order, as screp
// returns them.
func Detect(cmds []repcmd.Cmd, endFrame int32) map[byte]Signal {
	if endFrame < minGameFrames {
		return nil
	}
	byPlayer := map[byte][]repcmd.Cmd{}
	for _, c := range cmds {
		id := c.BaseCmd().PlayerID
		byPlayer[id] = append(byPlayer[id], c)
	}
	out := map[byte]Signal{}
	for id, own := range byPlayer {
		if s := detectPlayer(own, int(endFrame)); s != 0 {
			out[id] = s
		}
	}
	return out
}

func detectPlayer(cmds []repcmd.Cmd, endFrame int) Signal {
	frames := countedFrames(cmds)
	if len(frames) < minCommands {
		return 0
	}
	var s Signal
	if sameFrameRatio(frames) > sameFrameShare {
		s |= SignalSameFrame
	}
	if burstFrameCount(cmds) >= burstFrames {
		s |= SignalSelectOrderBursts
	}
	if median, pairs := jumpMedian(cmds); pairs >= jumpMinPairs && median > jumpMedianPixels {
		s |= SignalFarJumps
	}
	if apm, minutes := lateAPM(frames, endFrame); minutes >= apmMinMinutes && apm > apmSustained {
		s |= SignalSustainedAPM
	}
	return s
}

func isBookkeeping(c repcmd.Cmd) bool {
	switch c.BaseCmd().Type.ID {
	case repcmd.TypeIDKeepAlive, repcmd.TypeIDSync, repcmd.TypeIDLeaveGame, repcmd.TypeIDChat:
		return true
	}
	return false
}

func countedFrames(cmds []repcmd.Cmd) []int {
	out := make([]int, 0, len(cmds))
	for _, c := range cmds {
		if !isBookkeeping(c) {
			out = append(out, int(c.BaseCmd().Frame))
		}
	}
	return out
}

func sameFrameRatio(frames []int) float64 {
	same := 0
	for i := 1; i < len(frames); i++ {
		if frames[i] == frames[i-1] {
			same++
		}
	}
	return float64(same) / float64(len(frames))
}

type kind int

const (
	kindNeutral kind = iota
	kindSelect
	kindOrder
)

func classify(c repcmd.Cmd) kind {
	switch x := c.(type) {
	case *repcmd.SelectCmd:
		return kindSelect
	case *repcmd.HotkeyCmd:
		if x.HotkeyType != nil && x.HotkeyType.ID == repcmd.HotkeyTypeIDSelect {
			return kindSelect
		}
		return kindNeutral
	}
	switch c.BaseCmd().Type.ID {
	case repcmd.TypeIDKeepAlive, repcmd.TypeIDSync, repcmd.TypeIDChat, repcmd.TypeIDLeaveGame,
		repcmd.TypeIDMinimapPing, repcmd.TypeIDSaveGame, repcmd.TypeIDLoadGame:
		return kindNeutral
	}
	return kindOrder
}

func burstFrameCount(cmds []repcmd.Cmd) int {
	bursts, cycles, frame := 0, 0, -1
	pending := false
	flush := func() {
		if cycles >= burstCyclesPerFrame {
			bursts++
		}
	}
	for _, c := range cmds {
		if f := int(c.BaseCmd().Frame); f != frame {
			flush()
			frame, cycles, pending = f, 0, false
		}
		switch classify(c) {
		case kindSelect:
			pending = true
		case kindOrder:
			if pending {
				cycles++
				pending = false
			}
		}
	}
	flush()
	return bursts
}

type point struct {
	frame int
	x, y  float64
}

func target(c repcmd.Cmd) (point, bool) {
	f := int(c.BaseCmd().Frame)
	switch x := c.(type) {
	case *repcmd.RightClickCmd:
		return point{f, float64(x.Pos.X), float64(x.Pos.Y)}, true
	case *repcmd.TargetedOrderCmd:
		return point{f, float64(x.Pos.X), float64(x.Pos.Y)}, true
	case *repcmd.BuildCmd:
		return point{f, float64(x.Pos.X) * pixelsPerTile, float64(x.Pos.Y) * pixelsPerTile}, true
	}
	return point{}, false
}

func jumpMedian(cmds []repcmd.Cmd) (median float64, pairs int) {
	var dists []float64
	var prev point
	havePrev := false
	for _, c := range cmds {
		p, ok := target(c)
		if !ok {
			continue
		}
		if havePrev {
			if gap := p.frame - prev.frame; gap >= jumpMinGapFrames && gap <= jumpMaxGapFrames {
				dists = append(dists, math.Hypot(p.x-prev.x, p.y-prev.y))
			}
		}
		prev, havePrev = p, true
	}
	if len(dists) == 0 {
		return 0, 0
	}
	slices.Sort(dists)
	return dists[len(dists)/2], len(dists)
}

// lateAPM is the median per-minute command count from apmFromMinute on,
// leaving out the final, partial minute.
func lateAPM(frames []int, endFrame int) (apm float64, minutes int) {
	perMinute := make([]float64, endFrame/framesPerMinute+1)
	for _, f := range frames {
		if m := f / framesPerMinute; m < len(perMinute) {
			perMinute[m]++
		}
	}
	if len(perMinute)-1 <= apmFromMinute {
		return 0, 0
	}
	late := slices.Clone(perMinute[apmFromMinute : len(perMinute)-1])
	slices.Sort(late)
	return late[len(late)/2], len(late)
}
