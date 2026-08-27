package parser

import (
	"sync"

	"github.com/marianogappa/scfingerprint"
	"github.com/marianogappa/screpdb/internal/models"
)

// fingerprintModelTag loads the embedded scoring model once to identify it;
// an empty tag means the model could not be loaded, which never blocks
// extraction because raw vectors don't depend on the scoring model.
var fingerprintModelTag = sync.OnceValue(func() string {
	tag, err := scfingerprint.ModelTag()
	if err != nil {
		return ""
	}
	return tag
})

// extractFingerprintVectors computes one scfingerprint feature vector per
// eligible human player. Failure is non-fatal by design: a replay that yields
// no vectors (too short, too few commands, extraction error) still ingests
// fully — it just contributes nothing to fingerprint coverage.
func extractFingerprintVectors(r *scfingerprint.Replay) []models.PlayerFingerprintVector {
	pvs, err := scfingerprint.Extract(r)
	if err != nil || len(pvs) == 0 {
		return nil
	}
	out := make([]models.PlayerFingerprintVector, 0, len(pvs))
	for _, pv := range pvs {
		out = append(out, models.PlayerFingerprintVector{
			PlayerID:       pv.PlayerID,
			Race:           pv.Race,
			Vector:         pv.Vector,
			Frames:         pv.Frames,
			CmdCount:       pv.CmdCount,
			FeatureVersion: scfingerprint.FeatureVersion(),
			ModelTag:       fingerprintModelTag(),
		})
	}
	return out
}
