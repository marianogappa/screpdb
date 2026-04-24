package detectors

import (
	"encoding/json"
	"strings"

	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
	"github.com/marianogappa/screpdb/internal/patterns/worldstate"
)

// BaseDetector provides common functionality for all detectors
type BaseDetector struct {
	replay     *models.Replay
	players    []*models.Player
	worldState *worldstate.Engine
	finished   bool
}

// Initialize stores the replay and players
func (d *BaseDetector) Initialize(replay *models.Replay, players []*models.Player) {
	d.replay = replay
	d.players = players
}

// IsFinished returns whether the detector has finished
func (d *BaseDetector) IsFinished() bool {
	return d.finished
}

// SetFinished marks the detector as finished
func (d *BaseDetector) SetFinished(finished bool) {
	d.finished = finished
}

// Finalize marks the detector as finished at end-of-replay.
func (d *BaseDetector) Finalize() {
	d.finished = true
}

// GetReplay returns the replay
func (d *BaseDetector) GetReplay() *models.Replay {
	return d.replay
}

func (d *BaseDetector) HasReplayDurationAtLeast(minSeconds int) bool {
	if d.replay == nil {
		return false
	}
	return d.replay.DurationSeconds >= minSeconds
}

// GetPlayers returns the players
func (d *BaseDetector) GetPlayers() []*models.Player {
	return d.players
}

// SetWorldState provides orchestrator-owned runtime world state context.
func (d *BaseDetector) SetWorldState(worldState *worldstate.Engine) {
	d.worldState = worldState
}

// GetWorldState returns runtime world state context if available.
func (d *BaseDetector) GetWorldState() *worldstate.Engine {
	return d.worldState
}

// BasePlayerDetector extends BaseDetector with player-specific functionality
type BasePlayerDetector struct {
	BaseDetector
	replayPlayerID byte
}

// SetReplayPlayerID sets the replay player ID this detector is monitoring
func (d *BasePlayerDetector) SetReplayPlayerID(replayPlayerID byte) {
	d.replayPlayerID = replayPlayerID
}

// GetReplayPlayerID returns the replay player ID this detector is monitoring
func (d *BasePlayerDetector) GetReplayPlayerID() byte {
	return d.replayPlayerID
}

// Level returns LevelPlayer
func (d *BasePlayerDetector) Level() core.DetectorLevel {
	return core.LevelPlayer
}

// ShouldProcessCommand checks if the command is for this player
func (d *BasePlayerDetector) ShouldProcessCommand(command *models.Command) bool {
	return command.Player != nil && command.Player.PlayerID == d.replayPlayerID
}

// BuildPlayerResult creates a PatternResult for a player-level detector
func (d *BasePlayerDetector) BuildPlayerResult(patternName string, detectedAtSecond int, payload json.RawMessage) *core.PatternResult {
	if !d.finished {
		return nil
	}
	replayPlayerID := d.replayPlayerID
	return &core.PatternResult{
		PatternName:      patternName,
		Level:            core.LevelPlayer,
		ReplayID:         d.replay.ID,
		PlayerID:         nil, // Will be set when converting to database IDs
		ReplayPlayerID:   &replayPlayerID,
		DetectedAtSecond: detectedAtSecond,
		Payload:          payload,
	}
}

// BaseReplayDetector extends BaseDetector with replay-specific functionality
type BaseReplayDetector struct {
	BaseDetector
}

// Level returns LevelReplay
func (d *BaseReplayDetector) Level() core.DetectorLevel {
	return core.LevelReplay
}

// ShouldProcessCommand checks if the command has a player (for replay-level detectors)
func (d *BaseReplayDetector) ShouldProcessCommand(command *models.Command) bool {
	return command.Player != nil
}

// BuildReplayResult creates a PatternResult for a replay-level detector
func (d *BaseReplayDetector) BuildReplayResult(patternName string, detectedAtSecond int, payload json.RawMessage) *core.PatternResult {
	if !d.finished {
		return nil
	}
	return &core.PatternResult{
		PatternName:      patternName,
		Level:            core.LevelReplay,
		ReplayID:         d.replay.ID,
		DetectedAtSecond: detectedAtSecond,
		Payload:          payload,
	}
}

// CommandMatcher is a function type that checks if a command matches certain criteria
type CommandMatcher func(command *models.Command) bool

// MatchActionType creates a CommandMatcher that matches a specific action type
func MatchActionType(actionType string) CommandMatcher {
	return func(command *models.Command) bool {
		return command.ActionType == actionType
	}
}

// MatchUnitType creates a CommandMatcher that matches a specific unit type
func MatchUnitType(unitType string) CommandMatcher {
	return func(command *models.Command) bool {
		return command.UnitType != nil && *command.UnitType == unitType
	}
}

// MatchActionAndUnit creates a CommandMatcher that matches both action type and unit type
func MatchActionAndUnit(actionType, unitType string) CommandMatcher {
	return func(command *models.Command) bool {
		return command.ActionType == actionType &&
			command.UnitType != nil && *command.UnitType == unitType
	}
}

