package dashboard

import "encoding/json"

// WidgetType represents the type of widget/chart
type WidgetType string

const (
	WidgetTypeGauge       WidgetType = "gauge"
	WidgetTypeTable       WidgetType = "table"
	WidgetTypePieChart   WidgetType = "pie_chart"
	WidgetTypeBarChart    WidgetType = "bar_chart"
	WidgetTypeLineChart   WidgetType = "line_chart"
	WidgetTypeScatterPlot WidgetType = "scatter_plot"
	WidgetTypeHistogram   WidgetType = "histogram"
	WidgetTypeHeatmap     WidgetType = "heatmap"
)

// WidgetConfig represents the configuration for a dashboard widget
type WidgetConfig struct {
	Type WidgetType `json:"type"`

	// Gauge-specific
	GaugeValueColumn string  `json:"gauge_value_column,omitempty"` // Column name for the value
	GaugeMin         *float64 `json:"gauge_min,omitempty"`         // Minimum value (optional)
	GaugeMax         *float64 `json:"gauge_max,omitempty"`         // Maximum value (optional)
	GaugeLabel       string  `json:"gauge_label,omitempty"`        // Label to display

	// Pie chart specific
	PieLabelColumn string `json:"pie_label_column,omitempty"` // Column name for slice labels
	PieValueColumn string `json:"pie_value_column,omitempty"` // Column name for slice values

	// Bar chart specific
	BarLabelColumn string `json:"bar_label_column,omitempty"` // Column name for bar labels
	BarValueColumn string `json:"bar_value_column,omitempty"` // Column name for bar values
	BarHorizontal bool   `json:"bar_horizontal,omitempty"`     // Horizontal bars (default: false)

	// Line chart specific
	LineXColumn      string `json:"line_x_column,omitempty"`       // Column name for X axis
	LineYColumns     []string `json:"line_y_columns,omitempty"`    // Column names for Y axis (multiple series)
	LineYAxisFromZero bool   `json:"line_y_axis_from_zero,omitempty"` // Start Y axis from zero
	LineXAxisType     string `json:"line_x_axis_type,omitempty"`   // e.g., "seconds_from_game_start", "timestamp", "numeric"

	// Scatter plot specific
	ScatterXColumn string `json:"scatter_x_column,omitempty"` // Column name for X axis
	ScatterYColumn string `json:"scatter_y_column,omitempty"` // Column name for Y axis
	ScatterSizeColumn string `json:"scatter_size_column,omitempty"` // Optional: column for point size
	ScatterColorColumn string `json:"scatter_color_column,omitempty"` // Optional: column for point color

	// Histogram specific
	HistogramValueColumn string `json:"histogram_value_column,omitempty"` // Column name for values to bin
	HistogramBins        *int   `json:"histogram_bins,omitempty"`         // Number of bins (optional, auto if not set)

	// Heatmap specific
	HeatmapXColumn string `json:"heatmap_x_column,omitempty"` // Column name for X axis categories
	HeatmapYColumn string `json:"heatmap_y_column,omitempty"` // Column name for Y axis categories
	HeatmapValueColumn string `json:"heatmap_value_column,omitempty"` // Column name for cell values
}

// UnmarshalJSON custom unmarshaler to handle JSONB from database
func (wc *WidgetConfig) UnmarshalJSON(data []byte) error {
	// Handle both string (from JSONB) and object cases
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// If it's a string, unmarshal it again
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return json.Unmarshal([]byte(str), wc)
	}

	// Otherwise, unmarshal directly
	type Alias WidgetConfig
	aux := (*Alias)(wc)
	return json.Unmarshal(raw, aux)
}

// MarshalJSON custom marshaler for JSONB
func (wc WidgetConfig) MarshalJSON() ([]byte, error) {
	type Alias WidgetConfig
	return json.Marshal(Alias(wc))
}

