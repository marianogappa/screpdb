package db

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/models"
)

func (s *LibStore) GetReplaySummary(ctx context.Context, replayID int64) (*ReplaySummaryRow, error) {
	r, err := s.replay(replayID)
	if err != nil {
		return nil, err
	}
	return &ReplaySummaryRow{
		ReplayID: r.ID,
		// The SQL store persisted replay_date as time.Time.String(), which is
		// the only layout the dashboard's parseReplayDate treats as canonical.
		ReplayDate:         r.Date.String(),
		FileName:           r.FileName(),
		FilePath:           r.Path(),
		FileChecksum:       library.ChecksumHex(r.Checksum),
		MapName:            library.Strings.Name(r.Map),
		MapKind:            r.MapKind.String(),
		MapTileWidth:       int(r.MapWidth),
		MapTileHeight:      int(r.MapHeight),
		GameSource:         library.Strings.Name(r.GameSource),
		LobbyKind:          library.Strings.Name(r.LobbyKind),
		DurationSeconds:    int64(r.Duration),
		GameType:           library.Strings.Name(r.GameType),
		TeamStacking:       r.Flags.Has(library.FlagTeamStacking),
		TeamInfoIncomplete: r.Flags.Has(library.FlagTeamInfoIncomplete),
	}, nil
}

func (s *LibStore) GetReplayFilePathByID(ctx context.Context, replayID int64) (string, error) {
	r, err := s.replay(replayID)
	if err != nil {
		return "", err
	}
	return r.Path(), nil
}

// GetReplayWatchFiles resolves the two watchable copies of a game (issue
// #341). For a complete-copy record the user's own file is its superseded
// sibling, looked up in the unfiltered snapshot because the global filter
// hides superseded records. completePath is empty when the game has no
// complete copy.
func (s *LibStore) GetReplayWatchFiles(ctx context.Context, replayID int64) (ownPath, completePath string, err error) {
	r, err := s.replay(replayID)
	if err != nil {
		return "", "", err
	}
	for _, f := range r.Paths {
		base, ok := library.CompleteBasePath(f.Path)
		if !ok {
			continue
		}
		completePath = f.Path
		if _, exists := s.view().Snapshot().ByPath(base); exists {
			return base, completePath, nil
		}
		return "", completePath, nil
	}
	return r.Path(), "", nil
}

func (s *LibStore) ListReplayPlayersForDetail(ctx context.Context, replayID int64) ([]ReplayPlayerDetailRow, error) {
	r, err := s.replay(replayID)
	if err != nil {
		return nil, err
	}
	out := make([]ReplayPlayerDetailRow, 0, len(r.Players))
	for i := range r.Players {
		p := &r.Players[i]
		if p.IsObserver() {
			continue
		}
		row := ReplayPlayerDetailRow{
			PlayerID: rowPlayerID(r, uint8(i)),
			Name:     p.Name,
			Color:    library.Strings.Name(p.Color),
			Race:     p.Race.String(),
			Team:     int64(p.Team),
			TeamWon:  p.TeamOutcome() == models.OutcomeWon,
			APM:      int64(p.APM),
			EAPM:     int64(p.EAPM),
		}
		if p.Flags.Has(library.PlayerHasStartLocation) {
			oclock := int64(p.StartOclock)
			row.StartLocationOclock = &oclock
		}
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Team != out[j].Team {
			return out[i].Team < out[j].Team
		}
		return out[i].PlayerID < out[j].PlayerID
	})
	return out, nil
}

// ListReplayAllianceSnapshots is the alliance timeline exactly as ingest
// computed it. Re-deriving it from commands cannot work: a player allied into
// two cliques at once has no single command that reproduces both.
func (s *LibStore) ListReplayAllianceSnapshots(ctx context.Context, replayID int64) ([]ReplayAllianceSnapshotRow, error) {
	r, err := s.replay(replayID)
	if err != nil {
		return nil, err
	}
	out := []ReplayAllianceSnapshotRow{}
	if r.Alliance == nil {
		return out, nil
	}
	for _, snap := range r.Alliance.Snapshots {
		teams := make([][]int64, 0, len(snap.Teams))
		for _, team := range snap.Teams {
			ids := make([]int64, 0, len(team))
			for _, ordinal := range team {
				if int(ordinal) < len(r.Players) {
					ids = append(ids, rowPlayerID(r, ordinal))
				}
			}
			if len(ids) > 0 {
				teams = append(teams, ids)
			}
		}
		out = append(out, ReplayAllianceSnapshotRow{Second: int64(snap.Sec), Teams: teams, Stacking: snap.Stacking})
	}
	return out, nil
}

