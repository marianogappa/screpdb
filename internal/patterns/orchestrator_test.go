package patterns

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
	"github.com/marianogappa/screpdb/internal/patterns/detectors"
	"github.com/marianogappa/screpdb/internal/patterns/markers"
	"github.com/marianogappa/screpdb/internal/unittags"
)

func bytePtr(b byte) *byte { return &b }

func int64Ptr(v int64) *int64 { return &v }

func openerName(tier int) string {
	for _, m := range markers.Markers() {
		if m.Kind == markers.KindInitialBuildOrder && m.Tier == tier {
			return m.PatternName
		}
	}
	return ""
}

func markerName() string {
	for _, m := range markers.Markers() {
		if m.Kind == markers.KindMarker {
			return m.PatternName
		}
	}
	return ""
}

func TestNewOrchestratorEmptyState(t *testing.T) {
	o := NewOrchestrator()
	if o == nil {
		t.Fatal("NewOrchestrator returned nil")
	}
	if len(o.detectors) != 0 {
		t.Errorf("expected 0 detectors before Initialize, got %d", len(o.detectors))
	}
	if got := o.GetResults(); len(got) != 0 {
		t.Errorf("expected 0 results from a fresh orchestrator, got %d", len(got))
	}
	if o.WorldStateEngine() != nil {
		t.Error("expected nil worldstate engine before Initialize")
	}
	if o.ReplayEvents() != nil {
		t.Error("expected nil replay events before Initialize")
	}
}

func TestNilWorldStateGuards(t *testing.T) {
	o := NewOrchestrator()

	o.ProcessCommand(&models.Command{ActionType: "Build"})
	o.AppendReplayEvents(nil)
	o.SetProductionSignals(nil)
	o.SetProductionSignals(&unittags.Evidence{})
	o.SetMutaHarass(nil)
	o.SetMutaHarass([]unittags.MutaHarassEpisode{{PlayerID: 1}})

	if len(o.results) != 0 {
		t.Errorf("expected no results accumulated, got %d", len(o.results))
	}
}

func TestInitializeDetectorRegistration(t *testing.T) {
	perPlayerDetectors := len(playerLevelDetectors)
	replayDetectors := len(replayLevelDetectors)

	tests := []struct {
		name         string
		build        func() (*models.Replay, []*models.Player)
		wantPlayers  int
		wantReplayLv int
	}{
		{
			name: "two human players",
			build: func() (*models.Replay, []*models.Player) {
				return detectors.NewTestReplayBuilder().
					WithPlayer(0, "A", "Terran", 1).
					WithPlayer(1, "B", "Zerg", 2).
					Build()
			},
			wantPlayers:  2,
			wantReplayLv: replayDetectors,
		},
		{
			name: "observer is skipped",
			build: func() (*models.Replay, []*models.Player) {
				r, players := detectors.NewTestReplayBuilder().
					WithPlayer(0, "A", "Terran", 1).
					WithPlayer(1, "Obs", "Protoss", 2).
					Build()
				players[1].IsObserver = true
				return r, players
			},
			wantPlayers:  1,
			wantReplayLv: replayDetectors,
		},
		{
			name: "no players still gets replay-level detectors",
			build: func() (*models.Replay, []*models.Player) {
				return detectors.NewTestReplayBuilder().Build()
			},
			wantPlayers:  0,
			wantReplayLv: replayDetectors,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replay, players := tt.build()
			o := NewOrchestrator()
			o.Initialize(replay, players, nil)

			want := tt.wantReplayLv + tt.wantPlayers*perPlayerDetectors
			if len(o.detectors) != want {
				t.Errorf("detector count = %d, want %d (%d replay-level + %d players * %d per-player)",
					len(o.detectors), want, tt.wantReplayLv, tt.wantPlayers, perPlayerDetectors)
			}
			if o.WorldStateEngine() == nil {
				t.Error("expected non-nil worldstate engine after Initialize")
			}
		})
	}
}

func TestInitializeAllDetectorsInitialized(t *testing.T) {
	replay, players := detectors.NewTestReplayBuilder().
		WithPlayer(0, "A", "Terran", 1).
		WithPlayer(1, "B", "Zerg", 2).
		Build()
	o := NewOrchestrator()
	o.Initialize(replay, players, nil)

	for i, d := range o.detectors {
		if d.Name() == "" {
			t.Errorf("detector %d has empty Name()", i)
		}
	}
}

