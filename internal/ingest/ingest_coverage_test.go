package ingest

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/hotkeystream"
	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/storage"

	_ "modernc.org/sqlite"
)

func countLowValueByAction(t *testing.T, dbPath, actionType string) int64 {
	t.Helper()
	store, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	rows, err := store.Query(context.Background(),
		"SELECT COUNT(*) AS c FROM commands_low_value WHERE action_type = ?", actionType)
	if err != nil {
		t.Fatalf("count low_value %s: %v", actionType, err)
	}
	c, ok := rows[0]["c"].(int64)
	if !ok {
		t.Fatalf("count %s: non-int64 result %T", actionType, rows[0]["c"])
	}
	return c
}

func querySingleString(t *testing.T, dbPath, sql string) string {
	t.Helper()
	store, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	rows, err := store.Query(context.Background(), sql)
	if err != nil {
		t.Fatalf("query %q: %v", sql, err)
	}
	if len(rows) == 0 {
		t.Fatalf("query %q returned no rows", sql)
	}
	for _, v := range rows[0] {
		if s, ok := v.(string); ok {
			return s
		}
	}
	t.Fatalf("query %q returned no string column: %#v", sql, rows[0])
	return ""
}

// TestRun_StoreRightClicksDefaultDropsRightClicks proves that the default
// (StoreRightClicks=false) drops Right Click commands entirely — they must not
// land in commands_low_value — while StoreRightClicks=true persists them.
func TestRun_StoreRightClicksDefaultDropsRightClicks(t *testing.T) {
	dbOff := filepath.Join(t.TempDir(), "off.db")
	if err := Run(context.Background(), Config{
		InputDir:   seedReplayDir(t, smallTestReplays...),
		SQLitePath: dbOff,
		Logger:     quietLogger(),
	}); err != nil {
		t.Fatalf("Run (default): %v", err)
	}
	if got := countLowValueByAction(t, dbOff, "Right Click"); got != 0 {
		t.Fatalf("StoreRightClicks default (false) should drop all Right Click commands, got %d", got)
	}

	dbOn := filepath.Join(t.TempDir(), "on.db")
	if err := Run(context.Background(), Config{
		InputDir:         seedReplayDir(t, smallTestReplays...),
		SQLitePath:       dbOn,
		StoreRightClicks: true,
		Logger:           quietLogger(),
	}); err != nil {
		t.Fatalf("Run (StoreRightClicks): %v", err)
	}
	if got := countLowValueByAction(t, dbOn, "Right Click"); got == 0 {
		t.Fatal("StoreRightClicks=true should persist Right Click commands, got 0")
	}
}

