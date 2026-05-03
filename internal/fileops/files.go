package fileops

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// FileInfo represents a replay file with metadata
type FileInfo struct {
	Path     string
	Name     string
	Size     int64
	ModTime  time.Time
	Checksum string
}

var errReplayFound = errors.New("replay file found")

func shouldIgnoreReplayFilePath(path string) bool {
	name := strings.TrimSpace(filepath.Base(path))
	return strings.EqualFold(name, "LastReplay.rep")
}

func ValidateReplayDir(rootDir string) error {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		return errors.New("replay folder is required")
	}

	info, err := os.Stat(rootDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("replay folder does not exist")
		}
		return fmt.Errorf("stat replay folder: %w", err)
	}
	if !info.IsDir() {
		return errors.New("replay folder is not a directory")
	}

	hasReplayFiles, err := HasReplayFiles(rootDir)
	if err != nil {
		return err
	}
	if !hasReplayFiles {
		return errors.New("replay folder does not contain any .rep files")
	}
	return nil
}

func HasReplayFiles(rootDir string) (bool, error) {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		return false, nil
	}

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(path), ".rep") && !shouldIgnoreReplayFilePath(path) {
			return errReplayFound
		}
		return nil
	})
	if err == nil {
		return false, nil
	}
	if errors.Is(err, errReplayFound) {
		return true, nil
	}
	return false, fmt.Errorf("walk replay folder: %w", err)
}

// WalkReplayFiles recursively finds all .rep files in the given directory and
// returns FileInfo entries with Path/Name/Size/ModTime populated. Checksum is
// left empty — callers that need it should use HashFiles to populate it for
// the subset that survives a cheaper dedup step (e.g. path-based prefilter).
func WalkReplayFiles(rootDir string) ([]FileInfo, error) {
	var files []FileInfo

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".rep") {
			if shouldIgnoreReplayFilePath(path) {
				return nil
			}
			files = append(files, FileInfo{
				Path:    path,
				Name:    info.Name(),
				Size:    info.Size(),
				ModTime: info.ModTime(),
			})
		}

		return nil
	})

	return files, err
}

// HashFiles computes SHA256 for every entry whose Checksum is empty, fanning
// the work out across runtime.GOMAXPROCS goroutines. Already-hashed entries
// are passed through untouched. Cancellation via ctx aborts in-flight workers.
func HashFiles(ctx context.Context, files []FileInfo) ([]FileInfo, error) {
	if len(files) == 0 {
		return files, nil
	}

	out := make([]FileInfo, len(files))
	copy(out, files)

	workers := runtime.GOMAXPROCS(0)
	if workers > len(out) {
		workers = len(out)
	}
	if workers < 1 {
		workers = 1
	}

	g, gCtx := errgroup.WithContext(ctx)
	jobs := make(chan int)

	var mu sync.Mutex
	var firstErr error

	for w := 0; w < workers; w++ {
		g.Go(func() error {
			for i := range jobs {
				if out[i].Checksum != "" {
					continue
				}
				sum, err := calculateChecksum(out[i].Path)
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("failed to calculate checksum for %s: %w", out[i].Path, err)
					}
					mu.Unlock()
					return err
				}
				out[i].Checksum = sum
			}
			return nil
		})
	}

	g.Go(func() error {
		defer close(jobs)
		for i := range out {
			select {
			case jobs <- i:
			case <-gCtx.Done():
				return gCtx.Err()
			}
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		mu.Lock()
		fe := firstErr
		mu.Unlock()
		if fe != nil {
			return nil, fe
		}
		return nil, err
	}

	return out, nil
}

// GetReplayFiles recursively finds all .rep files and computes SHA256 for each.
// Equivalent to WalkReplayFiles followed by HashFiles. Kept for callers that
// need both results in one shot (tests, benchmarks). Hot ingest paths should
// prefer the split form so checksum work can be skipped for files already
// known to the database.
func GetReplayFiles(rootDir string) ([]FileInfo, error) {
	files, err := WalkReplayFiles(rootDir)
	if err != nil {
		return files, err
	}
	return HashFiles(context.Background(), files)
}

// SortFilesByModTime sorts files by modification time (newest first)
func SortFilesByModTime(files []FileInfo) {
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.After(files[j].ModTime)
	})
}

// FilterFilesByDate filters files by date constraints
func FilterFilesByDate(files []FileInfo, upToDate *time.Time, upToMonths *int) []FileInfo {
	var filtered []FileInfo

	for _, file := range files {
		include := true

		if upToDate != nil && file.ModTime.After(*upToDate) {
			include = false
		}

		if upToMonths != nil {
			cutoff := time.Now().AddDate(0, -*upToMonths, 0)
			if file.ModTime.Before(cutoff) {
				include = false
			}
		}

		if include {
			filtered = append(filtered, file)
		}
	}

	return filtered
}

// LimitFiles limits the number of files returned
func LimitFiles(files []FileInfo, limit int) []FileInfo {
	if limit <= 0 || len(files) <= limit {
		return files
	}
	return files[:limit]
}

// calculateChecksum calculates SHA256 checksum of a file
func calculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// NewFileInfoFromPath stats the path and computes its checksum, returning a FileInfo
// shaped like one produced by GetReplayFiles. Used by paths that already know which
// .rep file to ingest (e.g. bulk re-analyze) and don't want to walk a directory.
func NewFileInfoFromPath(filePath string) (*FileInfo, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}
	checksum, err := calculateChecksum(filePath)
	if err != nil {
		return nil, err
	}
	return &FileInfo{
		Path:     filePath,
		Name:     info.Name(),
		Size:     info.Size(),
		ModTime:  info.ModTime(),
		Checksum: checksum,
	}, nil
}
