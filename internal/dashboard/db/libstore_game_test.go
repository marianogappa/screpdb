package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/gamerules"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
	"github.com/marianogappa/screpdb/internal/library/load"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/parser"
)

// newUnfilteredLibStore serves every replay handed to it, so a fixture can
// carry computers or a 30-second game without the global filter hiding it.
func newUnfilteredLibStore(t *testing.T, replays ...*library.Replay) *LibStore {
	t.Helper()
	lib := library.New(library.Options{})
	t.Cleanup(lib.Close)
	if err := lib.SetFilter(library.FilterConfig{}); err != nil {
		t.Fatalf("SetFilter: %v", err)
	}
	lib.Add(0, replays...)
	lib.Flush()
	return NewLibStore(lib, nil, nil)
}

func TestLibStoreGetReplaySummary(t *testing.T) {
	r := melee("summary",
		librarytest.WithDate(time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)),
		librarytest.WithDuration(1234),
		librarytest.WithMap("Polypoid"),
		librarytest.WithFlags(library.FlagTeamStacking|library.FlagTeamInfoIncomplete),
	)
	r.GameSource = library.Strings.Intern("Local")
	r.LobbyKind = library.Strings.Intern("Ladder")
	short := melee("short", librarytest.WithDuration(30))
	s := newTestLibStore(t, r, short)

	got, err := s.GetReplaySummary(context.Background(), r.ID)
	if err != nil {
		t.Fatalf("GetReplaySummary: %v", err)
	}
	want := ReplaySummaryRow{
		ReplayID:           r.ID,
		ReplayDate:         r.Date.String(),
		FileName:           filepath.Base(r.Path()),
		FilePath:           r.Path(),
		FileChecksum:       library.ChecksumHex(r.Checksum),
		MapName:            "Polypoid",
		MapKind:            "Regular",
		GameSource:         "Local",
		LobbyKind:          "Ladder",
		DurationSeconds:    1234,
		GameType:           "Melee",
		TeamStacking:       true,
		TeamInfoIncomplete: true,
	}
	if *got != want {
		t.Fatalf("summary =\n%+v\nwant\n%+v", *got, want)
	}

	if _, err := s.GetReplaySummary(context.Background(), short.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("filtered-out replay = %v, want ErrNotFound", err)
	}
}

func TestLibStoreGetReplayFilePathByID(t *testing.T) {
	r := melee("paths", librarytest.WithPath("/replays/newer.rep", librarytest.BaseDate))
	s := newTestLibStore(t, r)

	got, err := s.GetReplayFilePathByID(context.Background(), r.ID)
	if err != nil {
		t.Fatalf("GetReplayFilePathByID: %v", err)
	}
	if got != r.Path() {
		t.Fatalf("path = %q, want the newest file %q", got, r.Path())
	}
	if got != "/replays/newer.rep" {
		t.Fatalf("path = %q, want the newest-ModTime file", got)
	}
}

func TestLibStoreListReplayPlayersForDetailExcludesObserversAndOrdersByTeam(t *testing.T) {
	r := librarytest.Replay(
		librarytest.WithPlayer("Second", librarytest.Team(2), librarytest.Race(library.RaceZerg)),
		librarytest.WithPlayer("Watcher", librarytest.Team(1), librarytest.Observer()),
		librarytest.WithPlayer("First", librarytest.Team(1), librarytest.Winner(), librarytest.APM(210, 180)),
	)
	r.Players[0].Color = library.Strings.Intern("Red")
	r.Players[2].Flags |= library.PlayerHasStartLocation
	r.Players[2].StartOclock = 11
	s := newTestLibStore(t, r)

	rows, err := s.ListReplayPlayersForDetail(context.Background(), r.ID)
	if err != nil {
		t.Fatalf("ListReplayPlayersForDetail: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want the two non-observers", len(rows))
	}
	if rows[0].Name != "First" || rows[1].Name != "Second" {
		t.Fatalf("rows must be team-ordered, got %q then %q", rows[0].Name, rows[1].Name)
	}
	if rows[0].PlayerID != rowPlayerID(r, 2) || !rows[0].IsWinner || rows[0].APM != 210 || rows[0].EAPM != 180 {
		t.Fatalf("first row = %+v", rows[0])
	}
	if rows[0].StartLocationOclock == nil || *rows[0].StartLocationOclock != 11 {
		t.Fatalf("start oclock = %v, want 11", rows[0].StartLocationOclock)
	}
	if rows[1].StartLocationOclock != nil {
		t.Fatalf("a player with no start location must report nil, got %v", *rows[1].StartLocationOclock)
	}
	if rows[1].Color != "Red" || rows[1].Race != "Zerg" || rows[1].Team != 2 {
		t.Fatalf("second row = %+v", rows[1])
	}
	if rows[0].Color != "" {
		t.Fatalf("an uncoloured player must report the empty string, got %q", rows[0].Color)
	}
}

