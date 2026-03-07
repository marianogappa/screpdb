export const DEFAULT_COLORS = ['#4e79a7', '#f28e2c', '#e15759', '#76b7b2', '#59a14f', '#edc949', '#af7aa1', '#ff9d9a', '#9c755f', '#bab0ac'];

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

/** Human-readable labels for tables (Advanced mode) */
export const TABLE_LABELS = {
  replays: 'Games',
  players: 'Players',
  commands: 'In-game actions',
  detected_patterns_replay: 'Strategy patterns (replay)',
  detected_patterns_replay_team: 'Strategy patterns (team)',
  detected_patterns_replay_player: 'Strategy patterns (player)',
};

/** Milestone options for "time to reach" semi-template (pattern_name => short label) */
export const MILESTONE_OPTIONS = [
  { value: 'Seconds to First Carrier Build Triggered', label: 'First Carrier' },
  { value: 'Seconds to First Gateway Build Triggered', label: 'First Gateway' },
  { value: 'Seconds to First Factory Build Triggered', label: 'First Factory' },
  { value: 'Seconds to First Spawning Pool Morph Triggered', label: 'First Spawning Pool' },
  { value: 'Seconds to First Zergling Morph Triggered', label: 'First Zerglings' },
  { value: 'Seconds to First Mutalisk Morph Triggered', label: 'First Mutalisks' },
];

