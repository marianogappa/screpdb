package db

import (
	"context"
	"strings"

	"github.com/marianogappa/screpdb/internal/dashboard/db/sqlcgen"
)

func (s *Store) GetIngestInputDir(ctx context.Context, configKey string) (string, error) {
	inputDir, err := sqlcgen.New(s.defaultDB).GetIngestInputDir(ctx, configKey)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(inputDir), nil
}

func (s *Store) SetIngestInputDir(ctx context.Context, configKey, inputDir string) error {
	return sqlcgen.New(s.defaultDB).SetIngestInputDir(ctx, sqlcgen.SetIngestInputDirParams{
		IngestInputDir: strings.TrimSpace(inputDir),
		ConfigKey:      configKey,
	})
}

func (s *Store) CountReplays(ctx context.Context) (int64, error) {
	return sqlcgen.New(s.defaultDB).CountReplays(ctx)
}

func (s *Store) GetReplayFilePathByID(ctx context.Context, replayID int64) (string, error) {
	return sqlcgen.New(s.replayScoped()).GetReplayFilePathByID(ctx, replayID)
}

type PlayerColorRow struct {
	PlayerKey string
	Games     int64
}

func (s *Store) ListTopPlayerColorRows(ctx context.Context) ([]PlayerColorRow, error) {
	sqlcRows, err := sqlcgen.New(s.replayScoped()).ListTopPlayerColorRows(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]PlayerColorRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		result = append(result, PlayerColorRow{
			PlayerKey: row.PlayerKey,
			Games:     row.Games,
		})
	}
	return result, nil
}
