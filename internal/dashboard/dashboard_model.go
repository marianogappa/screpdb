package dashboard

import (
	"context"
	"database/sql/driver"
	"encoding/json"

	jetmodel "github.com/marianogappa/screpdb/internal/jet/screpdb/public/model"
)

// DashboardWithVariables extends the jet model with Variables
type DashboardWithVariables struct {
	jetmodel.Dashboards
	Variables DashboardVariables `json:"variables,omitempty"`
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
	// Use raw SQL to get the dashboard with variables
	query := `
		SELECT url, name, description, variables, created_at
		FROM dashboards
		WHERE url = $1
	`

	var dash DashboardWithVariables
	var variablesJSON []byte
	err := d.db.QueryRowContext(ctx, query, url).Scan(
		&dash.URL,
		&dash.Name,
		&dash.Description,
		&variablesJSON,
		&dash.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Parse variables JSON
	if len(variablesJSON) > 0 {
		if err := json.Unmarshal(variablesJSON, &dash.Variables); err != nil {
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

	query := `
		UPDATE dashboards
		SET variables = $1::jsonb
		WHERE url = $2
	`

	_, err = d.db.ExecContext(ctx, query, string(variablesJSON), url)
	return err
}