func TestSelectBestTierOpeners(t *testing.T) {
	tier1 := openerName(markers.TierPreferred)
	tier2 := openerName(markers.TierBackup)
	tier3 := openerName(markers.TierResidual)
	mkr := markerName()
	for _, n := range []string{tier1, tier2, tier3, mkr} {
		if n == "" {
			t.Fatalf("could not resolve a marker name for the test corpus (tier1=%q tier2=%q tier3=%q marker=%q)", tier1, tier2, tier3, mkr)
		}
	}

	res := func(name string, rp byte) *core.PatternResult {
		return &core.PatternResult{
			PatternName:    name,
			Level:          core.LevelPlayer,
			ReplayID:       1,
			ReplayPlayerID: bytePtr(rp),
		}
	}
	names := func(rs []*core.PatternResult) []string {
		out := make([]string, 0, len(rs))
		for _, r := range rs {
			out = append(out, r.PatternName)
		}
		return out
	}
	contains := func(rs []*core.PatternResult, name string) bool {
		for _, r := range rs {
			if r.PatternName == name {
				return true
			}
		}
		return false
	}

	tests := []struct {
		name     string
		input    []*core.PatternResult
		wantLen  int
		wantKeep []string
		wantDrop []string
	}{
		{
			name:     "preferred suppresses backup and residual for same player",
			input:    []*core.PatternResult{res(tier1, 0), res(tier2, 0), res(tier3, 0)},
			wantLen:  1,
			wantKeep: []string{tier1},
			wantDrop: []string{tier2, tier3},
		},
		{
			name:     "backup wins over residual when no preferred",
			input:    []*core.PatternResult{res(tier2, 0), res(tier3, 0)},
			wantLen:  1,
			wantKeep: []string{tier2},
			wantDrop: []string{tier3},
		},
		{
			name:     "residual survives alone",
			input:    []*core.PatternResult{res(tier3, 0)},
			wantLen:  1,
			wantKeep: []string{tier3},
		},
		{
			name:     "KindMarker passes through untouched alongside an opener",
			input:    []*core.PatternResult{res(tier1, 0), res(tier3, 0), res(mkr, 0)},
			wantLen:  2,
			wantKeep: []string{tier1, mkr},
			wantDrop: []string{tier3},
		},
		{
			name:     "unknown pattern name passes through",
			input:    []*core.PatternResult{res("Totally Unknown Pattern", 0), res(tier2, 0), res(tier3, 0)},
			wantLen:  2,
			wantKeep: []string{"Totally Unknown Pattern", tier2},
			wantDrop: []string{tier3},
		},
		{
			name:     "suppression is per-player independent",
			input:    []*core.PatternResult{res(tier1, 0), res(tier3, 0), res(tier3, 1)},
			wantLen:  2,
			wantKeep: []string{tier1, tier3},
			wantDrop: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectBestTierOpeners(tt.input)
			if len(got) != tt.wantLen {
				t.Fatalf("len = %d, want %d (%v)", len(got), tt.wantLen, names(got))
			}
			for _, keep := range tt.wantKeep {
				if !contains(got, keep) {
					t.Errorf("expected %q kept, got %v", keep, names(got))
				}
			}
			for _, drop := range tt.wantDrop {
				if contains(got, drop) {
					t.Errorf("expected %q dropped, got %v", drop, names(got))
				}
			}

			again := selectBestTierOpeners(got)
			if len(again) != len(got) {
				t.Errorf("not idempotent: second pass len = %d, first = %d", len(again), len(got))
			}
		})
	}
}

func TestSelectBestTierOpenersEmpty(t *testing.T) {
	if got := selectBestTierOpeners(nil); len(got) != 0 {
		t.Errorf("expected empty result for nil input, got %d", len(got))
	}
	if got := selectBestTierOpeners([]*core.PatternResult{}); len(got) != 0 {
		t.Errorf("expected empty result for empty input, got %d", len(got))
	}
}

func TestConvertResultsToDatabaseIDs(t *testing.T) {
	tests := []struct {
		name       string
		result     *core.PatternResult
		idMap      map[byte]int64
		wantDBID   *int64
		wantRPStay bool
	}{
		{
			name:     "player-level replay id is mapped to db id",
			result:   &core.PatternResult{Level: core.LevelPlayer, ReplayPlayerID: bytePtr(0)},
			idMap:    map[byte]int64{0: 42},
			wantDBID: int64Ptr(42),
		},
		{
			name:     "unmapped replay player id is left nil",
			result:   &core.PatternResult{Level: core.LevelPlayer, ReplayPlayerID: bytePtr(7)},
			idMap:    map[byte]int64{0: 42},
			wantDBID: nil,
		},
		{
			name:     "already-set db id is not overwritten",
			result:   &core.PatternResult{Level: core.LevelPlayer, PlayerID: int64Ptr(99), ReplayPlayerID: bytePtr(0)},
			idMap:    map[byte]int64{0: 42},
			wantDBID: int64Ptr(99),
		},
		{
			name:     "replay-level result is untouched",
			result:   &core.PatternResult{Level: core.LevelReplay, ReplayPlayerID: bytePtr(0)},
			idMap:    map[byte]int64{0: 42},
			wantDBID: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := NewOrchestrator()
			o.results = []*core.PatternResult{tt.result}
			o.ConvertResultsToDatabaseIDs(tt.idMap)

			got := o.results[0].PlayerID
			switch {
			case tt.wantDBID == nil && got != nil:
				t.Errorf("PlayerID = %d, want nil", *got)
			case tt.wantDBID != nil && got == nil:
				t.Errorf("PlayerID = nil, want %d", *tt.wantDBID)
			case tt.wantDBID != nil && got != nil && *got != *tt.wantDBID:
				t.Errorf("PlayerID = %d, want %d", *got, *tt.wantDBID)
			}
		})
	}
}

func TestGetResultsRunsSelectBestTier(t *testing.T) {
	o := NewOrchestrator()
	tier1 := openerName(markers.TierPreferred)
	tier3 := openerName(markers.TierResidual)
	o.results = []*core.PatternResult{
		{PatternName: tier1, Level: core.LevelPlayer, ReplayID: 1, ReplayPlayerID: bytePtr(0)},
		{PatternName: tier3, Level: core.LevelPlayer, ReplayID: 1, ReplayPlayerID: bytePtr(0)},
	}
	got := o.GetResults()
	if len(got) != 1 {
		t.Fatalf("GetResults did not apply tier selection: len = %d, want 1", len(got))
	}
	if got[0].PatternName != tier1 {
		t.Errorf("kept %q, want the preferred opener %q", got[0].PatternName, tier1)
	}
}
