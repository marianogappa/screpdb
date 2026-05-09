package dashboard

import "testing"

func TestNeverAlliedMultiTeamEligible(t *testing.T) {
	cases := []struct {
		name             string
		multiTeamGames   int64
		allianceCommands int64
		want             bool
	}{
		{"no multi-team games -> ineligible", 0, 0, false},
		{"one multi-team game with no alliance command -> eligible", 1, 0, true},
		{"many multi-team games with no alliance command -> eligible", 17, 0, true},
		{"one alliance command kills eligibility", 1, 1, false},
		{"many alliance commands kills eligibility", 5, 4, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := neverAlliedMultiTeamEligible(tc.multiTeamGames, tc.allianceCommands)
			if got != tc.want {
				t.Fatalf("multiTeamGames=%d allianceCommands=%d -> got %v want %v", tc.multiTeamGames, tc.allianceCommands, got, tc.want)
			}
		})
	}
}

// Pinned-mapping coverage for the upgrade pill icon resolver. The
// in-parens pattern fallback ("Templar" extracted from "Khaydarin Amulet
// (Templar Energy)") doesn't match the icon registry key "hightemplar"
// on its own, so this map needs an explicit entry. Test guards against
// silently dropping the map entry.
func TestOutlierUpgradeIconMappings(t *testing.T) {
	cases := []struct {
		upgrade string
		want    string
	}{
		{"Khaydarin Amulet (Templar Energy)", "hightemplar"},
		{"Khaydarin Core (Arbiter Energy)", "arbiter"},
		{"Argus Jewel (Corsair Energy)", "corsair"},
		{"Argus Talisman (Dark Archon Energy)", "darkarchon"},
		{"U-238 Shells (Marine Range)", "marine"},
		{"Singularity Charge (Dragoon Range)", "dragoon"},
		{"Carrier Capacity", "carrier"},
		{"Scarab Damage", "reaver"},
	}
	for _, tc := range cases {
		t.Run(tc.upgrade, func(t *testing.T) {
			got := outlierIconKey("Upgrade", tc.upgrade)
			if got != tc.want {
				t.Fatalf("outlierIconKey(Upgrade, %q) = %q; want %q", tc.upgrade, got, tc.want)
			}
		})
	}
}

func TestNeverHotkeysEligible(t *testing.T) {
	cases := []struct {
		name        string
		totalGames  int64
		ratePercent float64
		want        bool
	}{
		{"zero games -> ineligible regardless", 0, 0, false},
		{"one game with zero rate -> eligible", 1, 0, true},
		{"any nonzero rate kills eligibility", 5, 1, false},
		{"100% rate kills eligibility", 5, 100, false},
		{"fractional rate kills eligibility", 50, 0.5, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := neverHotkeysEligible(tc.totalGames, tc.ratePercent)
			if got != tc.want {
				t.Fatalf("totalGames=%d ratePercent=%v -> got %v want %v", tc.totalGames, tc.ratePercent, got, tc.want)
			}
		})
	}
}

// TestPlayerSummarySpecial_OneVOneCorpus verifies that against a corpus of
// 1v1 replays (the existing dashboard test fixtures are all 1v1), the
// never-allied pill is correctly *not* eligible — the SQL predicate
// `team_format LIKE '%v%v%'` should not match "1v1". This exercises the
// full read path including the JSON->Go shape conversion that the API
// returns.
func TestPlayerSummarySpecial_OneVOneCorpus(t *testing.T) {
	dash := newTestDashboard(t)

	var playerKey string
	if err := dash.dbStore.DefaultQueryRow(`SELECT lower(trim(name)) FROM players WHERE is_observer = 0 LIMIT 1`).Scan(&playerKey); err != nil {
		t.Fatalf("look up player_key: %v", err)
	}
	if playerKey == "" {
		t.Fatal("expected at least one player in the test fixtures")
	}

	result, err := dash.buildWorkflowPlayerSummarySpecial(playerKey)
	if err != nil {
		t.Fatalf("buildWorkflowPlayerSummarySpecial: %v", err)
	}
	if result.PlayerName == "" {
		t.Fatal("expected player_name to be populated")
	}
	if result.NeverAlliedMultiTeam.Games != 0 {
		t.Fatalf("expected 0 multi-team melee games for 1v1 corpus, got %d", result.NeverAlliedMultiTeam.Games)
	}
	if result.NeverAlliedMultiTeam.Eligible {
		t.Fatal("expected never-allied pill to be ineligible when there are no multi-team melee games")
	}
}
