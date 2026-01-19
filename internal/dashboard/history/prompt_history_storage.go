package history

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"text/template"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/jackc/pgx/v5"
	"github.com/marianogappa/screpdb/internal/dashboard/variables"
	jetmodel "github.com/marianogappa/screpdb/internal/jet/screpdb/public/model"
	"github.com/marianogappa/screpdb/internal/jet/screpdb/public/table"
	"github.com/tmc/langchaingo/llms"
)

type PromptHistoryStorage struct {
	db        *sql.DB
	histories map[int64][]llms.MessageContent
	mutex     sync.Mutex
	debug     bool
}

func NewPromptHistoryStorage(db *sql.DB, debug bool) *PromptHistoryStorage {
	return &PromptHistoryStorage{
		db:        db,
		histories: map[int64][]llms.MessageContent{},
		mutex:     sync.Mutex{},
		debug:     debug,
	}
}

func (s *PromptHistoryStorage) get(ctx context.Context, widgetID int64) ([]llms.MessageContent, error) {
	history, ok := s.getFromMem(widgetID)
	if ok {
		return history, nil
	}
	history, ok, err := s.getFromDB(ctx, widgetID)
	if err != nil {
		return nil, err
	}
	if !ok {
		history, err := generateNewHistory()
		if err != nil {
			return nil, fmt.Errorf("error generating new history: %w", err)
		}
		err = s.set(ctx, widgetID, history)
		return history, err
	}
	s.setOnMem(widgetID, history)
	return history, nil
}

func (s *PromptHistoryStorage) set(ctx context.Context, widgetID int64, history []llms.MessageContent) error {
	s.setOnMem(widgetID, history)
	return s.setOnDB(ctx, widgetID, history)
}

func (s *PromptHistoryStorage) getFromMem(widgetID int64) ([]llms.MessageContent, bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	history, ok := s.histories[widgetID]
	return history, ok
}

func (s *PromptHistoryStorage) setOnMem(widgetID int64, history []llms.MessageContent) {
	s.mutex.Lock()
	s.histories[widgetID] = history
	s.mutex.Unlock()
}

func (s *PromptHistoryStorage) addOnMem(widgetID int64, history []llms.MessageContent) {
	s.mutex.Lock()
	s.histories[widgetID] = append(s.histories[widgetID], history...)
	s.mutex.Unlock()
}

func (s *PromptHistoryStorage) add(ctx context.Context, widgetID int64, history []llms.MessageContent) error {
	if _, err := s.get(ctx, widgetID); err != nil {
		return err
	}
	s.addOnMem(widgetID, history)
	allHistory, _ := s.getFromMem(widgetID)
	s.logf("added %v entries; total is now %v", len(history), len(allHistory))
	return s.set(ctx, widgetID, allHistory)
}

func (s *PromptHistoryStorage) setOnDB(ctx context.Context, widgetID int64, history []llms.MessageContent) error {
	historyBytes := historyToBytes(history)
	now := time.Now()

	// Use raw SQL for UPSERT since go-jet's ON_CONFLICT support may be limited
	// Cast to JSONB since the column is JSONB type
	query := `
		INSERT INTO dashboard_widget_prompt_history (widget_id, prompt_history, updated_at)
		VALUES ($1, $2::jsonb, $3)
		ON CONFLICT (widget_id) DO UPDATE SET
			prompt_history = EXCLUDED.prompt_history,
			updated_at = EXCLUDED.updated_at
	`

	_, err := s.db.ExecContext(ctx, query, widgetID, string(historyBytes), now)
	return err
}

func (s *PromptHistoryStorage) getFromDB(ctx context.Context, widgetID int64) ([]llms.MessageContent, bool, error) {
	var historyRow jetmodel.DashboardWidgetPromptHistory
	stmt := table.DashboardWidgetPromptHistory.SELECT(table.DashboardWidgetPromptHistory.AllColumns).
		WHERE(table.DashboardWidgetPromptHistory.WidgetID.EQ(postgres.Int64(widgetID)))

	err := stmt.QueryContext(ctx, s.db, &historyRow)
	if err == sql.ErrNoRows || err == pgx.ErrNoRows || err == qrm.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	history, err := bytesToHistory([]byte(historyRow.PromptHistory))
	if err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal prompt history bytes: %w", err)
	}
	return history, true, nil
}

