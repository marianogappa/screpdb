package markers

import (
	"encoding/json"
	"math"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
)

// mutalisk_turret_timing detects the canonical 1v1 TvZ "muta-rush vs turret-defense"
// timing dance: Zerg mass-morphs Mutalisks while Terran simultaneously throws up
// Missile Turrets to defend. Two markers (mutalisk_timing on Zerg,
// turret_timing on Terran) fire iff the same shared cross-player condition holds.
//
// Cross-player coordination: the marker framework runs one CustomEvaluator per
// (player × marker) and CustomEvalContext.Observe only sees the current
// player's command stream. To gate on opponent activity, both evaluators read
// the full per-replay enriched stream from ctx.WorldState.EnrichedStream() at
// Finalize and compute both sides' bursts independently. No shared evaluator
// state required.

const (
	mutaBurstWindowSec   = 30
	mutaBurstMinCount    = 3
	turretBurstWindowSec = 60
	turretBurstMinCount  = 3
)

type mutaTurretSignals struct {
	zergPID       byte
	terranPID     byte
	zergFound     bool
	terranFound   bool
	spireCmd      int
	spireFound    bool
	firstMutaCmd  int
	firstMutaOK   bool
	mutaBurst     int
	ebayCmd       int
	ebayFound     bool
	firstTurretCmd int
	firstTurretOK bool
	turretBurst   int
}

// computeMutaTurretSignals walks the full enriched stream once, classifies the
// Zerg + Terran 1v1 players from the players list, and computes both sides'
// first-event timings + burst counts.
func computeMutaTurretSignals(ctx CustomEvalContext) (mutaTurretSignals, bool) {
	var s mutaTurretSignals
	if ctx.Replay == nil || ctx.WorldState == nil {
		return s, false
	}
	if ctx.Replay.TeamFormat != "1v1" || ctx.Replay.Matchup != "TvZ" {
		return s, false
	}

	// Resolve Zerg + Terran PIDs from the player list.
	for _, p := range ctx.Replay.Players {
		if p == nil || p.IsObserver {
			continue
		}
		switch p.Race {
		case "Zerg":
			s.zergPID = p.PlayerID
			s.zergFound = true
		case "Terran":
			s.terranPID = p.PlayerID
			s.terranFound = true
		}
	}
	if !s.zergFound || !s.terranFound {
		return s, false
	}

	stream := ctx.WorldState.EnrichedStream()
	var mutaTimes, turretTimes []int
	for _, f := range stream {
		switch f.Kind {
		case cmdenrich.KindMakeBuilding:
			if byte(f.PlayerID) == s.zergPID && f.Subject == models.GeneralUnitSpire {
				if !s.spireFound {
					s.spireCmd = f.Second
					s.spireFound = true
				}
			} else if byte(f.PlayerID) == s.terranPID {
				switch f.Subject {
				case models.GeneralUnitEngineeringBay:
					if !s.ebayFound {
						s.ebayCmd = f.Second
						s.ebayFound = true
					}
				case models.GeneralUnitMissileTurret:
					turretTimes = append(turretTimes, f.Second)
				}
			}
		case cmdenrich.KindMakeUnit:
			if byte(f.PlayerID) == s.zergPID && f.Subject == models.GeneralUnitMutalisk {
				mutaTimes = append(mutaTimes, f.Second)
			}
		}
	}

	if len(mutaTimes) > 0 {
		s.firstMutaCmd = mutaTimes[0]
		s.firstMutaOK = true
		first := mutaTimes[0]
		count := 0
		for _, t := range mutaTimes {
			if t-first > mutaBurstWindowSec {
				break
			}
			count++
		}
		s.mutaBurst = count
	}
	if len(turretTimes) > 0 {
		s.firstTurretCmd = turretTimes[0]
		s.firstTurretOK = true
		first := turretTimes[0]
		count := 0
		for _, t := range turretTimes {
			if t-first > turretBurstWindowSec {
				break
			}
			count++
		}
		s.turretBurst = count
	}
	return s, true
}

// bothBurstsMatch reports whether the cross-player condition holds.
func (s mutaTurretSignals) bothBurstsMatch() bool {
	if !s.firstMutaOK || s.mutaBurst < mutaBurstMinCount {
		return false
	}
	if !s.firstTurretOK || s.turretBurst < turretBurstMinCount {
		return false
	}
	return true
}

// -----------------------------------------------------------------------------
// mutaTimingEvaluator (Race=Zerg, Matchup=TvZ)
// -----------------------------------------------------------------------------

type mutaTimingEvaluator struct{}

func (e *mutaTimingEvaluator) Observe(_ cmdenrich.EnrichedCommand) {}

func (e *mutaTimingEvaluator) Finalize(ctx CustomEvalContext) CustomResult {
	s, ok := computeMutaTurretSignals(ctx)
	if !ok || !s.bothBurstsMatch() {
		return CustomResult{}
	}
	if ctx.ReplayPlayerID != s.zergPID {
		return CustomResult{}
	}
	// True first-Mutalisk hatch time: a Larva can only start morphing once
	// the Spire is up, so the morph cmd is effectively a queued click when it
	// arrives before spire-finish (common in spam-heavy games). Clamp accordingly.
	spireFinish := s.spireCmd + int(models.BuildTimeSpire)
	morphStart := s.firstMutaCmd
	if s.spireFound && morphStart < spireFinish {
		morphStart = spireFinish
	}
	finished := morphStart + int(models.BuildTimeMutalisk)
	payload, err := json.Marshal(map[string]any{
		"spire_cmd":         s.spireCmd,
		"first_muta_cmd":    s.firstMutaCmd,
		"muta_burst_count":  s.mutaBurst,
		"mutalisk_finished": finished,
	})
	if err != nil {
		return CustomResult{}
	}
	return CustomResult{
		Matched:          true,
		DetectedAtSecond: finished,
		Payload:          payload,
	}
}

// -----------------------------------------------------------------------------
// turretTimingEvaluator (Race=Terran, Matchup=TvZ)
// -----------------------------------------------------------------------------

type turretTimingEvaluator struct{}

func (e *turretTimingEvaluator) Observe(_ cmdenrich.EnrichedCommand) {}

func (e *turretTimingEvaluator) Finalize(ctx CustomEvalContext) CustomResult {
	s, ok := computeMutaTurretSignals(ctx)
	if !ok || !s.bothBurstsMatch() {
		return CustomResult{}
	}
	if ctx.ReplayPlayerID != s.terranPID {
		return CustomResult{}
	}
	// Missile Turret needs an Engineering Bay; clamp the turret build start to
	// max(turret cmd, ebay finish) so the reported "built" time reflects when
	// the SCV could actually start construction.
	ebayFinish := s.ebayCmd + int(models.BuildTimeEngineeringBay)
	turretStart := s.firstTurretCmd
	if s.ebayFound && turretStart < ebayFinish {
		turretStart = ebayFinish
	}
	finished := turretStart + int(math.Round(models.BuildTimeMissileTurret))
	payload, err := json.Marshal(map[string]any{
		"ebay_cmd":           s.ebayCmd,
		"first_turret_cmd":   s.firstTurretCmd,
		"turret_burst_count": s.turretBurst,
		"turret_finished":    finished,
	})
	if err != nil {
		return CustomResult{}
	}
	return CustomResult{
		Matched:          true,
		DetectedAtSecond: finished,
		Payload:          payload,
	}
}
