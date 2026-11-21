// Package sbfacade is the sanctioned surface for talking to ShieldBattery's
// servers in screpdb's Go binary.
//
// It makes only these anonymous GETs, all of which ShieldBattery serves to
// anyone without signing in:
//
//   - one game's record from https://shieldbattery.net/api/1/games/<uuid>,
//     where the game id comes from the ShieldBattery section of the user's own
//     replay;
//   - one player's public profile from
//     https://shieldbattery.net/api/1/users/<id>/profile (ladder standing,
//     win/loss totals and the last 40 games), where the numeric user id comes
//     from a replay or from a game or profile record fetched here;
//   - the names behind numeric user ids from
//     https://shieldbattery.net/api/1/users/batch-info?u=<id>&u=<id>..., at
//     most 50 ids a request;
//   - a replay file from https://cdn.shieldbattery.net/replays/..., only at a
//     URL a game record handed out, and only kept when its bytes hash to the
//     sha256 that record states and look like a replay.
//
// A request reveals nothing about the user beyond an interest in those games
// and players. No credentials, cookies or account data are ever sent, and
// redirects are never followed.
//
// ShieldBattery throttles its API per IP (40 a minute, bursts of 80), so every
// request above, replay downloads included, shares one pacer at the facade
// boundary, where no caller can bypass it; a 429 pauses every request for a
// while.
//
// This package is exempt from TestNoDirectIOOutsideFacades alongside
// internal/netfacade, internal/bnetfacade and internal/selfupdate.
package sbfacade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"
)

const (
	apiURL   = "https://shieldbattery.net/api/1/"
	gamesURL = apiURL + "games/"
	// maxJSONSize bounds every API response read; a game record is a few KB
	// and a profile a few tens of KB.
	maxJSONSize = 1 << 20

	requestInterval = 2 * time.Second
	rateLimitPause  = 2 * time.Minute
)

var (
	ErrNotFound    = errors.New("sbfacade: not found")
	ErrRateLimited = errors.New("sbfacade: rate limited")
	ErrBadGameID   = errors.New("sbfacade: malformed game id")

	gameIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

	httpClient = &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
)

// Result is one player's result as ShieldBattery records it.
type Result string

const (
	ResultWin     Result = "win"
	ResultLoss    Result = "loss"
	ResultDraw    Result = "draw"
	ResultUnknown Result = "unknown"
)

const (
	SourceMatchmaking = "MATCHMAKING"
	SourceLobby       = "LOBBY"
)

// Game is the part of ShieldBattery's game record screpdb uses.
type Game struct {
	ID string
	// Results is keyed by ShieldBattery user id. It is nil until the server
	// has reconciled the players' reports, which happens after the game ends.
	Results map[uint32]Result
	// Canceled games never get results.
	Canceled bool

	StartTime time.Time
	// GameLength is zero while the game has no reconciled length.
	GameLength time.Duration
	// GameSource is SourceMatchmaking or SourceLobby.
	GameSource string
	// MatchmakingType is "1v1", "2v2", "2v2bgh"... for matchmaking games.
	MatchmakingType string

	// The replay fields are empty unless ShieldBattery keeps the replay, which
	// it does for ranked games and public lobbies. ReplayURL is signed and
	// expires after a day.
	ReplayURL      string
	ReplayHash     string
	ReplayFilename string
}

type gameConfig struct {
	Teams [][]struct {
		ID         uint32 `json:"id"`
		Race       string `json:"race"`
		IsComputer bool   `json:"isComputer"`
	} `json:"teams"`
	GameSource      string `json:"gameSource"`
	GameSourceExtra struct {
		Type string `json:"type"`
	} `json:"gameSourceExtra"`
}

type gameRecord struct {
	ID         string            `json:"id"`
	StartTime  int64             `json:"startTime"`
	MapID      string            `json:"mapId"`
	Config     gameConfig        `json:"config"`
	GameLength *int64            `json:"gameLength"`
	Results    []json.RawMessage `json:"results"`
	CanceledAt *int64            `json:"canceledAt"`
}

type replayRecord struct {
	GameID   string `json:"gameId"`
	URL      string `json:"url"`
	Hash     string `json:"hash"`
	Filename string `json:"filename"`
}

type gameResponse struct {
	Game   gameRecord    `json:"game"`
	Replay *replayRecord `json:"replay"`
}