// TestRun_HotkeysStoredAsPlayerStream proves Hotkey commands never land as
// rows and are instead encoded into the players.hotkey_stream blob
// (issue #357). The decoded streams must be non-empty and frame-ordered,
// confirming the corpus actually contains hotkeys.
func TestRun_HotkeysStoredAsPlayerStream(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "hotkeys.db")
	if err := Run(context.Background(), Config{
		InputDir:   seedReplayDir(t, smallTestReplays...),
		SQLitePath: dbPath,
		Logger:     quietLogger(),
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := countLowValueByAction(t, dbPath, "Hotkey"); got != 0 {
		t.Fatalf("Hotkey commands must never be stored as rows, got %d", got)
	}

	store, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	rows, err := store.Query(context.Background(), `SELECT hotkey_stream FROM players WHERE hotkey_stream IS NOT NULL`)
	if err != nil {
		t.Fatalf("query hotkey_stream: %v", err)
	}

	totalEvents := 0
	annotatedAssigns := 0
	for _, row := range rows {
		var blob []byte
		switch v := row["hotkey_stream"].(type) {
		case []byte:
			blob = v
		case string:
			blob = []byte(v)
		default:
			t.Fatalf("hotkey_stream: non-blob result %T", v)
		}
		events, err := hotkeystream.Decode(blob)
		if err != nil {
			t.Fatalf("decode hotkey_stream: %v", err)
		}
		if len(events) == 0 {
			t.Fatal("non-NULL hotkey_stream decoded to zero events")
		}
		for i := 1; i < len(events); i++ {
			if events[i].Sec < events[i-1].Sec {
				t.Fatalf("decoded stream not time-ordered: %d after %d", events[i].Sec, events[i-1].Sec)
			}
		}
		for _, e := range events {
			if e.Type == hotkeystream.TypeAssignBuilding {
				annotatedAssigns++
				if hotkeystream.BuildingName(e.Building) == "" {
					t.Fatalf("building assign with unknown building ID %d", e.Building)
				}
			}
		}
		totalEvents += len(events)
	}
	if len(rows) == 0 || totalEvents == 0 {
		t.Fatal("expected the corpus to produce non-empty hotkey streams, got none")
	}
	if annotatedAssigns == 0 {
		t.Fatal("expected at least one building-annotated assign across the corpus")
	}
}

// TestRun_UpToMonthsFilters exercises the UpToMonths branch, which filters on
// file modification time. Files backdated well beyond the window are excluded;
// a comparison run without the filter confirms they would otherwise be ingested.
func TestRun_UpToMonthsFilters(t *testing.T) {
	oldMtime := time.Now().AddDate(-2, 0, 0)

	dbAll := filepath.Join(t.TempDir(), "all.db")
	inputAll := seedReplayDir(t, smallTestReplays...)
	backdateReplays(t, inputAll, oldMtime)
	if err := Run(context.Background(), Config{
		InputDir:   inputAll,
		SQLitePath: dbAll,
		Logger:     quietLogger(),
	}); err != nil {
		t.Fatalf("Run (no month filter): %v", err)
	}
	if got := countRows(t, dbAll, "replays"); got != int64(len(smallTestReplays)) {
		t.Fatalf("baseline ingest: got %d replays, want %d", got, len(smallTestReplays))
	}

	dbRecent := filepath.Join(t.TempDir(), "recent.db")
	inputRecent := seedReplayDir(t, smallTestReplays...)
	backdateReplays(t, inputRecent, oldMtime)
	if err := Run(context.Background(), Config{
		InputDir:   inputRecent,
		SQLitePath: dbRecent,
		UpToMonths: 1,
		Logger:     quietLogger(),
	}); err != nil {
		t.Fatalf("Run (UpToMonths=1): %v", err)
	}
	if got := countRows(t, dbRecent, "replays"); got != 0 {
		t.Fatalf("UpToMonths=1 should exclude files older than one month, got %d replays", got)
	}
}

// TestRun_CleanDropsReplayData proves Clean=true wipes previously-ingested
// replay data before re-ingesting. Running Clean=true against an empty input
// dir leaves zero replays, whereas a plain re-run keeps them.
func TestRun_CleanDropsReplayData(t *testing.T) {
	seeded := seedReplayDir(t, smallTestReplays...)
	dbPath := filepath.Join(t.TempDir(), "x.db")

	if err := Run(context.Background(), Config{
		InputDir: seeded, SQLitePath: dbPath, Logger: quietLogger(),
	}); err != nil {
		t.Fatalf("seed Run: %v", err)
	}
	if got := countRows(t, dbPath, "replays"); got != int64(len(smallTestReplays)) {
		t.Fatalf("seed ingest: got %d replays, want %d", got, len(smallTestReplays))
	}

	// Second run with Clean=true over an empty folder: the replay tables are
	// dropped and re-created, and nothing is re-ingested, so the count is zero.
	empty := t.TempDir()
	if err := Run(context.Background(), Config{
		InputDir: empty, SQLitePath: dbPath, Clean: true, Logger: quietLogger(),
	}); err != nil {
		t.Fatalf("clean Run: %v", err)
	}
	if got := countRows(t, dbPath, "replays"); got != 0 {
		t.Fatalf("Clean=true over an empty dir should wipe replays, got %d", got)
	}
}

// TestRun_CorruptReplayIsCountedNotFatal covers the per-file error branch in
// runBatchMode: a corrupt .rep is counted as an error and skipped, while a
// valid replay in the same batch still ingests and the run returns nil.
func TestRun_CorruptReplayIsCountedNotFatal(t *testing.T) {
	inputDir := seedReplayDir(t, smallTestReplays[0])
	if err := os.WriteFile(filepath.Join(inputDir, "corrupt.rep"),
		[]byte("this is not a valid starcraft replay"), 0o644); err != nil {
		t.Fatalf("write corrupt replay: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "x.db")
	if err := Run(context.Background(), Config{
		InputDir: inputDir, SQLitePath: dbPath, Logger: quietLogger(),
	}); err != nil {
		t.Fatalf("Run should not fail on a corrupt replay, got %v", err)
	}

	if got := countRows(t, dbPath, "replays"); got != 1 {
		t.Fatalf("only the one valid replay should ingest, got %d", got)
	}
}

// TestRunForFiles_CorruptReplayIsCountedNotFatal covers the equivalent per-file
// error branch in RunForFiles.
func TestRunForFiles_CorruptReplayIsCountedNotFatal(t *testing.T) {
	inputDir := seedReplayDir(t, smallTestReplays[0])
	if err := os.WriteFile(filepath.Join(inputDir, "corrupt.rep"),
		[]byte("this is not a valid starcraft replay"), 0o644); err != nil {
		t.Fatalf("write corrupt replay: %v", err)
	}
	_ = iofacade.AllowDir(inputDir)

	files, err := fileops.GetReplayFiles(inputDir)
	if err != nil {
		t.Fatalf("GetReplayFiles: %v", err)
	}
	hashed, err := fileops.HashFiles(context.Background(), files)
	if err != nil {
		t.Fatalf("HashFiles: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "x.db")
	if err := RunForFiles(context.Background(), Config{
		SQLitePath: dbPath, Logger: quietLogger(),
	}, hashed); err != nil {
		t.Fatalf("RunForFiles should not fail on a corrupt replay, got %v", err)
	}
	if got := countRows(t, dbPath, "replays"); got != 1 {
		t.Fatalf("only the one valid replay should ingest, got %d", got)
	}
}

// TestRun_BadInputDirErrors covers the WalkReplayFiles error branch: a
// nonexistent input directory surfaces a "failed to get replay files" error.
func TestRun_BadInputDirErrors(t *testing.T) {
	badDir := filepath.Join(t.TempDir(), "does-not-exist")
	err := Run(context.Background(), Config{
		InputDir:   badDir,
		SQLitePath: filepath.Join(t.TempDir(), "x.db"),
		Logger:     quietLogger(),
	})
	if err == nil {
		t.Fatal("expected an error for a nonexistent input directory")
	}
	if !strings.Contains(err.Error(), "failed to get replay files") {
		t.Fatalf("expected walk error, got %v", err)
	}
}

// TestRun_StorageOpenFailureErrors covers Run's store-open error branch: an
// unwritable SQLite path fails at NewSQLiteStorage.
func TestRun_StorageOpenFailureErrors(t *testing.T) {
	err := Run(context.Background(), Config{
		InputDir:   seedReplayDir(t, smallTestReplays[0]),
		SQLitePath: "/nonexistent-dir/should-not-be-created/x.db",
		Logger:     quietLogger(),
	})
	if err == nil {
		t.Fatal("expected an error opening storage at an unwritable path")
	}
	if !strings.Contains(err.Error(), "SQLite storage") {
		t.Fatalf("expected a storage-creation error, got %v", err)
	}
}

// TestRunForFiles_StorageOpenFailureErrors covers RunForFiles' store-open
// error branch with a non-empty file list and an unwritable path.
func TestRunForFiles_StorageOpenFailureErrors(t *testing.T) {
	inputDir := seedReplayDir(t, smallTestReplays[0])
	_ = iofacade.AllowDir(inputDir)
	files, err := fileops.GetReplayFiles(inputDir)
	if err != nil {
		t.Fatalf("GetReplayFiles: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected at least one seeded file")
	}

	err = RunForFiles(context.Background(), Config{
		SQLitePath: "/nonexistent-dir/should-not-be-created/x.db",
		Logger:     quietLogger(),
	}, files)
	if err == nil {
		t.Fatal("expected an error opening storage at an unwritable path")
	}
	if !strings.Contains(err.Error(), "SQLite storage") {
		t.Fatalf("expected a storage-creation error, got %v", err)
	}
}

// TestRunForFiles_StoreRightClicksHonored proves RunForFiles honors the command
// storage options: StoreRightClicks=true persists Right Click commands.
func TestRunForFiles_StoreRightClicksHonored(t *testing.T) {
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

	if err := RunForFiles(context.Background(), Config{
		SQLitePath:       dbPath,
		StoreRightClicks: true,
		Logger:           quietLogger(),
	}, hashed); err != nil {
		t.Fatalf("RunForFiles: %v", err)
	}
	if got := countLowValueByAction(t, dbPath, "Right Click"); got == 0 {
		t.Fatal("RunForFiles with StoreRightClicks=true should persist Right Click commands, got 0")
	}
}

// TestRun_CPUProfileWritesFile exercises the CPUProfilePath wiring end-to-end:
// a profile file is created and non-empty after the run.
func TestRun_CPUProfileWritesFile(t *testing.T) {
	inputDir := seedReplayDir(t, smallTestReplays...)
	tmp := t.TempDir()
	if err := iofacade.AllowDir(tmp); err != nil {
		t.Fatalf("AllowDir: %v", err)
	}
	profilePath := filepath.Join(tmp, "cpu.pprof")
	dbPath := filepath.Join(t.TempDir(), "x.db")

	if err := Run(context.Background(), Config{
		InputDir:       inputDir,
		SQLitePath:     dbPath,
		CPUProfilePath: profilePath,
		Logger:         quietLogger(),
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if info := statOrFatal(t, profilePath); info == 0 {
		t.Fatal("CPU profile file is empty; profiling did not run")
	}
}

// TestStartCPUProfile_WritesAndStops covers startCPUProfile directly: the
// returned stop closure flushes a non-empty profile to the given path.
func TestStartCPUProfile_WritesAndStops(t *testing.T) {
	tmp := t.TempDir()
	if err := iofacade.AllowDir(tmp); err != nil {
		t.Fatalf("AllowDir: %v", err)
	}
	profilePath := filepath.Join(tmp, "direct.pprof")

	stop, err := startCPUProfile(profilePath)
	if err != nil {
		t.Fatalf("startCPUProfile: %v", err)
	}
	// Do a little work so the profile has at least one sample.
	sum := 0
	for i := 0; i < 1_000_000; i++ {
		sum += i
	}
	_ = sum
	stop()

	if info := statOrFatal(t, profilePath); info == 0 {
		t.Fatal("startCPUProfile produced an empty file")
	}
}

// TestStartCPUProfile_ForbiddenPathErrors covers the create-error branch:
// a path outside every registered iofacade root is rejected.
func TestStartCPUProfile_ForbiddenPathErrors(t *testing.T) {
	allowed := t.TempDir()
	if err := iofacade.AllowDir(allowed); err != nil {
		t.Fatalf("AllowDir: %v", err)
	}
	// A path guaranteed to be outside the (now non-empty) allowlist.
	stop, err := startCPUProfile("/definitely-not-allowed/cpu.pprof")
	if err == nil {
		if stop != nil {
			stop()
		}
		t.Fatal("expected startCPUProfile to reject a forbidden path")
	}
	if !strings.Contains(err.Error(), "create profile file") {
		t.Fatalf("expected a create-profile error, got %v", err)
	}
}

func statOrFatal(t *testing.T, path string) int64 {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return fi.Size()
}

func backdateReplays(t *testing.T, dir string, mtime time.Time) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatalf("chtimes %s: %v", p, err)
		}
	}
}

func mutateSettingsGameType(t *testing.T, dbPath, value string) {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(
		"UPDATE settings SET game_type = ? WHERE config_key = 'global'", value); err != nil {
		t.Fatalf("mutate settings: %v", err)
	}
}

// TestLogger_ErrorfWritesTimestampedLine covers Errorf: it emits a timestamped
// line ending in a newline and containing the formatted message.
func TestLogger_ErrorfWritesTimestampedLine(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf, false, nil)
	l.Errorf("boom", "boom %d", 42)

	out := buf.String()
	if !strings.Contains(out, "boom 42") {
		t.Fatalf("Errorf output missing formatted message: %q", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("Errorf output should end with a newline: %q", out)
	}
	// Timestamp prefix HH:MM:SS is 8 chars followed by a space.
	if len(out) < 9 || out[2] != ':' || out[5] != ':' || out[8] != ' ' {
		t.Fatalf("Errorf output missing HH:MM:SS timestamp prefix: %q", out)
	}
}

// TestLogger_ErrorfForwardsEvent covers the onEvent callback path for Errorf.
func TestLogger_ErrorfForwardsEvent(t *testing.T) {
	var got LogEvent
	l := NewLogger(nil, false, func(e LogEvent) { got = e })
	l.Errorf("failed", "failed: %s", "reason")

	if got.Level != LogLevelError {
		t.Fatalf("event level: got %q, want %q", got.Level, LogLevelError)
	}
	if got.Message != "failed: reason" {
		t.Fatalf("event message: got %q", got.Message)
	}
	if got.Key != "failed" {
		t.Fatalf("event key: got %q, want %q", got.Key, "failed")
	}
	if len(got.Args) != 1 || got.Args[0] != "reason" {
		t.Fatalf("event args: got %v, want [reason]", got.Args)
	}
}

// TestLogger_ColorizeOffIsPlainText proves colorize returns the message
// unchanged when color is disabled (no ANSI escape codes injected).
func TestLogger_ColorizeOffIsPlainText(t *testing.T) {
	l := NewLogger(nil, false, nil)
	for _, level := range []LogLevel{LogLevelInfo, LogLevelSuccess, LogLevelWarn, LogLevelError, LogLevelProgress} {
		got := l.colorize(level, "hello")
		if got != "hello" {
			t.Fatalf("colorize(%q) with color off should be plain, got %q", level, got)
		}
	}
}

// TestLogger_ColorizeOnWrapsWithAnsi proves colorize wraps the message in ANSI
// escape sequences per level when color is enabled, and that levels differ.
func TestLogger_ColorizeOnWrapsWithAnsi(t *testing.T) {
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = true })

	l := NewLogger(nil, true, nil)

	success := l.colorize(LogLevelSuccess, "msg")
	warn := l.colorize(LogLevelWarn, "msg")
	errLine := l.colorize(LogLevelError, "msg")
	info := l.colorize(LogLevelInfo, "msg")
	progress := l.colorize(LogLevelProgress, "msg")

	// Info falls through to the default cyan branch; Progress is explicitly
	// cyan. Both must still be colorized (non-plain).
	for name, got := range map[string]string{"info": info, "progress": progress} {
		if got == "msg" || !strings.Contains(got, "\x1b[") {
			t.Fatalf("colorize %s with color on should add escape codes, got %q", name, got)
		}
	}

	for name, got := range map[string]string{"success": success, "warn": warn, "error": errLine} {
		if got == "msg" {
			t.Fatalf("colorize %s with color on should add escape codes, got plain %q", name, got)
		}
		if !strings.Contains(got, "\x1b[") {
			t.Fatalf("colorize %s should contain an ANSI escape, got %q", name, got)
		}
		if !strings.Contains(got, "msg") {
			t.Fatalf("colorize %s should still contain the message, got %q", name, got)
		}
	}

	// Distinct levels use distinct color codes.
	if success == warn || warn == errLine || success == errLine {
		t.Fatalf("expected distinct colors per level: success=%q warn=%q error=%q", success, warn, errLine)
	}
}

// TestLogger_ProgressAppendsThenLineFlushes covers the emit append branch:
// Progress writes a dotted continuation with no timestamp, and the next full
// line flushes the open progress line with a leading newline.
func TestLogger_ProgressAppendsThenLineFlushes(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf, false, nil)

	l.Progress()
	l.Progress()
	if got := buf.String(); got != ".." {
		t.Fatalf("two Progress calls should append two dots, got %q", got)
	}

	l.Infof("done", "done")
	out := buf.String()
	if !strings.HasPrefix(out, "..\n") {
		t.Fatalf("a full line after Progress should flush with a leading newline, got %q", out)
	}
	if !strings.Contains(out, "done") {
		t.Fatalf("expected the flushed line to contain the message, got %q", out)
	}
}

// TestLogger_NilEmitIsSafe covers the nil-receiver guard in emit.
func TestLogger_NilEmitIsSafe(t *testing.T) {
	var l *Logger
	l.Errorf("must_not_panic", "must not panic %d", 1)
}

func TestTrimLogMessage(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"no trailing newline", "hello", "hello"},
		{"single trailing newline", "hello\n", "hello"},
		{"multiple trailing newlines", "hello\n\n\n", "hello"},
		{"only newlines", "\n\n", ""},
		{"empty", "", ""},
		{"interior newline preserved", "a\nb\n", "a\nb"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := trimLogMessage(c.in); got != c.want {
				t.Fatalf("trimLogMessage(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
