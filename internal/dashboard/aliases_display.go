package dashboard

import (
	"fmt"
	"strings"

	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
)

func aliasSourcePriority(source string) int {
	switch strings.TrimSpace(source) {
	case aliasSourceYou:
		return 3
	case aliasSourceManual:
		return 2
	case aliasSourceImported:
		return 1
	default:
		return 0
	}
}

func chooseBetterAlias(current *dashboarddb.PlayerAliasRow, candidate dashboarddb.PlayerAliasRow) bool {
	if current == nil {
		return true
	}
	currentPriority := aliasSourcePriority(current.Source)
	candidatePriority := aliasSourcePriority(candidate.Source)
	if candidatePriority != currentPriority {
		return candidatePriority > currentPriority
	}
	currentUpdated := strings.TrimSpace(current.UpdatedAt)
	candidateUpdated := strings.TrimSpace(candidate.UpdatedAt)
	if candidateUpdated != currentUpdated {
		return candidateUpdated > currentUpdated
	}
	return strings.TrimSpace(candidate.CanonicalAlias) < strings.TrimSpace(current.CanonicalAlias)
}

func formatDisplayNameWithAlias(name string, alias string) string {
	name = strings.TrimSpace(name)
	alias = strings.TrimSpace(alias)
	if name == "" || alias == "" {
		return name
	}
	suffix := fmt.Sprintf("(%s)", alias)
	if strings.EqualFold(name, alias) || strings.Contains(name, suffix) {
		return name
	}
	return fmt.Sprintf("%s %s", name, suffix)
}

func buildBestAliasRowByLookupKey(rows []dashboarddb.PlayerAliasRow) map[string]dashboarddb.PlayerAliasRow {
	bestByTag := map[string]dashboarddb.PlayerAliasRow{}
	for _, row := range rows {
		tag := normalizeAliasBattleTag(row.BattleTagNormalized)
		if tag == "" {
			continue
		}
		for _, key := range battleTagLookupKeys(tag) {
			current, hasCurrent := bestByTag[key]
			if !hasCurrent || chooseBetterAlias(&current, row) {
				bestByTag[key] = row
			}
		}
	}
	return bestByTag
}

func displayNamesWithAliasRows(names []string, bestByTag map[string]dashboarddb.PlayerAliasRow) map[string]string {
	display := map[string]string{}
	for _, name := range names {
		normalizedName := normalizeAliasBattleTag(name)
		var best *dashboarddb.PlayerAliasRow
		for _, key := range battleTagLookupKeys(normalizedName) {
			row, ok := bestByTag[key]
			if !ok {
				continue
			}
			if best == nil || chooseBetterAlias(best, row) {
				next := row
				best = &next
			}
		}
		if best != nil {
			display[name] = formatDisplayNameWithAlias(name, best.CanonicalAlias)
		}
	}
	return display
}

func (d *Dashboard) aliasDisplayNames(names []string) (map[string]string, error) {
	display := map[string]string{}
	if len(names) == 0 {
		return display, nil
	}
	rows, err := d.dbStore.ListPlayerAliases(d.ctx)
	if err != nil {
		return nil, err
	}
	bestByTag := buildBestAliasRowByLookupKey(rows)
	return displayNamesWithAliasRows(names, bestByTag), nil
}