func TestLibStoreListReplayPlayersForAllianceKeepsEveryone(t *testing.T) {
	r := librarytest.Replay(
		librarytest.WithPlayer("Human", librarytest.Team(1)),
		librarytest.WithPlayer("Bot", librarytest.Team(2), librarytest.Computer()),
		librarytest.WithPlayer("Watcher", librarytest.Team(0), librarytest.Observer()),
		librarytest.WithPlayer("Slot", librarytest.Team(3), librarytest.Type(library.PlayerTypeRescuePassive)),
	)
	s := newUnfilteredLibStore(t, r)

	rows, err := s.ListReplayPlayersForAlliance(context.Background(), r.ID)
	if err != nil {
		t.Fatalf("ListReplayPlayersForAlliance: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("got %d rows, want every slot", len(rows))
	}
	for i, row := range rows {
		if row.PlayerID != rowPlayerID(r, uint8(i)) {
			t.Fatalf("row %d must stay in ordinal order, got id %d", i, row.PlayerID)
		}
		if row.SlotID != int64(i) {
			t.Fatalf("row %d slot = %d", i, row.SlotID)
		}
	}
	// The alliance endpoint compares Type against screp's own spelling.
	wantTypes := []string{"Human", "Computer", "Human", "Rescue Passive"}
	for i, want := range wantTypes {
		if rows[i].Type != want {
			t.Fatalf("row %d type = %q, want %q", i, rows[i].Type, want)
		}
	}
	if !rows[2].IsObserver || rows[0].IsObserver {
		t.Fatalf("observer flags = %v, %v", rows[0].IsObserver, rows[2].IsObserver)
	}
}

func TestLibStoreListReplayAllianceCommands(t *testing.T) {
	r := librarytest.Replay(
		librarytest.WithPlayer("A", librarytest.Team(1)),
		librarytest.WithPlayer("B", librarytest.Team(2)),
		librarytest.WithPlayer("C", librarytest.Team(3)),
	)
	r.Alliance = &library.AllianceTimeline{Snapshots: []library.AllianceSnapshot{
		{Sec: 0, Teams: [][]uint8{{0}, {1}, {2}}},
		{Sec: 300, Teams: [][]uint8{{0, 2}, {1}}},
	}}
	s := newTestLibStore(t, r)

	rows, err := s.ListReplayAllianceCommands(context.Background(), r.ID)
	if err != nil {
		t.Fatalf("ListReplayAllianceCommands: %v", err)
	}
	if len(rows) != 6 {
		t.Fatalf("got %d rows, want one per member per snapshot", len(rows))
	}
	for i := 1; i < len(rows); i++ {
		if rows[i].SecondsFromGameStart < rows[i-1].SecondsFromGameStart {
			t.Fatalf("rows must be chronological, got %+v", rows)
		}
	}
	// The pairing snapshot names both slots on both members' commands, which
	// is what makes the alliance mutual when the analyzer replays them.
	pair := rows[3:5]
	for _, row := range pair {
		var slots []int64
		if err := json.Unmarshal([]byte(row.AlliancePlayerIDs), &slots); err != nil {
			t.Fatalf("alliance ids %q: %v", row.AlliancePlayerIDs, err)
		}
		if len(slots) != 2 || slots[0] != 0 || slots[1] != 2 {
			t.Fatalf("alliance ids = %v, want sorted [0 2]", slots)
		}
		if row.SecondsFromGameStart != 300 {
			t.Fatalf("pair row second = %d", row.SecondsFromGameStart)
		}
	}
	if rows[3].PlayerID != rowPlayerID(r, 0) || rows[4].PlayerID != rowPlayerID(r, 2) {
		t.Fatalf("pair issuers = %d, %d", rows[3].PlayerID, rows[4].PlayerID)
	}

	noAlliance := melee("solo")
	empty, err := newTestLibStore(t, noAlliance).ListReplayAllianceCommands(context.Background(), noAlliance.ID)
	if err != nil {
		t.Fatalf("ListReplayAllianceCommands(no timeline): %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("a replay with no timeline must yield an empty non-nil slice, got %v", empty)
	}
}

// TestLibStoreAllianceCommandsReplayIntoTheSameTopology is the real contract of
// ListReplayAllianceCommands: the rows exist only to be fed back through
// parser.AnalyzeAlliances by the game-detail endpoint, so what matters is that
// the analyzer lands on the topology the library already holds.
func TestLibStoreAllianceCommandsReplayIntoTheSameTopology(t *testing.T) {
	r := librarytest.Replay(
		librarytest.WithPlayer("A", librarytest.Team(1)),
		librarytest.WithPlayer("B", librarytest.Team(2)),
		librarytest.WithPlayer("C", librarytest.Team(3)),
		librarytest.WithPlayer("D", librarytest.Team(4)),
	)
	r.Alliance = &library.AllianceTimeline{Snapshots: []library.AllianceSnapshot{
		{Sec: 0, Teams: [][]uint8{{0}, {1}, {2}, {3}}},
		{Sec: 100, Teams: [][]uint8{{0, 1}, {2}, {3}}},
		{Sec: 200, Teams: [][]uint8{{0, 1}, {2, 3}}},
	}}
	s := newTestLibStore(t, r)
	ctx := context.Background()

	playerRows, err := s.ListReplayPlayersForAlliance(ctx, r.ID)
	if err != nil {
		t.Fatalf("ListReplayPlayersForAlliance: %v", err)
	}
	cmdRows, err := s.ListReplayAllianceCommands(ctx, r.ID)
	if err != nil {
		t.Fatalf("ListReplayAllianceCommands: %v", err)
	}

	ordinalByPID := map[byte]uint8{}
	pidByPlayerID := map[int64]byte{}
	players := make([]*models.Player, 0, len(playerRows))
	pid := byte(1)
	for i, row := range playerRows {
		p := &models.Player{
			SlotID:     uint16(row.SlotID),
			PlayerID:   pid,
			Name:       row.Name,
			Race:       row.Race,
			Type:       row.Type,
			Team:       byte(row.Team),
			IsObserver: row.IsObserver,
		}
		ordinalByPID[pid] = uint8(i)
		pidByPlayerID[row.PlayerID] = pid
		players = append(players, p)
		pid++
	}
	commands := make([]*models.Command, 0, len(cmdRows))
	for _, row := range cmdRows {
		issuer, ok := pidByPlayerID[row.PlayerID]
		if !ok {
			t.Fatalf("alliance command names an unknown player id %d", row.PlayerID)
		}
		var slots []int64
		if err := json.Unmarshal([]byte(row.AlliancePlayerIDs), &slots); err != nil {
			t.Fatalf("alliance ids %q: %v", row.AlliancePlayerIDs, err)
		}
		commands = append(commands, &models.Command{
			ActionType:           "Alliance",
			SecondsFromGameStart: int(row.SecondsFromGameStart),
			Player:               players[issuer-1],
			AlliancePlayerIDs:    &slots,
		})
	}

	result := parser.AnalyzeAlliances(players, commands, int(r.Duration), parser.Activity{
		StoppedSecByPID: map[byte]int{},
		LeaveSecByPID:   map[byte]int{},
	})
	if len(result.Snapshots) != len(r.Alliance.Snapshots) {
		t.Fatalf("replayed %d snapshots, want %d: %+v", len(result.Snapshots), len(r.Alliance.Snapshots), result.Snapshots)
	}
	for i, snap := range result.Snapshots {
		want := r.Alliance.Snapshots[i]
		if snap.Sec != int(want.Sec) {
			t.Fatalf("snapshot %d second = %d, want %d", i, snap.Sec, want.Sec)
		}
		got := map[string]bool{}
		for _, team := range snap.Teams {
			ordinals := make([]int, 0, len(team))
			for _, member := range team {
				ordinals = append(ordinals, int(ordinalByPID[member]))
			}
			sort.Ints(ordinals)
			got[fmt.Sprint(ordinals)] = true
		}
		for _, team := range want.Teams {
			ordinals := make([]int, 0, len(team))
			for _, member := range team {
				ordinals = append(ordinals, int(member))
			}
			sort.Ints(ordinals)
			if !got[fmt.Sprint(ordinals)] {
				t.Fatalf("snapshot %d is missing team %v; replayed %v", i, ordinals, got)
			}
		}
	}
}

func TestLibStoreListReplayAndPlayerPatternsSplitByLevel(t *testing.T) {
	r := melee("patterns",
		librarytest.WithMarker("zzz_replay_level", library.NoPlayer, 10, `{"n":1}`),
		librarytest.WithMarker("aaa_replay_level", library.NoPlayer, 20, ""),
		librarytest.WithMarker("zzz_player_level", 1, 30, `{"n":2}`),
		librarytest.WithMarker("aaa_player_level", 1, 40, ""),
		librarytest.WithMarker("mmm_player_level", 0, 50, `{"n":3}`),
	)
	s := newTestLibStore(t, r)

	replayRows, err := s.ListReplayPatterns(context.Background(), r.ID)
	if err != nil {
		t.Fatalf("ListReplayPatterns: %v", err)
	}
	if len(replayRows) != 2 {
		t.Fatalf("got %d replay-level rows, want 2", len(replayRows))
	}
	if replayRows[0].PatternName != "aaa_replay_level" || replayRows[1].PatternName != "zzz_replay_level" {
		t.Fatalf("replay rows must be name-ordered, got %+v", replayRows)
	}
	if replayRows[0].Value != "true" || replayRows[0].Payload != "" || replayRows[0].DetectedSecond != 20 {
		t.Fatalf("payload-less marker = %+v", replayRows[0])
	}
	if replayRows[1].Value != `{"n":1}` || replayRows[1].Payload != `{"n":1}` {
		t.Fatalf("payload passthrough = %+v", replayRows[1])
	}

	playerRows, err := s.ListPlayerPatterns(context.Background(), r.ID)
	if err != nil {
		t.Fatalf("ListPlayerPatterns: %v", err)
	}
	if len(playerRows) != 3 {
		t.Fatalf("got %d player-level rows, want 3", len(playerRows))
	}
	if playerRows[0].PlayerID != rowPlayerID(r, 0) || playerRows[0].PatternName != "mmm_player_level" {
		t.Fatalf("first player row = %+v", playerRows[0])
	}
	if playerRows[1].PatternName != "aaa_player_level" || playerRows[2].PatternName != "zzz_player_level" {
		t.Fatalf("rows must sort by player then name, got %+v", playerRows)
	}
	if playerRows[1].Value != "true" || playerRows[2].Value != `{"n":2}` {
		t.Fatalf("player row values = %q, %q", playerRows[1].Value, playerRows[2].Value)
	}
}

func TestLibStoreListReplayEvents(t *testing.T) {
	r := melee("events",
		librarytest.WithEvent("zzz_late", 100, 0, 1),
		librarytest.WithEvent("aaa_late", 100, library.NoPlayer, library.NoPlayer),
		librarytest.WithEvent("first", 10, 1, library.NoPlayer),
	)
	r.Players[0].Color = library.Strings.Intern("Red")
	r.Players[1].Color = library.Strings.Intern("Blue")
	r.Events[2].LocationBaseType = library.Strings.Intern("natural")
	r.Events[2].LocationOclock = 7
	r.Events[2].LocationNaturalOclock = 5
	r.Events[2].LocationMineralOnly = true
	r.Events[2].Detail = &library.EventDetail{
		AttackUnits: []uint16{library.Units.Intern("Terran Marine"), library.Units.Intern("Terran Medic")},
		CastCounts:  []library.CastCount{{Order: library.Orders.Intern("Stim Pack"), Count: 3}},
		Payload:     []byte(`{"loc":"natural"}`),
	}
	s := newTestLibStore(t, r)

	rows, err := s.ListReplayEvents(context.Background(), r.ID)
	if err != nil {
		t.Fatalf("ListReplayEvents: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows", len(rows))
	}
	if rows[0].EventType != "first" || rows[1].EventType != "aaa_late" || rows[2].EventType != "zzz_late" {
		t.Fatalf("rows must sort by second then type, got %q %q %q", rows[0].EventType, rows[1].EventType, rows[2].EventType)
	}

	first := rows[0]
	if first.SourcePlayerID == nil || *first.SourcePlayerID != rowPlayerID(r, 1) {
		t.Fatalf("source id = %v", first.SourcePlayerID)
	}
	if first.SourcePlayerName != "Bisu" || first.SourcePlayerColor != "Blue" {
		t.Fatalf("source name/colour = %q/%q", first.SourcePlayerName, first.SourcePlayerColor)
	}
	if first.TargetPlayerID != nil || first.TargetPlayerName != "" || first.TargetPlayerColor != "" {
		t.Fatalf("absent target must be nil/empty, got %+v", first)
	}
	if first.LocationBaseType == nil || *first.LocationBaseType != "natural" {
		t.Fatalf("location base type = %v", first.LocationBaseType)
	}
	if first.LocationBaseOclock == nil || *first.LocationBaseOclock != 7 {
		t.Fatalf("location oclock = %v", first.LocationBaseOclock)
	}
	if first.LocationNaturalOfClock == nil || *first.LocationNaturalOfClock != 5 {
		t.Fatalf("natural oclock = %v", first.LocationNaturalOfClock)
	}
	if first.LocationMineralOnly == nil || !*first.LocationMineralOnly {
		t.Fatalf("mineral only = %v", first.LocationMineralOnly)
	}
	if first.AttackUnitTypes == nil || *first.AttackUnitTypes != `["Terran Marine","Terran Medic"]` {
		t.Fatalf("attack units = %v", first.AttackUnitTypes)
	}
	if first.AttackCastCounts == nil || *first.AttackCastCounts != `{"Stim Pack":3}` {
		t.Fatalf("cast counts = %v", first.AttackCastCounts)
	}
	if first.Payload == nil || *first.Payload != `{"loc":"natural"}` {
		t.Fatalf("payload = %v", first.Payload)
	}

	bare := rows[1]
	if bare.SourcePlayerID != nil || bare.TargetPlayerID != nil {
		t.Fatalf("replay-level event must have no players, got %+v", bare)
	}
	if bare.LocationBaseType != nil || bare.LocationBaseOclock != nil || bare.LocationMineralOnly != nil {
		t.Fatalf("locationless event must be all nil, got %+v", bare)
	}
	if bare.AttackUnitTypes != nil || bare.AttackCastCounts != nil || bare.Payload != nil {
		t.Fatalf("detail-less event must be all nil, got %+v", bare)
	}
}

func TestLibStoreGetPhaseBoundariesForReplay(t *testing.T) {
	both := melee("both",
		librarytest.WithMarker("mid_game_starts", library.NoPlayer, 400, ""),
		librarytest.WithMarker("late_game_starts", library.NoPlayer, 900, ""),
	)
	partial := melee("partial",
		librarytest.WithMarker("mid_game_starts", library.NoPlayer, 500, ""),
		// A player-level marker of the same name must not be read.
		librarytest.WithMarker("late_game_starts", 0, 800, ""),
	)
	none := melee("none")
	s := newTestLibStore(t, both, partial, none)

	got, err := s.GetPhaseBoundariesForReplay(context.Background(), both.ID)
	if err != nil {
		t.Fatalf("GetPhaseBoundariesForReplay: %v", err)
	}
	if got != (PhaseBoundaries{EarlyEndsAtSecond: 400, MidEndsAtSecond: 900}) {
		t.Fatalf("boundaries = %+v", got)
	}
	if got, _ := s.GetPhaseBoundariesForReplay(context.Background(), partial.ID); got != (PhaseBoundaries{EarlyEndsAtSecond: 500}) {
		t.Fatalf("partial boundaries = %+v", got)
	}
	if got, _ := s.GetPhaseBoundariesForReplay(context.Background(), none.ID); got != (PhaseBoundaries{}) {
		t.Fatalf("absent boundaries = %+v", got)
	}
}

func TestLibStoreListGameUnitProductionAndCasts(t *testing.T) {
	r := librarytest.Replay(
		librarytest.WithPlayer("Flash", librarytest.Team(1)),
		librarytest.WithPlayer("Bot", librarytest.Team(2), librarytest.Computer()),
		librarytest.WithPlayer("Watcher", librarytest.Team(3), librarytest.Observer()),
		librarytest.WithProd(0, 200, library.ProdTrain, "Terran Marine"),
		librarytest.WithProd(0, 100, library.ProdUnitMorph, "Lurker"),
		librarytest.WithProd(0, 150, library.ProdCast, "Stim Pack"),
		librarytest.WithProd(0, 120, library.ProdBuild, "Terran Barracks"),
		librarytest.WithProd(1, 90, library.ProdTrain, "Terran Marine"),
		librarytest.WithProd(2, 80, library.ProdTrain, "Terran Marine"),
	)
	s := newUnfilteredLibStore(t, r)

	rows, err := s.ListGameUnitProductionAndCasts(context.Background(), r.ID)
	if err != nil {
		t.Fatalf("ListGameUnitProductionAndCasts: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want the human's Train/Morph/Cast only: %+v", len(rows), rows)
	}
	for _, row := range rows {
		if row.PlayerID != rowPlayerID(r, 0) {
			t.Fatalf("computers and observers must be excluded, got %+v", row)
		}
	}
	wantSeconds := []int64{100, 150, 200}
	wantActions := []string{"Unit Morph", "Targeted Order", "Train"}
	for i, row := range rows {
		if row.SecondsFromGameStart != wantSeconds[i] || row.ActionType != wantActions[i] {
			t.Fatalf("row %d = %+v", i, row)
		}
		if row.UnitTypes != nil {
			t.Fatalf("row %d must not carry a unit_types list, got %q", i, *row.UnitTypes)
		}
	}
	if rows[0].UnitType == nil || *rows[0].UnitType != "Lurker" || rows[0].OrderName != nil {
		t.Fatalf("morph row = %+v", rows[0])
	}
	if rows[1].OrderName == nil || *rows[1].OrderName != "Stim Pack" || rows[1].UnitType != nil {
		t.Fatalf("cast row = %+v", rows[1])
	}
}

func TestLibStoreListUnitSliceCommandRows(t *testing.T) {
	r := librarytest.Replay(
		librarytest.WithPlayer("Flash"),
		librarytest.WithPlayer("Watcher", librarytest.Observer()),
		librarytest.WithProd(1, 100, library.ProdTrain, "Terran Marine"),
		librarytest.WithProd(0, 100, library.ProdBuildingMorph, "Lair"),
		librarytest.WithProd(0, 50, library.ProdBuild, "Terran Barracks"),
		librarytest.WithProd(0, 150, library.ProdCast, "Stim Pack"),
		librarytest.WithProd(0, 160, library.ProdUpgrade, "Terran Infantry Weapons"),
	)
	s := newTestLibStore(t, r)

	rows, err := s.ListUnitSliceCommandRows(context.Background(), r.ID)
	if err != nil {
		t.Fatalf("ListUnitSliceCommandRows: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want the three unit-bearing commands: %+v", len(rows), rows)
	}
	if rows[0].Second != 50 || rows[0].UnitType != "Terran Barracks" {
		t.Fatalf("first row = %+v", rows[0])
	}
	// Ordered by second, then player: the observer is not filtered out here,
	// mirroring the query's lack of a players join.
	if rows[1].Second != 100 || rows[1].PlayerID != rowPlayerID(r, 0) || rows[1].UnitType != "Lair" {
		t.Fatalf("second row = %+v", rows[1])
	}
	if rows[2].Second != 100 || rows[2].PlayerID != rowPlayerID(r, 1) {
		t.Fatalf("third row = %+v", rows[2])
	}
}

func TestLibStoreListFirstUnitCommandRows(t *testing.T) {
	r := librarytest.Replay(
		librarytest.WithPlayer("Flash"),
		librarytest.WithPlayer("Bisu"),
		librarytest.WithProd(1, 40, library.ProdBuild, "Protoss Gateway"),
		librarytest.WithProd(0, 200, library.ProdTrain, "Terran Marine"),
		librarytest.WithProd(0, 60, library.ProdBuildingMorph, "Lair"),
		librarytest.WithProdCount(0, 100, library.ProdUnitMorph, "Zergling", 3),
	)
	s := newTestLibStore(t, r)

	rows, err := s.ListFirstUnitCommandRows(context.Background(), r.ID)
	if err != nil {
		t.Fatalf("ListFirstUnitCommandRows: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want Build/Train/Unit Morph only: %+v", len(rows), rows)
	}
	if rows[0].PlayerID != rowPlayerID(r, 0) || rows[0].Second != 100 || rows[0].ActionType != "Unit Morph" {
		t.Fatalf("first row = %+v", rows[0])
	}
	// A three-larva morph is still one command, exactly as the SQL stored it.
	if rows[0].UnitType == nil || *rows[0].UnitType != "Zergling" || rows[0].UnitTypes != nil {
		t.Fatalf("morph row = %+v", rows[0])
	}
	if rows[1].Second != 200 || rows[1].ActionType != "Train" {
		t.Fatalf("second row = %+v", rows[1])
	}
	if rows[2].PlayerID != rowPlayerID(r, 1) || rows[2].ActionType != "Build" {
		t.Fatalf("third row = %+v", rows[2])
	}
}

func TestLibStoreListGameUnitCadenceRowsUsesThePrecomputedCadence(t *testing.T) {
	r := melee("cadence",
		librarytest.WithCadence(0, library.Cadence{
			WindowSec: 300, Units: 10, Gaps: 9,
			RatePerMinute: 2, CVGap: 0.5, Burstiness: -1.0 / 3.0, Idle20Ratio: 0.25, Score: 4.0 / 3.0,
		}),
		librarytest.WithCadence(1, library.Cadence{
			WindowSec: 300, Units: 1, Gaps: 0, RatePerMinute: 0.2, Score: 0.2 / 10000.0,
		}),
	)
	s := newTestLibStore(t, r)

	rows, err := s.ListGameUnitCadenceRows(
		context.Background(), r.ID, int64(r.Duration),
		gamerules.UnitCadenceExcludedUnits,
		gamerules.UnitCadenceStartSeconds, gamerules.UnitCadenceEndFraction, gamerules.UnitCadenceIdleGapSeconds,
	)
	if err != nil {
		t.Fatalf("ListGameUnitCadenceRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want one per scored player", len(rows))
	}
	scored := rows[0]
	if scored.PlayerID != rowPlayerID(r, 0) || scored.WindowSeconds != 300 || scored.UnitsProduced != 10 || scored.GapCount != 9 {
		t.Fatalf("scored row = %+v", scored)
	}
	if scored.CVGap == nil || *scored.CVGap != 0.5 || scored.Burstiness == nil || scored.Idle20Ratio == nil {
		t.Fatalf("scored row nullability = %+v", scored)
	}
	single := rows[1]
	if single.GapCount != 0 {
		t.Fatalf("single-unit row = %+v", single)
	}
	if single.CVGap != nil || single.Burstiness != nil || single.Idle20Ratio != nil {
		t.Fatalf("a gapless row must report NULL deviations, got %+v", single)
	}
	if single.RatePerMinute == nil || single.CadenceScore == nil {
		t.Fatalf("rate and score are never NULL, got %+v", single)
	}

	// A player the loader never scored yields no row at all.
	bare := melee("unscored")
	rows, err = s.ListGameUnitCadenceRows(
		context.Background(), bare.ID, int64(bare.Duration),
		gamerules.UnitCadenceExcludedUnits,
		gamerules.UnitCadenceStartSeconds, gamerules.UnitCadenceEndFraction, gamerules.UnitCadenceIdleGapSeconds,
	)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("a replay outside the store must be ErrNotFound, got %v (%d rows)", err, len(rows))
	}
}

func TestLibStoreListGameUnitCadenceRowsRecomputesForOtherTuning(t *testing.T) {
	r := librarytest.Replay(
		librarytest.WithPlayer("Flash"),
		librarytest.WithPlayer("Bisu"),
		librarytest.WithDuration(900),
		librarytest.WithProd(0, 50, library.ProdTrain, "SCV"),
		librarytest.WithProd(0, 100, library.ProdTrain, "Terran Marine"),
		librarytest.WithProd(0, 130, library.ProdTrain, "Terran Marine"),
		librarytest.WithProd(0, 200, library.ProdUnitMorph, "Lurker"),
	)
	s := newTestLibStore(t, r)

	rows, err := s.ListGameUnitCadenceRows(context.Background(), r.ID, 900, []string{"SCV"}, 0, 1.0, 20)
	if err != nil {
		t.Fatalf("ListGameUnitCadenceRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want only the producing player: %+v", len(rows), rows)
	}
	row := rows[0]
	if row.PlayerID != rowPlayerID(r, 0) || row.WindowSeconds != 900 || row.UnitsProduced != 3 || row.GapCount != 2 {
		t.Fatalf("row = %+v", row)
	}
	// Gaps are 30 and 70: mean 50, population variance 400, cv 0.4.
	assertNullFloat(t, "rate", row.RatePerMinute, 0.2)
	assertNullFloat(t, "cv", row.CVGap, 0.4)
	assertNullFloat(t, "burstiness", row.Burstiness, (0.4-1.0)/(0.4+1.0))
	assertNullFloat(t, "idle", row.Idle20Ratio, 1.0)
	assertNullFloat(t, "score", row.CadenceScore, 0.2/1.4)

	// A window that never opens produces nothing at all.
	empty, err := s.ListGameUnitCadenceRows(context.Background(), r.ID, 900, []string{"SCV"}, 5000, 0.8, 20)
	if err != nil {
		t.Fatalf("ListGameUnitCadenceRows(empty window): %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty window = %+v, want an empty non-nil slice", empty)
	}
}

func assertNullFloat(t *testing.T, label string, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s is NULL, want %v", label, want)
	}
	if math.Abs(*got-want) > 1e-9 {
		t.Fatalf("%s = %v, want %v", label, *got, want)
	}
}

