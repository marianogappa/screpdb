package db

import (
	"strings"
	"testing"
)

func TestBuildGlobalReplayFilterSQL(t *testing.T) {
	tests := []struct {
		name              string
		excludeShortGames bool
		shortGameSeconds  int
		excludeComputers  bool
		gameTypes         []string
		mapKinds          []string
		wantContains      []string
		wantMissing       []string
	}{
		{
			name:         "no filters keeps only the hardcoded UMS exclusion",
			wantContains: []string{"SELECT r.* FROM replays r", "WHERE r.map_kind != 'UseMapSettings'"},
			wantMissing:  []string{"duration_seconds", "computer", "game_type", "AND"},
		},
		{
			name:              "short games and computers",
			excludeShortGames: true,
			shortGameSeconds:  120,
			excludeComputers:  true,
			wantContains: []string{
				"r.duration_seconds >= 120",
				"NOT EXISTS",
				"IN ('computer', 'computer controlled')",
			},
		},
		{
			name:         "game type predicate melee",
			gameTypes:    []string{"melee"},
			wantContains: []string{"lower(trim(coalesce(r.game_type, ''))) = 'melee'"},
		},
		{
			name:         "map kinds translate to storage casing",
			mapKinds:     []string{"regular", "money"},
			wantContains: []string{"r.map_kind = 'Regular'", "r.map_kind = 'Money'", " OR "},
		},
		{
			name:         "unknown enum values are dropped",
			gameTypes:    []string{"bogus"},
			mapKinds:     []string{"bogus"},
			wantContains: []string{"WHERE r.map_kind != 'UseMapSettings'"},
			wantMissing:  []string{"game_type", "r.map_kind = "},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildGlobalReplayFilterSQL(tt.excludeShortGames, tt.shortGameSeconds, tt.excludeComputers, tt.gameTypes, tt.mapKinds)
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("expected SQL to contain %q\ngot: %s", want, got)
				}
			}
			for _, missing := range tt.wantMissing {
				if strings.Contains(got, missing) {
					t.Errorf("expected SQL NOT to contain %q\ngot: %s", missing, got)
				}
			}
		})
	}
}

