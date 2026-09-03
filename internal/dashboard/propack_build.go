package dashboard

import (
	"context"
	"fmt"

	"github.com/marianogappa/screpdb/internal/hotkeystream"
	"github.com/marianogappa/screpdb/internal/propack"
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

// ComputeProMetrics opens the scratch database at sqlitePath as a dashboard
// (running its migrations, applying the default global replay filter) and
// aggregates each player key with the production code paths. Offline tooling
// only: scripts/pro-pack calls it after renaming the resolved progamer rows to
// their pack keys.
func ComputeProMetrics(ctx context.Context, sqlitePath string, playerKeys []string) (map[string]ProMetrics, error) {
	d, err := New(ctx, sqlitePath, true)
	if err != nil {
		return nil, err
	}
	defer d.db.Close()

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
	for _, key := range playerKeys {
		key = normalizePlayerKey(key)
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
