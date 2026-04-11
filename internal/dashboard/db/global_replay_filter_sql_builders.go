package db

import (
	"fmt"
	"strings"
)

const (
	globalReplayFilterModeOnlyThese      = "only_these"
	globalReplayFilterModeAllExceptThese = "all_except_these"
)

func BuildGlobalReplayFilterSQL(
	excludeShortGames bool,
	shortGameSeconds int,
	excludeComputers bool,
	gameTypesMode string,
	gameTypes []string,
	mapFilterMode string,
	maps []string,
	playerFilterMode string,
	players []string,
) string {
	clauses := []string{}
	if excludeShortGames {
		clauses = append(clauses, fmt.Sprintf("r.duration_seconds >= %d", shortGameSeconds))
	}
	if excludeComputers {
		clauses = append(clauses, `NOT EXISTS (
			SELECT 1
			FROM players p
			WHERE p.replay_id = r.id
				AND p.is_observer = 0
				AND lower(trim(coalesce(p.type, ''))) IN ('computer', 'computer controlled')
		)`)
	}

	appendModeClause(&clauses, gameTypesMode, gameTypePredicateSQL(gameTypes))
	appendModeClause(&clauses, mapFilterMode, mapPredicateSQL(maps))
	appendModeClause(&clauses, playerFilterMode, playerPredicateSQL(players))

	query := "SELECT r.* FROM replays r"
	if len(clauses) == 0 {
		return query
	}
	return query + " WHERE " + strings.Join(clauses, " AND ")
}

func ComposeReplayFilterSQL(globalFilterSQL string, localFilterSQL string) string {
	globalNormalized := normalizeSQL(globalFilterSQL)
	localNormalized := normalizeSQL(localFilterSQL)
	switch {
	case globalNormalized == "" && localNormalized == "":
		return ""
	case globalNormalized == "":
		return localNormalized
	case localNormalized == "":
		return globalNormalized
	}
	return normalizeSQLWhitespace(fmt.Sprintf(
		"SELECT * FROM (%s) AS global_replays WHERE id IN (SELECT id FROM (%s) AS local_replays)",
		globalNormalized,
		localNormalized,
	))
}

func normalizeSQL(value string) string {
	trimmed := strings.TrimSpace(value)
	for strings.HasSuffix(trimmed, ";") {
		trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, ";"))
	}
	return trimmed
}

func normalizeSQLWhitespace(value string) string {
	fields := strings.Fields(normalizeSQL(value))
	return strings.Join(fields, " ")
}

func appendModeClause(clauses *[]string, mode string, predicate string) {
	predicate = normalizeSQLWhitespace(strings.TrimSpace(predicate))
	if predicate == "" {
		return
	}
	switch mode {
	case globalReplayFilterModeAllExceptThese:
		*clauses = append(*clauses, "NOT ("+predicate+")")
	default:
		*clauses = append(*clauses, "("+predicate+")")
	}
}

func gameTypePredicateSQL(values []string) string {
	predicates := make([]string, 0, len(values))
	for _, value := range values {
		switch value {
		case "top_vs_bottom":
			predicates = append(predicates, "lower(trim(coalesce(r.game_type, ''))) = 'top vs bottom'")
		case "melee":
			predicates = append(predicates, "lower(trim(coalesce(r.game_type, ''))) = 'melee'")
		case "one_on_one":
			predicates = append(predicates, `(2 = (
				SELECT COUNT(*)
				FROM players p
				WHERE p.replay_id = r.id
					AND p.is_observer = 0
			) AND 2 = (
				SELECT COUNT(DISTINCT p.team)
				FROM players p
				WHERE p.replay_id = r.id
					AND p.is_observer = 0
			))`)
		case "ums":
			predicates = append(predicates, "lower(trim(coalesce(r.game_type, ''))) IN ('use map settings', 'ums')")
		case "free_for_all":
			predicates = append(predicates, "lower(trim(coalesce(r.game_type, ''))) = 'free for all'")
		}
	}
	if len(predicates) == 0 {
		return ""
	}
	return strings.Join(predicates, " OR ")
}

func mapPredicateSQL(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return "lower(trim(coalesce(r.map_name, ''))) IN (" + joinQuotedSQLStrings(values) + ")"
}

func playerPredicateSQL(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return `EXISTS (
		SELECT 1
		FROM players p
		WHERE p.replay_id = r.id
			AND p.is_observer = 0
			AND lower(trim(coalesce(p.name, ''))) IN (` + joinQuotedSQLStrings(values) + `)
	)`
}

func joinQuotedSQLStrings(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		escaped := strings.ReplaceAll(value, "'", "''")
		quoted = append(quoted, "'"+escaped+"'")
	}
	return strings.Join(quoted, ", ")
}
