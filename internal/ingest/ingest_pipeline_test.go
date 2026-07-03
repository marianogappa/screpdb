package ingest

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/storage"
)

// smallTestReplays are the two smallest committed replays; keeping the corpus
// tiny keeps the end-to-end ingest tests fast.
var smallTestReplays = []string{
	"bo_bbs_tvp_standordie.rep",
	"bo_8pool_zvt_loveaddio.rep",
}

func testdataReplayDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "..", "patterns", "markers", "testdata", "replays")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("testdata replay dir not found at %s: %v", dir, err)
	}
	return dir
}

// seedReplayDir copies the named replays into a fresh temp dir and returns it.
// The temp dir is used as an ingest InputDir; Run registers it with the facade.
func seedReplayDir(t *testing.T, names ...string) string {
	t.Helper()
	src := testdataReplayDir(t)
	dst := t.TempDir()
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatalf("read source replay %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dst, name), data, 0o644); err != nil {
			t.Fatalf("write replay %s: %v", name, err)
		}
	}
	return dst
}

func quietLogger() *Logger {
	return NewLogger(&bytes.Buffer{}, false, nil)
}

func countRows(t *testing.T, dbPath, table string) int64 {
	t.Helper()
	store, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	rows, err := store.Query(context.Background(), "SELECT COUNT(*) AS c FROM "+table)
	if err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	c, ok := rows[0]["c"].(int64)
	if !ok {
		t.Fatalf("count %s: non-int64 result %T", table, rows[0]["c"])
	}
	return c
}

func TestRun_EndToEndPopulatesTables(t *testing.T) {
	inputDir := seedReplayDir(t, smallTestReplays...)
	dbPath := filepath.Join(t.TempDir(), "x.db")

	cfg := Config{
		InputDir:   inputDir,
		SQLitePath: dbPath,
		Logger:     quietLogger(),
	}
	if err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := countRows(t, dbPath, "replays"); got != int64(len(smallTestReplays)) {
		t.Fatalf("replays: got %d, want %d", got, len(smallTestReplays))
	}
	// Both replays are 1v1, so exactly two players each.
	if got := countRows(t, dbPath, "players"); got != int64(2*len(smallTestReplays)) {
		t.Fatalf("players: got %d, want %d", got, 2*len(smallTestReplays))
	}
}

func TestRun_ReingestSkipsExisting(t *testing.T) {
	inputDir := seedReplayDir(t, smallTestReplays...)
	dbPath := filepath.Join(t.TempDir(), "x.db")
	cfg := Config{InputDir: inputDir, SQLitePath: dbPath, Logger: quietLogger()}

	if err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	firstCount := countRows(t, dbPath, "replays")

	// A second run over the same folder must dedup (path + checksum) and not
	// duplicate any rows.
	if err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if got := countRows(t, dbPath, "replays"); got != firstCount {
		t.Fatalf("reingest changed replay count: got %d, want %d", got, firstCount)
	}
}

func TestRun_StopAfterNLimitsIngest(t *testing.T) {
	inputDir := seedReplayDir(t, smallTestReplays...)
	dbPath := filepath.Join(t.TempDir(), "x.db")
	cfg := Config{
		InputDir:   inputDir,
		SQLitePath: dbPath,
		StopAfterN: 1,
		Logger:     quietLogger(),
	}
	if err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := countRows(t, dbPath, "replays"); got != 1 {
		t.Fatalf("StopAfterN=1: got %d replays, want 1", got)
	}
}

func TestRun_UpToDatePastExcludesAll(t *testing.T) {
	inputDir := seedReplayDir(t, smallTestReplays...)
	dbPath := filepath.Join(t.TempDir(), "x.db")
	cfg := Config{
		InputDir:   inputDir,
		SQLitePath: dbPath,
		UpToDate:   "1990-01-01",
		Logger:     quietLogger(),
	}
	if err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := countRows(t, dbPath, "replays"); got != 0 {
		t.Fatalf("UpToDate in the past should exclude every file, got %d replays", got)
	}
}

func TestRun_InvalidUpToDateErrors(t *testing.T) {
	inputDir := seedReplayDir(t, smallTestReplays[0])
	cfg := Config{
		InputDir:   inputDir,
		SQLitePath: filepath.Join(t.TempDir(), "x.db"),
		UpToDate:   "not-a-date",
		Logger:     quietLogger(),
	}
	err := Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected an error for a malformed UpToDate")
	}
	if !strings.Contains(err.Error(), "invalid date format") {
		t.Fatalf("expected invalid-date error, got %v", err)
	}
}