/** Optional column labels for display (e.g. replay_date -> Game date) */
export const COLUMN_LABELS = {
  replay_date: 'Game date',
  map_name: 'Map',
  duration_seconds: 'Duration (s)',
  game_type: 'Game type',
  name: 'Player name',
  race: 'Race',
  type: 'Player type',
  team: 'Team',
  apm: 'APM',
  is_winner: 'Winner',
  action_type: 'Action type',
  pattern_name: 'Pattern',
  value_bool: 'Value (yes/no)',
  value_int: 'Value (number)',
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
    category: 'overview',
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
    category: 'win_rates',
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
    category: 'overview',
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
    category: 'apm_skill',
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
    category: 'overview',
  },
  {
    id: 'total_replays',
    name: 'Total Replays',
    description: 'Count of all replays in database',
    query: `SELECT COUNT(*) AS total_replays FROM replays`,
    chartType: 'gauge',
    config: { gauge_value_column: 'total_replays', gauge_label: 'Total Replays' },
    category: 'overview',
  },
  // BGH & teams
  {
    id: 'no_alliance_replays',
    name: 'Replays with No Alliance',
    description: 'Games where nobody used the ally command (suspicious in 2v2v2v2)',
    query: `SELECT r.id, r.map_name, r.replay_date, r.duration_seconds
FROM replays r
LEFT JOIN (
  SELECT replay_id FROM commands WHERE action_type = 'Alliance' LIMIT 1
) c ON c.replay_id = r.id
WHERE c.replay_id IS NULL
ORDER BY r.replay_date DESC
LIMIT 50`,
    chartType: 'table',
    category: 'bgh_teams',
  },
  {
    id: 'alliance_activity_replays',
    name: 'Replays with Alliance Activity',
    description: 'Games that have at least one Alliance command',
    query: `SELECT r.id, r.map_name, r.replay_date, COUNT(c.id) AS alliance_commands
FROM replays r
JOIN commands c ON c.replay_id = r.id AND c.action_type = 'Alliance'
GROUP BY r.id
ORDER BY r.replay_date DESC
LIMIT 50`,
    chartType: 'table',
    category: 'bgh_teams',
  },
  {
    id: 'alliance_changed_suspicious',
    name: 'Alliance Changed (Suspicious)',
    description: 'Games where someone re-allied (more than one Alliance command); possible 3-way teaming',
    query: `SELECT r.id, r.map_name, r.replay_date, r.duration_seconds, COUNT(c.id) AS alliance_cmd_count
FROM replays r
JOIN commands c ON c.replay_id = r.id AND c.action_type = 'Alliance'
GROUP BY r.id
HAVING COUNT(c.id) > 1
ORDER BY r.replay_date DESC
LIMIT 50`,
    chartType: 'table',
    category: 'suspicious',
  },
  {
    id: 'most_alliance_changes',
    name: 'Most Alliance Changes',
    description: 'Replays with the most ally switching (ranked by Alliance command count)',
    query: `SELECT r.id, r.replay_date, r.map_name, COUNT(c.id) AS alliance_cmd_count
FROM replays r
JOIN commands c ON c.replay_id = r.id AND c.action_type = 'Alliance'
GROUP BY r.id
ORDER BY alliance_cmd_count DESC
LIMIT 20`,
    chartType: 'bar_chart',
    config: { bar_label_column: 'map_name', bar_value_column: 'alliance_cmd_count', bar_horizontal: true },
    category: 'suspicious',
  },
  {
    id: 'early_leavers',
    name: 'Early Leavers',
    description: 'Short games with a LeaveGame command (under 5 min); worth reviewing',
    query: `SELECT r.id, r.map_name, r.replay_date, r.duration_seconds
FROM replays r
JOIN commands c ON c.replay_id = r.id AND c.action_type = 'LeaveGame'
WHERE r.duration_seconds < 300
GROUP BY r.id
ORDER BY r.replay_date DESC
LIMIT 50`,
    chartType: 'table',
    category: 'suspicious',
  },
  // Carriers & build order
  {
    id: 'who_goes_carriers',
    name: 'Who Goes Carriers Most',
    description: 'Players who built Carriers most often',
    query: `SELECT p.name, COUNT(*) AS carrier_games
FROM players p
JOIN detected_patterns_replay_player d ON d.player_id = p.id
  AND d.pattern_name = 'Did Carriers' AND d.value_bool = 1
WHERE p.type = 'Human'
GROUP BY p.name
ORDER BY carrier_games DESC
LIMIT 20`,
    chartType: 'bar_chart',
    config: { bar_label_column: 'name', bar_value_column: 'carrier_games' },
    category: 'carriers_build',
  },
  {
    id: 'fastest_to_carriers',
    name: 'Fastest to Carriers',
    description: 'Players with lowest average time to first Carrier build',
    query: `SELECT p.name, ROUND(AVG(d.value_int), 0) AS avg_seconds_to_carriers
FROM players p
JOIN detected_patterns_replay_player d ON d.player_id = p.id
  AND d.pattern_name = 'Seconds to First Carrier Build Triggered' AND d.value_int IS NOT NULL
WHERE p.type = 'Human'
GROUP BY p.name
HAVING COUNT(*) >= 2
ORDER BY avg_seconds_to_carriers ASC
LIMIT 15`,
    chartType: 'bar_chart',
    config: { bar_label_column: 'name', bar_value_column: 'avg_seconds_to_carriers', bar_horizontal: true },
    category: 'carriers_build',
  },
  {
    id: 'bgh_only_games',
    name: 'BGH-Style Maps',
    description: 'Game count per map (BGH or Big Game style)',
    query: `SELECT map_name, COUNT(*) AS games
FROM replays
WHERE map_name LIKE '%BGH%' OR map_name LIKE '%Big Game%'
GROUP BY map_name
ORDER BY games DESC`,
    chartType: 'bar_chart',
    config: { bar_label_column: 'map_name', bar_value_column: 'games' },
    category: 'bgh_teams',
  },
  {
    id: 'fastest_to_gateway',
    name: 'Fastest to Gateway',
    description: 'Players with lowest average time to first Gateway',
    query: `SELECT p.name, ROUND(AVG(d.value_int), 0) AS avg_seconds
FROM players p
JOIN detected_patterns_replay_player d ON d.player_id = p.id
  AND d.pattern_name = 'Seconds to First Gateway Build Triggered' AND d.value_int IS NOT NULL
WHERE p.type = 'Human'
GROUP BY p.name
HAVING COUNT(*) >= 2
ORDER BY avg_seconds ASC
LIMIT 15`,
    chartType: 'bar_chart',
    config: { bar_label_column: 'name', bar_value_column: 'avg_seconds', bar_horizontal: true },
    category: 'carriers_build',
  },
  {
    id: 'fastest_to_factory',
    name: 'Fastest to Factory',
    description: 'Players with lowest average time to first Factory',
    query: `SELECT p.name, ROUND(AVG(d.value_int), 0) AS avg_seconds
FROM players p
JOIN detected_patterns_replay_player d ON d.player_id = p.id
  AND d.pattern_name = 'Seconds to First Factory Build Triggered' AND d.value_int IS NOT NULL
WHERE p.type = 'Human'
GROUP BY p.name
HAVING COUNT(*) >= 2
ORDER BY avg_seconds ASC
LIMIT 15`,
    chartType: 'bar_chart',
    config: { bar_label_column: 'name', bar_value_column: 'avg_seconds', bar_horizontal: true },
    category: 'carriers_build',
  },
  {
    id: 'fastest_to_pool',
    name: 'Fastest to Spawning Pool',
    description: 'Players with lowest average time to first Spawning Pool morph',
    query: `SELECT p.name, ROUND(AVG(d.value_int), 0) AS avg_seconds
FROM players p
JOIN detected_patterns_replay_player d ON d.player_id = p.id
  AND d.pattern_name = 'Seconds to First Spawning Pool Morph Triggered' AND d.value_int IS NOT NULL
WHERE p.type = 'Human'
GROUP BY p.name
HAVING COUNT(*) >= 2
ORDER BY avg_seconds ASC
LIMIT 15`,
    chartType: 'bar_chart',
    config: { bar_label_column: 'name', bar_value_column: 'avg_seconds', bar_horizontal: true },
    category: 'carriers_build',
  },
  // Semi-template: one choice (which milestone) → histogram of time to reach it
  {
    id: 'time_to_milestone_histogram',
    name: 'Time to reach a milestone (distribution)',
    description: 'How long players take to reach a chosen milestone. Pick one below; see distribution as histogram. Track if you\'re getting faster.',
    queryTemplate: `SELECT d.value_int AS seconds
FROM detected_patterns_replay_player d
JOIN players p ON p.id = d.player_id
WHERE d.pattern_name = __MILESTONE__ AND d.value_int IS NOT NULL`,
    chartType: 'histogram',
    config: { histogram_value_column: 'seconds', histogram_bins: 20 },
    category: 'carriers_build',
    milestoneOptions: MILESTONE_OPTIONS,
  },
];