// MatchAny creates a CommandMatcher that matches if any of the provided matchers match
func MatchAny(matchers ...CommandMatcher) CommandMatcher {
	return func(command *models.Command) bool {
		for _, matcher := range matchers {
			if matcher(command) {
				return true
			}
		}
		return false
	}
}

// MatchAll creates a CommandMatcher that matches if all of the provided matchers match
func MatchAll(matchers ...CommandMatcher) CommandMatcher {
	return func(command *models.Command) bool {
		for _, matcher := range matchers {
			if !matcher(command) {
				return false
			}
		}
		return true
	}
}

// FirstOccurrenceDetector is a helper for detecting the first occurrence of a command
type FirstOccurrenceDetector struct {
	matched bool
	seconds *int
}

// ProcessFirstOccurrence processes a command and detects the first occurrence
// Returns true if this was the first match and the detector should finish
func (f *FirstOccurrenceDetector) ProcessFirstOccurrence(command *models.Command, matcher CommandMatcher) bool {
	if f.matched {
		return false
	}
	if matcher(command) {
		seconds := command.SecondsFromGameStart
		f.seconds = &seconds
		f.matched = true
		return true
	}
	return false
}

// GetSeconds returns the seconds when the first occurrence was detected
func (f *FirstOccurrenceDetector) GetSeconds() *int {
	return f.seconds
}

// IsMatched returns whether a match was found
func (f *FirstOccurrenceDetector) IsMatched() bool {
	return f.matched
}

// CountDetector is a helper for counting occurrences of commands
type CountDetector struct {
	counts map[int64]int // player ID -> count
}

// NewCountDetector creates a new CountDetector
func NewCountDetector() *CountDetector {
	return &CountDetector{
		counts: make(map[int64]int),
	}
}

// ProcessCount processes a command and increments the count for the player
// Returns true if the threshold is reached and the detector should finish
func (c *CountDetector) ProcessCount(command *models.Command, matcher CommandMatcher, threshold int) bool {
	if command.Player == nil {
		return false
	}
	if matcher(command) {
		replayPlayerID := int64(command.Player.PlayerID)
		c.counts[replayPlayerID]++
		if c.counts[replayPlayerID] >= threshold {
			return true
		}
	}
	return false
}

// GetCounts returns the counts map
func (c *CountDetector) GetCounts() map[int64]int {
	return c.counts
}

// HasAnyCountAbove returns true if any count is above the threshold
func (c *CountDetector) HasAnyCountAbove(threshold int) bool {
	for _, count := range c.counts {
		if count >= threshold {
			return true
		}
	}
	return false
}

func getPlayerByReplayPlayerID(players []*models.Player, replayPlayerID byte) *models.Player {
	for _, player := range players {
		if player != nil && player.PlayerID == replayPlayerID {
			return player
		}
	}
	return nil
}

func isPlayerRace(players []*models.Player, replayPlayerID byte, race string) bool {
	player := getPlayerByReplayPlayerID(players, replayPlayerID)
	if player == nil {
		return false
	}
	return strings.EqualFold(player.Race, race)
}

func isBuildOf(command *models.Command, unitType string) bool {
	return command.ActionType == models.ActionTypeBuild &&
		command.UnitType != nil &&
		*command.UnitType == unitType
}

func isUnitProductionOf(command *models.Command, unitType string) bool {
	return command.IsUnitBuild() &&
		command.UnitType != nil &&
		*command.UnitType == unitType
}

// PlayerUnitCounter counts unit production by unit type for one player.
type PlayerUnitCounter struct {
	replayPlayerID byte
	counts         map[string]int
}

func NewPlayerUnitCounter(replayPlayerID byte) *PlayerUnitCounter {
	return &PlayerUnitCounter{
		replayPlayerID: replayPlayerID,
		counts:         map[string]int{},
	}
}

func (c *PlayerUnitCounter) ProcessCommand(command *models.Command, maxSecondInclusive *int) {
	if command == nil || command.Player == nil || command.Player.PlayerID != c.replayPlayerID {
		return
	}
	if maxSecondInclusive != nil && command.SecondsFromGameStart > *maxSecondInclusive {
		return
	}
	if !command.IsUnitBuild() || command.UnitType == nil {
		return
	}
	c.counts[*command.UnitType]++
}

func (c *PlayerUnitCounter) Count(unitType string) int {
	return c.counts[unitType]
}

func (c *PlayerUnitCounter) TotalExcluding(unitTypes ...string) int {
	excluded := map[string]struct{}{}
	for _, unitType := range unitTypes {
		excluded[unitType] = struct{}{}
	}
	total := 0
	for unitType, count := range c.counts {
		if _, isExcluded := excluded[unitType]; isExcluded {
			continue
		}
		total += count
	}
	return total
}
