package dashboard

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/persist"
)

func replayFolder(t *testing.T, names ...string) string {
	t.Helper()
	source := filepath.Join("..", "testdata", "replays")
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Skipf("no replay corpus: %v", err)
	}
	dir := t.TempDir()
	wanted := map[string]bool{}
	for _, name := range names {
		wanted[name] = true
	}
	copied := 0
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".rep" || (len(wanted) > 0 && !wanted[entry.Name()]) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, entry.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
		copied++
	}
	if copied == 0 {
		t.Skip("no replay files in corpus")
	}
	return dir
}

func newTestRuntime(t *testing.T, opts libraryRuntimeOptions) *libraryRuntime {
	t.Helper()
	t.Cleanup(iofacade.Reset)
	if opts.Root == "" {
		opts.Root = t.TempDir()
	}
	runtime, err := newLibraryRuntime(context.Background(), opts)
	if err != nil {
		t.Fatalf("newLibraryRuntime: %v", err)
	}
	t.Cleanup(runtime.Close)
	return runtime
}

func waitForCorpus(t *testing.T, runtime *libraryRuntime, want int) {
	t.Helper()
	deadline := time.After(20 * time.Second)
	for {
		if runtime.lib.Snapshot().Len() == want {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("corpus holds %d replays, want %d (phase %q)", runtime.lib.Snapshot().Len(), want, runtime.lib.Progress().Phase)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func TestLibraryRuntimeLoadsTheGivenFolder(t *testing.T) {
	folder := replayFolder(t, "SomaJyJ.rep")
	runtime := newTestRuntime(t, libraryRuntimeOptions{ReplayDir: folder, Hub: newLibraryHub()})

	if got := runtime.Folder(); got != folder {
		t.Fatalf("folder = %q, want %q", got, folder)
	}
	if runtime.sampleSetAutoLoaded {
		t.Fatal("an explicit folder is not the example replays")
	}
	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForCorpus(t, runtime, 1)
	if !runtime.settings.Settings().GlobalFilter.Equal(library.DefaultFilterConfig()) {
		t.Fatalf("filter = %+v, want the defaults", runtime.settings.Settings().GlobalFilter)
	}
}

func TestLibraryRuntimeRemembersTheFolderAcrossRestarts(t *testing.T) {
	folder := replayFolder(t, "SomaJyJ.rep")
	root := t.TempDir()
	first := newTestRuntime(t, libraryRuntimeOptions{Root: root, ReplayDir: folder})
	first.Close()

	second := newTestRuntime(t, libraryRuntimeOptions{Root: root})
	if got := second.Folder(); got != folder {
		t.Fatalf("a restart forgot the folder: %q", got)
	}
	if second.sampleSetAutoLoaded {
		t.Fatal("a remembered folder is not the example replays")
	}
}

func TestLibraryRuntimeSwitchesFolders(t *testing.T) {
	first := replayFolder(t, "SomaJyJ.rep")
	second := replayFolder(t, "bgh.rep")
	runtime := newTestRuntime(t, libraryRuntimeOptions{ReplayDir: first})
	if err := runtime.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitForCorpus(t, runtime, 1)

	if err := runtime.SetFolder(context.Background(), second); err != nil {
		t.Fatalf("SetFolder: %v", err)
	}
	deadline := time.After(20 * time.Second)
	for {
		snap := runtime.lib.Snapshot()
		if snap.Len() == 1 && snap.Replays[0].FileName() == "bgh.rep" {
			break
		}
		select {
		case <-deadline:
			t.Fatal("the corpus never switched to the second folder")
		case <-time.After(20 * time.Millisecond):
		}
	}
	if got := runtime.settings.Settings().ReplayFolder; got != second {
		t.Fatalf("the new folder was not persisted: %q", got)
	}
	if err := runtime.SetFolder(context.Background(), t.TempDir()); err == nil {
		t.Fatal("a folder with no replays should be rejected")
	}
}

func TestLibraryRuntimeUsesTheExampleReplaysWhenAskedTo(t *testing.T) {
	folder := replayFolder(t, "SomaJyJ.rep")
	runtime := newTestRuntime(t, libraryRuntimeOptions{ReplayDir: folder})
	if err := runtime.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	dir, err := runtime.UseSampleSet(context.Background())
	if err != nil {
		t.Fatalf("UseSampleSet: %v", err)
	}
	if dir != runtime.sampleSetDir() || !runtime.IsSampleSetActive() {
		t.Fatalf("sample set not active: folder %q", runtime.Folder())
	}
}

func TestLibraryRuntimeCarriesOverTheLegacyDatabaseOnce(t *testing.T) {
	folder := replayFolder(t, "SomaJyJ.rep")
	root := t.TempDir()
	legacy := filepath.Join(root, "screp.db")
	conn, err := sql.Open("sqlite", "file:"+legacy)
	if err != nil {
		t.Fatal(err)
	}
	// Spelled out because the migrations no longer create these: they belonged
	// to the dashboard, which keeps its state in JSON files now.
	if _, err := conn.Exec(`
		CREATE TABLE settings (
			config_key TEXT PRIMARY KEY,
			game_type TEXT NOT NULL DEFAULT 'all',
			exclude_short_games BOOLEAN NOT NULL DEFAULT 1,
			exclude_computers BOOLEAN NOT NULL DEFAULT 1,
			ingest_input_dir TEXT NOT NULL DEFAULT '',
			game_types TEXT NOT NULL DEFAULT '[]',
			map_kinds TEXT NOT NULL DEFAULT '[]'
		);
		INSERT INTO settings (config_key) VALUES ('global');
		CREATE TABLE bnet_profiles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			toon TEXT NOT NULL,
			gateway INTEGER NOT NULL,
			found BOOLEAN NOT NULL,
			aurora_id INTEGER NOT NULL DEFAULT 0,
			battle_tag TEXT NOT NULL DEFAULT '',
			country_code TEXT NOT NULL DEFAULT '',
			payload TEXT NOT NULL,
			fetched_at TEXT NOT NULL,
			UNIQUE (toon, gateway)
		);`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`UPDATE settings SET ingest_input_dir = ?, exclude_short_games = 0, game_types = '["one_on_one"]', map_kinds = '["money"]' WHERE config_key = 'global'`, folder); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO bnet_profiles (toon, gateway, found, aurora_id, battle_tag, country_code, payload, fetched_at) VALUES ('Bisu', 30, 1, 42, 'Bisu#1', 'KR', '{"a":1}', ?)`,
		time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	conn.Close()

	runtime := newTestRuntime(t, libraryRuntimeOptions{Root: root, LegacyDBPath: legacy})
	if got := runtime.Folder(); got != folder {
		t.Fatalf("the legacy replay folder was not carried over: %q", got)
	}
	filter := runtime.settings.Settings().GlobalFilter
	if filter.ExcludeShortGames || len(filter.GameTypes) != 1 || filter.GameTypes[0] != "one_on_one" {
		t.Fatalf("the legacy filter was not carried over: %+v", filter)
	}
	profile, err := runtime.bnet.Get("Bisu", 30)
	if err != nil || profile == nil || profile.AuroraID != 42 {
		t.Fatalf("the legacy Battle.net cache was not carried over: %+v %v", profile, err)
	}
	if _, err := os.Stat(persist.SettingsPath(root)); err != nil {
		t.Fatalf("settings.json was not written: %v", err)
	}
}