func historyToBytes(history []llms.MessageContent) []byte {
	byts, _ := json.Marshal(history)
	return byts
}

func bytesToHistory(byts []byte) ([]llms.MessageContent, error) {
	history := []llms.MessageContent{}
	err := json.Unmarshal(byts, &history)
	return history, err
}

func generateNewHistory() ([]llms.MessageContent, error) {
	var sp bytes.Buffer
	vars := variables.GetAllVariables()
	if err := systemPromptTpl.Execute(&sp, struct {
		Variables map[string]variables.Variable
	}{
		Variables: vars,
	}); err != nil {
		return nil, err
	}

	return []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeSystem, sp.String())}, nil
}

func (s *PromptHistoryStorage) ForWidgetID(ctx context.Context, widgetID int64) (*PromptHistoryStorageForWidget, error) {
	phsw := &PromptHistoryStorageForWidget{
		phs:      s,
		ctx:      ctx,
		widgetID: widgetID,
	}
	_, err := phsw.phs.get(ctx, widgetID)
	return phsw, err
}

func (s *PromptHistoryStorage) logf(message string, args ...any) {
	if !s.debug {
		return
	}
	log.Printf(message, args...)
}

const (
	systemPromptTemplate = `You help to create Starcraft: Remastered dashboards. The prompts ask to create dashboard widgets. Each widget is a UI component fed from one SQL query.

You must choose ONE of the following widget types and provide the appropriate configuration:
1. gauge - Shows a single numeric value (e.g., total games, average APM)
2. table - Shows data in rows and columns
3. pie_chart - Shows proportions as pie slices (label, value columns)
4. bar_chart - Shows values as bars (label, value columns, optionally horizontal)
5. line_chart - Shows one or more series over time (x column, y columns, optional x-axis type like "seconds_from_game_start")
6. scatter_plot - Shows points on X/Y axes (x column, y column, optional size/color columns)
7. histogram - Shows distribution of values (value column, optional bins count)
8. heatmap - Shows 2D data as colored cells (x column, y column, value column)

The responses must be structured JSON which return:
- widget title
- widget description
- widget PostgreSQL query
- widget config (object with type and type-specific fields)

IMPORTANT CONFIGURATION RULES:
- For gauge: Set "type": "gauge", "gauge_value_column" to the column name with the value, optionally "gauge_min"/"gauge_max"/"gauge_label"
- For table: Set "type": "table"
- For pie_chart: Set "type": "pie_chart", "pie_label_column" and "pie_value_column"
- For bar_chart: Set "type": "bar_chart", "bar_label_column" and "bar_value_column", optionally "bar_horizontal": true
- For line_chart: Set "type": "line_chart", "line_x_column", "line_y_columns" (array), optionally "line_y_axis_from_zero": true, "line_x_axis_type": "seconds_from_game_start"|"timestamp"|"numeric"
- For scatter_plot: Set "type": "scatter_plot", "scatter_x_column", "scatter_y_column", optionally "scatter_size_column" and "scatter_color_column"
- For histogram: Set "type": "histogram", "histogram_value_column", optionally "histogram_bins" (number)
- For heatmap: Set "type": "heatmap", "heatmap_x_column", "heatmap_y_column", "heatmap_value_column"

VARIABLES:
You can use variables in your SQL queries to make them dynamic. Variables are specified using the syntax @variable_name. When a variable is used, the frontend will show a dropdown allowing users to select a value for that variable.

Available variables:
{{range $name, $var := .Variables}}
- @{{$name}} ({{$var.DisplayName}}): {{$var.Description}}
{{end}}

Example: If you want to filter by player name, you could use: SELECT * FROM players WHERE name = @all_players_name

You must first use the available tools to figure out how to construct the query, and then to run it and make sure that the results make sense. The query must return columns that match the config you specify (e.g., if you set "pie_label_column": "race", the query must return a "race" column).
`
)

var (
	systemPromptTpl, _ = template.New("").Parse(systemPromptTemplate)
)
