package db

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strings"

	"github.com/marianogappa/screpdb/internal/gamerules"
	"github.com/marianogappa/screpdb/internal/library"
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
			IsWinner: p.IsWinner(),
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

func (s *LibStore) ListReplayPlayersForAlliance(ctx context.Context, replayID int64) ([]ReplayPlayerForAllianceRow, error) {
	r, err := s.replay(replayID)
	if err != nil {
		return nil, err
	}
	out := make([]ReplayPlayerForAllianceRow, 0, len(r.Players))
	for i := range r.Players {
		p := &r.Players[i]
		out = append(out, ReplayPlayerForAllianceRow{
			PlayerID:   rowPlayerID(r, uint8(i)),
			Name:       p.Name,
			Race:       p.Race.String(),
			Type:       screpPlayerTypeName(p.Type),
			Team:       int64(p.Team),
			IsObserver: p.IsObserver(),
			SlotID:     int64(p.Slot),
		})
	}
	return out, nil
}

// ListReplayAllianceCommands rebuilds an Alliance command stream that replays
// into the topology the library already holds. The raw commands are not kept,
// so each snapshot is expressed as one mutual-alliance command per member: the
// last command of a snapshot completes exactly that snapshot's teams, which is
// what parser.AnalyzeAlliances keys its dedupe on.
func (s *LibStore) ListReplayAllianceCommands(ctx context.Context, replayID int64) ([]ReplayAllianceCommandRow, error) {
	r, err := s.replay(replayID)
	if err != nil {
		return nil, err
	}
	out := []ReplayAllianceCommandRow{}
	if r.Alliance == nil {
		return out, nil
	}
	for _, snap := range r.Alliance.Snapshots {
		for _, team := range snap.Teams {
			slots := make([]int64, 0, len(team))
			for _, ordinal := range team {
				if int(ordinal) < len(r.Players) {
					slots = append(slots, int64(r.Players[ordinal].Slot))
				}
			}
			sort.Slice(slots, func(i, j int) bool { return slots[i] < slots[j] })
			encoded, marshalErr := json.Marshal(slots)
			if marshalErr != nil {
				continue
			}
			for _, ordinal := range team {
				if int(ordinal) >= len(r.Players) {
					continue
				}
				out = append(out, ReplayAllianceCommandRow{
					PlayerID:             rowPlayerID(r, ordinal),
					SecondsFromGameStart: int64(snap.Sec),
					AlliancePlayerIDs:    string(encoded),
				})
			}
		}
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

func (s *LibStore) ListFirstUnitCommandRows(ctx context.Context, replayID int64) ([]FirstUnitCommandRow, error) {
	r, err := s.replay(replayID)
	if err != nil {
		return nil, err
	}
	refs := prodRefs(r, func(kind library.ProdKind, _ uint8, _ uint16) bool {
		switch kind {
		case library.ProdBuild, library.ProdTrain, library.ProdUnitMorph:
			return true
		}
		return false
	})
	sortProdRefsByPlayerThenSecond(refs)

	out := make([]FirstUnitCommandRow, 0, len(refs))
	for _, ref := range refs {
		name := r.Prod.SubjectName(ref.index)
		out = append(out, FirstUnitCommandRow{
			PlayerID:   rowPlayerID(r, ref.ordinal),
			Second:     int64(ref.sec),
			ActionType: r.Prod.Kind[ref.index].String(),
			UnitType:   &name,
		})
	}
	return out, nil
}

func (s *LibStore) ListGameUnitCadenceRows(
	ctx context.Context,
	replayID int64,
	durationSeconds int64,
	excludedUnits []string,
	startSeconds int64,
	endFraction float64,
	idleGapSeconds int64,
) ([]GameUnitCadenceRow, error) {
	r, err := s.replay(replayID)
	if err != nil {
		return nil, err
	}
	precomputed := durationSeconds == int64(r.Duration) &&
		cadenceTuningIsDefault(excludedUnits, startSeconds, endFraction, idleGapSeconds)
	var excluded map[string]struct{}
	if !precomputed {
		excluded = make(map[string]struct{}, len(excludedUnits))
		for _, name := range excludedUnits {
			excluded[name] = struct{}{}
		}
	}

	out := []GameUnitCadenceRow{}
	for i := range r.Players {
		p := &r.Players[i]
		if !humanNonObserver(p) {
			continue
		}
		playerID := rowPlayerID(r, uint8(i))
		var row *GameUnitCadenceRow
		if precomputed {
			row = cadenceRowFromCadence(playerID, p.Cadence)
		} else {
			row = cadenceRowFromProd(r, uint8(i), playerID, durationSeconds, excluded, startSeconds, endFraction, idleGapSeconds)
		}
		if row != nil {
			out = append(out, *row)
		}
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

func (s *LibStore) ListViewportGameRows(ctx context.Context, replayID int64, eventType string) ([]ViewportGameRow, error) {
	r, err := s.replay(replayID)
	if err != nil {
		return nil, err
	}
	out := make([]ViewportGameRow, 0, len(r.Markers))
	for i := range r.Markers {
		m := &r.Markers[i]
		if library.Features.Name(m.Feature) != eventType {
			continue
		}
		if len(m.Payload) == 0 || strings.TrimSpace(string(m.Payload)) == "" {
			continue
		}
		var playerID int64
		if m.Player != library.NoPlayer && int(m.Player) < len(r.Players) {
			playerID = rowPlayerID(r, m.Player)
		}
		out = append(out, ViewportGameRow{PlayerID: playerID, RawValue: string(m.Payload)})
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

func cadenceTuningIsDefault(excludedUnits []string, startSeconds int64, endFraction float64, idleGapSeconds int64) bool {
	if startSeconds != gamerules.UnitCadenceStartSeconds ||
		endFraction != gamerules.UnitCadenceEndFraction ||
		idleGapSeconds != gamerules.UnitCadenceIdleGapSeconds {
		return false
	}
	if len(excludedUnits) != len(gamerules.UnitCadenceExcludedUnits) {
		return false
	}
	want := make(map[string]struct{}, len(gamerules.UnitCadenceExcludedUnits))
	for _, name := range gamerules.UnitCadenceExcludedUnits {
		want[name] = struct{}{}
	}
	for _, name := range excludedUnits {
		if _, ok := want[name]; !ok {
			return false
		}
	}
	return true
}

func cadenceRowFromCadence(playerID int64, c *library.Cadence) *GameUnitCadenceRow {
	if c == nil {
		return nil
	}
	row := &GameUnitCadenceRow{
		PlayerID:      playerID,
		WindowSeconds: int64(c.WindowSec),
		UnitsProduced: int64(c.Units),
		GapCount:      int64(c.Gaps),
		RatePerMinute: ptrFloat64(c.RatePerMinute),
		CadenceScore:  ptrFloat64(c.Score),
	}
	if c.Gaps == 0 {
		return row
	}
	row.Idle20Ratio = ptrFloat64(c.Idle20Ratio)
	// Cadence flattens the SQL NULLs to zero, so the undefined-deviation branch
	// has to be recognised by its fingerprint: only it leaves both the
	// coefficient of variation and the burstiness at zero, because a real
	// coefficient of zero yields a burstiness of -1.
	if c.CVGap == 0 && c.Burstiness == 0 {
		return row
	}
	row.CVGap = ptrFloat64(c.CVGap)
	row.Burstiness = ptrFloat64(c.Burstiness)
	return row
}

func cadenceRowFromProd(
	r *library.Replay,
	ordinal uint8,
	playerID int64,
	durationSeconds int64,
	excluded map[string]struct{},
	startSeconds int64,
	endFraction float64,
	idleGapSeconds int64,
) *GameUnitCadenceRow {
	// The SQL bounded the command window with the replay's own duration and
	// sized the reported window with the caller's, so both are kept apart here.
	windowEnd := int64(endFraction * float64(r.Duration))
	if windowEnd <= startSeconds {
		return nil
	}
	windowSeconds := int64(endFraction*float64(durationSeconds)) - startSeconds
	if windowSeconds <= 0 {
		return nil
	}

	var times []int64
	prod := &r.Prod
	for i := 0; i < prod.Len(); i++ {
		if prod.Player[i] != ordinal {
			continue
		}
		if kind := prod.Kind[i]; kind != library.ProdTrain && kind != library.ProdUnitMorph {
			continue
		}
		name := library.Units.Name(prod.Subject[i])
		if strings.TrimSpace(name) == "" {
			continue
		}
		if _, skip := excluded[name]; skip {
			continue
		}
		sec := int64(prod.Sec[i])
		if sec < startSeconds || sec > windowEnd {
			continue
		}
		times = append(times, sec)
	}
	if len(times) == 0 {
		return nil
	}

	rate := float64(len(times)) * 60.0 / float64(windowSeconds)
	row := &GameUnitCadenceRow{
		PlayerID:      playerID,
		WindowSeconds: windowSeconds,
		UnitsProduced: int64(len(times)),
		GapCount:      int64(len(times) - 1),
		RatePerMinute: ptrFloat64(rate),
	}
	if len(times) < 2 {
		row.CadenceScore = ptrFloat64(rate / (1.0 + cadenceUndefinedDeviationFallback))
		return row
	}

	var sum, sumSquares float64
	idle := 0
	for i := 1; i < len(times); i++ {
		gap := float64(times[i] - times[i-1])
		sum += gap
		sumSquares += gap * gap
		if gap >= float64(idleGapSeconds) {
			idle++
		}
	}
	n := float64(len(times) - 1)
	mean := sum / n
	variance := sumSquares/n - mean*mean
	row.Idle20Ratio = ptrFloat64(float64(idle) / n)
	if variance < 0 || mean == 0 {
		row.CadenceScore = ptrFloat64(rate / (1.0 + cadenceUndefinedDeviationFallback))
		return row
	}
	cv := math.Sqrt(variance) / mean
	row.CVGap = ptrFloat64(cv)
	row.Burstiness = ptrFloat64((cv - 1.0) / (cv + 1.0))
	row.CadenceScore = ptrFloat64(rate / (1.0 + cv))
	return row
}
