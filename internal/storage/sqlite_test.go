package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/parser"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// Regression for #234: a panic while storing one replay must be recovered into
// a per-replay error (not crash the ingest), so the loop can skip it and keep
// going. Passing nil data forces a nil dereference inside storeReplayWithBatching.
func TestStoreReplayRecovered_PanicBecomesError(t *testing.T) {
	dir := t.TempDir()
	prev, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })

	s := &SQLiteStorage{}
	err, panicked := s.storeReplayRecovered(context.Background(), nil)
	if !panicked {
		t.Fatalf("expected panicked=true for nil replay data")
	}
	if err == nil {
		t.Fatalf("expected a non-nil error describing the recovered panic")
	}
}

// Regression for #234: a command whose Player could not be resolved (PlayerIDs
// are not guaranteed contiguous) must not panic the persistence pass.
func TestUpdateEntityIDs_NilCommandPlayer(t *testing.T) {
	s := &SQLiteStorage{}
	player := &models.Player{PlayerID: 0, ID: 42}
	data := &models.ReplayData{
		Replay:  &models.Replay{},
		Players: []*models.Player{player},
		Commands: []*models.Command{
			{Player: player, PlayerID: 0},
			{Player: nil, PlayerID: 7},
		},
	}

	s.updateEntityIDs(data, 99, map[byte]int64{0: 42})

	if data.Commands[0].PlayerID != 42 {
		t.Fatalf("resolvable command: got PlayerID %d, want 42", data.Commands[0].PlayerID)
	}
	if data.Commands[0].ReplayID != 99 || data.Commands[1].ReplayID != 99 {
		t.Fatalf("both commands should get replayID 99, got %d and %d", data.Commands[0].ReplayID, data.Commands[1].ReplayID)
	}
}

func TestEncodeInt64ArrayJSON_MatchesEncodingJSON(t *testing.T) {
	cases := [][]int64{
		nil,
		{},
		{0},
		{1, 2, 3},
		{-1, 0, 1},
		{9223372036854775807, -9223372036854775808},
		{42, 42, 42, 42, 42, 42, 42, 42},
	}
	for _, ids := range cases {
		got := encodeInt64ArrayJSON(ids)
		var want string
		if ids == nil {
			want = "null"
		} else {
			b, err := json.Marshal(ids)
			if err != nil {
				t.Fatalf("json.Marshal(%v): %v", ids, err)
			}
			want = string(b)
		}
		if got != want {
			t.Fatalf("encodeInt64ArrayJSON(%v) = %q, want %q", ids, got, want)
		}
	}
}

const (
	testDBPath      = "file:screpdb_test?mode=memory&cache=shared"
	testDBPathFlags = "file:screpdb_test_flags?mode=memory&cache=shared"
)

