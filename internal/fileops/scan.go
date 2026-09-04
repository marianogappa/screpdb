package fileops

import (
	"path/filepath"
	"strings"
)

// ScanReplayFolder lists the replay files that belong in the corpus, newest
// first.
func ScanReplayFolder(folder string) ([]FileInfo, error) {
	files, err := WalkReplayFiles(folder)
	if err != nil {
		return nil, err
	}
	kept := make([]FileInfo, 0, len(files))
	for _, file := range files {
		if IgnoredReplayPath(folder, file.Path) {
			continue
		}
		kept = append(kept, file)
	}
	SortFilesByModTime(kept)
	return kept, nil
}

// IgnoredReplayPath reports whether a path must stay out of the corpus: the
// folder the dashboard stages "watch me" replays into (its content is a copy of
// a game already in the corpus), hidden directories, half-written files, and
// StarCraft's live LastReplay.rep.
func IgnoredReplayPath(folder, path string) bool {
	rel, err := filepath.Rel(folder, path)
	if err != nil {
		rel = path
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if part == "" || part == "." {
			continue
		}
		if part == WatchMeDirName || strings.HasPrefix(part, ".") {
			return true
		}
	}
	name := strings.ToLower(filepath.Base(path))
	if strings.HasSuffix(name, ".tmp") || strings.HasSuffix(name, ".part") {
		return true
	}
	return !strings.HasSuffix(name, ".rep") || shouldIgnoreReplayFilePath(path)
}
