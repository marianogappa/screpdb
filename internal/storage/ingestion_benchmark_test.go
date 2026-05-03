package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/parser"
	"github.com/marianogappa/screpdb/internal/profile"
)

// BenchmarkSQLiteIngestionCorpus runs ingestion against every replay in the
// testdata corpus and reports per-phase metrics so a regression in any single
// phase (commands, events, patterns, commit) is visible in the bench output.
//
// The corpus size is variable: drop more .rep files into testdata/replays to
// strengthen the signal. Per-replay-average metrics are reported alongside
// totals so results stay comparable as the corpus grows.
func BenchmarkSQLiteIngestionCorpus(b *testing.B) {
	ctx := context.Background()
	replaysDir := os.Getenv("SCREPDB_BENCH_CORPUS")
	if replaysDir == "" {
		var err error
		replaysDir, err = resolveReplayDir()
		if err != nil {
			b.Fatalf("resolveReplayDir: %v", err)
		}
	}
	if path := os.Getenv("SCREPDB_BENCH_CPUPROFILE"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			b.Fatalf("create cpuprofile: %v", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			b.Fatalf("start cpuprofile: %v", err)
		}
		b.Cleanup(func() {
			pprof.StopCPUProfile()
			f.Close()
		})
	}
	files, err := fileops.GetReplayFiles(replaysDir)
	if err != nil {
		b.Fatalf("GetReplayFiles: %v", err)
	}
	if len(files) == 0 {
		b.Fatalf("no replays in %s", replaysDir)
	}
	fileops.SortFilesByModTime(files)
	if capStr := os.Getenv("SCREPDB_BENCH_CORPUS_CAP"); capStr != "" {
		var capN int
		if _, err := fmt.Sscanf(capStr, "%d", &capN); err == nil && capN > 0 && capN < len(files) {
			files = files[:capN]
		}
	}
	corpusSize := len(files)

	b.ResetTimer()
	sink := profile.NewSink(profile.ModeSummary)
	sink.SetWriter(io.Discard) // silence per-replay log spam during bench
	var totalDur time.Duration
	for i := 0; i < b.N; i++ {
		dbPath := filepath.Join(b.TempDir(), fmt.Sprintf("bench_%d.db", i))
		store, err := NewSQLiteStorage(dbPath)
		if err != nil {
			b.Fatalf("NewSQLiteStorage: %v", err)
		}
		if err := store.Initialize(ctx, true, true); err != nil {
			_ = store.Close()
			b.Fatalf("Initialize: %v", err)
		}

		iterStart := time.Now()
		for idx := range files {
			fileInfo := files[idx]

			run := sink.Replay(fileInfo.Name)
			stop := run.Phase("parse")
			replay := parser.CreateReplayFromFileInfo(fileInfo.Path, fileInfo.Name, fileInfo.Size, fileInfo.Checksum)
			data, err := parser.ParseReplay(fileInfo.Path, replay)
			stop()
			if err != nil {
				_ = store.Close()
				b.Fatalf("ParseReplay(%s): %v", fileInfo.Name, err)
			}
			data.Profile = run

			if err := store.storeReplayWithBatching(ctx, data); err != nil {
				_ = store.Close()
				b.Fatalf("storeReplayWithBatching(%s): %v", fileInfo.Name, err)
			}
		}
		totalDur += time.Since(iterStart)

		if err := store.Close(); err != nil {
			b.Fatalf("Close: %v", err)
		}
	}

	ops := float64(b.N)
	totals := sink.PhaseTotals()
	b.ReportMetric(totalDur.Seconds()/ops, fmt.Sprintf("s/ingest_%dreplays", corpusSize))
	b.ReportMetric(totalDur.Seconds()/ops/float64(corpusSize), "s/replay")
	for _, phase := range []string{"parse", "replay_ins", "players", "cmds", "events", "patterns", "commit"} {
		if d, ok := totals[phase]; ok {
			b.ReportMetric(d.Seconds()/ops, fmt.Sprintf("s/%s", phase))
		}
	}
}
