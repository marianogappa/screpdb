package dashboard

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"time"
)

// DashboardWithVariables represents a dashboard and its variables
type DashboardWithVariables struct {
	URL              string             `json:"url"`
	Name             string             `json:"name"`
	Description      *string            `json:"description,omitempty"`
	ReplaysFilterSQL *string            `json:"replays_filter_sql,omitempty"`
	CreatedAt        *time.Time         `json:"created_at,omitempty"`
	Variables        DashboardVariables `json:"variables,omitempty"`
}

// DashboardVariables is a type alias for JSONB handling
type DashboardVariables []DashboardVariable

// Value implements driver.Valuer for JSONB
func (v DashboardVariables) Value() (driver.Value, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

// Scan implements sql.Scanner for JSONB
func (v *DashboardVariables) Scan(value any) error {
	if value == nil {
		*v = nil
		return nil
	}

	var bytes []byte
	switch val := value.(type) {
	case []byte:
		bytes = val
	case string:
		bytes = []byte(val)
	default:
		return nil
	}

	if len(bytes) == 0 {
		*v = nil
		return nil
	}

	return json.Unmarshal(bytes, v)
}

// GetDashboardByURL retrieves a dashboard by URL with variables
func (d *Dashboard) GetDashboardByURL(ctx context.Context, url string) (*DashboardWithVariables, error) {
	var dash DashboardWithVariables
	row, err := d.dbStore.GetDashboardWithVariablesByURL(ctx, url)
	if err != nil {
		return nil, err
	}
	dash.URL = row.URL
	dash.Name = row.Name
	dash.Description = row.Description
	dash.ReplaysFilterSQL = row.ReplaysFilterSQL
	dash.CreatedAt = row.CreatedAt

	// Parse variables JSON
	if len(row.VariablesJSON) > 0 {
		if err := json.Unmarshal(row.VariablesJSON, &dash.Variables); err != nil {
			return nil, err
		}
	}

	return &dash, nil
}

// UpdateDashboardVariables updates dashboard variables
func (d *Dashboard) UpdateDashboardVariables(ctx context.Context, url string, variables DashboardVariables) error {
	variablesJSON, err := json.Marshal(variables)
	if err != nil {
		return err
	}
	return d.dbStore.UpdateDashboardVariables(ctx, url, string(variablesJSON))
}