func (s *LibStore) ListReplayPatterns(ctx context.Context, replayID int64) ([]PatternValueRow, error) {
	r, err := s.replay(replayID)
	if err != nil {
		return nil, err
	}
	out := make([]PatternValueRow, 0, len(r.Markers))
	for i := range r.Markers {
		m := &r.Markers[i]
		if m.Player != library.NoPlayer {
			continue
		}
		out = append(out, PatternValueRow{
			PatternName:    library.Features.Name(m.Feature),
			Value:          markerPatternValue(m),
			DetectedSecond: int64(m.Sec),
			Payload:        string(m.Payload),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].PatternName < out[j].PatternName })
	return out, nil
}

func (s *LibStore) ListPlayerPatterns(ctx context.Context, replayID int64) ([]PlayerPatternValueRow, error) {
	r, err := s.replay(replayID)
	if err != nil {
		return nil, err
	}
	out := make([]PlayerPatternValueRow, 0, len(r.Markers))
	for i := range r.Markers {
		m := &r.Markers[i]
		if m.Player == library.NoPlayer || int(m.Player) >= len(r.Players) {
			continue
		}
		out = append(out, PlayerPatternValueRow{
			PlayerID:       rowPlayerID(r, m.Player),
			PatternName:    library.Features.Name(m.Feature),
			Value:          markerPatternValue(m),
			DetectedSecond: int64(m.Sec),
			Payload:        string(m.Payload),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].PlayerID != out[j].PlayerID {
			return out[i].PlayerID < out[j].PlayerID
		}
		return out[i].PatternName < out[j].PatternName
	})
	return out, nil
}

func (s *LibStore) ListReplayEvents(ctx context.Context, replayID int64) ([]ReplayEventRow, error) {
	r, err := s.replay(replayID)
	if err != nil {
		return nil, err
	}
	out := make([]ReplayEventRow, 0, len(r.Events))
	for i := range r.Events {
		ev := &r.Events[i]
		row := ReplayEventRow{
			EventType: library.EventTypes.Name(ev.Type),
			Second:    int64(ev.Sec),
		}
		if id, name, color, ok := eventPlayer(r, ev.Source); ok {
			row.SourcePlayerID = &id
			row.SourcePlayerName = name
			row.SourcePlayerColor = color
		}
		if id, name, color, ok := eventPlayer(r, ev.Target); ok {
			row.TargetPlayerID = &id
			row.TargetPlayerName = name
			row.TargetPlayerColor = color
		}
		if baseType := library.Strings.Name(ev.LocationBaseType); baseType != "" {
			row.LocationBaseType = &baseType
		}
		// The compact record flattens the SQL NULL to zero, and screp's clock
		// positions are 1-12, so zero can only mean "no location".
		if ev.LocationOclock != 0 {
			oclock := int64(ev.LocationOclock)
			row.LocationBaseOclock = &oclock
		}
		if ev.LocationNaturalOclock != 0 {
			natural := int64(ev.LocationNaturalOclock)
			row.LocationNaturalOfClock = &natural
		}
		if ev.LocationMineralOnly {
			mineralOnly := true
			row.LocationMineralOnly = &mineralOnly
		}
		if raw, ok := encodeAttackUnitTypes(ev.AttackUnits()); ok {
			row.AttackUnitTypes = &raw
		}
		if raw, ok := encodeAttackCastCounts(ev.CastCounts()); ok {
			row.AttackCastCounts = &raw
		}
		if payload := ev.Payload(); len(payload) > 0 {
			raw := string(payload)
			row.Payload = &raw
		}
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Second != out[j].Second {
			return out[i].Second < out[j].Second
		}
		return out[i].EventType < out[j].EventType
	})
	return out, nil
}

