package dashboard

import (
	"fmt"
	"strings"
)

func (d *Dashboard) resolveWorkflowMapVisual(replayID int64, mapName string, replayFilePath string) workflowMapVisual {
	out := workflowMapVisual{
		RequestedMap: strings.TrimSpace(mapName),
	}
	if out.RequestedMap == "" {
		out.ResolutionNote = "missing replay map name"
		return out
	}
	if replayID <= 0 || strings.TrimSpace(replayFilePath) == "" {
		out.ResolutionNote = "replay file unavailable for map preview"
		return out
	}

	url := fmt.Sprintf("/api/custom/game-assets/map?replay_id=%d", replayID)
	out.Available = true
	out.URL = url
	out.ThumbnailURL = url
	out.MatchedImage = "rendered"
	out.MatchedScore = 1
	return out
}