func TestSQLiteStorage_IngestionAndQueries(t *testing.T) {
	ctx := context.Background()

	store, err := NewSQLiteStorage(testDBPath)
	if err != nil {
		t.Fatalf("NewSQLiteStorage: %v", err)
	}
	defer store.Close()

	if err := store.Initialize(ctx, true, true); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	replaysDir, err := resolveReplayDir()
	if err != nil {
		t.Fatalf("resolveReplayDir: %v", err)
	}
	files, err := fileops.GetReplayFiles(replaysDir)
	if err != nil {
		t.Fatalf("GetReplayFiles: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no replays found in %s", replaysDir)
	}

	if err := ingestFiles(ctx, store, files); err != nil {
		t.Fatalf("ingestFiles: %v", err)
	}

	// Golden counts (established from current test replays). Post markers-migration,
	// all marker detections live in replay_events with event_kind='marker' alongside
	// narrative game_events — the single count here is their sum.
	expectedCounts := map[string]int64{
		"replays": 4,
		"players": 14,
		// Most classifiable players land on an opener (named BO or a per-race
		// residual). The Terran composition BOs (issue #155) are matchup-gated
		// (Wraith/Goliath TvZ, Bio TvZ-or-non-1v1), so a few off-matchup Terran
		// players whose composition matches a gated BO get no opener row — the
		// deliberate coverage gap documented on tNamed.
		//
		// Down from 207 after unit-production began maintaining base ownership:
		// a Train/Morph proves the producing building's base is alive, so
		// location_inactive (timeout) events dropped 35→21 (-14) and one attack
		// on a now-owned base surfaced (+1), for a net -13. Then -1 (issue #163):
		// a player who did only non-HP upgrades no longer trips Never-researched.
		// Then -2 (issue #175): inferred production/research coordinates now flow
		// through the ownership pass, so a producing building further refreshes
		// its base's inactivity clock — netting two fewer events.
		// Then +1 (issue #182): a tier-1 preferred opener now matches a player
		// whose tier-2 opener previously didn't (the muta/reaver tech-pathway
		// openers detect where the broad bucket fell through).
		// Then +13 (issue #186): the 1v1 bilateral-fight attack model replaces
		// the unit-novelty filter, surfacing the distinct real engagements it
		// used to collapse. Multiplayer/BGH replays here are unchanged (the new
		// path is gated to exactly-two-opposing-player games); the gain is on
		// the 1v1 test replays.
		// Then +N (issue #185 folded in): dt_drop/reaver_drop removed (subtype
		// change, no count effect) and the one-reaver_drop-per-player
		// suppression replaced by a per-target time-window dedup.
		// Then +1 (issue #194): the conservative "Muta hit-n-run" presence
		// marker fires on the one ingested muta game (one marker row).
		// Then +10 (issue #225): new Protoss timing/composition/spatial signals on
		// the ingested Protoss games — First Reaver/Corsair, Speedlot timing,
		// Sair/Speedlot and manner_pylon markers, plus the first_reaver/
		// first_corsair/speedlot/manner_pylon timeline game_events.
		// Then +1 (round 8): 3 Hatch Muta is now a composition marker that fires
		// on top of the hatch opener, adding one marker row on the ingested
		// 3-Hatch-Muta game.
		"replay_events": 215,
	}
	actualCounts, err := collectCounts(store, keys(expectedCounts))
	if err != nil {
		t.Fatalf("collectCounts: %v", err)
	}
	if mismatch := compareCounts(expectedCounts, actualCounts); mismatch != "" {
		t.Fatalf("count mismatch: %s (actual=%v)", mismatch, actualCounts)
	}

	commandRows, err := countAcrossCommandTables(ctx, store, "")
	if err != nil {
		t.Fatalf("countAcrossCommandTables total: %v", err)
	}
	if commandRows <= 0 {
		t.Fatalf("expected command rows to be present after ingestion")
	}

	highValueCommands, err := countTable(ctx, store, "commands")
	if err != nil {
		t.Fatalf("count commands: %v", err)
	}
	lowValueCommands, err := countTable(ctx, store, "commands_low_value")
	if err != nil {
		t.Fatalf("count commands_low_value: %v", err)
	}
	if highValueCommands <= 0 {
		t.Fatalf("expected high-value commands to be present")
	}
	if lowValueCommands <= 0 {
		t.Fatalf("expected low-value commands to be present")
	}

	rightClickRows, err := countAcrossCommandTables(ctx, store, "Right Click")
	if err != nil {
		t.Fatalf("countAcrossCommandTables right click: %v", err)
	}
	if rightClickRows != 0 {
		t.Fatalf("expected default ingestion to skip Right Click commands, got %d rows", rightClickRows)
	}

	hotkeyRows, err := store.Query(ctx, "SELECT COUNT(*) AS c FROM replay_events WHERE event_kind = 'marker' AND event_type = 'used_hotkey_groups'")
	if err != nil {
		t.Fatalf("query hotkey pattern count: %v", err)
	}
	hotkeyCount, ok := asInt64(hotkeyRows[0]["c"])
	if !ok {
		t.Fatalf("expected numeric hotkey pattern count")
	}
	if hotkeyCount == 0 {
		t.Fatalf("expected used_hotkey_groups marker detections to be present")
	}

	baseStateRows, err := store.Query(ctx, "SELECT event_type, seconds_from_game_start, source_player_id FROM replay_events ORDER BY id ASC")
	if err != nil {
		t.Fatalf("query replay events: %v", err)
	}
	if len(baseStateRows) == 0 {
		t.Fatalf("expected replay events to be present")
	}
	hasPlayerStart := false
	for _, row := range baseStateRows {
		eventType, ok := asString(row["event_type"])
		if !ok || strings.TrimSpace(eventType) == "" {
			t.Fatalf("expected replay event_type to be present")
		}
		if strings.EqualFold(eventType, "player_start") {
			hasPlayerStart = true
		}
	}
	if !hasPlayerStart {
		t.Fatalf("expected player_start replay events to be present")
	}

	// ReplayExists and FilterOutExistingReplays
	first := files[0]
	exists, err := store.ReplayExists(ctx, first.Path, first.Checksum)
	if err != nil {
		t.Fatalf("ReplayExists: %v", err)
	}
	if !exists {
		t.Fatalf("expected replay to exist after ingestion")
	}

	filtered, err := store.FilterOutExistingReplays(ctx, files)
	if err != nil {
		t.Fatalf("FilterOutExistingReplays: %v", err)
	}
	if len(filtered) != 0 {
		t.Fatalf("expected 0 filtered files after ingestion, got %d", len(filtered))
	}

	// Path-only dedup must agree with checksum-aware dedup when paths match.
	pathFiltered, err := store.FilterOutExistingReplaysByPath(ctx, files)
	if err != nil {
		t.Fatalf("FilterOutExistingReplaysByPath: %v", err)
	}
	if len(pathFiltered) != 0 {
		t.Fatalf("expected 0 path-filtered files after ingestion, got %d", len(pathFiltered))
	}

	// And it must NOT need Checksum to be set — that's the whole point.
	withoutChecksums := make([]fileops.FileInfo, len(files))
	for i, f := range files {
		f.Checksum = ""
		withoutChecksums[i] = f
	}
	pathFilteredNoSum, err := store.FilterOutExistingReplaysByPath(ctx, withoutChecksums)
	if err != nil {
		t.Fatalf("FilterOutExistingReplaysByPath without checksums: %v", err)
	}
	if len(pathFilteredNoSum) != 0 {
		t.Fatalf("expected 0 path-filtered files (no-checksum input), got %d", len(pathFilteredNoSum))
	}

	// A path that's not in the DB must survive.
	novel := []fileops.FileInfo{{Path: "/path/that/does/not/exist.rep", Name: "does-not-exist.rep"}}
	survivors, err := store.FilterOutExistingReplaysByPath(ctx, novel)
	if err != nil {
		t.Fatalf("FilterOutExistingReplaysByPath novel: %v", err)
	}
	if len(survivors) != 1 {
		t.Fatalf("expected novel path to survive, got %d", len(survivors))
	}

	// Schema should mention key tables
	schema, err := store.GetDatabaseSchema(ctx)
	if err != nil {
		t.Fatalf("GetDatabaseSchema: %v", err)
	}
	assertContains(t, schema, "## replays")
	assertContains(t, schema, "## players")
	assertContains(t, schema, "## commands")
	assertContains(t, schema, "## commands_low_value")

	// Legacy detected_patterns_replay table is dropped post markers-migration.
	// Assert replay_events (the unified home for markers + game events) carries
	// event_kind + payload columns introduced by migration 000008.
	patternSchemaRows, err := store.Query(ctx, "PRAGMA table_info(replay_events)")
	if err != nil {
		t.Fatalf("PRAGMA table_info(replay_events): %v", err)
	}
	hasEventKind := false
	hasPayload := false
	for _, row := range patternSchemaRows {
		colName, ok := asString(row["name"])
		if !ok {
			continue
		}
		if colName == "event_kind" {
			hasEventKind = true
		}
		if colName == "payload" {
			hasPayload = true
		}
	}
	if !hasEventKind {
		t.Fatalf("expected replay_events.event_kind column after markers migration")
	}
	if !hasPayload {
		t.Fatalf("expected replay_events.payload column after markers migration")
	}

	// Pattern detection filters and deletes
	patternFiltered, err := store.FilterOutExistingPatternDetections(ctx, files, core.AlgorithmVersion)
	if err != nil {
		t.Fatalf("FilterOutExistingPatternDetections: %v", err)
	}
	if len(patternFiltered) != 0 {
		t.Fatalf("expected 0 filtered pattern files after ingestion, got %d", len(patternFiltered))
	}

	replayID, replayPath, replayChecksum := firstReplayInfo(t, store)
	if err := store.DeletePatternDetectionsForReplay(ctx, replayID); err != nil {
		t.Fatalf("DeletePatternDetectionsForReplay: %v", err)
	}

	afterDeleteFiltered, err := store.FilterOutExistingPatternDetections(ctx, files, core.AlgorithmVersion)
	if err != nil {
		t.Fatalf("FilterOutExistingPatternDetections after delete: %v", err)
	}
	if !containsFile(afterDeleteFiltered, replayPath, replayChecksum) {
		t.Fatalf("expected replay %s to require pattern detection after delete", replayPath)
	}
}

