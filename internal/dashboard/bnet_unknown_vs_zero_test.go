package dashboard

import (
	"context"
	"testing"
	"time"

	"github.com/marianogappa/scfingerprint"

	"github.com/marianogappa/screpdb/internal/library/persist"
)

// A stat Battle.net reported as zero and a stat we never received must not
// arrive at the page looking the same. Issue #333.
func TestBnetProfileDetail_KnownZeroIsNotUnknown(t *testing.T) {
	// An account that laddered, lost everything and holds a zero rating. Every
	// number here is real and must survive as a number.
	payload := []byte(`{
		"aurora_id": 99,
		"toons": [{"toon": "zeroed", "gateway_id": 30, "games_last_week": 0}],
		"matchmaked_stats": [{"rating": 0, "highest_rating": 0, "wins": 0, "losses": 0}],
		"stats": [{"season_id": 0, "gateway_id": 30, "toon": "zeroed", "terran": {"wins": 0, "losses": 0}}]
	}`)
	record, _ := persist.DistillBnetProfile("zeroed", 30, time.Now(), payload)
	got := bnetProfileDetailFromRecord(record, nil)

	if !got.Found {
		t.Fatal("a distilled profile with an aurora id is found")
	}
	for name, ptr := range map[string]*int{
		"mmr":             got.MMR,
		"highest_mmr":     got.HighestMMR,
		"ladder_wins":     got.LadderWins,
		"ladder_losses":   got.LadderLosses,
		"lifetime_games":  got.LifetimeGames,
		"lifetime_wins":   got.LifetimeWins,
		"games_last_week": got.GamesLastWeek,
	} {
		if ptr == nil {
			t.Errorf("%s is nil, but Battle.net reported it as zero", name)
			continue
		}
		if *ptr != 0 {
			t.Errorf("%s = %d, want the reported zero", name, *ptr)
		}
	}
	// An average over no games has no value to report, which is a different
	// fact from an average of zero.
	if got.AverageAPM != nil {
		t.Errorf("average_apm = %v, want nil over zero games", *got.AverageAPM)
	}
}

func TestBnetProfileDetail_MissCarriesNoStatsAtAll(t *testing.T) {
	fetched := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	record, _ := persist.DistillBnetProfile("ghost", 30, fetched, nil)
	got := bnetProfileDetailFromRecord(record, nil)

	if got.Found {
		t.Fatal("an empty payload is a miss")
	}
	if got.FetchedAt != "2026-09-20T10:00:00Z" {
		t.Errorf("fetched_at = %q, want the fetch time so the page can date the answer", got.FetchedAt)
	}
	// Not one of these may be zero: inventing an unranked player with no games
	// is exactly the bug.
	if got.PlaysLadder != nil || got.MMR != nil || got.LifetimeGames != nil || got.GamesLastWeek != nil {
		t.Errorf("a miss must carry no stats: %+v", got)
	}
}

func TestBnetProfileDetail_NonLadderingIsAnswered(t *testing.T) {
	payload := []byte(`{"aurora_id": 5, "toons": [{"toon": "casual", "gateway_id": 30, "games_last_week": 3}]}`)
	record, _ := persist.DistillBnetProfile("casual", 30, time.Now(), payload)
	got := bnetProfileDetailFromRecord(record, nil)

	// "Battle.net holds no matchmaking record" is knowledge, so plays_ladder is
	// a definite false rather than an absence.
	if got.PlaysLadder == nil || *got.PlaysLadder {
		t.Fatalf("plays_ladder = %v, want a definite false", got.PlaysLadder)
	}
	if got.MMR != nil {
		t.Errorf("mmr = %d, want nil: a rating does not apply to an account with no record", *got.MMR)
	}
	if got.GamesLastWeek == nil || *got.GamesLastWeek != 3 {
		t.Errorf("games_last_week = %v, want 3 from the toons row", got.GamesLastWeek)
	}
}

