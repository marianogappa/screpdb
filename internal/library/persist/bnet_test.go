package persist

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBnetCacheRoundTrip(t *testing.T) {
	root := t.TempDir()
	cache := NewBnetCache(root)
	if err := cache.Load(); err != nil || cache.Len() != 0 {
		t.Fatalf("empty cache: len=%d err=%v", cache.Len(), err)
	}

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	flash := BnetProfile{Toon: "Flash", Gateway: 30, Found: true, AuroraID: 11, BattleTag: "Flash#1", CountryCode: "KR", FetchedAt: now,
		Toons:  []BnetToon{{Toon: "Flash", Gateway: 30, GamesLastWeek: 5}},
		Ladder: []BnetLadder{{Season: 21, Rating: 2100, HighestRating: 2200, Wins: 30, Losses: 10}},
	}
	flashOld := BnetProfile{Toon: "FLASH", Gateway: 10, Found: true, AuroraID: 12, CountryCode: "US", FetchedAt: now.Add(-48 * time.Hour)}
	missing := BnetProfile{Toon: "Nobody", Gateway: 30, Found: false, FetchedAt: now}
	noCountry := BnetProfile{Toon: "Bisu", Gateway: 30, Found: true, AuroraID: 13, FetchedAt: now}
	for _, p := range []BnetProfile{flash, flashOld, missing, noCountry} {
		if err := cache.Upsert(p); err != nil {
			t.Fatal(err)
		}
	}
	if err := cache.Upsert(BnetProfile{}); err == nil {
		t.Fatal("empty toon must be rejected")
	}
	if _, err := os.Stat(BnetProfilesPath(root) + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("temp file must not survive a save")
	}

	reloaded := NewBnetCache(root)
	if err := reloaded.Load(); err != nil {
		t.Fatal(err)
	}
	if reloaded.Len() != 4 {
		t.Fatalf("reloaded %d entries", reloaded.Len())
	}
	got := reloaded.Get("Flash", 30)
	if got == nil || got.AuroraID != 11 || !got.FetchedAt.Equal(now) || len(got.Ladder) != 1 || got.Ladder[0].Rating != 2100 {
		t.Fatalf("Get = %+v", got)
	}
	if got := reloaded.Get("Flash", 99); got != nil {
		t.Fatal("unknown gateway must return nil")
	}

	codes := reloaded.CountryCodesByToons([]string{" flash ", "nobody", "bisu", "ghost"})
	if len(codes) != 1 || codes["flash"] != "KR" {
		t.Fatalf("country codes %+v", codes)
	}
	profiles := reloaded.ProfilesByToons([]string{"Flash", "Nobody", "Bisu"})
	if len(profiles) != 3 {
		t.Fatalf("profiles %d, want the found entries only", len(profiles))
	}
	for _, p := range profiles {
		if !p.Found {
			t.Fatalf("profile entry %+v", p)
		}
	}

	fetchTimes := reloaded.FetchedAtByToons([]string{"flash", "nobody", "bisu", "ghost"})
	if len(fetchTimes) != 2 {
		t.Fatalf("FetchedAtByToons = %d entries, want 2 (flash + bisu, not nobody/ghost)", len(fetchTimes))
	}
	if !fetchTimes["flash"].Equal(now) {
		t.Fatalf("flash FetchedAt = %v, want %v", fetchTimes["flash"], now)
	}

	updated := flash
	updated.CountryCode = "JP"
	if err := reloaded.Upsert(updated); err != nil {
		t.Fatal(err)
	}
	if reloaded.Len() != 4 {
		t.Fatal("upsert must replace, not duplicate")
	}
	if got := reloaded.Get("Flash", 30); got.CountryCode != "JP" {
		t.Fatalf("after upsert %+v", got)
	}

	pruned, err := reloaded.PruneOlderThan(time.Since(now) + 24*time.Hour)
	if err != nil || pruned != 1 || reloaded.Len() != 3 {
		t.Fatalf("pruned %d err %v len %d", pruned, err, reloaded.Len())
	}
	fresh := NewBnetCache(root)
	if err := fresh.Load(); err != nil || fresh.Len() != 3 {
		t.Fatalf("prune must persist: len=%d err=%v", fresh.Len(), err)
	}
	if fresh.Get("FLASH", 10) != nil {
		t.Fatal("the pruned entry must be gone after a reload")
	}
}