func TestSQLiteStorage_CommandStorageFlags(t *testing.T) {
	ctx := context.Background()

	store, err := NewSQLiteStorage(testDBPathFlags)
	if err != nil {
		t.Fatalf("NewSQLiteStorage: %v", err)
	}
	defer store.Close()

	store.SetCommandStorageOptions(true, true)
	if err := store.Initialize(ctx, true, true); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	replaysDir, err := resolveReplayDir()
	if err != nil {
		t.Fatalf("resolveReplayDir: %v", err)
	}
	files, err := fileops.GetReplayFiles(replaysDir)
	if err != nil {
		t.Fatalf("GetReplayFiles: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no replays found in %s", replaysDir)
	}
	if err := ingestFiles(ctx, store, files); err != nil {
		t.Fatalf("ingestFiles: %v", err)
	}

	rightClickRows, err := countAcrossCommandTables(ctx, store, "Right Click")
	if err != nil {
		t.Fatalf("countAcrossCommandTables right click: %v", err)
	}
	if rightClickRows <= 0 {
		t.Fatalf("expected Right Click commands when store-right-clicks is enabled")
	}

	hotkeyRows, err := countAcrossCommandTables(ctx, store, "Hotkey")
	if err != nil {
		t.Fatalf("countAcrossCommandTables hotkey: %v", err)
	}
	if hotkeyRows != 0 {
		t.Fatalf("expected skip-hotkeys to remove Hotkey commands, got %d rows", hotkeyRows)
	}
}

