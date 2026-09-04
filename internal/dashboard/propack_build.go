package dashboard

import (
	"context"
	"fmt"
	"sort"

	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
	"github.com/marianogappa/screpdb/internal/hotkeystream"
	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/load"
	"github.com/marianogappa/screpdb/internal/library/persist"
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

// ProCorpusPlayer is one human non-observer player of a ProCorpusReplay.
type ProCorpusPlayer struct {
	PlayerID int64
	Name     string
	Race     string
}

// ProCorpusReplay is one analysed 1v1 replay of a pack-building corpus, which
// the caller joins its labelled progamer sides against.
type ProCorpusReplay struct {
	ReplayID int64
	FileName string
	Matchup  string
	Players  []ProCorpusPlayer
}

// ProCorpus is an offline replay library for building the built-in progamer
// pack. The caller loads a folder of labelled ladder replays, joins its own
// labels against Replays1v1, renames the players it resolved to a progamer so
// one pro reads as one player however many accounts they used, and then reads
// Metrics, which run the dashboard's own aggregators.
//
// Offline tooling only. Nothing is persisted: the settings and caches the
// dashboard keeps on disk are held in memory here.
type ProCorpus struct {
	dash *Dashboard
	lib  *library.Library
}

// LoadProCorpus parses every replay in replayDir. It applies the same default
// global filter the dashboard ships with, so a pro's numbers are measured on
// the games the app would have counted.
func LoadProCorpus(ctx context.Context, replayDir string, log func(load.LogEvent)) (*ProCorpus, error) {
	if err := iofacade.AllowDir(replayDir); err != nil {
		return nil, err
	}
	lib := library.New(library.Options{})
	settings, err := dashboarddb.NewMemorySettings(lib)
	if err != nil {
		lib.Close()
		return nil, err
	}
	loader := load.New(lib, load.Options{Folder: replayDir, Generation: 1, Log: log})
	if err := loader.Run(ctx); err != nil {
		lib.Close()
		return nil, fmt.Errorf("read %s: %w", replayDir, err)
	}

	dash := &Dashboard{ctx: ctx, libraryHub: newLibraryHub(), headless: true}
	dash.dbStore = dashboarddb.NewLibStore(lib, persist.NewBnetCache(""), settings)
	return &ProCorpus{dash: dash, lib: lib}, nil
}

func (c *ProCorpus) Close() { c.lib.Close() }

// Len is how many replays were analysed.
func (c *ProCorpus) Len() int { return c.lib.Snapshot().Len() }

// Replays1v1 lists every analysed 1v1 replay with its human non-observer
// players. A replay reached by more than one file name appears once per name,
// because the corpus collapses identical content into one game.
func (c *ProCorpus) Replays1v1() []ProCorpusReplay {
	snap := c.lib.Snapshot()
	out := make([]ProCorpusReplay, 0, snap.Len())
	for _, replay := range snap.Replays {
		if replay.Flags&library.FlagIsOneOnOne == 0 {
			continue
		}
		players := make([]ProCorpusPlayer, 0, len(replay.Players))
		for ordinal := range replay.Players {
			player := &replay.Players[ordinal]
			if player.IsObserver() || !player.IsHuman() {
				continue
			}
			players = append(players, ProCorpusPlayer{
				PlayerID: replay.PlayerID(uint8(ordinal)),
				Name:     player.Name,
				Race:     player.Race.String(),
			})
		}
		if len(players) == 0 {
			continue
		}
		for _, file := range replay.Paths {
			out = append(out, ProCorpusReplay{
				ReplayID: replay.ID,
				FileName: fileNameOf(file.Path),
				Matchup:  library.Strings.Name(replay.Matchup),
				Players:  players,
			})
		}
	}
	return out
}

// RenamePlayers renames the given players in place of the ingest-time rename
// the SQL pack builder used to do: every per-player aggregator groups by
// player name, so pointing a pro's accounts at one name is what makes their
// games aggregate together.
func (c *ProCorpus) RenamePlayers(nameByPlayerID map[int64]string) int {
	if len(nameByPlayerID) == 0 {
		return 0
	}
	snap := c.lib.Snapshot()
	generation := snap.Generation + 1
	records := make([]*library.Replay, 0, snap.Len())
	renamed := 0
	for _, replay := range snap.Replays {
		next, n := renameReplayPlayers(replay, nameByPlayerID)
		renamed += n
		records = append(records, next)
	}
	c.lib.Reset(generation)
	c.lib.Add(generation, records...)
	c.lib.Flush()
	return renamed
}

// renameReplayPlayers returns a copy of replay with the named players renamed,
// or replay itself when none of them are. Records are immutable once the
// library holds them, so the ones that change are copied rather than written.
func renameReplayPlayers(replay *library.Replay, nameByPlayerID map[int64]string) (*library.Replay, int) {
	renamed := 0
	var next *library.Replay
	for ordinal := range replay.Players {
		name, ok := nameByPlayerID[replay.PlayerID(uint8(ordinal))]
		if !ok {
			continue
		}
		if next == nil {
			copied := *replay
			copied.Players = make([]library.Player, len(replay.Players))
			copy(copied.Players, replay.Players)
			next = &copied
		}
		next.Players[ordinal].Name = name
		next.Players[ordinal].Key = library.PlayerKey(name)
		renamed++
	}
	if next == nil {
		return replay, 0
	}
	return next, renamed
}

func fileNameOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}

// Metrics runs the dashboard's per-player aggregators for each of playerKeys.
func (c *ProCorpus) Metrics(ctx context.Context, playerKeys []string) (map[string]ProMetrics, error) {
	d := c.dash
	sort.Strings(playerKeys)
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
