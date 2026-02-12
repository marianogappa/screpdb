package storage

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/parser"
)

type ingestPhaseTimings struct {
	parse    time.Duration
	replay   time.Duration
	players  time.Duration
	commands time.Duration
	patterns time.Duration
	commit   time.Duration
	total    time.Duration
}

func BenchmarkSQLiteIngestion3Replays(b *testing.B) {
	ctx := context.Background()
	replaysDir, err := resolveReplayDir()
	if err != nil {
		b.Fatalf("resolveReplayDir: %v", err)
	}
	files, err := fileops.GetReplayFiles(replaysDir)
	if err != nil {
		b.Fatalf("GetReplayFiles: %v", err)
	}
	if len(files) != 3 {
		b.Fatalf("expected 3 replays in %s, got %d", replaysDir, len(files))
	}
	fileops.SortFilesByModTime(files)

	b.ResetTimer()
	var all ingestPhaseTimings
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
		var iter ingestPhaseTimings
		for idx := range files {
			fileInfo := files[idx]

			parseStart := time.Now()
			replay := parser.CreateReplayFromFileInfo(fileInfo.Path, fileInfo.Name, fileInfo.Size, fileInfo.Checksum)
			data, err := parser.ParseReplay(fileInfo.Path, replay)
			if err != nil {
				_ = store.Close()
				b.Fatalf("ParseReplay(%s): %v", fileInfo.Name, err)
			}
			iter.parse += time.Since(parseStart)

			phaseTimings, err := storeReplayWithPhaseTimings(ctx, store, data)
			if err != nil {
				_ = store.Close()
				b.Fatalf("storeReplayWithPhaseTimings(%s): %v", fileInfo.Name, err)
			}
			iter.replay += phaseTimings.replay
			iter.players += phaseTimings.players
			iter.commands += phaseTimings.commands
			iter.patterns += phaseTimings.patterns
			iter.commit += phaseTimings.commit
		}
		iter.total = time.Since(iterStart)

		all.parse += iter.parse
		all.replay += iter.replay
		all.players += iter.players
		all.commands += iter.commands
		all.patterns += iter.patterns
		all.commit += iter.commit
		all.total += iter.total

		if err := store.Close(); err != nil {
			b.Fatalf("Close: %v", err)
		}
	}

	ops := float64(b.N)
	b.ReportMetric(all.total.Seconds()/ops, "s/ingest_3replays")
	b.ReportMetric(all.parse.Seconds()/ops, "s/parse")
	b.ReportMetric(all.replay.Seconds()/ops, "s/db_replays")
	b.ReportMetric(all.players.Seconds()/ops, "s/db_players")
	b.ReportMetric(all.commands.Seconds()/ops, "s/db_commands")
	b.ReportMetric(all.patterns.Seconds()/ops, "s/db_patterns")
	b.ReportMetric(all.commit.Seconds()/ops, "s/db_commit")
}

func storeReplayWithPhaseTimings(ctx context.Context, s *SQLiteStorage, data *models.ReplayData) (ingestPhaseTimings, error) {
	var timings ingestPhaseTimings

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return timings, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	start := time.Now()
	replayID, err := s.insertReplaySequentialTx(ctx, tx, data.Replay)
	timings.replay = time.Since(start)
	if err != nil {
		return timings, fmt.Errorf("failed to insert replay: %w", err)
	}
	if replayID == 0 {
		return timings, fmt.Errorf("replay insert returned invalid ID: 0")
	}

	start = time.Now()
	playerIDs, err := s.insertPlayersBatchTx(ctx, tx, replayID, data.Players)
	timings.players = time.Since(start)
	if err != nil {
		return timings, fmt.Errorf("failed to insert players: %w", err)
	}

	s.updateEntityIDs(data, replayID, playerIDs)

	if len(data.Commands) > 0 {
		start = time.Now()
		if err := s.insertCommandsBatchTx(ctx, tx, data.Commands); err != nil {
			return timings, fmt.Errorf("failed to insert commands: %w", err)
		}
		timings.commands = time.Since(start)
	}

	if data.PatternOrchestrator != nil {
		start = time.Now()
		if err := s.processPatternResultsTx(ctx, tx, data.PatternOrchestrator, replayID, playerIDs); err != nil {
			return timings, fmt.Errorf("failed to process pattern results: %w", err)
		}
		timings.patterns = time.Since(start)
	}

	start = time.Now()
	if err := tx.Commit(); err != nil {
		return timings, fmt.Errorf("failed to commit transaction: %w", err)
	}
	timings.commit = time.Since(start)

	return timings, nil
}