func TestRunForFiles_PopulatesTables(t *testing.T) {
	inputDir := seedReplayDir(t, smallTestReplays...)
	dbPath := filepath.Join(t.TempDir(), "x.db")
	_ = iofacade.AllowDir(inputDir)

	files, err := fileops.GetReplayFiles(inputDir)
	if err != nil {
		t.Fatalf("GetReplayFiles: %v", err)
	}
	hashed, err := fileops.HashFiles(context.Background(), files)
	if err != nil {
		t.Fatalf("HashFiles: %v", err)
	}

	cfg := Config{SQLitePath: dbPath, Logger: quietLogger()}
	if err := RunForFiles(context.Background(), cfg, hashed); err != nil {
		t.Fatalf("RunForFiles: %v", err)
	}
	if got := countRows(t, dbPath, "replays"); got != int64(len(smallTestReplays)) {
		t.Fatalf("RunForFiles replays: got %d, want %d", got, len(smallTestReplays))
	}
}

func TestRunForFiles_EmptyIsNoop(t *testing.T) {
	// An empty file list must not even open storage; passing an unwritable path
	// proves storage was never touched.
	cfg := Config{SQLitePath: "/nonexistent-dir/should-not-be-created/x.db", Logger: quietLogger()}
	if err := RunForFiles(context.Background(), cfg, nil); err != nil {
		t.Fatalf("RunForFiles with no files should be a no-op, got %v", err)
	}
}

func TestBatchCheckExistingReplays_DedupSemantics(t *testing.T) {
	inputDir := seedReplayDir(t, smallTestReplays...)
	dbPath := filepath.Join(t.TempDir(), "x.db")
	cfg := Config{InputDir: inputDir, SQLitePath: dbPath, Logger: quietLogger()}
	if err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("seed Run: %v", err)
	}

	store, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.Initialize(context.Background(), false, false); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	_ = iofacade.AllowDir(inputDir)
	known, err := fileops.GetReplayFiles(inputDir)
	if err != nil {
		t.Fatalf("GetReplayFiles: %v", err)
	}
	hashedKnown, err := fileops.HashFiles(context.Background(), known)
	if err != nil {
		t.Fatalf("HashFiles: %v", err)
	}

	ctx := context.Background()
	logger := quietLogger()

	// All seeded files are known by path.
	pathSurvivors, err := batchCheckExistingReplaysByPath(ctx, store, known, logger)
	if err != nil {
		t.Fatalf("batchCheckExistingReplaysByPath: %v", err)
	}
	if len(pathSurvivors) != 0 {
		t.Fatalf("expected all known files skipped by path, %d survived", len(pathSurvivors))
	}

	// All seeded files are known by checksum.
	sumSurvivors, err := batchCheckExistingReplays(ctx, store, hashedKnown, logger)
	if err != nil {
		t.Fatalf("batchCheckExistingReplays: %v", err)
	}
	if len(sumSurvivors) != 0 {
		t.Fatalf("expected all known files skipped by checksum, %d survived", len(sumSurvivors))
	}

	// A novel path survives the path pass.
	novel := []fileops.FileInfo{{Path: filepath.Join(inputDir, "unknown.rep"), Name: "unknown.rep"}}
	novelSurvivors, err := batchCheckExistingReplaysByPath(ctx, store, novel, logger)
	if err != nil {
		t.Fatalf("batchCheckExistingReplaysByPath novel: %v", err)
	}
	if len(novelSurvivors) != 1 {
		t.Fatalf("expected novel path to survive, got %d", len(novelSurvivors))
	}
}

func TestWithDefaults_FillsMissingAndPreservesSet(t *testing.T) {
	got := withDefaults(Config{})
	if got.InputDir == "" {
		// GetDefaultReplayDir can legitimately return "" on a machine with no
		// StarCraft install; only assert on the fields we can guarantee.
		t.Logf("InputDir defaulted to empty (no replay dir resolvable)")
	}
	if got.SQLitePath == "" {
		t.Fatal("withDefaults should populate a non-empty SQLitePath")
	}
	if got.Logger == nil {
		t.Fatal("withDefaults should populate a Logger")
	}

	// Provided values must be preserved untouched.
	custom := Config{
		InputDir:   "/my/replays",
		SQLitePath: "/my/db.sqlite",
		Logger:     quietLogger(),
	}
	out := withDefaults(custom)
	if out.InputDir != custom.InputDir {
		t.Fatalf("InputDir overwritten: got %q", out.InputDir)
	}
	if out.SQLitePath != custom.SQLitePath {
		t.Fatalf("SQLitePath overwritten: got %q", out.SQLitePath)
	}
	if out.Logger != custom.Logger {
		t.Fatal("Logger overwritten")
	}
}

func TestRun_UpToDateFutureIncludesAll(t *testing.T) {
	inputDir := seedReplayDir(t, smallTestReplays...)
	dbPath := filepath.Join(t.TempDir(), "x.db")
	future := time.Now().AddDate(1, 0, 0).Format("2006-01-02")
	cfg := Config{
		InputDir:   inputDir,
		SQLitePath: dbPath,
		UpToDate:   future,
		Logger:     quietLogger(),
	}
	if err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := countRows(t, dbPath, "replays"); got != int64(len(smallTestReplays)) {
		t.Fatalf("future UpToDate should include all files, got %d", got)
	}
}