// FetchGame fetches one game's record.
func FetchGame(ctx context.Context, gameID string) (*Game, error) {
	if !gameIDPattern.MatchString(gameID) {
		return nil, fmt.Errorf("%w: %q", ErrBadGameID, gameID)
	}
	if err := thePacer.wait(ctx); err != nil {
		return nil, err
	}
	return fetchGameFrom(ctx, gamesURL+gameID)
}

func fetchGameFrom(ctx context.Context, url string) (*Game, error) {
	body, err := getJSON(ctx, url)
	if err != nil {
		return nil, err
	}
	return parseGame(body)
}

func getJSON(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp, url); err != nil {
		return nil, err
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxJSONSize))
	if err != nil {
		return nil, fmt.Errorf("sbfacade: reading %s: %w", url, err)
	}
	return body, nil
}

func checkStatus(resp *http.Response, url string) error {
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusTooManyRequests:
		thePacer.pause(rateLimitPause)
		return ErrRateLimited
	default:
		return fmt.Errorf("sbfacade: %s for %s", resp.Status, url)
	}
}

func parseGame(body []byte) (*Game, error) {
	var resp gameResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("sbfacade: parsing game: %w", err)
	}
	g := resp.Game
	game := &Game{
		ID:              g.ID,
		Canceled:        g.CanceledAt != nil,
		StartTime:       unixMilli(g.StartTime),
		GameLength:      millis(g.GameLength),
		GameSource:      g.Config.GameSource,
		MatchmakingType: g.Config.GameSourceExtra.Type,
	}
	if resp.Replay != nil {
		game.ReplayURL = resp.Replay.URL
		game.ReplayHash = resp.Replay.Hash
		game.ReplayFilename = resp.Replay.Filename
	}
	results, err := parseResults(g.Results)
	if err != nil {
		return nil, err
	}
	if results != nil {
		game.Results = make(map[uint32]Result, len(results))
		for userID, r := range results {
			game.Results[userID] = r.Result
		}
	}
	return game, nil
}

type playerResult struct {
	Result Result `json:"result"`
	Race   string `json:"race"`
}

// parseResults reads results, which arrive as [userId, {result, race, ...}]
// pairs, or as null until reconciled.
func parseResults(raws []json.RawMessage) (map[uint32]playerResult, error) {
	if raws == nil {
		return nil, nil
	}
	results := make(map[uint32]playerResult, len(raws))
	for _, raw := range raws {
		var pair []json.RawMessage
		if err := json.Unmarshal(raw, &pair); err != nil || len(pair) != 2 {
			return nil, fmt.Errorf("sbfacade: malformed result %s", raw)
		}
		var userID uint32
		var result playerResult
		if err := json.Unmarshal(pair[0], &userID); err != nil {
			return nil, fmt.Errorf("sbfacade: malformed result user %s", pair[0])
		}
		if err := json.Unmarshal(pair[1], &result); err != nil {
			return nil, fmt.Errorf("sbfacade: malformed result %s", pair[1])
		}
		results[userID] = result
	}
	return results, nil
}

func unixMilli(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}

func millis(ms *int64) time.Duration {
	if ms == nil {
		return 0
	}
	return time.Duration(*ms) * time.Millisecond
}

// pacer spaces requests requestInterval apart and holds them all back after a
// 429.
type pacer struct {
	mu     sync.Mutex
	next   time.Time
	day    string
	used   int
	paused time.Time
}

var thePacer = &pacer{}

func (p *pacer) wait(ctx context.Context) error {
	p.mu.Lock()
	now := time.Now()
	at := p.next
	if at.Before(now) {
		at = now
	}
	p.next = at.Add(requestInterval)
	if day := now.Format(time.DateOnly); day != p.day {
		p.day, p.used = day, 0
	}
	p.used++
	p.mu.Unlock()
	if delay := at.Sub(now); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}

func (p *pacer) pause(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	until := time.Now().Add(d)
	if until.After(p.next) {
		p.next = until
	}
	p.paused = until
}

// Usage reports how many requests this process sent to ShieldBattery today, and until when requests are
// held back after ShieldBattery throttled them, or the zero time when they are not.
func Usage(now time.Time) (usedToday int, pausedUntil time.Time) {
	thePacer.mu.Lock()
	defer thePacer.mu.Unlock()
	if thePacer.day == now.Format(time.DateOnly) {
		usedToday = thePacer.used
	}
	if thePacer.paused.After(now) {
		pausedUntil = thePacer.paused
	}
	return usedToday, pausedUntil
}