func ingestFiles(ctx context.Context, store *SQLiteStorage, files []fileops.FileInfo) error {
	dataChan, errChan := store.StartIngestion(ctx, IngestionHooks{})
	for i := range files {
		fileInfo := files[i]
		replay := parser.CreateReplayFromFileInfo(fileInfo.Path, fileInfo.Name, fileInfo.Size, fileInfo.Checksum)
		data, err := parser.ParseReplay(fileInfo.Path, replay)
		if err != nil {
			return err
		}
		dataChan <- data
	}
	close(dataChan)
	if err := <-errChan; err != nil {
		return err
	}
	return nil
}

func collectCounts(store *SQLiteStorage, tables []string) (map[string]int64, error) {
	counts := make(map[string]int64, len(tables))
	for _, table := range tables {
		rows, err := store.Query(context.Background(), "SELECT COUNT(*) AS c FROM "+table)
		if err != nil {
			return nil, err
		}
		if len(rows) != 1 {
			return nil, fmt.Errorf("expected 1 row for %s count, got %d", table, len(rows))
		}
		got, ok := asInt64(rows[0]["c"])
		if !ok {
			return nil, fmt.Errorf("expected numeric count for %s", table)
		}
		counts[table] = got
	}
	return counts, nil
}

func countTable(ctx context.Context, store *SQLiteStorage, table string) (int64, error) {
	rows, err := store.Query(ctx, "SELECT COUNT(*) AS c FROM "+table)
	if err != nil {
		return 0, err
	}
	if len(rows) != 1 {
		return 0, fmt.Errorf("expected 1 row for %s count, got %d", table, len(rows))
	}
	got, ok := asInt64(rows[0]["c"])
	if !ok {
		return 0, fmt.Errorf("expected numeric count for %s", table)
	}
	return got, nil
}

