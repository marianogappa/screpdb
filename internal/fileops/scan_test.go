package fileops

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanReplayFolderKeepsOnlyCorpusFilesNewestFirst(t *testing.T) {
	dir := t.TempDir()
	write := func(rel string, age time.Duration) string {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		stamp := time.Now().Add(-age)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		return path
	}
	oldest := write("old.rep", 2*time.Hour)
	newest := write("sub/new.rep", time.Minute)
	write(filepath.Join(WatchMeDirName, "watch_me.rep"), time.Minute)
	write(".trash/gone.rep", time.Minute)
	write("LastReplay.rep", time.Minute)
	write("notes.txt", time.Minute)
	write("half.rep.tmp", time.Minute)

	files, err := ScanReplayFolder(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("scanned %d files, want 2: %+v", len(files), files)
	}
	if files[0].Path != newest || files[1].Path != oldest {
		t.Fatalf("wrong order: %s then %s", files[0].Path, files[1].Path)
	}
}

func TestScanReplayFolderWorksWithARelativeFolderUnderADotDirectory(t *testing.T) {
	root := t.TempDir()
	hidden := filepath.Join(root, ".tooling", "replays")
	if err := os.MkdirAll(filepath.Join(hidden, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"game.rep", filepath.Join("sub", "other.rep")} {
		if err := os.WriteFile(filepath.Join(hidden, rel), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(hidden, "sub", "LastReplay.rep"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(cwd) })

	// A dot-directory above the replay folder must not disqualify the corpus,
	// and a relative folder must resolve the same as an absolute one.
	files, err := ScanReplayFolder(filepath.Join(".tooling", "replays"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("scanned %d files, want 2: %+v", len(files), files)
	}
	absolute, err := ScanReplayFolder(hidden)
	if err != nil {
		t.Fatal(err)
	}
	if len(absolute) != len(files) {
		t.Fatalf("relative folder found %d files, absolute found %d", len(files), len(absolute))
	}
}

func TestIgnoredReplayPathIgnoresOnlyDirectoriesInsideTheFolder(t *testing.T) {
	if IgnoredReplayPath("/home/.config/sc/replays", "/home/.config/sc/replays/game.rep") {
		t.Error("a dot-directory above the replay folder must not disqualify a replay")
	}
	if !IgnoredReplayPath("/home/.config/sc/replays", "/home/.config/sc/replays/.trash/game.rep") {
		t.Error("a dot-directory inside the replay folder must be ignored")
	}
	if IgnoredReplayPath("/replays", "/elsewhere/game.rep") {
		t.Error("a path outside the folder should be judged by its name alone")
	}
}

func TestIgnoredReplayPath(t *testing.T) {
	dir := "/replays"
	for path, want := range map[string]bool{
		"/replays/game.rep":                             false,
		"/replays/sub/game.rep":                         false,
		"/replays/" + WatchMeDirName + "/watch_me.rep":  true,
		"/replays/.trash/game.rep":                      true,
		"/replays/LastReplay.rep":                       true,
		"/replays/lastreplay.rep":                       true,
		"/replays/game.rep.tmp":                         true,
		"/replays/game.part":                            true,
		"/replays/notes.txt":                            true,
		"/replays/sub/" + WatchMeDirName + "/watch.rep": true,
	} {
		if got := IgnoredReplayPath(dir, path); got != want {
			t.Errorf("IgnoredReplayPath(%q) = %v, want %v", path, got, want)
		}
	}
}