func TestBnetCacheLoadSkipsTornLines(t *testing.T) {
	root := t.TempDir()
	cache := NewBnetCache(root)
	if err := cache.Upsert(BnetProfile{Toon: "Flash", Gateway: 30, Found: true, FetchedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(BnetProfilesPath(root))
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, []byte(`{"t":"Torn","g":30,`)...)
	if err := os.WriteFile(BnetProfilesPath(root), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	reloaded := NewBnetCache(root)
	if err := reloaded.Load(); err != nil || reloaded.Len() != 1 {
		t.Fatalf("len=%d err=%v, want the torn trailing line skipped", reloaded.Len(), err)
	}
}

func TestBnetCacheLoadDeletesStaleProfileVersions(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, BnetDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(dir, "profiles.v1.jsonl")
	if err := os.WriteFile(stale, []byte(`{"t":"Old","g":30}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The game archive must never be swept by a profile version bump: it is
	// the one store that cannot refill itself.
	games := filepath.Join(dir, "games.v1.jsonl")
	if err := os.WriteFile(games, []byte(`{"i":"g1"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cache := NewBnetCache(root)
	if err := cache.Load(); err != nil || cache.Len() != 0 {
		t.Fatalf("len=%d err=%v", cache.Len(), err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("the stale profile version must be deleted at load")
	}
	if _, err := os.Stat(games); err != nil {
		t.Fatalf("a stale game archive must never be silently deleted: %v", err)
	}
}

func TestBnetCacheBudgetEvictsOldest(t *testing.T) {
	root := t.TempDir()
	cache := NewBnetCache(root)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	batch := make([]BnetProfile, 0, bnetProfileBudgetEntries+5)
	for i := 0; i < bnetProfileBudgetEntries+5; i++ {
		batch = append(batch, BnetProfile{
			Toon:      fmt.Sprintf("toon-%06d", i),
			Gateway:   30,
			Found:     true,
			FetchedAt: base.Add(time.Duration(i) * time.Second),
		})
	}
	if err := cache.UpsertBatch(batch); err != nil {
		t.Fatal(err)
	}
	if cache.Len() != bnetProfileBudgetEntries {
		t.Fatalf("len = %d, want the budget", cache.Len())
	}
	for i := 0; i < 5; i++ {
		if cache.Get(fmt.Sprintf("toon-%06d", i), 30) != nil {
			t.Fatalf("toon-%06d is the oldest and must be evicted", i)
		}
	}
	if cache.Get(fmt.Sprintf("toon-%06d", bnetProfileBudgetEntries+4), 30) == nil {
		t.Fatal("the newest entry must survive the budget")
	}
}

func TestBnetCacheConcurrentUpserts(t *testing.T) {
	root := t.TempDir()
	cache := NewBnetCache(root)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_ = cache.Upsert(BnetProfile{Toon: fmt.Sprintf("toon-%d-%d", i, j), Gateway: 30, Found: true, FetchedAt: time.Now()})
				cache.Get(fmt.Sprintf("toon-%d-%d", i, j), 30)
			}
		}(i)
	}
	wg.Wait()
	if cache.Len() != 80 {
		t.Fatalf("len = %d, want 80", cache.Len())
	}
}

func TestBnetGameArchiveRoundTripAndMerge(t *testing.T) {
	root := t.TempDir()
	archive := NewBnetGameArchive(root)
	if err := archive.Load(); err != nil || archive.Len() != 0 {
		t.Fatalf("empty archive: len=%d err=%v", archive.Len(), err)
	}

	played := time.Date(2026, 5, 1, 20, 0, 0, 0, time.UTC)
	partialA := BnetGame{
		GameID: "g1", CreateTime: played, Gateway: 30, MapName: "Fighting Spirit",
		Accounts: []int64{100},
		Players: []BnetGamePlayer{
			{Toon: "A", Race: "zerg", Result: BnetResultWin, APM: 200},
			{Toon: "B", Race: "terran"},
		},
	}
	partialB := BnetGame{
		GameID: "g1", CreateTime: played, GameName: "3v3 BGH", Host: "A", Type: 15, SubType: 3,
		MapWidth: 128, MapHeight: 128, TileSet: 4,
		Accounts: []int64{200},
		Players: []BnetGamePlayer{
			{Toon: "B", Race: "terran", Result: BnetResultLoss, Seconds: 900},
		},
	}
	if err := archive.Upsert([]BnetGame{partialA}); err != nil {
		t.Fatal(err)
	}
	if err := archive.Upsert([]BnetGame{partialB}); err != nil {
		t.Fatal(err)
	}

	reloaded := NewBnetGameArchive(root)
	if err := reloaded.Load(); err != nil || reloaded.Len() != 1 {
		t.Fatalf("len=%d err=%v", reloaded.Len(), err)
	}
	games := reloaded.GamesForAccount(100)
	if len(games) != 1 {
		t.Fatalf("games for 100: %d", len(games))
	}
	g := games[0]
	if g.GameName != "3v3 BGH" || g.Host != "A" || g.Type != 15 || g.SubType != 3 || g.MapName != "Fighting Spirit" {
		t.Fatalf("merge lost fields: %+v", g)
	}
	if len(g.Accounts) != 2 {
		t.Fatalf("accounts must union: %+v", g.Accounts)
	}
	if len(g.Players) != 2 {
		t.Fatalf("players must merge by toon: %+v", g.Players)
	}
	for _, p := range g.Players {
		switch p.Toon {
		case "A":
			if p.Result != BnetResultWin || p.APM != 200 {
				t.Fatalf("player A regressed: %+v", p)
			}
		case "B":
			if p.Result != BnetResultLoss || p.Seconds != 900 || p.Race != "terran" {
				t.Fatalf("player B not filled: %+v", p)
			}
		}
	}

	// A known result is never overwritten by a different known one.
	conflict := BnetGame{GameID: "g1", Players: []BnetGamePlayer{{Toon: "A", Result: BnetResultLoss}}}
	if err := reloaded.Upsert([]BnetGame{conflict}); err != nil {
		t.Fatal(err)
	}
	for _, p := range reloaded.GamesForAccount(100)[0].Players {
		if p.Toon == "A" && p.Result != BnetResultWin {
			t.Fatalf("a known result was overwritten: %+v", p)
		}
	}

	times := reloaded.TimesSince(200, played.Add(-time.Hour))
	if len(times) != 1 || !times[0].Equal(played) {
		t.Fatalf("times = %v", times)
	}
	if got := reloaded.TimesSince(200, played.Add(time.Hour)); len(got) != 0 {
		t.Fatalf("since after the game must return nothing, got %v", got)
	}
	if got := reloaded.TimesSince(999, played.Add(-time.Hour)); len(got) != 0 {
		t.Fatalf("unknown account must return nothing, got %v", got)
	}
}

func TestBnetGameArchiveSkipsTornTrailingLine(t *testing.T) {
	root := t.TempDir()
	archive := NewBnetGameArchive(root)
	if err := archive.Upsert([]BnetGame{{GameID: "g1", CreateTime: time.Now(), Accounts: []int64{1}}}); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(BnetGamesPath(root), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"i":"g2","c":`); err != nil {
		t.Fatal(err)
	}
	f.Close()
	reloaded := NewBnetGameArchive(root)
	if err := reloaded.Load(); err != nil || reloaded.Len() != 1 {
		t.Fatalf("len=%d err=%v, want the torn line skipped", reloaded.Len(), err)
	}
}

func TestBnetGameArchiveCompactionKeepsLastWrite(t *testing.T) {
	root := t.TempDir()
	archive := NewBnetGameArchive(root)
	games := make([]BnetGame, 0, bnetGamesCompactMinLines)
	for i := 0; i < bnetGamesCompactMinLines; i++ {
		games = append(games, BnetGame{GameID: fmt.Sprintf("g%d", i), CreateTime: time.Now(), Accounts: []int64{1}})
	}
	if err := archive.Upsert(games); err != nil {
		t.Fatal(err)
	}
	// Re-observe a third of the games with a new field each, forcing appended
	// duplicates past the compaction threshold.
	for i := 0; i < bnetGamesCompactMinLines/3; i++ {
		if err := archive.Upsert([]BnetGame{{GameID: fmt.Sprintf("g%d", i), MapName: fmt.Sprintf("map-%d", i)}}); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(BnetGamesPath(root))
	if err != nil {
		t.Fatal(err)
	}
	// Compaction fires once the duplicate share crosses the threshold, so the
	// log must be well under the never-compacted line count.
	lines := strings.Count(string(raw), "\n")
	if lines >= bnetGamesCompactMinLines+bnetGamesCompactMinLines/3 {
		t.Fatalf("the log never compacted: %d lines for %d games", lines, archive.Len())
	}
	if err := archive.CompactIfNeeded(); err != nil {
		t.Fatal(err)
	}
	reloaded := NewBnetGameArchive(root)
	if err := reloaded.Load(); err != nil {
		t.Fatal(err)
	}
	if reloaded.Len() != archive.Len() {
		t.Fatalf("reloaded %d games, want %d", reloaded.Len(), archive.Len())
	}
	got := reloaded.GamesForAccount(1)
	byID := map[string]BnetGame{}
	for _, g := range got {
		byID[g.GameID] = g
	}
	if byID["g0"].MapName != "map-0" {
		t.Fatalf("compaction lost the last write: %+v", byID["g0"])
	}
}

func TestBnetGameArchiveClearsExpiredReplayRefs(t *testing.T) {
	root := t.TempDir()
	archive := NewBnetGameArchive(root)
	old := time.Now().Add(-60 * 24 * time.Hour)
	fresh := time.Now().Add(-time.Hour)
	if err := archive.Upsert([]BnetGame{
		{GameID: "old", CreateTime: old, MD5: "aaa", URL: "https://x/old", Accounts: []int64{1}},
		{GameID: "fresh", CreateTime: fresh, MD5: "bbb", URL: "https://x/fresh", Accounts: []int64{1}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := archive.ClearReplayRefsOlderThan(40 * 24 * time.Hour); err != nil {
		t.Fatal(err)
	}
	reloaded := NewBnetGameArchive(root)
	if err := reloaded.Load(); err != nil {
		t.Fatal(err)
	}
	for _, g := range reloaded.GamesForAccount(1) {
		switch g.GameID {
		case "old":
			if g.MD5 != "" || g.URL != "" {
				t.Fatalf("expired refs must be cleared: %+v", g)
			}
		case "fresh":
			if g.MD5 != "bbb" || g.URL == "" {
				t.Fatalf("fresh refs must survive: %+v", g)
			}
		}
	}
}

const distillFixture = `{
	"aurora_id": 1000,
	"battle_tag": "Someone#123",
	"country_code": " KR ",
	"program_id": "S1",
	"profiles": [{"toon": "Main", "avatar_id": "avatar_zerg_drone.jpg?f=1", "description": "", "private": false}],
	"avatars": {"a1": "url1"},
	"avatars_framed": {"a1": "url2"},
	"avatars_unlocked": {"a1": "url3"},
	"toon_guid_by_gateway": {"30": 5},
	"matchmaked_current_season": 22,
	"matchmaked_current_season_buckets": [0, 1163, 1397, 1561, 1742, 2028, 2244, 9999],
	"toons": [
		{"toon": "Main", "gateway_id": 30, "games_last_week": 4, "guid": 1},
		{"toon": "Smurf", "gateway_id": 10, "games_last_week": 1, "guid": 2}
	],
	"matchmaked_stats": [
		{"season_id": 21, "toon": "Main", "rating": 1653, "highest_rating": 1700, "wins": 4, "losses": 1, "disconnects": 0, "bucket": 4, "benefactor_id": "0"}
	],
	"stats": [
		{"season_id": 0, "gateway_id": 30, "toon": "Main", "benefactor_id": "0", "raw": {
			"zerg_wins_sum": 100, "zerg_losses_sum": 50, "zerg_draws_sum": 2, "zerg_disconnects_sum": 3,
			"zerg_apm_sum": 15500, "zerg_play_time_sum": 90000,
			"zerg_resources_minerals_sum": 999999, "zerg_units_score_sum": 888888,
			"terran_wins_sum": 0, "legacy_wins": 7, "zerg_apm_max": 400, "zerg_apm_min": 1
		}}
	],
	"game_results": [
		{
			"game_id": "111", "create_time": "1780000000", "gateway_id": 30, "match_guid": "",
			"attributes": {"mapName": "\u0007Fighting \u0005Spirit", "tileset": "4", "client_version": "1.23"},
			"players": [
				{"toon": "Main", "result": "win", "attributes": {"race": "zerg", "team": "1", "type": "player", "left": "0", "gPlayerData_idx": "0"}, "stats": {"zerg_apm": "119", "zerg_play_time": "455", "zerg_resources_minerals": "3386"}},
				{"toon": "Rival", "result": "loss", "attributes": {"race": "terran", "team": "2", "type": "player", "left": "1"}, "stats": {"terran_apm": "80", "terran_play_time": "400"}},
				{"toon": "Bot", "result": "", "attributes": {"race": "protoss", "team": "2", "type": "ai", "left": "0"}, "stats": {}},
				{"toon": "", "result": "", "attributes": {"type": "none"}, "stats": {}}
			]
		},
		{
			"game_id": "222", "create_time": "1780000600", "gateway_id": 30, "match_guid": "MM-abc-def",
			"attributes": {"mapName": "Polypoid", "tileset": "1"},
			"players": [
				{"toon": "Main", "result": "undecided", "attributes": {"race": "zerg", "team": "1", "type": "player", "left": "0"}, "stats": {"zerg_apm": "130", "zerg_play_time": "600"}}
			]
		}
	],
	"replays": [
		{
			"link": "111", "create_time": 1780000000, "md5": "deadbeef", "url": "https://storage.googleapis.com/starcraft-user-uploads-prod/S1-replays/1/2/deadbeef.replay",
			"attributes": {
				"game_name": "\u00043v3 \u0007BGH noobs only", "game_creator": "Rival", "game_type": "15", "game_sub_type": "3",
				"map_width": "128", "map_height": "128", "map_title": "Fighting Spirit", "map_era": "4",
				"replay_result": "1", "replay_map_number": "0", "game_speed": "6", "replay_max_players": "8",
				"replay_min_players": "1", "game_id": "37", "benefactor_id": "0", "game_save_id": "0",
				"replay_player_names": "Main\nRival", "replay_player_types": "1 1 0", "replay_humans": "2"
			}
		},
		{
			"link": "", "create_time": 4294967295,
			"attributes": {"game_type": "2"}
		}
	]
}`

func TestDistillBnetProfile(t *testing.T) {
	fetchedAt := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	profile, games := DistillBnetProfile("Main", 30, fetchedAt, []byte(distillFixture))

	if !profile.Found || profile.AuroraID != 1000 || profile.BattleTag != "Someone#123" || profile.CountryCode != "KR" {
		t.Fatalf("identity: %+v", profile)
	}
	if profile.AvatarID != "avatar_zerg_drone.jpg?f=1" {
		t.Fatalf("avatar: %q", profile.AvatarID)
	}
	if len(profile.Toons) != 2 || profile.Toons[0].Toon != "Main" || profile.Toons[0].Gateway != 30 || profile.Toons[0].GamesLastWeek != 4 {
		t.Fatalf("toons: %+v", profile.Toons)
	}
	if len(profile.Ladder) != 1 {
		t.Fatalf("ladder: %+v", profile.Ladder)
	}
	ladder := profile.Ladder[0]
	if ladder.Season != 21 || ladder.Rating != 1653 || ladder.HighestRating != 1700 || ladder.Wins != 4 || ladder.Losses != 1 || ladder.Bucket != 4 {
		t.Fatalf("ladder row: %+v", ladder)
	}
	if len(profile.Lifetime) != 1 {
		t.Fatalf("lifetime: %+v", profile.Lifetime)
	}
	life := profile.Lifetime[0]
	if life.Season != 0 || life.Gateway != 30 {
		t.Fatalf("lifetime row: %+v", life)
	}
	zerg := life.Race["zerg"]
	if zerg.Wins != 100 || zerg.Losses != 50 || zerg.Draws != 2 || zerg.Disconnects != 3 || zerg.APMSum != 15500 || zerg.PlayTimeSec != 90000 {
		t.Fatalf("zerg totals: %+v", zerg)
	}
	if _, ok := life.Race["terran"]; ok {
		t.Fatal("an all-zero race must be dropped")
	}

	// Nothing dropped may survive into the serialized record.
	encoded, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"url1", "url2", "url3", "resources", "units_score", "legacy", "apm_max", "1163"} {
		if strings.Contains(string(encoded), banned) {
			t.Errorf("dropped data %q survived: %s", banned, encoded)
		}
	}

	if len(games) != 2 {
		t.Fatalf("games: %d", len(games))
	}
	byID := map[string]BnetGame{}
	for _, g := range games {
		byID[g.GameID] = g
	}
	g1 := byID["111"]
	if g1.CreateTime.Unix() != 1780000000 || g1.Gateway != 30 || g1.Ladder {
		t.Fatalf("g1 header: %+v", g1)
	}
	if g1.MapName != "Fighting Spirit" {
		t.Fatalf("control chars must be stripped from the map name: %q", g1.MapName)
	}
	if g1.GameName != "3v3 BGH noobs only" {
		t.Fatalf("control chars must be stripped from the lobby name: %q", g1.GameName)
	}
	if g1.Host != "Rival" || g1.Type != 15 || g1.SubType != 3 || g1.MapWidth != 128 || g1.MapHeight != 128 || g1.TileSet != 4 {
		t.Fatalf("g1 replay attributes: %+v", g1)
	}
	if g1.MD5 != "deadbeef" || g1.URL == "" {
		t.Fatalf("own replay refs must be kept: %+v", g1)
	}
	if len(g1.Accounts) != 1 || g1.Accounts[0] != 1000 {
		t.Fatalf("accounts: %+v", g1.Accounts)
	}
	if len(g1.Players) != 3 {
		t.Fatalf("the type=none slot must be dropped: %+v", g1.Players)
	}
	for _, p := range g1.Players {
		switch p.Toon {
		case "Main":
			// replay_result "1" (win) beats the own row's string; both agree here.
			if p.Result != BnetResultWin || p.Race != "zerg" || p.Team != 1 || p.APM != 119 || p.Seconds != 455 || p.Computer || p.Left {
				t.Fatalf("own player: %+v", p)
			}
		case "Rival":
			// Provenance: another player's result is unreliable and stays unknown.
			if p.Result != BnetResultUnknown {
				t.Fatalf("rival result must stay unknown: %+v", p)
			}
			if !p.Left || p.Computer || p.APM != 80 {
				t.Fatalf("rival: %+v", p)
			}
		case "Bot":
			// The bridge's type field, not the replay_player_types bitfield,
			// decides who is a computer: in replay_player_types 1 means human,
			// the inverse of screp's PlayerTypes enum.
			if !p.Computer {
				t.Fatalf("ai slot must be a computer: %+v", p)
			}
		default:
			t.Fatalf("unexpected player %+v", p)
		}
	}
	g2 := byID["222"]
	if !g2.Ladder {
		t.Fatal("match_guid must set the ladder flag")
	}
	for _, p := range g2.Players {
		if p.Toon == "Main" && p.Result != BnetResultUnknown {
			t.Fatalf("an undecided own result must stay unknown: %+v", p)
		}
	}
	// The sentinel-timestamped, linkless replay entry must not invent a game.
	for _, g := range games {
		if g.CreateTime.Year() >= 2100 {
			t.Fatalf("the 0xFFFFFFFF create_time sentinel leaked into a timestamp: %+v", g)
		}
	}
}

func TestDistillBnetProfile_NotFoundAndUndecodable(t *testing.T) {
	p, games := DistillBnetProfile("Ghost", 20, time.Now(), []byte(`{"aurora_id": 0, "toons": [], "replays": [], "game_results": []}`))
	if p.Found || len(games) != 0 {
		t.Fatalf("unknown toon: %+v %d games", p, len(games))
	}
	p, games = DistillBnetProfile("Ghost", 20, time.Now(), []byte("not json"))
	if p.Found || len(games) != 0 || p.Toon != "Ghost" || p.Gateway != 20 {
		t.Fatalf("undecodable payload: %+v %d games", p, len(games))
	}
}

func TestDistillBnetProfile_ReplayResultDecoding(t *testing.T) {
	template := `{
		"aurora_id": 5, "toons": [{"toon": "Me", "gateway_id": 30}],
		"game_results": [{"game_id": "1", "create_time": "1780000000", "players": [
			{"toon": "Me", "result": "", "attributes": {"race": "zerg", "type": "player"}, "stats": {}}
		]}],
		"replays": [{"link": "1", "create_time": 1780000000, "attributes": {%s}}]
	}`
	cases := []struct {
		attr string
		want int
	}{
		{`"replay_result": "1"`, BnetResultWin},
		{`"replay_result": "2"`, BnetResultLoss},
		{`"replay_result": "3"`, BnetResultDraw},
		{`"replay_result": "4"`, BnetResultDisconnect},
		{`"replay_result": "0"`, BnetResultUnknown},
		{``, BnetResultUnknown},
	}
	for _, c := range cases {
		_, games := DistillBnetProfile("Me", 30, time.Now(), []byte(fmt.Sprintf(template, c.attr)))
		if len(games) != 1 || len(games[0].Players) != 1 {
			t.Fatalf("%s: games %+v", c.attr, games)
		}
		if got := games[0].Players[0].Result; got != c.want {
			t.Errorf("%s: result = %d, want %d", c.attr, got, c.want)
		}
	}
}

func TestDistillBnetProfile_LinklessReplayJoinsByCreateTime(t *testing.T) {
	payload := `{
		"aurora_id": 5, "toons": [{"toon": "Me", "gateway_id": 30}],
		"game_results": [{"game_id": "777", "create_time": "1780000010", "players": []}],
		"replays": [{"link": "", "create_time": 1780000000, "md5": "abc", "url": "https://x", "attributes": {"game_type": "2"}}]
	}`
	_, games := DistillBnetProfile("Me", 30, time.Now(), []byte(payload))
	if len(games) != 1 || games[0].MD5 != "abc" || games[0].Type != 2 {
		t.Fatalf("the linkless replay must join by create time: %+v", games)
	}
}

func TestBnetArchiveProvenanceAcrossProfiles(t *testing.T) {
	root := t.TempDir()
	archive := NewBnetGameArchive(root)
	gameJSON := `{
		"aurora_id": %d, "toons": [{"toon": "%s", "gateway_id": 30}],
		"game_results": [{"game_id": "9", "create_time": "1780000000", "players": [
			{"toon": "A", "result": "win", "attributes": {"race": "zerg", "type": "player"}, "stats": {}},
			{"toon": "B", "result": "win", "attributes": {"race": "terran", "type": "player"}, "stats": {}}
		]}],
		"replays": []
	}`
	_, aGames := DistillBnetProfile("A", 30, time.Now(), []byte(fmt.Sprintf(gameJSON, 1, "A")))
	if err := archive.Upsert(aGames); err != nil {
		t.Fatal(err)
	}
	players := archive.GamesForAccount(1)[0].Players
	for _, p := range players {
		if p.Toon == "B" && p.Result != BnetResultUnknown {
			t.Fatalf("B's result from A's profile must stay unknown: %+v", p)
		}
		if p.Toon == "A" && p.Result != BnetResultWin {
			t.Fatalf("A's own result must be kept: %+v", p)
		}
	}
	// B's own profile then fills B's result.
	_, bGames := DistillBnetProfile("B", 30, time.Now(), []byte(fmt.Sprintf(gameJSON, 2, "B")))
	if err := archive.Upsert(bGames); err != nil {
		t.Fatal(err)
	}
	for _, p := range archive.GamesForAccount(1)[0].Players {
		if p.Toon == "B" && p.Result != BnetResultWin {
			t.Fatalf("B's result must be filled by B's own profile: %+v", p)
		}
	}
}

func TestMigrateLegacyBnetCaches(t *testing.T) {
	root := t.TempDir()

	profileDir := filepath.Join(root, legacyBnetProfilesDirName, "30")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fetchedAt := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	entry := legacyBnetProfileEntry{
		Toon: "Main", Gateway: 30, Found: true, AuroraID: 1000, BattleTag: "Someone#123",
		CountryCode: "KR", FetchedAt: fetchedAt, Payload: distillFixture,
	}
	raw, _ := json.Marshal(entry)
	if err := os.WriteFile(filepath.Join(profileDir, "aaaa.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	notFound := legacyBnetProfileEntry{Toon: "Ghost", Gateway: 20, Found: false, FetchedAt: fetchedAt, Payload: `{"aurora_id": 0}`}
	raw, _ = json.Marshal(notFound)
	if err := os.WriteFile(filepath.Join(profileDir, "bbbb.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "garbage.json"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}

	resultsDir := filepath.Join(root, legacyBnetGameResultsDirName)
	if err := os.MkdirAll(resultsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyGames := legacyBnetGameResultsFile{AuroraID: 1000, Games: map[string]legacyBnetGameResult{
		"555": {
			AuroraID: 1000, GameID: "555", CreateTime: fetchedAt.Add(-time.Hour), Toon: "Main",
			Gateway: 30, Race: "Zerg", Result: "loss", APM: 140, DurationSeconds: 700,
			MapName: "Polypoid", MatchGUID: "MM-1",
		},
	}}
	raw, _ = json.Marshal(legacyGames)
	if err := os.WriteFile(filepath.Join(resultsDir, "1000.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	cache := NewBnetCache(root)
	archive := NewBnetGameArchive(root)
	profiles, games, err := MigrateLegacyBnetCaches(root, cache, archive)
	if err != nil {
		t.Fatal(err)
	}
	if profiles != 2 {
		t.Fatalf("profiles migrated = %d, want 2", profiles)
	}
	// 2 games out of the profile payload + 1 out of the game-results file.
	if games != 3 {
		t.Fatalf("games migrated = %d, want 3", games)
	}

	main := cache.Get("Main", 30)
	if main == nil || !main.Found || main.AuroraID != 1000 || !main.FetchedAt.Equal(fetchedAt) || len(main.Ladder) != 1 {
		t.Fatalf("migrated profile: %+v", main)
	}
	ghost := cache.Get("Ghost", 20)
	if ghost == nil || ghost.Found {
		t.Fatalf("the negative entry must survive migration: %+v", ghost)
	}
	archived := archive.GamesForAccount(1000)
	if len(archived) != 3 {
		t.Fatalf("archived games = %d, want 3", len(archived))
	}
	byID := map[string]BnetGame{}
	for _, g := range archived {
		byID[g.GameID] = g
	}
	legacyGame := byID["555"]
	if !legacyGame.Ladder || legacyGame.MapName != "Polypoid" || len(legacyGame.Players) != 1 {
		t.Fatalf("legacy game: %+v", legacyGame)
	}
	if p := legacyGame.Players[0]; p.Race != "zerg" || p.Result != BnetResultLoss || p.APM != 140 || p.Seconds != 700 {
		t.Fatalf("legacy game player: %+v", p)
	}

	for _, dir := range []string{filepath.Join(root, legacyBnetProfilesDirName), resultsDir} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("legacy directory %s must be deleted after migration", dir)
		}
	}

	// The stores must persist what migration wrote.
	reloadedCache := NewBnetCache(root)
	reloadedArchive := NewBnetGameArchive(root)
	if err := reloadedCache.Load(); err != nil || reloadedCache.Len() != 2 {
		t.Fatalf("reloaded cache len=%d err=%v", reloadedCache.Len(), err)
	}
	if err := reloadedArchive.Load(); err != nil || reloadedArchive.Len() != 3 {
		t.Fatalf("reloaded archive len=%d err=%v", reloadedArchive.Len(), err)
	}

	// Running again is a no-op, not an error.
	profiles, games, err = MigrateLegacyBnetCaches(root, cache, archive)
	if err != nil || profiles != 0 || games != 0 {
		t.Fatalf("second run: %d %d %v", profiles, games, err)
	}
}

func TestMigrateLegacyBnetCachesPrefersNewerV2Entry(t *testing.T) {
	root := t.TempDir()
	profileDir := filepath.Join(root, legacyBnetProfilesDirName, "30")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := legacyBnetProfileEntry{Toon: "Main", Gateway: 30, Found: true, AuroraID: 1, FetchedAt: time.Now().Add(-48 * time.Hour), Payload: `{"aurora_id": 1}`}
	raw, _ := json.Marshal(old)
	if err := os.WriteFile(filepath.Join(profileDir, "aaaa.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	cache := NewBnetCache(root)
	if err := cache.Upsert(BnetProfile{Toon: "Main", Gateway: 30, Found: true, AuroraID: 2, FetchedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := MigrateLegacyBnetCaches(root, cache, NewBnetGameArchive(root)); err != nil {
		t.Fatal(err)
	}
	if got := cache.Get("Main", 30); got.AuroraID != 2 {
		t.Fatalf("a newer v2 entry must win over the legacy file: %+v", got)
	}
}
