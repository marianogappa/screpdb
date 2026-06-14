package patterns

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
	"github.com/marianogappa/screpdb/internal/patterns/detectors"
	"github.com/marianogappa/screpdb/internal/patterns/markers"
	"github.com/marianogappa/screpdb/internal/patterns/worldstate"
	"github.com/marianogappa/screpdb/internal/unittags"
)

// ReplayLevelDetectorFactory creates a replay-level detector
type ReplayLevelDetectorFactory func() core.Detector

// PlayerLevelDetectorFactory creates a player-level detector for a specific player
type PlayerLevelDetectorFactory func(replayPlayerID byte) core.Detector

var (
	// replayLevelDetectors is the list of replay-level detector factories.
	// Phase-boundary detectors emit hidden markers (no per-surface Pill)
	// that downstream feature code reads at request time — see
	// internal/patterns/detectors/phase_boundary_detector.go.
	replayLevelDetectors = []ReplayLevelDetectorFactory{
		detectors.NewMidGameStartsDetector,
		detectors.NewLateGameStartsDetector,
	}

	// playerLevelDetectors is seeded empty. The markers-loop in init()
	// appends one MarkerPlayerDetector factory per registered marker —
	// that's every player-level detector the orchestrator runs.
	playerLevelDetectors = []PlayerLevelDetectorFactory{}
)

// init registers one MarkerPlayerDetector factory per registered marker.
// Covers both KindInitialBuildOrder (openers) and KindMarker (signatures /
// worldstate-sourced events) in a single loop. Marker definitions live in
// internal/patterns/markers/definitions.go.
func init() {
	for _, m := range markers.Markers() {
		m := m // capture for closure
		playerLevelDetectors = append(playerLevelDetectors, func(replayPlayerID byte) core.Detector {
			detector := detectors.NewMarkerPlayerDetector(m)
			detector.SetReplayPlayerID(replayPlayerID)
			return detector
		})
	}
}

// Orchestrator manages all pattern detectors for a replay
type Orchestrator struct {
	detectors  []core.Detector
	results    []*core.PatternResult
	replay     *models.Replay
	players    []*models.Player
	worldState *worldstate.Engine
}

// NewOrchestrator creates a new pattern detection orchestrator
func NewOrchestrator() *Orchestrator {
	return &Orchestrator{
		detectors: make([]core.Detector, 0),
		results:   make([]*core.PatternResult, 0),
	}
}

// Initialize initializes all detectors with the replay and players
// This creates detector instances for each player and team as needed
func (o *Orchestrator) Initialize(replay *models.Replay, players []*models.Player, mapContext *models.ReplayMapContext) {
	o.replay = replay
	o.players = players
	o.worldState = worldstate.NewEngine(replay, players, mapContext)

	// Create replay-level detectors (one per replay)
	for _, factory := range replayLevelDetectors {
		o.detectors = append(o.detectors, factory())
	}

	// Create player-level detectors (one per player)
	for _, player := range players {
		if player.IsObserver {
			continue
		}
		for _, factory := range playerLevelDetectors {
			o.detectors = append(o.detectors, factory(player.PlayerID))
		}
	}

	// Initialize all detectors
	for _, detector := range o.detectors {
		if consumer, ok := detector.(core.WorldStateConsumer); ok {
			consumer.SetWorldState(o.worldState)
		}
		detector.Initialize(replay, players)
	}
}

// ProcessCommand processes a command through all active detectors
func (o *Orchestrator) ProcessCommand(command *models.Command) {
	if o.worldState != nil {
		o.worldState.ProcessCommand(command)
	}

	for _, detector := range o.detectors {
		if !detector.IsFinished() {
			finished := detector.ProcessCommand(command)
			if finished {
				// Detector finished, check if it has a result to save
				if result := detector.GetResult(); result != nil && detector.ShouldSave() {
					o.results = append(o.results, result)
				}
			}
		}
	}
}

// GetResults returns all pattern results that should be saved
func (o *Orchestrator) GetResults() []*core.PatternResult {
	// Run worldstate's batch pipeline before detector Finalize — markers'
	// worldstateFirstEventEvaluator reads worldstate at Finalize time.
	if o.worldState != nil {
		o.worldState.Finalize()
	}
	// Give detectors a chance to finish using full replay context.
	for _, detector := range o.detectors {
		detector.Finalize()
	}

	// Collect any remaining results from detectors that finished after command processing
	for _, detector := range o.detectors {
		if detector.IsFinished() {
			if result := detector.GetResult(); result != nil && detector.ShouldSave() {
				// Check if we already have this result
				found := false
				for _, existing := range o.results {
					samePlayer := (existing.PlayerID == nil && result.PlayerID == nil) || (existing.PlayerID != nil && result.PlayerID != nil && *existing.PlayerID == *result.PlayerID)
					// Before DB IDs are mapped, player-level results are identified by ReplayPlayerID.
					sameReplayPlayer := (existing.ReplayPlayerID == nil && result.ReplayPlayerID == nil) ||
						(existing.ReplayPlayerID != nil && result.ReplayPlayerID != nil && *existing.ReplayPlayerID == *result.ReplayPlayerID)

					if existing.PatternName == result.PatternName &&
						existing.Level == result.Level &&
						existing.ReplayID == result.ReplayID &&
						samePlayer &&
						sameReplayPlayer {
						found = true
						break
					}
				}
				if !found {
					o.results = append(o.results, result)
				}
			}
		}
	}
	o.results = selectBestTierOpeners(o.results)
	return o.results
}

