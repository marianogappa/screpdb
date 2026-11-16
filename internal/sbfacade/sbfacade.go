// Package sbfacade is the sanctioned surface for talking to ShieldBattery's
// servers in screpdb's Go binary.
//
// It makes exactly one kind of request: an anonymous GET of one game's record
// from https://shieldbattery.net/api/1/games/<uuid>, which ShieldBattery serves
// to anyone. The game id comes from the ShieldBattery section of the user's own
// replay, so a request reveals nothing about the user beyond having that
// replay. No credentials, cookies or account data are ever sent.
//
// ShieldBattery throttles that endpoint per IP (40 a minute, bursts of 80), so
// requests are paced well under it here, at the facade boundary, where no
// caller can bypass the pacing; a 429 pauses every request for a while.
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
	gamesURL = "https://shieldbattery.net/api/1/games/"
	// maxGameSize bounds the response read; a game record is a few KB.
	maxGameSize = 1 << 20

	requestInterval = 2 * time.Second
	rateLimitPause  = 2 * time.Minute
)

var (
	ErrNotFound    = errors.New("sbfacade: game not found")
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

// Game is the part of ShieldBattery's game record outcomes need.
type Game struct {
	ID string
	// Results is keyed by ShieldBattery user id. It is nil until the server
	// has reconciled the players' reports, which happens after the game ends.
	Results map[uint32]Result
	// Canceled games never get results.
	Canceled bool
}

type gameResponse struct {
	Game struct {
		ID         string            `json:"id"`
		Results    []json.RawMessage `json:"results"`
		CanceledAt *int64            `json:"canceledAt"`
	} `json:"game"`
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
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, ErrNotFound
	case http.StatusTooManyRequests:
		thePacer.pause(rateLimitPause)
		return nil, ErrRateLimited
	default:
		return nil, fmt.Errorf("sbfacade: %s for %s", resp.Status, url)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxGameSize))
	if err != nil {
		return nil, fmt.Errorf("sbfacade: reading game: %w", err)
	}
	return parseGame(body)
}

// parseGame reads a game record. Results arrive as [userId, {result, ...}]
// pairs.
func parseGame(body []byte) (*Game, error) {
	var resp gameResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("sbfacade: parsing game: %w", err)
	}
	game := &Game{ID: resp.Game.ID, Canceled: resp.Game.CanceledAt != nil}
	if resp.Game.Results == nil {
		return game, nil
	}
	game.Results = make(map[uint32]Result, len(resp.Game.Results))
	for _, raw := range resp.Game.Results {
		var pair []json.RawMessage
		if err := json.Unmarshal(raw, &pair); err != nil || len(pair) != 2 {
			return nil, fmt.Errorf("sbfacade: malformed result %s", raw)
		}
		var userID uint32
		var result struct {
			Result Result `json:"result"`
		}
		if err := json.Unmarshal(pair[0], &userID); err != nil {
			return nil, fmt.Errorf("sbfacade: malformed result user %s", pair[0])
		}
		if err := json.Unmarshal(pair[1], &result); err != nil {
			return nil, fmt.Errorf("sbfacade: malformed result %s", pair[1])
		}
		game.Results[userID] = result.Result
	}
	return game, nil
}

// pacer spaces requests requestInterval apart and holds them all back after a
// 429.
type pacer struct {
	mu   sync.Mutex
	next time.Time
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
	if until := time.Now().Add(d); until.After(p.next) {
		p.next = until
	}
}
