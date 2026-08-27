package storage

import (
	"context"
	"fmt"

	"github.com/marianogappa/screpdb/internal/fpvec"
	"github.com/marianogappa/screpdb/internal/models"
)

func (s *SQLiteStorage) insertFingerprintVectorsBatchTx(ctx context.Context, db dbtx, replayID int64, playerIDMap map[byte]int64, vectors []models.PlayerFingerprintVector) error {
	if len(vectors) == 0 {
		return nil
	}

	stmt, err := db.PrepareContext(ctx, `
		INSERT INTO player_fingerprint_vectors (
			replay_id, player_id, feature_version, model_tag, race, frames, cmd_count, vector
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare fingerprint vector insert: %w", err)
	}
	defer stmt.Close()

	for _, v := range vectors {
		dbPlayerID, ok := playerIDMap[v.PlayerID]
		if !ok {
			continue
		}
		if _, err := stmt.ExecContext(ctx,
			replayID,
			dbPlayerID,
			v.FeatureVersion,
			v.ModelTag,
			v.Race,
			v.Frames,
			v.CmdCount,
			fpvec.Encode(v.Vector),
		); err != nil {
			return fmt.Errorf("failed to insert fingerprint vector: %w", err)
		}
	}
	return nil
}
