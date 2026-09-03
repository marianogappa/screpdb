package library_test

import (
	"fmt"
	"math/rand"
	"runtime"
	"testing"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
)

// maxBytesPerRecord is the memory budget for one typical 1v1 replay record
// (~1000 production events, 25 markers, 50 events, two 3 KB hotkey blobs and
// two 40-float fingerprints) including its share of the snapshot indexes.
const maxBytesPerRecord = 25 * 1024

var (
	syntheticUnits = []string{"Marine", "Medic", "Vulture", "Siege Tank", "Goliath", "Wraith", "Zealot", "Dragoon",
		"High Templar", "Zergling", "Hydralisk", "Mutalisk", "Lurker", "SCV", "Probe", "Drone", "Overlord", "Barracks",
		"Factory", "Gateway", "Hatchery", "Spawning Pool", "Supply Depot", "Pylon", "Command Center", "Refinery"}
	syntheticFeatures = []string{"bo_2_fact", "carriers", "made_drops", "nuke", "mid_game_starts", "late_game_starts",
		"bo_z_fuzzy", "viewport_multitasking", "never_used_hotkeys", "team_stacking", "wraith_cloak", "proxy_starport",
		"muta_harass", "gas_steal", "fast_expand", "lurker_contain", "dark_templar", "reaver_drop", "storm_drop",
		"cannon_contain", "manner_pylon", "bunker_rush", "cheese", "allin", "macro_game"}
	syntheticEventTypes = []string{"attack", "leave_game", "base_established", "base_lost", "expansion", "drop",
		"first_gas", "tech_started", "upgrade_started", "alliance_changed", "player_inactive", "recall", "nuke_launched",
		"muta_harass_start", "muta_harass_end", "connection_lost", "contain_started", "contain_ended", "harass",
		"proxy_detected", "scout", "worker_rush", "cheese_detected", "timing_attack", "counter_attack", "defense",
		"army_wiped", "base_wiped", "gg", "surrender"}
)

func syntheticReplay(rng *rand.Rand, i int) *library.Replay {
	opts := []librarytest.Option{
		librarytest.WithChecksum(fmt.Sprintf("synthetic-%d", i)),
		librarytest.WithPath(fmt.Sprintf("/Users/player/Documents/StarCraft/Maps/Replays/Autosave/2026-01-%02d/LastReplay-%d.rep", i%28+1, i), librarytest.BaseDate),
		librarytest.WithMap([]string{"Fighting Spirit", "Polypoid", "Eclipse", "Circuit Breaker", "Vermeer"}[i%5]),
		librarytest.WithTitle(fmt.Sprintf("game %d", i)),
		librarytest.WithMatchup("TvZ"),
		librarytest.WithPlayer(fmt.Sprintf("Player%d", i%500), librarytest.Team(1), librarytest.Race(library.RaceTerran)),
		librarytest.WithPlayer(fmt.Sprintf("Opponent%d", rng.Intn(3000)), librarytest.Team(2), librarytest.Race(library.RaceZerg)),
	}
	for e := 0; e < 1000; e++ {
		kind := library.ProdTrain
		if e%9 == 0 {
			kind = library.ProdBuild
		}
		opts = append(opts, librarytest.WithProd(uint8(e%2), rng.Intn(1500), kind, syntheticUnits[rng.Intn(len(syntheticUnits))]))
	}
	for m := 0; m < 25; m++ {
		payload := ""
		if m%5 == 0 {
			payload = fmt.Sprintf(`{"switches_per_minute":%.3f}`, rng.Float64()*20)
		}
		opts = append(opts, librarytest.WithMarker(syntheticFeatures[m], uint8(m%2), rng.Intn(1500), payload))
	}
	for e := 0; e < 50; e++ {
		opts = append(opts, librarytest.WithEvent(syntheticEventTypes[e%len(syntheticEventTypes)], rng.Intn(1500), uint8(e%2), uint8((e+1)%2)))
	}
	for p := uint8(0); p < 2; p++ {
		blob := make([]byte, 3*1024)
		rng.Read(blob)
		vec := make([]float64, 40)
		for i := range vec {
			vec[i] = rng.Float64()
		}
		opts = append(opts, librarytest.WithHotkeyStream(p, blob), librarytest.WithFingerprint(p, vec))
	}
	return librarytest.Replay(opts...)
}

func TestSyntheticCorpusBytesPerRecord(t *testing.T) {
	if testing.Short() {
		t.Skip("allocates a 10k-record corpus")
	}
	const records = 10_000
	rng := rand.New(rand.NewSource(1))

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	lib := library.New(library.Options{CoalesceRecords: 1000})
	defer lib.Close()
	for i := 0; i < records; i++ {
		lib.Add(0, syntheticReplay(rng, i))
	}
	lib.Flush()
	snap := lib.Snapshot()
	if snap.Len() != records {
		t.Fatalf("committed %d records, want %d", snap.Len(), records)
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	perRecord := (after.HeapAlloc - before.HeapAlloc) / records
	t.Logf("synthetic corpus: %d records, %.1f KB/record, heap in use %.1f MB",
		records, float64(perRecord)/1024, float64(after.HeapInuse)/1024/1024)
	if perRecord > maxBytesPerRecord {
		t.Fatalf("%d bytes/record exceeds the %d budget", perRecord, maxBytesPerRecord)
	}
	runtime.KeepAlive(snap)
}

func BenchmarkSnapshotCommit10k(b *testing.B) {
	rng := rand.New(rand.NewSource(1))
	lib := library.New(library.Options{CoalesceRecords: 100_000})
	defer lib.Close()
	for i := 0; i < 10_000; i++ {
		lib.Add(0, syntheticReplay(rng, i))
	}
	lib.Flush()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lib.Add(0, syntheticReplay(rng, 100_000+i))
		lib.Flush()
	}
}