func (s *LibStore) GetPhaseBoundariesForReplay(ctx context.Context, replayID int64) (PhaseBoundaries, error) {
	r, err := s.replay(replayID)
	if err != nil {
		return PhaseBoundaries{}, err
	}
	out := PhaseBoundaries{}
	for i := range r.Markers {
		m := &r.Markers[i]
		if m.Player != library.NoPlayer {
			continue
		}
		switch library.Features.Name(m.Feature) {
		case "mid_game_starts":
			out.EarlyEndsAtSecond = int64(m.Sec)
		case "late_game_starts":
			out.MidEndsAtSecond = int64(m.Sec)
		}
	}
	return out, nil
}

func (s *LibStore) ListGameUnitProductionAndCasts(ctx context.Context, replayID int64) ([]UnitProductionOrCastRow, error) {
	r, err := s.replay(replayID)
	if err != nil {
		return nil, err
	}
	refs := prodRefs(r, func(kind library.ProdKind, ordinal uint8, _ uint16) bool {
		switch kind {
		case library.ProdTrain, library.ProdUnitMorph, library.ProdCast:
		default:
			return false
		}
		return int(ordinal) < len(r.Players) && humanNonObserver(&r.Players[ordinal])
	})
	sortProdRefsByPlayerThenSecond(refs)

	out := make([]UnitProductionOrCastRow, 0, len(refs))
	for _, ref := range refs {
		kind := r.Prod.Kind[ref.index]
		name := r.Prod.SubjectName(ref.index)
		row := UnitProductionOrCastRow{
			PlayerID:             rowPlayerID(r, ref.ordinal),
			ActionType:           kind.String(),
			SecondsFromGameStart: int64(ref.sec),
		}
		if kind == library.ProdCast {
			row.OrderName = &name
		} else {
			row.UnitType = &name
		}
		out = append(out, row)
	}
	return out, nil
}

func (s *LibStore) ListUnitSliceCommandRows(ctx context.Context, replayID int64) ([]UnitSliceCommandRow, error) {
	r, err := s.replay(replayID)
	if err != nil {
		return nil, err
	}
	refs := prodRefs(r, func(kind library.ProdKind, _ uint8, subject uint16) bool {
		switch kind {
		case library.ProdTrain, library.ProdUnitMorph, library.ProdBuildingMorph, library.ProdBuild:
		default:
			return false
		}
		return library.Units.Name(subject) != ""
	})
	sortProdRefsBySecondThenPlayer(refs)

	out := make([]UnitSliceCommandRow, 0, len(refs))
	for _, ref := range refs {
		out = append(out, UnitSliceCommandRow{
			PlayerID: rowPlayerID(r, ref.ordinal),
			Second:   int64(ref.sec),
			UnitType: r.Prod.SubjectName(ref.index),
		})
	}
	return out, nil
}

// gasBuildings are the three refineries the gas-timing row set tracks.
var gasBuildings = map[string]struct{}{
	"Assimilator": {},
	"Extractor":   {},
	"Refinery":    {},
}

func (s *LibStore) ListGasTimingRows(ctx context.Context, replayID int64) ([]TimingRow, error) {
	return s.timingRows(replayID, library.ProdBuild, func(name string) bool {
		_, ok := gasBuildings[name]
		return ok
	})
}

func (s *LibStore) ListUpgradeTimingRows(ctx context.Context, replayID int64) ([]TimingRow, error) {
	return s.timingRows(replayID, library.ProdUpgrade, func(name string) bool { return name != "" })
}

func (s *LibStore) ListTechTimingRows(ctx context.Context, replayID int64) ([]TimingRow, error) {
	return s.timingRows(replayID, library.ProdTech, func(name string) bool { return name != "" })
}

func (s *LibStore) timingRows(replayID int64, kind library.ProdKind, keep func(name string) bool) ([]TimingRow, error) {
	r, err := s.replay(replayID)
	if err != nil {
		return nil, err
	}
	table := kind.Table()
	refs := prodRefs(r, func(entryKind library.ProdKind, _ uint8, subject uint16) bool {
		return entryKind == kind && keep(table.Name(subject))
	})
	sortProdRefsByPlayerThenSecond(refs)

	out := make([]TimingRow, 0, len(refs))
	for _, ref := range refs {
		out = append(out, TimingRow{
			PlayerID: rowPlayerID(r, ref.ordinal),
			Second:   int64(ref.sec),
			Label:    r.Prod.SubjectName(ref.index),
		})
	}
	return out, nil
}

