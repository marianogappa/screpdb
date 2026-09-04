package fileops

import (
	"errors"
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
// a game already in the corpus), hidden directories inside the replay folder,
// half-written files, and StarCraft's live LastReplay.rep.
func IgnoredReplayPath(folder, path string) bool {
	name := strings.ToLower(filepath.Base(path))
	if strings.HasSuffix(name, ".tmp") || strings.HasSuffix(name, ".part") {
		return true
	}
	if !strings.HasSuffix(name, ".rep") || shouldIgnoreReplayFilePath(path) {
		return true
	}
	return ignoredWithinFolder(folder, path)
}

func ignoredWithinFolder(folder, path string) bool {
	rel, err := relativeToFolder(folder, path)
	if err != nil {
		// Without a usable path inside the folder we cannot tell the folder's
		// own directories from its ancestors, and judging the ancestors would
		// silently drop the whole corpus whenever the replay folder happens to
		// sit under a dot-directory. Judge the file by its name alone.
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if part == "" || part == "." {
			continue
		}
		if part == WatchMeDirName || strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

// relativeToFolder resolves both sides before comparing, so a relative folder
// and an absolute path (or the reverse) still yield the path inside the folder.
func relativeToFolder(folder, path string) (string, error) {
	absFolder, err := filepath.Abs(folder)
	if err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absFolder, absPath)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("fileops: path is outside the replay folder")
	}
	return rel, nil
}
