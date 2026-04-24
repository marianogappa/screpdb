package db

import (
	"strings"

	"github.com/marianogappa/screpdb/internal/patterns/markers"
)

var workflowDurationSQLByKey = map[string]string{
	"under_10m": "r.duration_seconds < 600",
	"10_20m":    "r.duration_seconds >= 600 AND r.duration_seconds < 1200",
	"20_30m":    "r.duration_seconds >= 1200 AND r.duration_seconds < 1800",
	"30_45m":    "r.duration_seconds >= 1800 AND r.duration_seconds < 2700",
	"45m_plus":  "r.duration_seconds >= 2700",
}

func WorkflowDurationSQLByKey() map[string]string {
	out := make(map[string]string, len(workflowDurationSQLByKey))
	for key, value := range workflowDurationSQLByKey {
		out[key] = value
	}
	return out
}

func BuildWorkflowPlayersListBaseSQL(nameContainsNormalized string) (string, []any) {
	baseWhere := []string{"p.is_observer = 0", "lower(trim(coalesce(p.type, ''))) = 'human'"}
	args := []any{}
	if nameContainsNormalized != "" {
		baseWhere = append(baseWhere, "lower(trim(p.name)) LIKE ?")
		args = append(args, "%"+nameContainsNormalized+"%")
	}
	sqlText := `
		SELECT
			player_key,
			player_name,
			games_played,
			average_apm,
			last_played,
			CASE
				WHEN games_played <= 0 THEN 'Random'
				WHEN protoss_games * 1.0 / games_played > 0.67 THEN 'Protoss'
				WHEN terran_games * 1.0 / games_played > 0.67 THEN 'Terran'
				WHEN zerg_games * 1.0 / games_played > 0.67 THEN 'Zerg'
				ELSE 'Random'
			END AS race,
			COALESCE(CAST(julianday('now') - julianday(substr(last_played, 1, 19)) AS INTEGER), 0) AS last_played_days_ago
		FROM (
			SELECT
				lower(trim(p.name)) AS player_key,
				MIN(p.name) AS player_name,
				COUNT(*) AS games_played,
				COALESCE(AVG(CASE WHEN p.apm > 0 THEN p.apm END), 0) AS average_apm,
				MAX(r.replay_date) AS last_played,
				SUM(CASE WHEN lower(trim(p.race)) = 'protoss' THEN 1 ELSE 0 END) AS protoss_games,
				SUM(CASE WHEN lower(trim(p.race)) = 'terran' THEN 1 ELSE 0 END) AS terran_games,
				SUM(CASE WHEN lower(trim(p.race)) = 'zerg' THEN 1 ELSE 0 END) AS zerg_games
			FROM players p
			JOIN replays r ON r.id = p.replay_id
			WHERE ` + strings.Join(baseWhere, " AND ") + `
			GROUP BY lower(trim(p.name))
		) grouped
	`
	return sqlText, args
}

func BuildWorkflowPlayersListWhere(onlyFivePlus bool, lastPlayedBuckets []string) (string, []any) {
	clauses := []string{}
	args := []any{}
	if onlyFivePlus {
		clauses = append(clauses, "games_played >= 5")
	}
	if len(lastPlayedBuckets) > 0 {
		bucketClauses := []string{}
		for _, bucket := range lastPlayedBuckets {
			switch strings.ToLower(strings.TrimSpace(bucket)) {
			case "1m", "30d":
				bucketClauses = append(bucketClauses, "last_played_days_ago <= 30")
			case "3m", "90d":
				bucketClauses = append(bucketClauses, "last_played_days_ago <= 90")
			}
		}
		if len(bucketClauses) > 0 {
			clauses = append(clauses, "("+strings.Join(bucketClauses, " OR ")+")")
		}
	}
	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func BuildWorkflowGamesListWhere(playerKeys, mapNames, durationBuckets, featuringKeys []string, durationSQLByKey map[string]string) (string, []any) {
	clauses := []string{}
	args := []any{}
	if len(playerKeys) > 0 {
		playerPlaceholders := strings.TrimRight(strings.Repeat("?, ", len(playerKeys)), ", ")
		clauses = append(clauses, "EXISTS (SELECT 1 FROM players p WHERE p.replay_id = r.id AND p.is_observer = 0 AND lower(trim(p.name)) IN ("+playerPlaceholders+"))")
		for _, key := range playerKeys {
			args = append(args, key)
		}
	}
	if len(mapNames) > 0 {
		mapPlaceholders := strings.TrimRight(strings.Repeat("?, ", len(mapNames)), ", ")
		clauses = append(clauses, "lower(trim(r.map_name)) IN ("+mapPlaceholders+")")
		for _, mapName := range mapNames {
			args = append(args, strings.ToLower(strings.TrimSpace(mapName)))
		}
	}
	if len(durationBuckets) > 0 {
		durationClauses := []string{}
		for _, key := range durationBuckets {
			if sqlExpr, ok := durationSQLByKey[key]; ok && strings.TrimSpace(sqlExpr) != "" {
				durationClauses = append(durationClauses, "("+sqlExpr+")")
			}
		}
		if len(durationClauses) > 0 {
			clauses = append(clauses, "("+strings.Join(durationClauses, " OR ")+")")
		}
	}
	if len(featuringKeys) > 0 {
		featureClauses := []string{}
		for _, featureKey := range featuringKeys {
			existsSQL, ok := workflowFeaturingExistsSQL(featureKey)
			if !ok {
				continue
			}
			featureClauses = append(featureClauses, existsSQL)
		}
		if len(featureClauses) > 0 {
			clauses = append(clauses, "("+strings.Join(featureClauses, " OR ")+")")
		}
	}
	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

// uiFeatureKeyToMarkerFeatureKey bridges the frontend filter keys (short aliases
// like "nukes" / "recalls") to the canonical marker FeatureKeys. Callers pass
// either form; this map normalises to the registry's FeatureKey.
var uiFeatureKeyToMarkerFeatureKey = map[string]string{
	"nukes":   "threw_nukes",
	"recalls": "made_recalls",
}

func workflowFeaturingExistsSQL(featureKey string) (string, bool) {
	normalized := strings.TrimSpace(strings.ToLower(featureKey))
	switch normalized {
	case "cannon_rush", "bunker_rush", "zergling_rush":
		// These are narrative game-events, not markers — they live in replay_events
		// with event_kind='game_event'.
		return `EXISTS (
			SELECT 1
			FROM replay_events re
			WHERE re.replay_id = r.id
				AND re.event_kind = 'game_event'
				AND re.event_type = '` + normalized + `'
		)`, true
	case "mind_control":
		// Composite: either became_terran OR became_zerg marker on any player.
		return `EXISTS (
			SELECT 1
			FROM replay_events re
			WHERE re.replay_id = r.id
				AND re.event_kind = 'marker'
				AND re.event_type IN ('became_terran', 'became_zerg')
		)`, true
	}
	lookup := normalized
	if alias, ok := uiFeatureKeyToMarkerFeatureKey[normalized]; ok {
		lookup = alias
	}
	// Everything else is a marker keyed by its FeatureKey. The markers registry is the
	// source of truth — if a feature-key isn't registered, the filter is a no-op.
	if marker := markers.ByFeatureKey(lookup); marker != nil {
		escaped := strings.ReplaceAll(marker.FeatureKey, "'", "''")
		return `EXISTS (
			SELECT 1
			FROM replay_events re
			WHERE re.replay_id = r.id
				AND re.event_kind = 'marker'
				AND re.event_type = '` + escaped + `'
		)`, true
	}
	return "", false
}