// earlyZergWindowSeconds bounds the build-order milestone scan; it mirrors the
// `seconds_from_game_start < 600` bound the SQL query carried.
const earlyZergWindowSeconds = 600

func (s *LibStore) LoadEarlyZergTimings(ctx context.Context, replayID int64) ([]EarlyZergTimingsRow, error) {
	r, err := s.replay(replayID)
	if err != nil {
		return nil, err
	}
	byPlayer := map[int64]*EarlyZergTimingsRow{}
	order := []int64{}
	refs := prodRefs(r, func(kind library.ProdKind, ordinal uint8, subject uint16) bool {
		switch kind {
		case library.ProdUnitMorph, library.ProdBuild:
		default:
			return false
		}
		if int(ordinal) >= len(r.Players) {
			return false
		}
		p := &r.Players[ordinal]
		if p.Race != library.RaceZerg || p.IsObserver() {
			return false
		}
		switch library.Units.Name(subject) {
		case "Drone", "Overlord", "Spawning Pool", "Hatchery":
			return true
		}
		return false
	})
	sortProdRefsByPlayerThenSecond(refs)

	for _, ref := range refs {
		if int64(ref.sec) >= earlyZergWindowSeconds {
			continue
		}
		playerID := rowPlayerID(r, ref.ordinal)
		row, ok := byPlayer[playerID]
		if !ok {
			row = &EarlyZergTimingsRow{PlayerID: playerID}
			byPlayer[playerID] = row
			order = append(order, playerID)
		}
		sec := int(ref.sec)
		kind := r.Prod.Kind[ref.index]
		switch name := r.Prod.SubjectName(ref.index); {
		case kind == library.ProdUnitMorph && name == "Drone":
			row.DroneMorphSecs = append(row.DroneMorphSecs, sec)
		case kind == library.ProdUnitMorph && name == "Overlord":
			if row.FirstOverlordSec == nil {
				row.FirstOverlordSec = &sec
			}
		case kind == library.ProdBuild && name == "Spawning Pool":
			if row.FirstPoolSec == nil {
				row.FirstPoolSec = &sec
			}
		case kind == library.ProdBuild && name == "Hatchery":
			if row.FirstHatcherySec == nil {
				row.FirstHatcherySec = &sec
			}
		}
	}

	out := make([]EarlyZergTimingsRow, 0, len(order))
	for _, playerID := range order {
		out = append(out, *byPlayer[playerID])
	}
	return out, nil
}

func (s *LibStore) ListReplayLeaveReasons(ctx context.Context, replayID int64) ([]ReplayLeaveReasonRow, error) {
	r, err := s.replay(replayID)
	if err != nil {
		return nil, err
	}
	out := []ReplayLeaveReasonRow{}
	for i := range r.Players {
		p := &r.Players[i]
		if !p.Flags.Has(library.PlayerLeft) {
			continue
		}
		reason := library.Strings.Name(p.LeaveReason)
		if reason == "" {
			continue
		}
		out = append(out, ReplayLeaveReasonRow{PlayerID: rowPlayerID(r, uint8(i)), Reason: reason})
	}
	return out, nil
}

