package persist

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/marianogappa/screpdb/internal/iofacade"
)

const BnetGameResultsDirName = "bnet_game_results"

// BnetGameResult is one game a cached Battle.net profile reported. Battle.net
// only returns an account's last handful of games, so these accumulate over
// weeks into enough history to describe when someone plays.
type BnetGameResult struct {
	AuroraID        int64     `json:"aurora_id"`
	GameID          string    `json:"game_id"`
	CreateTime      time.Time `json:"create_time"`
	Toon            string    `json:"toon"`
	Gateway         int       `json:"gateway"`
	Race            string    `json:"race"`
	Result          string    `json:"result"`
	APM             int       `json:"apm"`
	DurationSeconds int       `json:"duration_seconds"`
	MapName         string    `json:"map_name"`
	MatchGUID       string    `json:"match_guid"`
}

type bnetGameResultsFile struct {
	AuroraID int64                     `json:"aurora_id"`
	Games    map[string]BnetGameResult `json:"games"`
}

// BnetGameResults stores one JSON file per account at
// <root>/bnet_game_results/<aurora id>.json. Accounts are read on first use
// and kept in memory; one mutex serialises every read and write.
type BnetGameResults struct {
	root   string
	mu     sync.Mutex
	loaded map[int64]map[string]BnetGameResult
}

func NewBnetGameResults(root string) *BnetGameResults {
	return &BnetGameResults{root: root, loaded: map[int64]map[string]BnetGameResult{}}
}

func (r *BnetGameResults) Dir() string { return filepath.Join(r.root, BnetGameResultsDirName) }

// BnetGameResultsPath is where an account's games live under root.
func BnetGameResultsPath(root string, auroraID int64) string {
	return filepath.Join(root, BnetGameResultsDirName, strconv.FormatInt(auroraID, 10)+".json")
}

// Upsert records the games a profile fetch reported. Games already on record
// are left alone: a game does not change after it was played.
func (r *BnetGameResults) Upsert(rows []BnetGameResult) error {
	byAccount := map[int64][]BnetGameResult{}
	for _, row := range rows {
		if row.AuroraID == 0 || row.GameID == "" {
			continue
		}
		byAccount[row.AuroraID] = append(byAccount[row.AuroraID], row)
	}
	if len(byAccount) == 0 {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for auroraID, accountRows := range byAccount {
		games, err := r.gamesLocked(auroraID)
		if err != nil {
			return err
		}
		added := false
		for _, row := range accountRows {
			if _, ok := games[row.GameID]; ok {
				continue
			}
			games[row.GameID] = row
			added = true
		}
		if !added {
			continue
		}
		if err := r.writeLocked(auroraID, games); err != nil {
			return err
		}
	}
	return nil
}

// TimesSince returns when an account played, newest first, back to since.
func (r *BnetGameResults) TimesSince(auroraID int64, since time.Time) ([]time.Time, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	games, err := r.gamesLocked(auroraID)
	if err != nil {
		return nil, err
	}
	out := make([]time.Time, 0, len(games))
	for _, game := range games {
		if game.CreateTime.Before(since) {
			continue
		}
		out = append(out, game.CreateTime.UTC())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].After(out[j]) })
	return out, nil
}

func (r *BnetGameResults) gamesLocked(auroraID int64) (map[string]BnetGameResult, error) {
	if games, ok := r.loaded[auroraID]; ok {
		return games, nil
	}
	games := map[string]BnetGameResult{}
	raw, err := iofacade.ReadFile(BnetGameResultsPath(r.root, auroraID))
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		var file bnetGameResultsFile
		// A truncated or hand-edited file is treated as an empty history: this
		// is a cache that refills itself from the next profile fetch.
		if err := json.Unmarshal(raw, &file); err == nil && file.Games != nil {
			games = file.Games
		}
	}
	r.loaded[auroraID] = games
	return games, nil
}

func (r *BnetGameResults) writeLocked(auroraID int64, games map[string]BnetGameResult) error {
	path := BnetGameResultsPath(r.root, auroraID)
	if err := iofacade.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(bnetGameResultsFile{AuroraID: auroraID, Games: games})
	if err != nil {
		return err
	}
	temp := path + ".tmp"
	if err := iofacade.WriteFile(temp, raw, 0o644); err != nil {
		return err
	}
	return iofacade.Rename(temp, path)
}
