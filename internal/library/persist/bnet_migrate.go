package persist

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/marianogappa/screpdb/internal/bnetfacade"
	"github.com/marianogappa/screpdb/internal/iofacade"
)

// The pre-v2 caches: one JSON file per (toon, gateway) carrying the whole
// upstream payload, and one JSON file per account accumulating game results.
// The migration below distils them into the two v2 stores and deletes the
// directories. It must stay for as long as upgrades from those releases are
// possible — the game history in both is unrecoverable, Battle.net only
// returns an account's last ~25 games.
const (
	legacyBnetProfilesDirName    = "bnet_profiles"
	legacyBnetGameResultsDirName = "bnet_game_results"
)

type legacyBnetProfileEntry struct {
	Toon        string    `json:"toon"`
	Gateway     int64     `json:"gateway"`
	Found       bool      `json:"found"`
	AuroraID    int64     `json:"aurora_id"`
	BattleTag   string    `json:"battle_tag"`
	CountryCode string    `json:"country_code"`
	FetchedAt   time.Time `json:"fetched_at"`
	Payload     string    `json:"payload"`
}

type legacyBnetGameResult struct {
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

type legacyBnetGameResultsFile struct {
	AuroraID int64                           `json:"aurora_id"`
	Games    map[string]legacyBnetGameResult `json:"games"`
}

// MigrateLegacyBnetCaches carries the pre-v2 caches into the profile store and
// the game archive, then deletes them. Call it after both stores have loaded:
// a v2 entry always wins over a legacy one for the same key. Unreadable legacy
// files are skipped — everything they held either refills on the next fetch or
// was already beyond saving.
func MigrateLegacyBnetCaches(root string, cache *BnetCache, archive *BnetGameArchive) (profiles, games int, err error) {
	profiles, games, err = migrateLegacyBnetProfiles(root, cache, archive)
	if err != nil {
		return profiles, games, err
	}
	moreGames, err := migrateLegacyBnetGameResults(root, archive)
	return profiles, games + moreGames, err
}

func migrateLegacyBnetProfiles(root string, cache *BnetCache, archive *BnetGameArchive) (int, int, error) {
	dir := filepath.Join(root, legacyBnetProfilesDirName)
	if _, err := iofacade.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return 0, 0, nil
	} else if err != nil {
		return 0, 0, err
	}
	var toStore []BnetProfile
	var toArchive []BnetGame
	walkErr := iofacade.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		raw, readErr := iofacade.ReadFile(path)
		if readErr != nil {
			return nil
		}
		var entry legacyBnetProfileEntry
		if json.Unmarshal(raw, &entry) != nil || strings.TrimSpace(entry.Toon) == "" {
			return nil
		}
		if existing := cache.Get(entry.Toon, entry.Gateway); existing != nil && !existing.FetchedAt.Before(entry.FetchedAt) {
			return nil
		}
		// An entry an older build mojibaked would distil a double-encoded
		// battle tag; dropping it makes the next fetch replace it, exactly
		// what the old read path did.
		if bnetfacade.IsMojibakedPayload([]byte(entry.Payload)) {
			return nil
		}
		profile, entryGames := DistillBnetProfile(entry.Toon, entry.Gateway, entry.FetchedAt, []byte(entry.Payload))
		// The header knows more than an empty payload does; keep its identity
		// fields even when there is nothing to distil.
		if !profile.Found && entry.Found {
			profile.Found = true
			profile.AuroraID = entry.AuroraID
			profile.BattleTag = entry.BattleTag
			profile.CountryCode = entry.CountryCode
		}
		toStore = append(toStore, profile)
		toArchive = append(toArchive, entryGames...)
		return nil
	})
	if walkErr != nil {
		return 0, 0, walkErr
	}
	if err := cache.UpsertBatch(toStore); err != nil {
		return 0, 0, err
	}
	if err := archive.Upsert(toArchive); err != nil {
		return 0, 0, err
	}
	if err := iofacade.RemoveAll(dir); err != nil {
		return len(toStore), len(toArchive), err
	}
	return len(toStore), len(toArchive), nil
}

func migrateLegacyBnetGameResults(root string, archive *BnetGameArchive) (int, error) {
	dir := filepath.Join(root, legacyBnetGameResultsDirName)
	if _, err := iofacade.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	var toArchive []BnetGame
	walkErr := iofacade.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		raw, readErr := iofacade.ReadFile(path)
		if readErr != nil {
			return nil
		}
		var file legacyBnetGameResultsFile
		if json.Unmarshal(raw, &file) != nil || file.AuroraID == 0 {
			return nil
		}
		for _, g := range file.Games {
			if game, ok := legacyGameToBnetGame(file.AuroraID, g); ok {
				toArchive = append(toArchive, game)
			}
		}
		return nil
	})
	if walkErr != nil {
		return 0, walkErr
	}
	if err := archive.Upsert(toArchive); err != nil {
		return 0, err
	}
	if err := iofacade.RemoveAll(dir); err != nil {
		return len(toArchive), err
	}
	return len(toArchive), nil
}

func legacyGameToBnetGame(auroraID int64, g legacyBnetGameResult) (BnetGame, bool) {
	if g.GameID == "" || g.CreateTime.IsZero() {
		return BnetGame{}, false
	}
	game := BnetGame{
		GameID:     g.GameID,
		CreateTime: g.CreateTime.UTC(),
		Gateway:    g.Gateway,
		Ladder:     g.MatchGUID != "",
		MapName:    g.MapName,
		Accounts:   []int64{auroraID},
	}
	if toon := strings.TrimSpace(g.Toon); toon != "" {
		game.Players = []BnetGamePlayer{{
			Toon:    toon,
			Race:    strings.ToLower(strings.TrimSpace(g.Race)),
			Result:  BnetResultFromString(g.Result),
			APM:     g.APM,
			Seconds: g.DurationSeconds,
		}}
	}
	return game, true
}