// A single miss rules out one gateway, not Battle.net. The page needs the tally
// so a sweep still running does not read as "no such account".
func TestBnetProfileDetails_ReportsGatewaySweepProgress(t *testing.T) {
	d := newTestDashboardWithReplays(t)
	ctx := context.Background()
	now := time.Now()

	for _, gw := range defaultGatewayOrder[:2] {
		miss, _ := persist.DistillBnetProfile("ghost", gw, now, nil)
		if err := d.dbStore.UpsertBnetProfile(ctx, miss); err != nil {
			t.Fatal(err)
		}
	}
	got := d.bnetProfileDetailsByPlayerKeys(ctx, []string{"ghost"})
	detail := got[normalizePlayerKey("ghost")]
	if detail == nil {
		t.Fatal("a swept toon must reach the page, so the page can say what happened")
	}
	if detail.Found {
		t.Fatal("no gateway had this toon")
	}
	if detail.GatewaysChecked != 2 || detail.GatewaysTotal != len(defaultGatewayOrder) {
		t.Errorf("sweep = %d/%d, want 2/%d", detail.GatewaysChecked, detail.GatewaysTotal, len(defaultGatewayOrder))
	}
	if _, ok := got[normalizePlayerKey("never-asked")]; ok {
		t.Error("a toon nobody looked up must stay absent from the payload")
	}
}

// A found profile on any gateway outranks the misses on the others, so the
// sweep's failures never hide the account it eventually found.
func TestBnetProfileDetails_FoundBeatsMisses(t *testing.T) {
	d := newTestDashboardWithReplays(t)
	ctx := context.Background()
	now := time.Now()

	miss, _ := persist.DistillBnetProfile("Flash", 30, now, nil)
	if err := d.dbStore.UpsertBnetProfile(ctx, miss); err != nil {
		t.Fatal(err)
	}
	found, _ := persist.DistillBnetProfile("Flash", 10, now, []byte(`{"aurora_id": 11, "toons": [{"toon": "Flash", "gateway_id": 10, "games_last_week": 7}]}`))
	if err := d.dbStore.UpsertBnetProfile(ctx, found); err != nil {
		t.Fatal(err)
	}

	detail := d.bnetProfileDetailsByPlayerKeys(ctx, []string{"Flash"})[normalizePlayerKey("Flash")]
	if detail == nil || !detail.Found || detail.AuroraID != 11 {
		t.Fatalf("want the found profile to win, got %+v", detail)
	}
	if detail.GatewaysChecked != 0 {
		t.Errorf("gateways_checked = %d, want 0: the sweep succeeded, so its progress is moot", detail.GatewaysChecked)
	}
}

// "No fingerprint badge" used to mean four different things at once. The reason
// is what makes them readable apart. Issue #333, following the note on #321.
func TestFingerprintOutcome_NotEnoughGamesIsItsOwnAnswer(t *testing.T) {
	d := newTestDashboardWithReplays(t)

	outcome, err := d.fingerprintOutcomeFor("nobody-here", 1)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.match != nil {
		t.Fatalf("a player with no vectors cannot match: %+v", outcome.match)
	}
	if outcome.reason != fingerprintNoMatchNotEnoughGames {
		t.Errorf("reason = %q, want %q: we could not try, which is not the same as trying and failing",
			outcome.reason, fingerprintNoMatchNotEnoughGames)
	}
}

// Coverage must count the games the matcher will actually use. Counting games
// it discards told a Big Game Hunters regular that identification needs three
// games and they have two hundred, which is not an explanation of anything.
func TestFingerprintCoverage_CountsOnlyEligibleGames(t *testing.T) {
	d := newTestDashboard(t)

	rows, err := d.dbStore.ListPlayerFingerprintVectors(d.ctx, "chobo86", int64(scfingerprint.FeatureVersion()))
	if err != nil {
		t.Fatal(err)
	}
	coverage, err := d.dbStore.GetPlayerFingerprintCoverage(d.ctx, "chobo86", int64(scfingerprint.FeatureVersion()))
	if err != nil {
		t.Fatal(err)
	}
	if coverage != int64(len(rows)) {
		t.Errorf("coverage = %d but the matcher gets %d vectors; the number shown must be the number used", coverage, len(rows))
	}
}
