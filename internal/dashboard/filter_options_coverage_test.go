package dashboard

import (
	"context"
	"testing"
)

// TestWorkflowGamesListFilterOptions builds the omnibar filter options against
// the real ingested corpus: every axis must come back populated and the
// per-option game counts must reflect the corpus (the testdata replays carry
// featuring pills, three matchup shapes, and a money map).
func TestWorkflowGamesListFilterOptions(t *testing.T) {
	d := newTestDashboard(t)

	options, err := d.workflowGamesListFilterOptions()
	if err != nil {
		t.Fatalf("workflowGamesListFilterOptions: %v", err)
	}
	if len(options.Maps) == 0 {
		t.Fatalf("maps axis empty: %+v", options)
	}
	if len(options.Matchups) == 0 || len(options.MapKinds) == 0 {
		t.Fatalf("matchup/map-kind axes empty: %+v", options)
	}
	matchupGames := int64(0)
	for _, m := range options.Matchups {
		matchupGames += m.Games
	}
	if matchupGames == 0 {
		t.Fatal("matchup counts all zero over a non-empty corpus")
	}
	mapKindGames := int64(0)
	for _, mk := range options.MapKinds {
		mapKindGames += mk.Games
	}
	if mapKindGames == 0 {
		t.Fatal("map-kind counts all zero over a non-empty corpus")
	}
	featuringGames := int64(0)
	for _, f := range options.Featuring {
		featuringGames += f.Games
	}
	if len(options.Featuring) == 0 || featuringGames == 0 {
		t.Fatalf("featuring axis empty or uncounted: %d options, %d games", len(options.Featuring), featuringGames)
	}
}

// TestGamingSessionStoreQueries drives the session-scoped store queries
// directly with real replay IDs, since the session window itself only opens
// for freshly autosaved games.
func TestGamingSessionStoreQueries(t *testing.T) {
	d := newTestDashboard(t)
	ctx := context.Background()

	r := d.setupRouter()
	replayIDs := listTestGames(t, r)
	if len(replayIDs) == 0 {
		t.Fatal("no replays")
	}

	rows, err := d.dbStore.ListReplaysByIDs(ctx, replayIDs)
	if err != nil {
		t.Fatalf("ListReplaysByIDs: %v", err)
	}
	if len(rows) != len(replayIDs) {
		t.Fatalf("got %d session replay rows, want %d", len(rows), len(replayIDs))
	}
	for _, row := range rows {
		if row.ReplayID == 0 || row.MapName == "" {
			t.Fatalf("bad session replay row: %+v", row)
		}
	}

	apm, err := d.dbStore.ListPlayerAPMByReplayIDs(ctx, replayIDs)
	if err != nil {
		t.Fatalf("ListPlayerAPMByReplayIDs: %v", err)
	}
	if len(apm) == 0 {
		t.Fatal("no APM rows for the corpus")
	}

	if _, err := d.dbStore.ListReplaysByIDs(ctx, nil); err != nil {
		t.Fatalf("ListReplaysByIDs(nil): %v", err)
	}
	if _, err := d.dbStore.ListPlayerAPMByReplayIDs(ctx, nil); err != nil {
		t.Fatalf("ListPlayerAPMByReplayIDs(nil): %v", err)
	}
}
