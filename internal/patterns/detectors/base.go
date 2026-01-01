package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// BaseDetector provides common functionality for all detectors
type BaseDetector struct {
	replay   *models.Replay
	players  []*models.Player
	finished bool
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

// GetReplay returns the replay
func (d *BaseDetector) GetReplay() *models.Replay {
	return d.replay
}

// GetPlayers returns the players
func (d *BaseDetector) GetPlayers() []*models.Player {
	return d.players
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
func (d *BasePlayerDetector) BuildPlayerResult(patternName string, valueBool *bool, valueInt *int, valueString *string, valueTime *int64) *core.PatternResult {
	if !d.finished {
		return nil
	}
	replayPlayerID := d.replayPlayerID
	return &core.PatternResult{
		PatternName:    patternName,
		Level:          core.LevelPlayer,
		ReplayID:       d.replay.ID,
		PlayerID:       nil, // Will be set when converting to database IDs
		ReplayPlayerID: &replayPlayerID,
		ValueBool:      valueBool,
		ValueInt:       valueInt,
		ValueString:    valueString,
		ValueTime:      valueTime,
	}
}

// BaseTeamDetector extends BaseDetector with team-specific functionality
type BaseTeamDetector struct {
	BaseDetector
	team byte
}

// SetTeam sets the team this detector is monitoring
func (d *BaseTeamDetector) SetTeam(team byte) {
	d.team = team
}

// GetTeam returns the team this detector is monitoring
func (d *BaseTeamDetector) GetTeam() byte {
	return d.team
}

// Level returns LevelTeam
func (d *BaseTeamDetector) Level() core.DetectorLevel {
	return core.LevelTeam
}

// ShouldProcessCommand checks if the command is from a player on this team
func (d *BaseTeamDetector) ShouldProcessCommand(command *models.Command) bool {
	return command.Player != nil && command.Player.Team == d.team
}

// BuildTeamResult creates a PatternResult for a team-level detector
func (d *BaseTeamDetector) BuildTeamResult(patternName string, valueBool *bool, valueInt *int, valueString *string, valueTime *int64) *core.PatternResult {
	if !d.finished {
		return nil
	}
	team := d.team
	return &core.PatternResult{
		PatternName: patternName,
		Level:       core.LevelTeam,
		ReplayID:    d.replay.ID,
		Team:        &team,
		ValueBool:   valueBool,
		ValueInt:    valueInt,
		ValueString: valueString,
		ValueTime:   valueTime,
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
func (d *BaseReplayDetector) BuildReplayResult(patternName string, valueBool *bool, valueInt *int, valueString *string, valueTime *int64) *core.PatternResult {
	if !d.finished {
		return nil
	}
	return &core.PatternResult{
		PatternName: patternName,
		Level:       core.LevelReplay,
		ReplayID:    d.replay.ID,
		ValueBool:   valueBool,
		ValueInt:    valueInt,
		ValueString: valueString,
		ValueTime:   valueTime,
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
