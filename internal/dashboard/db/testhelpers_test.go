package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/marianogappa/screpdb/internal/migrations"
)

// newTestStore builds a fully-migrated temp database and returns a Store
// wired to a single shared connection. Both the default and replay-scoped
// connections point at the same DB; the sqlc queries reference the base
// tables unqualified, so no temp-view rewrite is needed in tests.
func newTestStore(t *testing.T) (*Store, *sql.DB) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	if err := migrations.RunMigrations(dbPath); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	conn, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	store := NewStore(conn,
		func() *sql.DB { return conn },
		func(_ *string, fn func(*sql.DB) error) error { return fn(conn) },
	)
	return store, conn
}

func mustExec(t *testing.T, conn *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

// seedReplay inserts one replay row and returns its id.
func seedReplay(t *testing.T, conn *sql.DB, r replayFixture) int64 {
	t.Helper()
	res, err := conn.Exec(`
		INSERT INTO replays (
			file_path, file_checksum, file_name, created_at, replay_date,
			map_name, map_width, map_height, duration_seconds, frame_count,
			engine_version, engine, game_speed, game_type, home_team_size,
			avail_slots_count, map_kind, team_format, matchup, team_stacking,
			team_info_incomplete
		) VALUES (?, ?, ?, ?, ?, ?, 128, 128, ?, 0, '1.16', 'BW', 'Fastest', ?, '', 8, ?, ?, ?, ?, ?)`,
		r.filePath, r.checksum, r.fileName, "2024-01-01T00:00:00Z", r.replayDate,
		r.mapName, r.durationSeconds, r.gameType, r.mapKind, r.teamFormat, r.matchup,
		boolToInt(r.teamStacking), boolToInt(r.teamInfoIncomplete))
	if err != nil {
		t.Fatalf("seed replay: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// seedPlayer inserts one player row and returns its id.
func seedPlayer(t *testing.T, conn *sql.DB, p playerFixture) int64 {
	t.Helper()
	res, err := conn.Exec(`
		INSERT INTO players (
			replay_id, name, race, type, color, team, is_observer,
			apm, eapm, is_winner, start_location_oclock, slot_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.replayID, p.name, p.race, orDefault(p.ptype, "Human"), orDefault(p.color, "Red"),
		p.team, boolToInt(p.isObserver), p.apm, p.eapm, boolToInt(p.isWinner),
		p.startLocationOclock, p.slotID)
	if err != nil {
		t.Fatalf("seed player: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func seedCommand(t *testing.T, conn *sql.DB, replayID, playerID, second int64, actionType string, unitType *string) {
	t.Helper()
	mustExec(t, conn, `
		INSERT INTO commands (replay_id, player_id, frame, seconds_from_game_start, action_type, unit_type)
		VALUES (?, ?, ?, ?, ?, ?)`,
		replayID, playerID, second*24, second, actionType, unitType)
}

func seedMarker(t *testing.T, conn *sql.DB, replayID int64, sourcePlayerID *int64, eventType string, second int64, payload *string) {
	t.Helper()
	mustExec(t, conn, `
		INSERT INTO replay_events (replay_id, seconds_from_game_start, event_kind, event_type, source_player_id, payload)
		VALUES (?, ?, 'marker', ?, ?, ?)`,
		replayID, second, eventType, sourcePlayerID, payload)
}

func seedGameEvent(t *testing.T, conn *sql.DB, replayID int64, eventType string, second int64) {
	t.Helper()
	mustExec(t, conn, `
		INSERT INTO replay_events (replay_id, seconds_from_game_start, event_kind, event_type)
		VALUES (?, ?, 'game_event', ?)`,
		replayID, second, eventType)
}

type replayFixture struct {
	filePath           string
	checksum           string
	fileName           string
	replayDate         string
	mapName            string
	durationSeconds    int64
	gameType           string
	mapKind            string
	teamFormat         string
	matchup            string
	teamStacking       bool
	teamInfoIncomplete bool
}

type playerFixture struct {
	replayID            int64
	name                string
	race                string
	ptype               string
	color               string
	team                int64
	isObserver          bool
	apm                 int64
	eapm                int64
	isWinner            bool
	startLocationOclock *int64
	slotID              int64
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func ptrStr(s string) *string { return &s }
func ptrI64(i int64) *int64   { return &i }
