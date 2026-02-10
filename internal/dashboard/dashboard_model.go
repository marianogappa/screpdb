package dashboard

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"time"
)

// DashboardWithVariables represents a dashboard and its variables
type DashboardWithVariables struct {
	URL         string             `json:"url"`
	Name        string             `json:"name"`
	Description *string            `json:"description,omitempty"`
	CreatedAt   *time.Time         `json:"created_at,omitempty"`
	Variables   DashboardVariables `json:"variables,omitempty"`
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
		WHERE url = ?
	`

	var dash DashboardWithVariables
	var description sql.NullString
	var variablesJSON []byte
	var createdAt any
	err := d.db.QueryRowContext(ctx, query, url).Scan(
		&dash.URL,
		&dash.Name,
		&description,
		&variablesJSON,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}

	if description.Valid {
		dash.Description = &description.String
	}
	if parsed, err := nullableTime(createdAt); err == nil {
		dash.CreatedAt = parsed
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
		SET variables = ?
		WHERE url = ?
	`

	_, err = d.db.ExecContext(ctx, query, string(variablesJSON), url)
	return err
}
