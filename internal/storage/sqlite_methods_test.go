package storage

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/parser"
)

// newIngestedStore spins up a fresh file-backed store in a temp dir, runs a
// clean Initialize, and ingests the shared testdata replays. It returns the
// live store (closed via t.Cleanup) so method-level tests can query real rows.
func newIngestedStore(t *testing.T) *SQLiteStorage {
	t.Helper()
	ctx := context.Background()

	dbPath := filepath.Join(t.TempDir(), "screpdb_methods.db")
	store, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStorage: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.Initialize(ctx, true); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	replaysDir, err := resolveReplayDir()
	if err != nil {
		t.Fatalf("resolveReplayDir: %v", err)
	}
	_ = iofacade.AllowDir(replaysDir)
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
	return store
}

func TestIsDuplicateReplayError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"unrelated", errors.New("something else went wrong"), false},
		{"other unique", errors.New("UNIQUE constraint failed: players.name"), false},
		{"duplicate checksum", errors.New("UNIQUE constraint failed: replays.file_checksum"), true},
		{"wrapped duplicate", fmt.Errorf("insert: %w", errors.New("UNIQUE constraint failed: replays.file_checksum")), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isDuplicateReplayError(tc.err); got != tc.want {
				t.Fatalf("isDuplicateReplayError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestNormalizeEnumValue(t *testing.T) {
	allowed := map[string]struct{}{
		"Move":  {},
		"Train": {},
	}
	allowed[unknownEnumValue] = struct{}{}

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"known", "Move", "Move"},
		{"known with surrounding space", "  Train  ", "Train"},
		{"empty", "", unknownEnumValue},
		{"whitespace only", "   ", unknownEnumValue},
		{"unknown", "Teleport", unknownEnumValue},
		{"case sensitive miss", "move", unknownEnumValue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeEnumValue(tc.input, allowed); got != tc.want {
				t.Fatalf("normalizeEnumValue(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNewSQLiteStorage_BadPath(t *testing.T) {
	// A path under a directory that does not exist cannot be opened/pinged.
	badPath := filepath.Join(t.TempDir(), "no-such-dir", "nested", "db.sqlite")
	store, err := NewSQLiteStorage(badPath)
	if err == nil {
		if store != nil {
			_ = store.Close()
		}
		t.Fatalf("expected error opening DB under nonexistent directory %q", badPath)
	}
}

func TestInitialize_NonCleanIsIdempotent(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "init.db")
	store, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStorage: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// First run does a clean init; the second is non-clean and must not error or
	// drop the schema (migrations already applied are skipped).
	if err := store.Initialize(ctx, true); err != nil {
		t.Fatalf("Initialize clean: %v", err)
	}
	if err := store.Initialize(ctx, false); err != nil {
		t.Fatalf("Initialize non-clean: %v", err)
	}

	rows, err := store.Query(ctx, "SELECT COUNT(*) AS c FROM replays")
	if err != nil {
		t.Fatalf("expected replays table to exist after non-clean re-init: %v", err)
	}
	if c, ok := asInt64(rows[0]["c"]); !ok || c != 0 {
		t.Fatalf("expected empty replays table, got %v", rows[0]["c"])
	}
}

func TestFilterOutExistingReplays_NovelSurvives(t *testing.T) {
	ctx := context.Background()
	store := newIngestedStore(t)

	novel := []fileops.FileInfo{
		{Path: "/brand/new/one.rep", Name: "one.rep", Checksum: "sum-not-in-db"},
	}
	survivors, err := store.FilterOutExistingReplays(ctx, novel)
	if err != nil {
		t.Fatalf("FilterOutExistingReplays: %v", err)
	}
	if len(survivors) != 1 || survivors[0].Path != "/brand/new/one.rep" {
		t.Fatalf("expected the novel file to survive filtering, got %v", survivors)
	}
}

func TestQuery_BadSQLReturnsError(t *testing.T) {
	ctx := context.Background()
	store := newIngestedStore(t)

	if _, err := store.Query(ctx, "SELECT * FROM table_that_does_not_exist"); err == nil {
		t.Fatalf("expected error querying a nonexistent table")
	}
}

func TestGetDatabaseSchema_ContainsTables(t *testing.T) {
	ctx := context.Background()
	store := newIngestedStore(t)

	schema, err := store.GetDatabaseSchema(ctx)
	if err != nil {
		t.Fatalf("GetDatabaseSchema: %v", err)
	}
	for _, want := range []string{"## replays", "## players", "## commands"} {
		if !strings.Contains(schema, want) {
			t.Fatalf("expected schema to contain %q", want)
		}
	}
}

func TestStartIngestion_PanicRoutedToStoreErrorHook(t *testing.T) {
	ctx := context.Background()
	// Keep the recovered-panic crash report inside a temp dir rather than the
	// real app-data root (issue #237); crashreport.write resolves app-data via
	// this seam.
	t.Setenv("SCREPDB_APPDATA_DIR", t.TempDir())
	store := newIngestedStore(t)

	var storeErrs int
	hooks := IngestionHooks{OnStoreError: func(error) { storeErrs++ }}
	dataChan, errChan := store.StartIngestion(ctx, hooks)
	// nil replay data forces a recovered panic inside storeReplayRecovered,
	// which must surface via OnStoreError and let the run continue/close cleanly.
	dataChan <- nil
	close(dataChan)
	if err := <-errChan; err != nil {
		t.Fatalf("expected clean run close after recovered per-replay panic, got %v", err)
	}
	if storeErrs != 1 {
		t.Fatalf("expected 1 OnStoreError call from the panicked replay, got %d", storeErrs)
	}
}

func TestStartIngestion_EmptyChannelReturnsNil(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "empty_ingest.db")
	store, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStorage: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Initialize(ctx, true); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	dataChan, errChan := store.StartIngestion(ctx, IngestionHooks{})
	close(dataChan)
	if err := <-errChan; err != nil {
		t.Fatalf("expected nil error for empty ingestion, got %v", err)
	}
}

func TestStartIngestion_CancelledContext(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "cancel_ingest.db")
	store, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStorage: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Initialize(context.Background(), true); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, errChan := store.StartIngestion(ctx, IngestionHooks{})
	if err := <-errChan; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled from ingestion, got %v", err)
	}
}