// selectBestTierOpeners enforces "one opener per player, best tier wins".
// Among the KindInitialBuildOrder results for a given player, only the lowest
// Marker.Tier survives — a preferred (tier 1) opener suppresses the broad
// bucket (tier 2) and residual (tier 3) it overlaps. KindMarker results and
// results whose PatternName isn't a known opener pass through untouched. The
// pass is idempotent: re-running it on already-filtered results is a no-op.
//
// Players are keyed by ReplayPlayerID (this runs before DB-ID mapping) and, as
// a fallback for replay-level openers (none today), by PlayerID. The fuzz test
// guarantees at most one opener matches per (race, matchup, tier), so within
// the winning tier there is exactly one survivor.
func selectBestTierOpeners(results []*core.PatternResult) []*core.PatternResult {
	type key struct {
		replayPlayer int // -1 when absent
		dbPlayer     int64
	}
	bestTier := map[key]int{}
	openerKey := func(r *core.PatternResult) (key, *markers.Marker, bool) {
		m := markers.ByPatternName(r.PatternName)
		if m == nil || m.Kind != markers.KindInitialBuildOrder {
			return key{}, nil, false
		}
		k := key{replayPlayer: -1}
		if r.ReplayPlayerID != nil {
			k.replayPlayer = int(*r.ReplayPlayerID)
		}
		if r.PlayerID != nil {
			k.dbPlayer = *r.PlayerID
		}
		return k, m, true
	}

	for _, r := range results {
		k, m, ok := openerKey(r)
		if !ok {
			continue
		}
		if t, seen := bestTier[k]; !seen || m.Tier < t {
			bestTier[k] = m.Tier
		}
	}

	filtered := results[:0:0]
	for _, r := range results {
		k, m, ok := openerKey(r)
		if ok && m.Tier > bestTier[k] {
			continue // a lower-tier opener won for this player
		}
		filtered = append(filtered, r)
	}
	return filtered
}

func (o *Orchestrator) ReplayEvents() []worldstate.ReplayEvent {
	if o.worldState == nil {
		return nil
	}
	return o.worldState.ReplayEvents()
}

// AppendReplayEvents pushes externally-produced events into the worldstate
// engine's event list (e.g. alliance-derived events emitted by the parser).
func (o *Orchestrator) AppendReplayEvents(events []worldstate.ReplayEvent) {
	if o.worldState == nil {
		return
	}
	o.worldState.AppendReplayEvents(events)
}

// WorldStateEngine returns the worldstate engine for diagnostic callers
// (e.g. attack-importance-filter reports). Production code should use
// ReplayEvents/Entries instead.
func (o *Orchestrator) WorldStateEngine() *worldstate.Engine {
	return o.worldState
}

// SetProductionSignals threads selection-derived production-location evidence
// into the worldstate engine so base ownership is maintained by unit production
// (see worldstate.ProductionSignal). Must be called before results are read.
func (o *Orchestrator) SetProductionSignals(ev *unittags.Evidence) {
	if o.worldState == nil || ev == nil {
		return
	}
	var signals []worldstate.ProductionSignal
	for pid, pe := range ev.Players {
		for _, s := range pe.ProductionSignals {
			signals = append(signals, worldstate.ProductionSignal{
				PlayerID: pid,
				Sec:      s.Sec,
				X:        s.X,
				Y:        s.Y,
				Anchored: s.Anchored,
			})
		}
	}
	o.worldState.SetProductionSignals(signals)
}

// ConvertResultsToDatabaseIDs converts pattern results from replay player IDs to database player IDs
func (o *Orchestrator) ConvertResultsToDatabaseIDs(playerIDMap map[byte]int64) {
	// Convert player-level results
	for _, result := range o.results {
		if result.Level == core.LevelPlayer && result.PlayerID == nil && result.ReplayPlayerID != nil {
			if dbPlayerID, exists := playerIDMap[*result.ReplayPlayerID]; exists {
				result.PlayerID = &dbPlayerID
			}
		}
	}
}