func TestLibStoreTimingRows(t *testing.T) {
	r := librarytest.Replay(
		librarytest.WithPlayer("Flash"),
		librarytest.WithPlayer("Bisu"),
		librarytest.WithProd(1, 90, library.ProdBuild, "Assimilator"),
		librarytest.WithProd(0, 200, library.ProdBuild, "Refinery"),
		librarytest.WithProd(0, 100, library.ProdBuild, "Terran Barracks"),
		librarytest.WithProd(0, 150, library.ProdUpgrade, "Terran Infantry Weapons"),
		librarytest.WithProd(0, 250, library.ProdTech, "Stim Packs"),
		librarytest.WithProd(1, 160, library.ProdUpgrade, "Protoss Ground Weapons"),
	)
	s := newTestLibStore(t, r)
	ctx := context.Background()

	gas, err := s.ListGasTimingRows(ctx, r.ID)
	if err != nil {
		t.Fatalf("ListGasTimingRows: %v", err)
	}
	if len(gas) != 2 {
		t.Fatalf("gas rows = %+v", gas)
	}
	if gas[0].PlayerID != rowPlayerID(r, 0) || gas[0].Label != "Refinery" || gas[0].Second != 200 {
		t.Fatalf("first gas row = %+v", gas[0])
	}
	if gas[1].PlayerID != rowPlayerID(r, 1) || gas[1].Label != "Assimilator" {
		t.Fatalf("second gas row = %+v", gas[1])
	}

	ups, err := s.ListUpgradeTimingRows(ctx, r.ID)
	if err != nil {
		t.Fatalf("ListUpgradeTimingRows: %v", err)
	}
	if len(ups) != 2 || ups[0].Label != "Terran Infantry Weapons" || ups[1].Label != "Protoss Ground Weapons" {
		t.Fatalf("upgrade rows = %+v", ups)
	}

	techs, err := s.ListTechTimingRows(ctx, r.ID)
	if err != nil {
		t.Fatalf("ListTechTimingRows: %v", err)
	}
	if len(techs) != 1 || techs[0].Label != "Stim Packs" || techs[0].Second != 250 {
		t.Fatalf("tech rows = %+v", techs)
	}
}

