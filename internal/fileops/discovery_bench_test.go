package fileops

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// envCorpus is the path to a directory of real .rep files. The benches in
// this file skip when the env var is empty so `go test ./...` stays cheap on
// machines without a large corpus on disk. Run with:
//
//	SCREPDB_BENCH_CORPUS=/abs/path/to/replays go test \
//	    -bench=. -benchtime=3x -run=^$ ./internal/fileops/...
const envCorpus = "SCREPDB_BENCH_CORPUS"

func corpusOrSkip(b *testing.B) string {
	dir := os.Getenv(envCorpus)
	if dir == "" {
		b.Skipf("set %s to a directory of .rep files to run this benchmark", envCorpus)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		b.Skipf("%s=%s not a directory", envCorpus, dir)
	}
	return dir
}

// legacyGetReplayFiles reproduces the pre-change implementation byte-for-byte:
// a single goroutine walks the dir and SHA256-hashes every .rep inline. Used
// only by the benchmark to give an honest before/after comparison; the
// production path is now WalkReplayFiles + HashFiles.
func legacyGetReplayFiles(rootDir string) ([]FileInfo, error) {
	var files []FileInfo
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(path), ".rep") {
			return nil
		}
		if shouldIgnoreReplayFilePath(path) {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			f.Close()
			return err
		}
		f.Close()
		files = append(files, FileInfo{
			Path:     path,
			Name:     info.Name(),
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			Checksum: fmt.Sprintf("%x", h.Sum(nil)),
		})
		return nil
	})
	return files, err
}

// BenchmarkDiscovery_LegacyWalkAndHash measures the pre-change behavior:
// walk the directory and SHA256-hash every single .rep, sequentially.
func BenchmarkDiscovery_LegacyWalkAndHash(b *testing.B) {
	dir := corpusOrSkip(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		files, err := legacyGetReplayFiles(dir)
		if err != nil {
			b.Fatalf("legacyGetReplayFiles: %v", err)
		}
		if len(files) == 0 {
			b.Fatalf("no replays in %s", dir)
		}
		b.ReportMetric(float64(len(files)), "files")
	}
}

// BenchmarkDiscovery_WalkOnly measures just the directory walk, no hashing.
// This is the lower bound on wall time after R1 prefilters out everything.
func BenchmarkDiscovery_WalkOnly(b *testing.B) {
	dir := corpusOrSkip(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		files, err := WalkReplayFiles(dir)
		if err != nil {
			b.Fatalf("WalkReplayFiles: %v", err)
		}
		if len(files) == 0 {
			b.Fatalf("no replays in %s", dir)
		}
		b.ReportMetric(float64(len(files)), "files")
	}
}

// BenchmarkDiscovery_WalkPlusParallelHash measures the new fast path:
// walk without hashing, then HashFiles in parallel for the full set.
// Compare against BenchmarkDiscovery_GetReplayFiles_Old to see R2's effect
// when the survivor set is the entire corpus (worst case for R1).
func BenchmarkDiscovery_WalkPlusParallelHash(b *testing.B) {
	dir := corpusOrSkip(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		files, err := WalkReplayFiles(dir)
		if err != nil {
			b.Fatalf("WalkReplayFiles: %v", err)
		}
		hashed, err := HashFiles(context.Background(), files)
		if err != nil {
			b.Fatalf("HashFiles: %v", err)
		}
		if len(hashed) == 0 {
			b.Fatalf("no replays in %s", dir)
		}
		b.ReportMetric(float64(len(hashed)), "files")
	}
}

// BenchmarkDiscovery_SequentialHashOnly measures hashing the full corpus on
// one goroutine. Direct comparison to ParallelHashOnly isolates R2's gain.
func BenchmarkDiscovery_SequentialHashOnly(b *testing.B) {
	dir := corpusOrSkip(b)
	files, err := WalkReplayFiles(dir)
	if err != nil {
		b.Fatalf("WalkReplayFiles: %v", err)
	}
	if len(files) == 0 {
		b.Fatalf("no replays in %s", dir)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, f := range files {
			file, err := os.Open(f.Path)
			if err != nil {
				b.Fatalf("open %s: %v", f.Path, err)
			}
			h := sha256.New()
			if _, err := io.Copy(h, file); err != nil {
				file.Close()
				b.Fatalf("copy %s: %v", f.Path, err)
			}
			file.Close()
			_ = fmt.Sprintf("%x", h.Sum(nil))
		}
		b.ReportMetric(float64(len(files)), "files")
	}
}

