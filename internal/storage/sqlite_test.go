package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/parser"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

const (
	testDBPath = "file:screpdb_test?mode=memory&cache=shared"
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

	// Golden counts (established from current test replays)
	expectedCounts := map[string]int64{
		"replays":                         3,
		"players":                         6,
		"commands":                        23633,
		"detected_patterns_replay":        11,
		"detected_patterns_replay_team":   0,
		"detected_patterns_replay_player": 11,
	}
	actualCounts, err := collectCounts(store, keys(expectedCounts))
	if err != nil {
		t.Fatalf("collectCounts: %v", err)
	}
	if mismatch := compareCounts(expectedCounts, actualCounts); mismatch != "" {
		t.Fatalf("count mismatch: %s (actual=%v)", mismatch, actualCounts)
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

	// Schema should mention key tables
	schema, err := store.GetDatabaseSchema(ctx)
	if err != nil {
		t.Fatalf("GetDatabaseSchema: %v", err)
	}
	assertContains(t, schema, "## replays")
	assertContains(t, schema, "## players")
	assertContains(t, schema, "## commands")

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

func ingestFiles(ctx context.Context, store *SQLiteStorage, files []fileops.FileInfo) error {
	dataChan, errChan := store.StartIngestion(ctx)
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
