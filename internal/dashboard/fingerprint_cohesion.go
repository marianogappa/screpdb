package dashboard

import (
	"github.com/marianogappa/scfingerprint"
	"github.com/marianogappa/screpdb/internal/dashboard/db"
)

// The gates and threshold were set by a leave-one-out study (issue #345) on
// 30k labelled ladder 1v1 player-games: with 20+ same-race reference games
// and an 8+ minute game, 0 of 8,692 games known to be the handle's own human
// scored below z 2.19, while 83% of games spliced in from other humans scored
// below 1.5. Shorter games and fewer references both let genuine games
// through, so loosen none of these without re-running the study.
const (
	cohesionMinReferenceGames = 20
	cohesionMinGameFrames     = 11429
	cohesionMaxZ              = 1.5
)

func (d *Dashboard) playsUnlikeHandle(playerKey string, replayID int64, featureVersion int) bool {
	rows, err := d.dbStore.ListPlayerFingerprintVectors(d.ctx, playerKey, int64(featureVersion))
	if err != nil {
		return false
	}
	return gamePlaysUnlikeOthers(rows, replayID)
}

func gamePlaysUnlikeOthers(rows []db.PlayerFingerprintVectorRow, replayID int64) bool {
	var probe *db.PlayerFingerprintVectorRow
	for i := range rows {
		if rows[i].ReplayID == replayID {
			probe = &rows[i]
			break
		}
	}
	if probe == nil || len(probe.Vector) == 0 || probe.Frames < cohesionMinGameFrames {
		return false
	}
	refs := make([]scfingerprint.PlayerGame, 0, len(rows))
	for _, row := range rows {
		if row.ReplayID == replayID || row.Race != probe.Race || len(row.Vector) == 0 {
			continue
		}
		refs = append(refs, scfingerprint.PlayerGame{Vector: row.Vector, Race: row.Race})
	}
	if len(refs) < cohesionMinReferenceGames {
		return false
	}
	verdict, err := scfingerprint.Same([]scfingerprint.PlayerGame{{Vector: probe.Vector, Race: probe.Race}}, refs)
	if err != nil {
		return false
	}
	return verdict.Z < cohesionMaxZ
}
