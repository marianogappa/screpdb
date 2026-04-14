package db

import (
	"context"
	"time"

	"github.com/samber/lo"

	"github.com/marianogappa/screpdb/internal/dashboard/db/sqlcgen"
)

type DashboardWithVariablesRow struct {
	URL              string
	Name             string
	Description      *string
	ReplaysFilterSQL *string
	VariablesJSON    []byte
	CreatedAt        *time.Time
}

func (s *Store) GetDashboardWithVariablesByURL(ctx context.Context, url string) (*DashboardWithVariablesRow, error) {
	sqlcRow, err := sqlcgen.New(s.defaultDB).GetDashboardWithVariablesByURL(ctx, url)
	if err != nil {
		return nil, err
	}
	row := DashboardWithVariablesRow{
		URL:              sqlcRow.Url,
		Name:             sqlcRow.Name,
		Description:      sqlcRow.Description,
		ReplaysFilterSQL: sqlcRow.ReplaysFilterSql,
		VariablesJSON:    []byte(lo.FromPtr(sqlcRow.Variables)),
	}
	parsedCreatedAt, parseErr := parseTimeString(lo.FromPtr(sqlcRow.CreatedAt))
	if parseErr == nil {
		row.CreatedAt = parsedCreatedAt
	}
	return &row, nil
}

func (s *Store) UpdateDashboardVariables(ctx context.Context, url string, variablesJSON string) error {
	return sqlcgen.New(s.defaultDB).UpdateDashboardVariables(ctx, sqlcgen.UpdateDashboardVariablesParams{
		Variables: lo.ToPtr(variablesJSON),
		Url:       url,
	})
}
