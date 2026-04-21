package dashboard

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func (d *Dashboard) resolveWorkflowMapVisual(replayID int64, mapName string, replayFilePath string, fileChecksum string) workflowMapVisual {
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

	q := url.Values{}
	q.Set("replay_id", strconv.FormatInt(replayID, 10))
	if ck := strings.TrimSpace(fileChecksum); ck != "" {
		q.Set("ck", ck)
	}
	urlStr := fmt.Sprintf("/api/custom/game-assets/map?%s", q.Encode())
	out.Available = true
	out.URL = urlStr
	out.ThumbnailURL = urlStr
	out.MatchedImage = "rendered"
	out.MatchedScore = 1
	return out
}