func TestComposeReplayFilterSQL(t *testing.T) {
	tests := []struct {
		name   string
		global string
		local  string
		want   string
	}{
		{name: "both empty", global: "", local: "", want: ""},
		{name: "both blank/semicolon", global: "  ;; ", local: ";", want: ""},
		{name: "only global", global: "SELECT * FROM replays;", local: "", want: "SELECT * FROM replays"},
		{name: "only local", global: "", local: "SELECT * FROM replays WHERE id = 1", want: "SELECT * FROM replays WHERE id = 1"},
		{
			name:   "both compose into nested filter",
			global: "SELECT r.* FROM replays r",
			local:  "SELECT * FROM replays WHERE id = 1",
			want:   "SELECT * FROM (SELECT r.* FROM replays r) AS global_replays WHERE id IN (SELECT id FROM (SELECT * FROM replays WHERE id = 1) AS local_replays)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ComposeReplayFilterSQL(tt.global, tt.local); got != tt.want {
				t.Errorf("ComposeReplayFilterSQL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReplayIDFilterSQL(t *testing.T) {
	if got, want := ReplayIDFilterSQL(42), "SELECT * FROM replays WHERE id = 42"; got != want {
		t.Errorf("ReplayIDFilterSQL(42) = %q, want %q", got, want)
	}
}

func TestBuildWorkflowPlayersListBaseSQL(t *testing.T) {
	sqlText, args := BuildWorkflowPlayersListBaseSQL("")
	if len(args) != 0 {
		t.Errorf("expected no args for empty filter, got %v", args)
	}
	if !strings.Contains(sqlText, "p.is_observer = 0") || !strings.Contains(sqlText, "= 'human'") {
		t.Errorf("base SQL missing human/observer filter: %s", sqlText)
	}
	if strings.Contains(sqlText, "LIKE ?") {
		t.Errorf("empty name filter should not add a LIKE clause")
	}

	sqlText, args = BuildWorkflowPlayersListBaseSQL("boxer")
	if len(args) != 1 || args[0] != "%boxer%" {
		t.Errorf("expected LIKE arg %q, got %v", "%boxer%", args)
	}
	if !strings.Contains(sqlText, "lower(trim(p.name)) LIKE ?") {
		t.Errorf("name filter should add a LIKE clause: %s", sqlText)
	}
}

func TestBuildWorkflowPlayersListWhere(t *testing.T) {
	tests := []struct {
		name         string
		onlyFivePlus bool
		buckets      []string
		wantSQL      string
	}{
		{name: "no filters", wantSQL: ""},
		{name: "five plus only", onlyFivePlus: true, wantSQL: "WHERE games_played >= 5"},
		{name: "30d bucket", buckets: []string{"1m"}, wantSQL: "WHERE (last_played_days_ago <= 30)"},
		{name: "90d bucket alias", buckets: []string{"90d"}, wantSQL: "WHERE (last_played_days_ago <= 90)"},
		{name: "both buckets OR", buckets: []string{"1m", "3m"}, wantSQL: "WHERE (last_played_days_ago <= 30 OR last_played_days_ago <= 90)"},
		{name: "unknown bucket dropped", buckets: []string{"nope"}, wantSQL: ""},
		{
			name:         "combined",
			onlyFivePlus: true,
			buckets:      []string{"1m"},
			wantSQL:      "WHERE games_played >= 5 AND (last_played_days_ago <= 30)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, args := BuildWorkflowPlayersListWhere(tt.onlyFivePlus, tt.buckets)
			if got != tt.wantSQL {
				t.Errorf("where = %q, want %q", got, tt.wantSQL)
			}
			if len(args) != 0 {
				t.Errorf("expected no args, got %v", args)
			}
		})
	}
}

func TestBuildWorkflowGamesListWhere(t *testing.T) {
	durSQL := WorkflowDurationSQLByKey()

	t.Run("empty inputs produce empty where", func(t *testing.T) {
		got, args := BuildWorkflowGamesListWhere(nil, nil, nil, nil, nil, nil, durSQL)
		if got != "" || len(args) != 0 {
			t.Errorf("expected empty where/args, got %q %v", got, args)
		}
	})

	t.Run("player keys build EXISTS with placeholders", func(t *testing.T) {
		got, args := BuildWorkflowGamesListWhere([]string{"a", "b"}, nil, nil, nil, nil, nil, durSQL)
		if !strings.Contains(got, "EXISTS (SELECT 1 FROM players p") || !strings.Contains(got, "IN (?, ?)") {
			t.Errorf("player clause malformed: %s", got)
		}
		if len(args) != 2 || args[0] != "a" || args[1] != "b" {
			t.Errorf("expected player args, got %v", args)
		}
	})

	t.Run("map names are lowercased/trimmed", func(t *testing.T) {
		_, args := BuildWorkflowGamesListWhere(nil, []string{"  Fighting Spirit "}, nil, nil, nil, nil, durSQL)
		if len(args) != 1 || args[0] != "fighting spirit" {
			t.Errorf("expected normalized map arg, got %v", args)
		}
	})

	t.Run("duration buckets", func(t *testing.T) {
		got, _ := BuildWorkflowGamesListWhere(nil, nil, []string{"under_10m", "10m_plus"}, nil, nil, nil, durSQL)
		if !strings.Contains(got, "r.duration_seconds < 600") || !strings.Contains(got, "r.duration_seconds >= 600") {
			t.Errorf("duration clause malformed: %s", got)
		}
	})

	t.Run("matchup keys validated and lowercased", func(t *testing.T) {
		got, args := BuildWorkflowGamesListWhere(nil, nil, nil, nil, []string{"PvZ", "garbage"}, nil, durSQL)
		if !strings.Contains(got, "lower(r.matchup) IN (?)") {
			t.Errorf("matchup clause malformed: %s", got)
		}
		if len(args) != 1 || args[0] != "pvz" {
			t.Errorf("expected only valid matchup arg, got %v", args)
		}
	})

	t.Run("map kind clause", func(t *testing.T) {
		got, _ := BuildWorkflowGamesListWhere(nil, nil, nil, nil, nil, []string{"money"}, durSQL)
		if !strings.Contains(got, "r.map_kind = 'Money'") {
			t.Errorf("map kind clause malformed: %s", got)
		}
	})
}

func TestPerValueFeatureKeyRoundTrip(t *testing.T) {
	key := PerValueFeatureKey("bo_z_fuzzy", "~10 Hatch")
	if key != "bo_z_fuzzy::~10 hatch" {
		t.Fatalf("PerValueFeatureKey() = %q", key)
	}
	fk, label, ok := splitPerValueFeatureKey(key)
	if !ok || fk != "bo_z_fuzzy" || label != "~10 hatch" {
		t.Fatalf("splitPerValueFeatureKey() = %q %q %v", fk, label, ok)
	}
	if _, _, ok := splitPerValueFeatureKey("plainkey"); ok {
		t.Errorf("plain key should not split")
	}
	if _, _, ok := splitPerValueFeatureKey("::onlylabel"); ok {
		t.Errorf("empty feature key should not split")
	}
}

func TestWorkflowFeaturingExistsSQL(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		wantOK       bool
		wantContains []string
	}{
		{name: "unknown key is a no-op", key: "not_a_real_feature", wantOK: false},
		{name: "team_stacking direct column", key: "team_stacking", wantOK: true, wantContains: []string{"COALESCE(r.team_stacking, 0) = 1"}},
		{name: "cannon_rush game event", key: "cannon_rush", wantOK: true, wantContains: []string{"event_kind = 'game_event'", "event_type = 'cannon_rush'"}},
		{name: "generic drop matches subtypes", key: "drop", wantOK: true, wantContains: []string{"event_type IN ('drop', 'cliff_drop')"}},
		{name: "mind_control composite", key: "mind_control", wantOK: true, wantContains: []string{"event_type IN ('became_terran', 'became_zerg')"}},
		{
			name:         "per-value key escapes and matches label",
			key:          "bo_z_fuzzy::~10 hatch",
			wantOK:       true,
			wantContains: []string{"event_type = 'bo_z_fuzzy'", "json_extract(re.payload, '$.label')) = '~10 hatch'"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := workflowFeaturingExistsSQL(tt.key)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (sql=%q)", ok, tt.wantOK, got)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("expected SQL to contain %q\ngot: %s", want, got)
				}
			}
		})
	}
}

func TestLeadingSupplyNumber(t *testing.T) {
	tests := []struct {
		label string
		want  int
	}{
		{"~10 hatch", 10},
		{"12 pool", 12},
		{"~9 pool 9 hatch", 9},
		{"no number here", 1 << 30},
		{"~", 1 << 30},
	}
	for _, tt := range tests {
		if got := leadingSupplyNumber(tt.label); got != tt.want {
			t.Errorf("leadingSupplyNumber(%q) = %d, want %d", tt.label, got, tt.want)
		}
	}
}

func TestCollapseWhitespace(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"  a   b\n\tc  ", "a b c"},
		{"\n\n\n", ""},
		{"single", "single"},
	}
	for _, tt := range tests {
		if got := collapseWhitespace(tt.in); got != tt.want {
			t.Errorf("collapseWhitespace(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