func TestLibStoreLoadEarlyZergTimings(t *testing.T) {
	r := librarytest.Replay(
		librarytest.WithPlayer("Zerg", librarytest.Race(library.RaceZerg)),
		librarytest.WithPlayer("Terran", librarytest.Race(library.RaceTerran)),
		librarytest.WithPlayer("ZergObs", librarytest.Race(library.RaceZerg), librarytest.Observer()),
		librarytest.WithProd(0, 40, library.ProdUnitMorph, "Drone"),
		librarytest.WithProd(0, 60, library.ProdUnitMorph, "Drone"),
		librarytest.WithProd(0, 80, library.ProdUnitMorph, "Overlord"),
		librarytest.WithProd(0, 90, library.ProdUnitMorph, "Overlord"),
		librarytest.WithProd(0, 100, library.ProdBuild, "Spawning Pool"),
		librarytest.WithProd(0, 120, library.ProdBuild, "Hatchery"),
		librarytest.WithProd(0, 140, library.ProdBuild, "Hatchery"),
		librarytest.WithProd(0, 700, library.ProdUnitMorph, "Drone"),
		librarytest.WithProd(0, 200, library.ProdUnitMorph, "Zergling"),
		librarytest.WithProd(1, 50, library.ProdBuild, "Refinery"),
		librarytest.WithProd(2, 50, library.ProdUnitMorph, "Drone"),
	)
	s := newTestLibStore(t, r)

	rows, err := s.LoadEarlyZergTimings(context.Background(), r.ID)
	if err != nil {
		t.Fatalf("LoadEarlyZergTimings: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want only the Zerg non-observer: %+v", len(rows), rows)
	}
	row := rows[0]
	if row.PlayerID != rowPlayerID(r, 0) {
		t.Fatalf("row player = %d", row.PlayerID)
	}
	if len(row.DroneMorphSecs) != 2 || row.DroneMorphSecs[0] != 40 || row.DroneMorphSecs[1] != 60 {
		t.Fatalf("drone morphs = %v, want the two inside the 600s window in order", row.DroneMorphSecs)
	}
	if row.FirstOverlordSec == nil || *row.FirstOverlordSec != 80 {
		t.Fatalf("first overlord = %v", row.FirstOverlordSec)
	}
	if row.FirstPoolSec == nil || *row.FirstPoolSec != 100 {
		t.Fatalf("first pool = %v", row.FirstPoolSec)
	}
	if row.FirstHatcherySec == nil || *row.FirstHatcherySec != 120 {
		t.Fatalf("first hatchery = %v", row.FirstHatcherySec)
	}

	noZerg := melee("no zerg")
	empty, err := newTestLibStore(t, noZerg).LoadEarlyZergTimings(context.Background(), noZerg.ID)
	if err != nil {
		t.Fatalf("LoadEarlyZergTimings(no zerg): %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("no Zerg = %+v, want an empty non-nil slice", empty)
	}
}

func TestLibStoreListViewportGameRows(t *testing.T) {
	r := melee("viewport",
		librarytest.WithMarker("viewport_multitasking", 0, 10, `{"switches_per_minute":12.5}`),
		librarytest.WithMarker("viewport_multitasking", 1, 20, "   "),
		librarytest.WithMarker("viewport_multitasking", library.NoPlayer, 30, `{"switches_per_minute":1}`),
		librarytest.WithMarker("other_marker", 0, 40, `{"x":1}`),
	)
	s := newTestLibStore(t, r)

	rows, err := s.ListViewportGameRows(context.Background(), r.ID, "viewport_multitasking")
	if err != nil {
		t.Fatalf("ListViewportGameRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want the two non-blank payloads: %+v", len(rows), rows)
	}
	if rows[0].PlayerID != rowPlayerID(r, 0) || rows[0].RawValue != `{"switches_per_minute":12.5}` {
		t.Fatalf("first row = %+v", rows[0])
	}
	if rows[1].PlayerID != 0 {
		t.Fatalf("a replay-level marker must report player id 0, got %d", rows[1].PlayerID)
	}

	none, err := s.ListViewportGameRows(context.Background(), r.ID, "no_such_marker")
	if err != nil {
		t.Fatalf("ListViewportGameRows(missing): %v", err)
	}
	if none == nil || len(none) != 0 {
		t.Fatalf("unknown marker = %+v, want an empty non-nil slice", none)
	}
}

func TestLibStoreListReplayLeaveReasons(t *testing.T) {
	r := librarytest.Replay(
		librarytest.WithPlayer("Quitter"),
		librarytest.WithPlayer("Stayer"),
		librarytest.WithPlayer("Silent"),
	)
	r.Players[0].Flags |= library.PlayerLeft
	r.Players[0].LeaveSec = 400
	r.Players[0].LeaveReason = library.Strings.Intern("Quit")
	// Left without a reason: the SQL required a non-null leave_reason.
	r.Players[2].Flags |= library.PlayerLeft
	r.Players[2].LeaveSec = 500
	s := newTestLibStore(t, r)

	rows, err := s.ListReplayLeaveReasons(context.Background(), r.ID)
	if err != nil {
		t.Fatalf("ListReplayLeaveReasons: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want only the reasoned leave: %+v", len(rows), rows)
	}
	if rows[0].PlayerID != rowPlayerID(r, 0) || rows[0].Reason != "Quit" {
		t.Fatalf("row = %+v", rows[0])
	}

	none := melee("nobody left")
	empty, err := newTestLibStore(t, none).ListReplayLeaveReasons(context.Background(), none.ID)
	if err != nil {
		t.Fatalf("ListReplayLeaveReasons(nobody left): %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("nobody left = %+v, want an empty non-nil slice", empty)
	}
}

func TestLibStoreListReplayChat(t *testing.T) {
	r := melee("chat",
		librarytest.WithChat(1, 100, "gg"),
		librarytest.WithChat(0, 100, "glhf"),
		librarytest.WithChat(0, 50, "hi"),
		librarytest.WithChat(1, 60, "   "),
	)
	s := newTestLibStore(t, r)

	rows, err := s.ListReplayChat(context.Background(), r.ID)
	if err != nil {
		t.Fatalf("ListReplayChat: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want the three non-blank lines: %+v", len(rows), rows)
	}
	if rows[0].Second != 50 || rows[0].Message != "hi" {
		t.Fatalf("first row = %+v", rows[0])
	}
	if rows[1].Second != 100 || rows[1].PlayerID != rowPlayerID(r, 0) || rows[1].Message != "glhf" {
		t.Fatalf("second row = %+v", rows[1])
	}
	if rows[2].PlayerID != rowPlayerID(r, 1) || rows[2].Message != "gg" {
		t.Fatalf("third row = %+v", rows[2])
	}
}

func TestLibStoreGameReadsRejectUnknownReplayIDs(t *testing.T) {
	s := newTestLibStore(t, melee("present"))
	ctx := context.Background()
	const missing int64 = 424242

	checks := map[string]func() error{
		"GetReplaySummary": func() error {
			_, err := s.GetReplaySummary(ctx, missing)
			return err
		},
		"GetReplayFilePathByID": func() error {
			_, err := s.GetReplayFilePathByID(ctx, missing)
			return err
		},
		"ListReplayPlayersForDetail": func() error {
			_, err := s.ListReplayPlayersForDetail(ctx, missing)
			return err
		},
		"ListReplayPlayersForAlliance": func() error {
			_, err := s.ListReplayPlayersForAlliance(ctx, missing)
			return err
		},
		"ListReplayAllianceCommands": func() error {
			_, err := s.ListReplayAllianceCommands(ctx, missing)
			return err
		},
		"ListReplayPatterns": func() error {
			_, err := s.ListReplayPatterns(ctx, missing)
			return err
		},
		"ListPlayerPatterns": func() error {
			_, err := s.ListPlayerPatterns(ctx, missing)
			return err
		},
		"ListReplayEvents": func() error {
			_, err := s.ListReplayEvents(ctx, missing)
			return err
		},
		"GetPhaseBoundariesForReplay": func() error {
			_, err := s.GetPhaseBoundariesForReplay(ctx, missing)
			return err
		},
		"ListGameUnitProductionAndCasts": func() error {
			_, err := s.ListGameUnitProductionAndCasts(ctx, missing)
			return err
		},
		"ListUnitSliceCommandRows": func() error {
			_, err := s.ListUnitSliceCommandRows(ctx, missing)
			return err
		},
		"ListFirstUnitCommandRows": func() error {
			_, err := s.ListFirstUnitCommandRows(ctx, missing)
			return err
		},
		"ListGameUnitCadenceRows": func() error {
			_, err := s.ListGameUnitCadenceRows(ctx, missing, 900, gamerules.UnitCadenceExcludedUnits, 420, 0.8, 20)
			return err
		},
		"ListGasTimingRows": func() error {
			_, err := s.ListGasTimingRows(ctx, missing)
			return err
		},
		"ListUpgradeTimingRows": func() error {
			_, err := s.ListUpgradeTimingRows(ctx, missing)
			return err
		},
		"ListTechTimingRows": func() error {
			_, err := s.ListTechTimingRows(ctx, missing)
			return err
		},
		"LoadEarlyZergTimings": func() error {
			_, err := s.LoadEarlyZergTimings(ctx, missing)
			return err
		},
		"ListViewportGameRows": func() error {
			_, err := s.ListViewportGameRows(ctx, missing, "viewport_multitasking")
			return err
		},
		"ListReplayLeaveReasons": func() error {
			_, err := s.ListReplayLeaveReasons(ctx, missing)
			return err
		},
		"ListReplayChat": func() error {
			_, err := s.ListReplayChat(ctx, missing)
			return err
		},
	}
	for name, call := range checks {
		if err := call(); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s(unknown id) = %v, want ErrNotFound", name, err)
		}
	}
}

func TestLibStoreGameReadsOverTheRealCorpus(t *testing.T) {
	// fileops only walks absolute paths, so a relative folder scans to nothing.
	folder, err := filepath.Abs(filepath.Join("..", "..", "testdata", "replays"))
	if err != nil {
		t.Fatalf("resolve corpus folder: %v", err)
	}
	present, err := filepath.Glob(filepath.Join(folder, "*.rep"))
	if err != nil || len(present) == 0 {
		t.Skipf("no replay corpus in %s: %v", folder, err)
	}
	lib := library.New(library.Options{CoalesceRecords: 1, CoalesceDelay: time.Millisecond})
	t.Cleanup(lib.Close)
	if err := lib.SetFilter(library.FilterConfig{}); err != nil {
		t.Fatalf("SetFilter: %v", err)
	}
	loader := load.New(lib, load.Options{Folder: folder, Generation: 1, Workers: 2})
	if err := loader.Run(context.Background()); err != nil {
		t.Fatalf("load corpus: %v", err)
	}
	lib.Flush()
	s := NewLibStore(lib, nil, nil)
	view := s.view()
	if view.Len() != len(present) {
		t.Fatalf("view has %d replays, want the %d in the corpus", view.Len(), len(present))
	}

	ctx := context.Background()
	totals := map[string]int{}
	for _, r := range view.Replays() {
		ids := map[int64]bool{}
		for i := range r.Players {
			ids[rowPlayerID(r, uint8(i))] = true
		}

		summary, err := s.GetReplaySummary(ctx, r.ID)
		if err != nil {
			t.Fatalf("GetReplaySummary(%d): %v", r.ID, err)
		}
		if summary.ReplayID != r.ID || summary.FilePath == "" || summary.FileChecksum == "" {
			t.Fatalf("summary = %+v", summary)
		}
		if summary.DurationSeconds <= 0 {
			t.Fatalf("replay %s has duration %d", summary.FileName, summary.DurationSeconds)
		}
		path, err := s.GetReplayFilePathByID(ctx, r.ID)
		if err != nil || path != summary.FilePath {
			t.Fatalf("GetReplayFilePathByID = %q, %v", path, err)
		}

		detail, err := s.ListReplayPlayersForDetail(ctx, r.ID)
		if err != nil {
			t.Fatalf("ListReplayPlayersForDetail(%d): %v", r.ID, err)
		}
		if len(detail) == 0 {
			t.Fatalf("replay %s has no non-observer players", summary.FileName)
		}
		for i, row := range detail {
			if !ids[row.PlayerID] {
				t.Fatalf("detail row %d has an unresolvable player id %d", i, row.PlayerID)
			}
			if i > 0 && (detail[i-1].Team > row.Team || (detail[i-1].Team == row.Team && detail[i-1].PlayerID > row.PlayerID)) {
				t.Fatalf("detail rows are not team-ordered: %+v", detail)
			}
		}
		totals["detail"] += len(detail)

		alliance, err := s.ListReplayPlayersForAlliance(ctx, r.ID)
		if err != nil {
			t.Fatalf("ListReplayPlayersForAlliance(%d): %v", r.ID, err)
		}
		if len(alliance) != len(r.Players) {
			t.Fatalf("alliance rows = %d, want %d players", len(alliance), len(r.Players))
		}
		allianceCmds, err := s.ListReplayAllianceCommands(ctx, r.ID)
		if err != nil {
			t.Fatalf("ListReplayAllianceCommands(%d): %v", r.ID, err)
		}
		for i, row := range allianceCmds {
			if !ids[row.PlayerID] {
				t.Fatalf("alliance command %d has an unresolvable player id %d", i, row.PlayerID)
			}
			var slots []int64
			if err := json.Unmarshal([]byte(row.AlliancePlayerIDs), &slots); err != nil {
				t.Fatalf("alliance ids %q: %v", row.AlliancePlayerIDs, err)
			}
			if i > 0 && allianceCmds[i-1].SecondsFromGameStart > row.SecondsFromGameStart {
				t.Fatalf("alliance commands are not chronological in %s", summary.FileName)
			}
		}
		totals["alliance_commands"] += len(allianceCmds)

		replayPatterns, err := s.ListReplayPatterns(ctx, r.ID)
		if err != nil {
			t.Fatalf("ListReplayPatterns(%d): %v", r.ID, err)
		}
		if !sort.SliceIsSorted(replayPatterns, func(i, j int) bool {
			return replayPatterns[i].PatternName < replayPatterns[j].PatternName
		}) {
			t.Fatalf("replay patterns are not name-ordered in %s", summary.FileName)
		}
		totals["replay_patterns"] += len(replayPatterns)

		playerPatterns, err := s.ListPlayerPatterns(ctx, r.ID)
		if err != nil {
			t.Fatalf("ListPlayerPatterns(%d): %v", r.ID, err)
		}
		for i, row := range playerPatterns {
			if !ids[row.PlayerID] {
				t.Fatalf("player pattern %d has an unresolvable player id %d", i, row.PlayerID)
			}
			if row.PatternName == "" {
				t.Fatalf("player pattern %d has no name", i)
			}
		}
		totals["player_patterns"] += len(playerPatterns)

		events, err := s.ListReplayEvents(ctx, r.ID)
		if err != nil {
			t.Fatalf("ListReplayEvents(%d): %v", r.ID, err)
		}
		for i, row := range events {
			if row.SourcePlayerID != nil && !ids[*row.SourcePlayerID] {
				t.Fatalf("event %d has an unresolvable source %d", i, *row.SourcePlayerID)
			}
			if row.TargetPlayerID != nil && !ids[*row.TargetPlayerID] {
				t.Fatalf("event %d has an unresolvable target %d", i, *row.TargetPlayerID)
			}
			if i > 0 && (events[i-1].Second > row.Second ||
				(events[i-1].Second == row.Second && events[i-1].EventType > row.EventType)) {
				t.Fatalf("events are not ordered in %s: %+v then %+v", summary.FileName, events[i-1], row)
			}
			if row.AttackUnitTypes != nil {
				var names []string
				if err := json.Unmarshal([]byte(*row.AttackUnitTypes), &names); err != nil {
					t.Fatalf("attack units %q: %v", *row.AttackUnitTypes, err)
				}
			}
			if row.AttackCastCounts != nil {
				var counts map[string]int
				if err := json.Unmarshal([]byte(*row.AttackCastCounts), &counts); err != nil {
					t.Fatalf("cast counts %q: %v", *row.AttackCastCounts, err)
				}
			}
		}
		totals["events"] += len(events)

		boundaries, err := s.GetPhaseBoundariesForReplay(ctx, r.ID)
		if err != nil {
			t.Fatalf("GetPhaseBoundariesForReplay(%d): %v", r.ID, err)
		}
		if boundaries.MidEndsAtSecond != 0 && boundaries.EarlyEndsAtSecond > boundaries.MidEndsAtSecond {
			t.Fatalf("phase boundaries out of order in %s: %+v", summary.FileName, boundaries)
		}

		production, err := s.ListGameUnitProductionAndCasts(ctx, r.ID)
		if err != nil {
			t.Fatalf("ListGameUnitProductionAndCasts(%d): %v", r.ID, err)
		}
		for i, row := range production {
			if !ids[row.PlayerID] {
				t.Fatalf("production row %d has an unresolvable player id %d", i, row.PlayerID)
			}
			switch row.ActionType {
			case "Train", "Unit Morph":
				if row.UnitType == nil || *row.UnitType == "" {
					t.Fatalf("production row %d has no unit type", i)
				}
			case "Targeted Order":
				if row.OrderName == nil || *row.OrderName == "" {
					t.Fatalf("cast row %d has no order name", i)
				}
			default:
				t.Fatalf("production row %d has action type %q", i, row.ActionType)
			}
			if i > 0 && (production[i-1].PlayerID > row.PlayerID ||
				(production[i-1].PlayerID == row.PlayerID && production[i-1].SecondsFromGameStart > row.SecondsFromGameStart)) {
				t.Fatalf("production rows are not ordered in %s", summary.FileName)
			}
		}
		totals["production"] += len(production)

		slices, err := s.ListUnitSliceCommandRows(ctx, r.ID)
		if err != nil {
			t.Fatalf("ListUnitSliceCommandRows(%d): %v", r.ID, err)
		}
		for i, row := range slices {
			if !ids[row.PlayerID] || row.UnitType == "" {
				t.Fatalf("slice row %d = %+v", i, row)
			}
			if i > 0 && (slices[i-1].Second > row.Second ||
				(slices[i-1].Second == row.Second && slices[i-1].PlayerID > row.PlayerID)) {
				t.Fatalf("slice rows are not ordered in %s", summary.FileName)
			}
		}
		totals["slices"] += len(slices)

		firsts, err := s.ListFirstUnitCommandRows(ctx, r.ID)
		if err != nil {
			t.Fatalf("ListFirstUnitCommandRows(%d): %v", r.ID, err)
		}
		for i, row := range firsts {
			if !ids[row.PlayerID] || row.UnitType == nil || *row.UnitType == "" {
				t.Fatalf("first-unit row %d = %+v", i, row)
			}
			if i > 0 && (firsts[i-1].PlayerID > row.PlayerID ||
				(firsts[i-1].PlayerID == row.PlayerID && firsts[i-1].Second > row.Second)) {
				t.Fatalf("first-unit rows are not ordered in %s", summary.FileName)
			}
		}
		totals["firsts"] += len(firsts)

		cadence, err := s.ListGameUnitCadenceRows(
			ctx, r.ID, int64(r.Duration),
			gamerules.UnitCadenceExcludedUnits,
			gamerules.UnitCadenceStartSeconds, gamerules.UnitCadenceEndFraction, gamerules.UnitCadenceIdleGapSeconds,
		)
		if err != nil {
			t.Fatalf("ListGameUnitCadenceRows(%d): %v", r.ID, err)
		}
		recomputed, err := s.ListGameUnitCadenceRows(
			ctx, r.ID, int64(r.Duration),
			append([]string{}, gamerules.UnitCadenceExcludedUnits...),
			gamerules.UnitCadenceStartSeconds, gamerules.UnitCadenceEndFraction, gamerules.UnitCadenceIdleGapSeconds+1,
		)
		if err != nil {
			t.Fatalf("ListGameUnitCadenceRows(recomputed, %d): %v", r.ID, err)
		}
		if len(cadence) != len(recomputed) {
			t.Fatalf("cadence row count differs between paths in %s: %d vs %d", summary.FileName, len(cadence), len(recomputed))
		}
		for i, row := range cadence {
			if !ids[row.PlayerID] {
				t.Fatalf("cadence row %d has an unresolvable player id %d", i, row.PlayerID)
			}
			if row.WindowSeconds <= 0 || row.UnitsProduced <= 0 {
				t.Fatalf("cadence row %d = %+v", i, row)
			}
			if row.GapCount != row.UnitsProduced-1 {
				t.Fatalf("cadence row %d gaps = %d for %d units", i, row.GapCount, row.UnitsProduced)
			}
			// Only the idle threshold changed, so everything but the idle
			// ratio has to agree between the stored and recomputed paths.
			other := recomputed[i]
			if other.PlayerID != row.PlayerID || other.WindowSeconds != row.WindowSeconds || other.UnitsProduced != row.UnitsProduced {
				t.Fatalf("cadence row %d differs between paths: %+v vs %+v", i, row, other)
			}
			if (row.CVGap == nil) != (other.CVGap == nil) || (row.CVGap != nil && math.Abs(*row.CVGap-*other.CVGap) > 1e-9) {
				t.Fatalf("cadence row %d cv differs between paths: %+v vs %+v", i, row.CVGap, other.CVGap)
			}
		}
		totals["cadence"] += len(cadence)

		for name, list := range map[string]func(context.Context, int64) ([]TimingRow, error){
			"gas":      s.ListGasTimingRows,
			"upgrades": s.ListUpgradeTimingRows,
			"techs":    s.ListTechTimingRows,
		} {
			rows, err := list(ctx, r.ID)
			if err != nil {
				t.Fatalf("%s timings(%d): %v", name, r.ID, err)
			}
			for i, row := range rows {
				if !ids[row.PlayerID] || row.Label == "" {
					t.Fatalf("%s row %d = %+v", name, i, row)
				}
				if i > 0 && (rows[i-1].PlayerID > row.PlayerID ||
					(rows[i-1].PlayerID == row.PlayerID && rows[i-1].Second > row.Second)) {
					t.Fatalf("%s rows are not ordered in %s", name, summary.FileName)
				}
			}
			totals[name] += len(rows)
		}

		zerg, err := s.LoadEarlyZergTimings(ctx, r.ID)
		if err != nil {
			t.Fatalf("LoadEarlyZergTimings(%d): %v", r.ID, err)
		}
		for i, row := range zerg {
			if !ids[row.PlayerID] {
				t.Fatalf("zerg row %d has an unresolvable player id %d", i, row.PlayerID)
			}
			if !sort.IntsAreSorted(row.DroneMorphSecs) {
				t.Fatalf("zerg row %d drone morphs are unsorted: %v", i, row.DroneMorphSecs)
			}
		}
		totals["zerg"] += len(zerg)

		viewport, err := s.ListViewportGameRows(ctx, r.ID, "viewport_multitasking")
		if err != nil {
			t.Fatalf("ListViewportGameRows(%d): %v", r.ID, err)
		}
		for i, row := range viewport {
			if row.RawValue == "" {
				t.Fatalf("viewport row %d has an empty payload", i)
			}
			if row.PlayerID != 0 && !ids[row.PlayerID] {
				t.Fatalf("viewport row %d has an unresolvable player id %d", i, row.PlayerID)
			}
		}
		totals["viewport"] += len(viewport)

		leaves, err := s.ListReplayLeaveReasons(ctx, r.ID)
		if err != nil {
			t.Fatalf("ListReplayLeaveReasons(%d): %v", r.ID, err)
		}
		for i, row := range leaves {
			if !ids[row.PlayerID] || row.Reason == "" {
				t.Fatalf("leave row %d = %+v", i, row)
			}
		}
		totals["leaves"] += len(leaves)

		chat, err := s.ListReplayChat(ctx, r.ID)
		if err != nil {
			t.Fatalf("ListReplayChat(%d): %v", r.ID, err)
		}
		for i, row := range chat {
			if !ids[row.PlayerID] || row.Message == "" {
				t.Fatalf("chat row %d = %+v", i, row)
			}
			if i > 0 && (chat[i-1].Second > row.Second ||
				(chat[i-1].Second == row.Second && chat[i-1].PlayerID > row.PlayerID)) {
				t.Fatalf("chat rows are not ordered in %s", summary.FileName)
			}
		}
		totals["chat"] += len(chat)
	}

	t.Logf("corpus row totals: %v", totals)
	for _, key := range []string{"detail", "replay_patterns", "player_patterns", "events", "production", "slices", "firsts", "cadence", "gas", "upgrades"} {
		if totals[key] == 0 {
			t.Errorf("the corpus produced no %s rows at all", key)
		}
	}
}