func (s *LibStore) ListReplayChat(ctx context.Context, replayID int64) ([]ReplayChatRow, error) {
	r, err := s.replay(replayID)
	if err != nil {
		return nil, err
	}
	out := []ReplayChatRow{}
	for _, line := range r.Chat {
		if strings.TrimSpace(line.Text) == "" {
			continue
		}
		if int(line.Player) >= len(r.Players) {
			continue
		}
		out = append(out, ReplayChatRow{
			Second:   int64(line.Sec),
			PlayerID: rowPlayerID(r, line.Player),
			Message:  line.Text,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Second != out[j].Second {
			return out[i].Second < out[j].Second
		}
		return out[i].PlayerID < out[j].PlayerID
	})
	return out, nil
}

// prodRef points at one entry of a replay's production stream. index keeps the
// stream position so a sort can fall back to it, which is how the SQL queries
// broke ties on the command rowid.
type prodRef struct {
	index   int
	ordinal uint8
	sec     uint16
}

func prodRefs(r *library.Replay, keep func(kind library.ProdKind, ordinal uint8, subject uint16) bool) []prodRef {
	prod := &r.Prod
	refs := make([]prodRef, 0, prod.Len())
	for i := 0; i < prod.Len(); i++ {
		if !keep(prod.Kind[i], prod.Player[i], prod.Subject[i]) {
			continue
		}
		refs = append(refs, prodRef{index: i, ordinal: prod.Player[i], sec: prod.Sec[i]})
	}
	return refs
}

// sortProdRefsByPlayerThenSecond reproduces `ORDER BY player_id, seconds, id`.
// The stream is already second-ordered with command order preserved inside a
// second, so a stable sort on the player alone is enough.
func sortProdRefsByPlayerThenSecond(refs []prodRef) {
	sort.SliceStable(refs, func(i, j int) bool { return refs[i].ordinal < refs[j].ordinal })
}

// sortProdRefsBySecondThenPlayer reproduces `ORDER BY seconds, player_id`.
func sortProdRefsBySecondThenPlayer(refs []prodRef) {
	sort.SliceStable(refs, func(i, j int) bool {
		if refs[i].sec != refs[j].sec {
			return refs[i].sec < refs[j].sec
		}
		return refs[i].ordinal < refs[j].ordinal
	})
}

// markerPatternValue mirrors `COALESCE(payload, 'true')`: a marker with no
// payload is a bare "this happened" flag.
func markerPatternValue(m *library.Marker) string {
	if len(m.Payload) == 0 {
		return "true"
	}
	return string(m.Payload)
}

func eventPlayer(r *library.Replay, ordinal uint8) (int64, string, string, bool) {
	if ordinal == library.NoPlayer || int(ordinal) >= len(r.Players) {
		return 0, "", "", false
	}
	p := &r.Players[ordinal]
	return rowPlayerID(r, ordinal), p.Name, library.Strings.Name(p.Color), true
}

func encodeAttackUnitTypes(units []uint16) (string, bool) {
	if len(units) == 0 {
		return "", false
	}
	names := make([]string, 0, len(units))
	for _, id := range units {
		if name := library.Units.Name(id); name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "", false
	}
	raw, err := json.Marshal(names)
	if err != nil {
		return "", false
	}
	return string(raw), true
}

func encodeAttackCastCounts(counts []library.CastCount) (string, bool) {
	if len(counts) == 0 {
		return "", false
	}
	byOrder := make(map[string]int, len(counts))
	for _, c := range counts {
		if name := library.Orders.Name(c.Order); name != "" {
			byOrder[name] = int(c.Count)
		}
	}
	if len(byOrder) == 0 {
		return "", false
	}
	raw, err := json.Marshal(byOrder)
	if err != nil {
		return "", false
	}
	return string(raw), true
}

// screpPlayerTypeName is screp's own spelling of a player type. The SQL store
// persisted that spelling, and the alliance endpoint matches "Computer"
// against it, so the lowercase library.PlayerType.String() cannot be used.
func screpPlayerTypeName(t library.PlayerType) string {
	switch t {
	case library.PlayerTypeInactive:
		return "Inactive"
	case library.PlayerTypeComputer:
		return "Computer"
	case library.PlayerTypeHuman:
		return "Human"
	case library.PlayerTypeRescuePassive:
		return "Rescue Passive"
	case library.PlayerTypeUnused:
		return "(Unused)"
	case library.PlayerTypeComputerControlled:
		return "Computer Controlled"
	case library.PlayerTypeOpen:
		return "Open"
	case library.PlayerTypeNeutral:
		return "Neutral"
	case library.PlayerTypeClosed:
		return "Closed"
	}
	return ""
}

// cadenceUndefinedDeviationFallback is the coefficient-of-variation stand-in
// the SQL used when the deviation is undefined; it keeps the cadence score
// finite and near zero instead of NULL.
const cadenceUndefinedDeviationFallback = 9999.0
