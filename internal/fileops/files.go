package fileops

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FileInfo represents a replay file with metadata
type FileInfo struct {
	Path     string
	Name     string
	Size     int64
	ModTime  time.Time
	Checksum string
}

// GetReplayFiles recursively finds all .rep files in the given directory
func GetReplayFiles(rootDir string) ([]FileInfo, error) {
	var files []FileInfo

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".rep") {
			checksum, err := calculateChecksum(path)
			if err != nil {
				return fmt.Errorf("failed to calculate checksum for %s: %w", path, err)
			}

			files = append(files, FileInfo{
				Path:     path,
				Name:     info.Name(),
				Size:     info.Size(),
				ModTime:  info.ModTime(),
				Checksum: checksum,
			})
		}

		return nil
	})

	return files, err
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
