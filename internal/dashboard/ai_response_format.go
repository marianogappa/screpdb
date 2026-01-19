package dashboard

import (
	"github.com/tmc/langchaingo/llms/openai"
)

var (
	responseFormat = &openai.ResponseFormat{
		Type: "json_schema",
		JSONSchema: &openai.ResponseFormatJSONSchema{
			Name:   "widget_schema",
			Strict: false,
			Schema: &openai.ResponseFormatJSONSchemaProperty{
				Type: "object",
				Properties: map[string]*openai.ResponseFormatJSONSchemaProperty{
					"title": {
						Type:        "string",
						Description: "Widget's title",
					},
					"description": {
						Type:        "string",
						Description: "Succinct description of the widget's content",
					},
					"sql_query": {
						Type:        "string",
						Description: "A valid PostgreSQL query that returns the rows that feed into the widget",
					},
					"config": {
						Type:        "object",
						Description: "Widget configuration specifying type and type-specific fields",
						Properties: map[string]*openai.ResponseFormatJSONSchemaProperty{
							"type": {
								Type:        "string",
								Description: "Widget type: gauge, table, pie_chart, bar_chart, line_chart, scatter_plot, histogram, or heatmap",
								Enum:        []any{"gauge", "table", "pie_chart", "bar_chart", "line_chart", "scatter_plot", "histogram", "heatmap"},
							},
							"gauge_value_column": {Type: "string", Description: "For gauge: column name for the value"},
							"gauge_min":          {Type: "number", Description: "For gauge: optional minimum value"},
							"gauge_max":          {Type: "number", Description: "For gauge: optional maximum value"},
							"gauge_label":        {Type: "string", Description: "For gauge: optional label"},
							"table_columns": {
								Type:        "array",
								Description: "For table: optional column names to display (empty = all)",
								Items: &openai.ResponseFormatJSONSchemaProperty{
									Type: "string",
								},
							},
							"pie_label_column": {Type: "string", Description: "For pie_chart: column name for slice labels"},
							"pie_value_column": {Type: "string", Description: "For pie_chart: column name for slice values"},
							"bar_label_column": {Type: "string", Description: "For bar_chart: column name for bar labels"},
							"bar_value_column": {Type: "string", Description: "For bar_chart: column name for bar values"},
							"bar_horizontal":   {Type: "boolean", Description: "For bar_chart: horizontal bars (default: false)"},
							"line_x_column": {
								Type:        "string",
								Description: "For line_chart: column name for X axis",
							},
							"line_y_columns": {
								Type:        "array",
								Description: "For line_chart: column names for Y axis (multiple series)",
								Items: &openai.ResponseFormatJSONSchemaProperty{
									Type: "string",
								},
							},
							"line_y_axis_from_zero": {
								Type:        "boolean",
								Description: "For line_chart: start Y axis from zero (default: false)",
							},
							"line_x_axis_type": {
								Type:        "string",
								Description: "For line_chart: x-axis type: seconds_from_game_start, timestamp, or numeric",
							},
							"scatter_x_column":       {Type: "string", Description: "For scatter_plot: column name for X axis"},
							"scatter_y_column":       {Type: "string", Description: "For scatter_plot: column name for Y axis"},
							"scatter_size_column":    {Type: "string", Description: "For scatter_plot: optional column for point size"},
							"scatter_color_column":   {Type: "string", Description: "For scatter_plot: optional column for point color"},
							"histogram_value_column": {Type: "string", Description: "For histogram: column name for values to bin"},
							"histogram_bins":         {Type: "number", Description: "For histogram: optional number of bins"},
							"heatmap_x_column":       {Type: "string", Description: "For heatmap: column name for X axis categories"},
							"heatmap_y_column":       {Type: "string", Description: "For heatmap: column name for Y axis categories"},
							"heatmap_value_column":   {Type: "string", Description: "For heatmap: column name for cell values"},
						},
						Required: []string{"type"},
					},
				},
				Required: []string{"title", "description", "sql_query", "config"},
			},
		},
	}
)