func TestStartIngestion_DuplicateReplayHooked(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "dup_ingest.db")
	store, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStorage: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Initialize(ctx, true); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	replaysDir, err := resolveReplayDir()
	if err != nil {
		t.Fatalf("resolveReplayDir: %v", err)
	}
	_ = iofacade.AllowDir(replaysDir)
	files, err := fileops.GetReplayFiles(replaysDir)
	if err != nil {
		t.Fatalf("GetReplayFiles: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no replays found")
	}

	parse := func(fi fileops.FileInfo) *models.ReplayData {
		replay := parser.CreateReplayFromFileInfo(fi.Path, fi.Name, fi.Size, fi.Checksum)
		data, err := parser.ParseReplay(fi.Path, replay)
		if err != nil {
			t.Fatalf("ParseReplay: %v", err)
		}
		return data
	}

	var stored, dupes int
	hooks := IngestionHooks{
		OnReplayStored:    func() { stored++ },
		OnDuplicateReplay: func(error) { dupes++ },
	}
	dataChan, errChan := store.StartIngestion(ctx, hooks)
	// Send the same replay twice: the second insert trips the file_checksum
	// UNIQUE constraint and must be routed to OnDuplicateReplay, not fail the run.
	dataChan <- parse(files[0])
	dataChan <- parse(files[0])
	close(dataChan)
	if err := <-errChan; err != nil {
		t.Fatalf("expected nil run error with a duplicate, got %v", err)
	}
	if stored != 1 {
		t.Fatalf("expected 1 stored replay, got %d", stored)
	}
	if dupes != 1 {
		t.Fatalf("expected 1 duplicate hook call, got %d", dupes)
	}
}