func countAcrossCommandTables(ctx context.Context, store *SQLiteStorage, actionType string) (int64, error) {
	baseQuery := `
		SELECT COUNT(*) AS c
		FROM (
			SELECT action_type FROM commands
			UNION ALL
			SELECT action_type FROM commands_low_value
		)
	`
	args := []any{}
	if actionType != "" {
		baseQuery += " WHERE action_type = ?"
		args = append(args, actionType)
	}
	rows, err := store.Query(ctx, baseQuery, args...)
	if err != nil {
		return 0, err
	}
	if len(rows) != 1 {
		return 0, fmt.Errorf("expected 1 row for command table count, got %d", len(rows))
	}
	got, ok := asInt64(rows[0]["c"])
	if !ok {
		return 0, errors.New("expected numeric command table count")
	}
	return got, nil
}

func compareCounts(expected, actual map[string]int64) string {
	for table, exp := range expected {
		if act, ok := actual[table]; !ok || act != exp {
			return table
		}
	}
	return ""
}

func keys(m map[string]int64) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

func resolveReplayDir() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", os.ErrNotExist
	}
	baseDir := filepath.Dir(thisFile)
	candidates := []string{
		filepath.Join(baseDir, "..", "testdata", "replays"),
		filepath.Join(baseDir, "..", "..", "testutils", "replays"),
	}
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir, nil
		}
	}
	return "", os.ErrNotExist
}

func firstReplayInfo(t *testing.T, store *SQLiteStorage) (int64, string, string) {
	t.Helper()
	rows, err := store.Query(context.Background(), "SELECT id, file_path, file_checksum FROM replays ORDER BY id LIMIT 1")
	if err != nil {
		t.Fatalf("Query first replay: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 replay row, got %d", len(rows))
	}
	id, ok := asInt64(rows[0]["id"])
	if !ok {
		t.Fatalf("expected numeric id for replay")
	}
	path, ok := asString(rows[0]["file_path"])
	if !ok {
		t.Fatalf("expected string file_path")
	}
	checksum, ok := asString(rows[0]["file_checksum"])
	if !ok {
		t.Fatalf("expected string file_checksum")
	}
	return id, path, checksum
}

func containsFile(files []fileops.FileInfo, path, checksum string) bool {
	for _, f := range files {
		if f.Path == path || f.Checksum == checksum {
			return true
		}
	}
	return false
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected schema to contain %q", needle)
	}
}

func asInt64(v any) (int64, bool) {
	switch val := v.(type) {
	case int64:
		return val, true
	case int32:
		return int64(val), true
	case int:
		return int64(val), true
	case uint64:
		return int64(val), true
	case []byte:
		parsed, err := strconv.ParseInt(string(val), 10, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	case string:
		parsed, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func asString(v any) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case []byte:
		return string(val), true
	default:
		return "", false
	}
}
