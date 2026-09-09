package bnetfacade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

var ErrReplayGone = errors.New("bnetfacade: replay no longer on GCS")

// GameInfoReplay is one participant's uploaded copy of a game, as returned by
// the matchmaker-gameinfo-playerinfo bridge endpoint. Every participant of the
// game gets an entry once their client uploads; URL points at the public GCS
// object and MD5 is the file's checksum (also the GCS object's basename).
type GameInfoReplay struct {
	MD5        string            `json:"md5"`
	URL        string            `json:"url"`
	CreateTime int64             `json:"create_time"`
	Attributes map[string]string `json:"attributes"`
}

// GameInfo is the subset of the matchmaker-gameinfo-playerinfo payload the
// enrichment flow needs (issue #341).
type GameInfo struct {
	Replays []GameInfoReplay `json:"replays"`
}

// FetchGameInfo fetches every participant's replay entry for one game through
// the rate-limited bridge facade. The game id is the `link` value of an own
// profile's replays[] entry and is keyed by game alone: no participation is
// required.
func FetchGameInfo(ctx context.Context, addr, gameID string, prio Priority) (*GameInfo, error) {
	if strings.TrimSpace(gameID) == "" {
		return nil, errors.New("bnetfacade: empty game id")
	}
	path := "/web-api/v1/matchmaker-gameinfo-playerinfo/" + url.PathEscape(gameID)
	body, err := BridgeGet(ctx, addr, path, prio)
	if err != nil {
		return nil, err
	}
	var info GameInfo
	if err := json.Unmarshal(normalizeBridgeJSON(body), &info); err != nil {
		return nil, fmt.Errorf("bnetfacade: parsing game info: %w", err)
	}
	return &info, nil
}

// GCSReplayPath validates a replay URL from a bridge payload against the
// facade's outbound allowlist and returns the path DownloadReplay and
// HeadReplay accept.
func GCSReplayPath(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if u.Scheme != "https" || u.Host != gcsHost {
		return "", fmt.Errorf("%w: %s", ErrForbiddenHost, u.Host)
	}
	if !strings.HasPrefix(u.Path, gcsReplayPrefix) {
		return "", fmt.Errorf("%w: %q does not start with %s", ErrForbiddenPath, u.Path, gcsReplayPrefix)
	}
	return u.Path, nil
}

// HeadReplay returns the Content-Length of a replay on GCS. HEADs are
// unauthenticated, cost no bridge session and no download budget, so ranking
// co-player copies by size is free; callers bound the fan-out to the
// participant list. A missing object (GCS retention is roughly a month)
// returns ErrReplayGone.
func HeadReplay(ctx context.Context, path string) (int64, error) {
	if !strings.HasPrefix(path, gcsReplayPrefix) {
		return 0, fmt.Errorf("%w: %q does not start with %s", ErrForbiddenPath, path, gcsReplayPrefix)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, "https://"+gcsHost+path, nil)
	if err != nil {
		return 0, err
	}
	resp, err := downloadHTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden:
		return 0, ErrReplayGone
	case resp.StatusCode != http.StatusOK:
		return 0, fmt.Errorf("bnetfacade: GCS returned %s for HEAD %s", resp.Status, path)
	}
	if resp.ContentLength < 0 {
		return 0, fmt.Errorf("bnetfacade: GCS sent no Content-Length for %s", path)
	}
	return resp.ContentLength, nil
}
