package db

import (
	"fmt"
	"strings"
)

func BuildGlobalReplayFilterSQL(
	excludeShortGames bool,
	shortGameSeconds int,
	excludeComputers bool,
	gameTypes []string,
	mapKinds []string,
) string {
	clauses := []string{}
	// UMS replays are unsupported globally — auto-discarded at ingest, and
	// any pre-existing rows from older databases are excluded here so the
	// rest of the app never sees them. Hardcoded so it survives any user
	// filter combination.
	clauses = append(clauses, "r.map_kind != 'UseMapSettings'")
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

	if pred := normalizeSQLWhitespace(strings.TrimSpace(gameTypePredicateSQL(gameTypes))); pred != "" {
		clauses = append(clauses, "("+pred+")")
	}
	if pred := normalizeSQLWhitespace(strings.TrimSpace(mapKindPredicateSQL(mapKinds))); pred != "" {
		clauses = append(clauses, "("+pred+")")
	}

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
		case "free_for_all":
			predicates = append(predicates, "lower(trim(coalesce(r.game_type, ''))) = 'free for all'")
		}
	}
	if len(predicates) == 0 {
		return ""
	}
	return strings.Join(predicates, " OR ")
}

// mapKindPredicateSQL builds a predicate over the replays.map_kind column.
// Inputs are the lowercase API enum values ('regular', 'money'); they're
// translated here to the storage-side casing ('Regular', 'Money'). UMS is
// not a valid input — it's excluded globally upstream.
func mapKindPredicateSQL(values []string) string {
	predicates := make([]string, 0, len(values))
	for _, value := range values {
		switch value {
		case "regular":
			predicates = append(predicates, "r.map_kind = 'Regular'")
		case "money":
			predicates = append(predicates, "r.map_kind = 'Money'")
		}
	}
	if len(predicates) == 0 {
		return ""
	}
	return strings.Join(predicates, " OR ")
}

