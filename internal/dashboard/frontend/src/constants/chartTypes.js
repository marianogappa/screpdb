export const WIDGET_TYPES = [
  { value: 'gauge', label: 'Gauge', icon: 'speed', description: 'Single value with progress bar' },
  { value: 'table', label: 'Table', icon: 'table', description: 'Rows and columns of data' },
  { value: 'pie_chart', label: 'Pie Chart', icon: 'pie', description: 'Proportional slices' },
  { value: 'bar_chart', label: 'Bar Chart', icon: 'bar', description: 'Compare categories' },
  { value: 'line_chart', label: 'Line Chart', icon: 'line', description: 'Trends over time' },
  { value: 'scatter_plot', label: 'Scatter Plot', icon: 'scatter', description: 'Correlation between values' },
  { value: 'histogram', label: 'Histogram', icon: 'histogram', description: 'Distribution of values' },
  { value: 'heatmap', label: 'Heatmap', icon: 'heatmap', description: 'Intensity grid' },
];

export const CHART_TYPE_FIELDS = {
  gauge: [
    { key: 'gauge_value_column', label: 'Value Column', type: 'column', required: true },
    { key: 'gauge_min', label: 'Min Value', type: 'number' },
    { key: 'gauge_max', label: 'Max Value', type: 'number' },
    { key: 'gauge_label', label: 'Label', type: 'text' },
  ],
  table: [],
  pie_chart: [
    { key: 'pie_label_column', label: 'Label Column', type: 'column', required: true },
    { key: 'pie_value_column', label: 'Value Column', type: 'column', required: true },
  ],
  bar_chart: [
    { key: 'bar_label_column', label: 'Label Column', type: 'column', required: true },
    { key: 'bar_value_column', label: 'Value Column', type: 'column', required: true },
    { key: 'bar_horizontal', label: 'Horizontal bars', type: 'checkbox' },
  ],
  line_chart: [
    { key: 'line_x_column', label: 'X Column', type: 'column', required: true },
    { key: 'line_y_columns', label: 'Y Columns', type: 'columns', required: true },
    { key: 'line_x_axis_type', label: 'X Axis Type', type: 'select', options: [
      { value: 'numeric', label: 'Numeric' },
      { value: 'seconds_from_game_start', label: 'Seconds from Game Start' },
      { value: 'timestamp', label: 'Timestamp' },
    ]},
    { key: 'line_y_axis_from_zero', label: 'Y axis starts from zero', type: 'checkbox' },
  ],
  scatter_plot: [
    { key: 'scatter_x_column', label: 'X Column', type: 'column', required: true },
    { key: 'scatter_y_column', label: 'Y Column', type: 'column', required: true },
    { key: 'scatter_size_column', label: 'Size Column', type: 'column' },
    { key: 'scatter_color_column', label: 'Color Column', type: 'column' },
  ],
  histogram: [
    { key: 'histogram_value_column', label: 'Value Column', type: 'column', required: true },
    { key: 'histogram_bins', label: 'Number of Bins', type: 'number' },
  ],
  heatmap: [
    { key: 'heatmap_x_column', label: 'X Column', type: 'column', required: true },
    { key: 'heatmap_y_column', label: 'Y Column', type: 'column', required: true },
    { key: 'heatmap_value_column', label: 'Value Column', type: 'column', required: true },
  ],
};

export const QUERY_TEMPLATES = [
  {
    id: 'recent_games',
    name: 'Recent Games',
    description: 'Last 100 replays with date and map',
    query: `SELECT r.replay_date, r.map_name, r.duration_seconds, r.game_type
FROM replays r
ORDER BY r.replay_date DESC
LIMIT 100`,
    chartType: 'table',
  },
  {
    id: 'win_rate_by_race',
    name: 'Win Rate by Race',
    description: 'Win percentage for each race',
    query: `SELECT p.race,
  COUNT(*) AS total_games,
  SUM(CASE WHEN p.is_winner THEN 1 ELSE 0 END) AS wins,
  ROUND(100.0 * SUM(CASE WHEN p.is_winner THEN 1 ELSE 0 END) / COUNT(*), 1) AS win_rate
FROM players p
WHERE p.type = 'Human'
GROUP BY p.race
ORDER BY win_rate DESC`,
    chartType: 'bar_chart',
    config: { bar_label_column: 'race', bar_value_column: 'win_rate' },
  },
  {
    id: 'games_per_map',
    name: 'Games per Map',
    description: 'Number of games played on each map',
    query: `SELECT r.map_name, COUNT(*) AS game_count
FROM replays r
GROUP BY r.map_name
ORDER BY game_count DESC
LIMIT 15`,
    chartType: 'pie_chart',
    config: { pie_label_column: 'map_name', pie_value_column: 'game_count' },
  },
  {
    id: 'top_players_apm',
    name: 'Top Players by APM',
    description: 'Highest APM players across all games',
    query: `SELECT p.name, ROUND(AVG(p.apm), 0) AS avg_apm, COUNT(*) AS games_played
FROM players p
WHERE p.type = 'Human'
GROUP BY p.name
HAVING games_played >= 3
ORDER BY avg_apm DESC
LIMIT 20`,
    chartType: 'bar_chart',
    config: { bar_label_column: 'name', bar_value_column: 'avg_apm' },
  },
  {
    id: 'game_duration_distribution',
    name: 'Game Duration Distribution',
    description: 'How long do games typically last',
    query: `SELECT duration_seconds
FROM replays
WHERE duration_seconds > 0`,
    chartType: 'histogram',
    config: { histogram_value_column: 'duration_seconds', histogram_bins: 20 },
  },
  {
    id: 'total_replays',
    name: 'Total Replays',
    description: 'Count of all replays in database',
    query: `SELECT COUNT(*) AS total_replays FROM replays`,
    chartType: 'gauge',
    config: { gauge_value_column: 'total_replays', gauge_label: 'Total Replays' },
  },
];
