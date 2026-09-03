// Package legacyimport reads the dashboard state that older releases kept in
// screp.db (the settings row and the Battle.net profile cache) so a first
// launch after the upgrade keeps the user's replay folder, filters and the
// rate-limited profile cache. It opens the database read-only and never
// writes to it; screp.db stays owned by the CLI ingest and MCP paths.
package legacyimport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/marianogappa/screpdb/internal/iofacade"
	_ "modernc.org/sqlite"
)

var ErrNoDatabase = errors.New("legacyimport: no legacy database")

type Settings struct {
	ReplayDir         string
	GameTypes         []string
	ExcludeShortGames bool
	ExcludeComputers  bool
	MapKinds          []string
	FeatureFlags      map[string]bool
}

type BnetProfile struct {
	Toon        string
	Gateway     int
	Found       bool
	AuroraID    int64
	BattleTag   string
	CountryCode string
	Payload     string
	FetchedAt   string
}

type Result struct {
	Settings *Settings
	Profiles []BnetProfile
}

func Read(ctx context.Context, dbPath string) (Result, error) {
	var out Result
	if _, err := iofacade.Stat(dbPath); err != nil {
		return out, fmt.Errorf("%w: %s", ErrNoDatabase, dbPath)
	}
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return out, err
	}
	defer db.Close()

	if has, err := hasTable(ctx, db, "settings"); err != nil {
		return out, err
	} else if has {
		out.Settings, err = readSettings(ctx, db)
		if err != nil {
			return out, err
		}
	}
	if has, err := hasTable(ctx, db, "bnet_profiles"); err != nil {
		return out, err
	} else if has {
		out.Profiles, err = readProfiles(ctx, db)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

func hasTable(ctx context.Context, db *sql.DB, name string) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&n)
	return n > 0, err
}

func readSettings(ctx context.Context, db *sql.DB) (*Settings, error) {
	rows, err := db.QueryContext(ctx, `SELECT * FROM settings WHERE config_key = 'global'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		return nil, rows.Err()
	}
	values := make([]any, len(columns))
	ptrs := make([]any, len(columns))
	for i := range values {
		ptrs[i] = &values[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return nil, err
	}
	row := map[string]any{}
	for i, col := range columns {
		row[col] = values[i]
	}

	s := &Settings{
		ReplayDir:         strings.TrimSpace(asString(row["ingest_input_dir"])),
		ExcludeShortGames: asBool(row["exclude_short_games"], true),
		ExcludeComputers:  asBool(row["exclude_computers"], true),
		GameTypes:         asStringSlice(row["game_types"]),
		MapKinds:          asStringSlice(row["map_kinds"]),
		FeatureFlags:      asBoolMap(row["feature_flags"]),
	}
	if len(s.GameTypes) == 0 {
		legacy := strings.ToLower(strings.TrimSpace(asString(row["game_type"])))
		// "ums" stopped being a game-type filter; UMS games are excluded outright.
		if legacy != "" && legacy != "all" && legacy != "ums" {
			s.GameTypes = []string{legacy}
		}
	}
	return s, nil
}

func readProfiles(ctx context.Context, db *sql.DB) ([]BnetProfile, error) {
	rows, err := db.QueryContext(ctx, `SELECT toon, gateway, found, aurora_id, battle_tag, country_code, payload, fetched_at FROM bnet_profiles ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BnetProfile
	for rows.Next() {
		var p BnetProfile
		if err := rows.Scan(&p.Toon, &p.Gateway, &p.Found, &p.AuroraID, &p.BattleTag, &p.CountryCode, &p.Payload, &p.FetchedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case nil:
		return ""
	default:
		return fmt.Sprint(t)
	}
}

func asBool(v any, fallback bool) bool {
	switch t := v.(type) {
	case bool:
		return t
	case int64:
		return t != 0
	case float64:
		return t != 0
	case string:
		return t == "1" || strings.EqualFold(t, "true")
	case []byte:
		return asBool(string(t), fallback)
	}
	return fallback
}

func asStringSlice(v any) []string {
	raw := strings.TrimSpace(asString(v))
	if raw == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func asBoolMap(v any) map[string]bool {
	raw := strings.TrimSpace(asString(v))
	out := map[string]bool{}
	if raw == "" {
		return out
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return map[string]bool{}
	}
	return out
}
