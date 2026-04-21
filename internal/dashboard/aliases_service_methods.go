package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/marianogappa/screpdb/internal/dashboard/apigen"
	dashboardservice "github.com/marianogappa/screpdb/internal/dashboard/service"
)

func (d *Dashboard) ListAliases(ctx context.Context, _ apigen.ListAliasesRequestObject) (any, error) {
	rows, err := d.dbStore.ListPlayerAliases(ctx)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return map[string]any{"aliases": rows}, nil
}

func (d *Dashboard) ImportAliases(ctx context.Context, request apigen.ImportAliasesRequestObject) (any, error) {
	if request.Body == nil {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("request body is required"))
	}
	raw, err := json.Marshal(request.Body.Aliases)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, err)
	}
	records, err := parseAliasImportJSON(raw, aliasSourceImported)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, err)
	}
	if err := upsertPlayerAliases(ctx, d.db, records); err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return map[string]any{"ok": true, "imported": len(records)}, nil
}

func (d *Dashboard) UpsertAliasEntry(ctx context.Context, request apigen.UpsertAliasEntryRequestObject) (any, error) {
	if request.Body == nil {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("request body is required"))
	}
	canonicalAlias := strings.TrimSpace(request.Body.CanonicalAlias)
	if canonicalAlias == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("canonical_alias is required"))
	}
	battleTagRaw := strings.TrimSpace(request.Body.BattleTag)
	if battleTagRaw == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("battle_tag is required"))
	}
	if aliasCanonicalEqualsBattleTag(canonicalAlias, battleTagRaw) {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("canonical_alias must differ from battle_tag"))
	}

	source := aliasSourceManual
	if request.Body.Source != nil {
		source = strings.TrimSpace(string(*request.Body.Source))
	}
	switch source {
	case aliasSourceManual, aliasSourceImported, aliasSourceYou:
	default:
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("invalid alias source"))
	}

	record := aliasUpsertRecord{
		CanonicalAlias:      canonicalAlias,
		BattleTagRaw:        battleTagRaw,
		BattleTagNormalized: normalizeAliasBattleTag(battleTagRaw),
		AuroraID:            request.Body.AuroraId,
		Source:              source,
	}
	if err := upsertPlayerAliases(ctx, d.db, []aliasUpsertRecord{record}); err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return map[string]any{"ok": true}, nil
}

func (d *Dashboard) DeleteAliasEntry(ctx context.Context, request apigen.DeleteAliasEntryRequestObject) (any, error) {
	if err := d.dbStore.DeletePlayerAliasByID(ctx, request.Id); err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return map[string]any{"ok": true}, nil
}