// BenchmarkDiscovery_ParallelHashOnly measures hashing the full corpus across
// GOMAXPROCS goroutines. R2 isolation.
func BenchmarkDiscovery_ParallelHashOnly(b *testing.B) {
	dir := corpusOrSkip(b)
	files, err := WalkReplayFiles(dir)
	if err != nil {
		b.Fatalf("WalkReplayFiles: %v", err)
	}
	if len(files) == 0 {
		b.Fatalf("no replays in %s", dir)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Force a fresh slice with empty checksums each iteration so HashFiles
		// actually rehashes on every run.
		fresh := make([]FileInfo, len(files))
		copy(fresh, files)
		for j := range fresh {
			fresh[j].Checksum = ""
		}
		out, err := HashFiles(context.Background(), fresh)
		if err != nil {
			b.Fatalf("HashFiles: %v", err)
		}
		if len(out) == 0 {
			b.Fatalf("no output")
		}
		b.ReportMetric(float64(len(files)), "files")
		b.ReportMetric(float64(runtime.GOMAXPROCS(0)), "workers")
	}
}

// BenchmarkDiscovery_ReScan_RealisticDB models the common workflow: user
// re-runs ingest against the same folder where most files are already in the
// DB. We simulate "already in DB" via a fake set; the only thing that matters
// for the discovery phase is which files have to get hashed.
//
// Old: every file gets hashed (the hash is what feeds the dedup check).
// New: walk → cheap path-set check → hash only the survivors.
//
// The env var SCREPDB_BENCH_KNOWN_PCT controls what fraction of the corpus
// is already known. Defaults to 95%.
func BenchmarkDiscovery_ReScan_RealisticDB_Old(b *testing.B) {
	dir := corpusOrSkip(b)
	files, err := WalkReplayFiles(dir)
	if err != nil {
		b.Fatalf("WalkReplayFiles: %v", err)
	}
	if len(files) == 0 {
		b.Fatalf("no replays in %s", dir)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Pre-change: everything gets hashed before any dedup happens.
		hashed, err := HashFiles(context.Background(), copyEmptyChecksums(files))
		if err != nil {
			b.Fatalf("HashFiles: %v", err)
		}
		_ = hashed
	}
	b.ReportMetric(float64(len(files)), "files")
}

func BenchmarkDiscovery_ReScan_RealisticDB_New(b *testing.B) {
	dir := corpusOrSkip(b)
	files, err := WalkReplayFiles(dir)
	if err != nil {
		b.Fatalf("WalkReplayFiles: %v", err)
	}
	if len(files) == 0 {
		b.Fatalf("no replays in %s", dir)
	}
	knownPct := envFloatDefault("SCREPDB_BENCH_KNOWN_PCT", 0.95)
	knownCount := int(float64(len(files)) * knownPct)
	knownPaths := make(map[string]struct{}, knownCount)
	for i := 0; i < knownCount; i++ {
		knownPaths[files[i].Path] = struct{}{}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Post-change phase 1: cheap path-based dedup against an in-memory
		// map (DB lookup is comparable in practice — a single indexed query).
		survivors := make([]FileInfo, 0, len(files)-knownCount)
		for _, f := range files {
			if _, ok := knownPaths[f.Path]; !ok {
				survivors = append(survivors, f)
			}
		}
		// Phase 2: parallel hash of the survivors only.
		hashed, err := HashFiles(context.Background(), survivors)
		if err != nil {
			b.Fatalf("HashFiles: %v", err)
		}
		_ = hashed
	}
	b.ReportMetric(float64(len(files)), "files_total")
	b.ReportMetric(float64(len(files)-knownCount), "files_hashed")
	b.ReportMetric(knownPct*100, "%_known")
}

func copyEmptyChecksums(files []FileInfo) []FileInfo {
	out := make([]FileInfo, len(files))
	for i, f := range files {
		f.Checksum = ""
		out[i] = f
	}
	return out
}

func envFloatDefault(key string, def float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	var f float64
	if _, err := fmt.Sscanf(v, "%f", &f); err != nil {
		return def
	}
	return f
}

// silence unused-import warning when filepath is dead-code in some refactor.
var _ = filepath.Separator
