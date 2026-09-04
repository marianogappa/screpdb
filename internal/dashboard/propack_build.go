package dashboard

import (
	"context"
	"database/sql"
	"fmt"

	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
	"github.com/marianogappa/screpdb/internal/hotkeystream"
	"github.com/marianogappa/screpdb/internal/propack"
	_ "modernc.org/sqlite"
)

// ProMetrics is everything the pro pack bakes for one player key. It is
// produced by the same aggregators the dashboard runs on local players, which
// is what makes a pro's value comparable with the user's dataset.
type ProMetrics struct {
	GamesSampled       int
	Races              map[string]int
	APM                *propack.Metric
	Cadence            *propack.Cadence
	ViewportSwitchRate *propack.Metric
	Hotkeys            []*hotkeystream.Signature
}

// ComputeProMetrics aggregates each player key in the scratch database with
// the production code paths, so a built pack reports what the app would.
//
// Offline tooling only, and the last SQL reader in this package: the shipped
// dashboard answers every read from the in-memory replay library, but
// scripts/pro-pack joins its labelled games against an ingested corpus and
// renames player rows to pack keys, neither of which has a library equivalent
// yet. Reads here are unfiltered, where the dashboard used to apply the
// default global filter.
func ComputeProMetrics(ctx context.Context, sqlitePath string, playerKeys []string) (map[string]ProMetrics, error) {
	d, closeDashboard, err := newSQLBackedDashboard(ctx, sqlitePath)
	if err != nil {
		return nil, err
	}
	defer closeDashboard()

	apmRows, err := d.dbStore.ListPlayerApmAggregates(ctx, 1)
	if err != nil {
		return nil, fmt.Errorf("apm aggregates: %w", err)
	}
	apmByKey := map[string]*propack.Metric{}
	for _, row := range apmRows {
		apmByKey[row.PlayerKey] = &propack.Metric{Value: row.AverageAPM, Games: int(row.GamesPlayed)}
	}
	viewportAll, err := d.loadWorkflowViewportMultitaskingAggregates()
	if err != nil {
		return nil, fmt.Errorf("viewport aggregates: %w", err)
	}

	out := make(map[string]ProMetrics, len(playerKeys))
	for _, rawKey := range playerKeys {
		key := normalizePlayerKey(rawKey)
		m := ProMetrics{Races: map[string]int{}}

		summary, err := d.dbStore.GetPlayerOverviewSummary(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("%s: overview: %w", key, err)
		}
		m.GamesSampled = int(summary.GamesPlayed)
		if m.GamesSampled == 0 {
			continue
		}
		raceRows, err := d.dbStore.ListRaceSections(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("%s: races: %w", key, err)
		}
		for _, row := range raceRows {
			m.Races[row.Race] = int(row.GameCount)
		}
		m.APM = apmByKey[key]

		cadence, err := d.buildWorkflowPlayerUnitCadenceInsight(key, workflowUnitCadenceFilterStrict)
		if err != nil {
			return nil, fmt.Errorf("%s: cadence: %w", key, err)
		}
		if cadence.GamesUsed >= workflowUnitCadenceMinGames {
			m.Cadence = &propack.Cadence{
				Score:      cadence.AverageCadenceScore,
				RatePerMin: cadence.AverageRatePerMin,
				CVGap:      cadence.AverageCVGap,
				Burstiness: cadence.AverageBurstiness,
				Idle20:     cadence.AverageIdle20,
				Games:      int(cadence.GamesUsed),
			}
		}
		if agg, ok := findWorkflowViewportMultitaskingAggregate(viewportAll, key); ok && agg.isEligible() {
			m.ViewportSwitchRate = &propack.Metric{Value: agg.averageViewportSwitchRate, Games: int(agg.GamesPlayed)}
		}

		payload, err := d.localHotkeySignature(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("%s: hotkeys: %w", key, err)
		}
		m.Hotkeys = payload.Cards
		out[key] = m
	}
	return out, nil
}

// newSQLBackedDashboard builds the minimum dashboard the pack builder needs:
// the aggregation methods and their helpers, reading an already-ingested
// database. It has no replay library, so anything that needs the replay folder
// is unavailable.
func newSQLBackedDashboard(ctx context.Context, sqlitePath string) (*Dashboard, func(), error) {
	db, err := sql.Open("sqlite", "file:"+sqlitePath+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	d := &Dashboard{
		ctx:        ctx,
		libraryHub: newLibraryHub(),
		headless:   true,
	}
	d.dbStore = dashboarddb.NewStore(db, func() *sql.DB { return db })
	return d, func() { _ = db.Close() }, nil
}
