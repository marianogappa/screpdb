package compact

import (
	"crypto/sha256"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/parser"
	"github.com/marianogappa/screpdb/internal/patterns"
	"github.com/marianogappa/screpdb/internal/patterns/markers"
)

func corpusPaths(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "replays", "*.rep"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no corpus replays found: %v", err)
	}
	sort.Strings(paths)
	return paths
}

func parseCorpusReplay(t *testing.T, path string) *models.ReplayData {
	t.Helper()
	data, err := parser.ParseReplay(path, &models.Replay{FilePath: path, FileName: filepath.Base(path)})
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return data
}

func fileMetaFor(t *testing.T, path string) FileMeta {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return FileMeta{Path: path, Size: info.Size(), ModTime: info.ModTime(), Checksum: sha256.Sum256(raw)}
}

func TestFromReplayDataCorpus(t *testing.T) {
	layoutsByMap := map[string]*library.MapLayout{}
	for _, path := range corpusPaths(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			reference := parseCorpusReplay(t, path)
			refOrch := reference.PatternOrchestrator.(*patterns.Orchestrator)
			refResults := refOrch.GetResults()
			refEvents := refOrch.ReplayEvents()

			data := parseCorpusReplay(t, path)
			meta := fileMetaFor(t, path)
			r, err := FromReplayData(data, meta)
			if err != nil {
				t.Fatal(err)
			}

			if r.ID != library.ReplayIDFromChecksum(meta.Checksum) || r.Checksum != meta.Checksum {
				t.Fatal("id must derive from the file checksum")
			}
			if r.Path() != path || r.FileName() != filepath.Base(path) || r.Paths[0].Size != meta.Size {
				t.Fatalf("file meta not carried: %+v", r.Paths)
			}
			if r.Duration != library.ClampU16(data.Replay.DurationSeconds) || !r.Date.Equal(data.Replay.ReplayDate) {
				t.Fatal("duration or date mismatch")
			}
			if library.Strings.Name(r.Map) != data.Replay.MapName || library.Strings.Name(r.GameType) != data.Replay.GameType {
				t.Fatal("interned meta mismatch")
			}
			if r.MapKind != library.ParseMapKind(data.Replay.MapKind) || r.MapKind == library.MapKindUnknown {
				t.Fatalf("map kind %q", data.Replay.MapKind)
			}

			if len(r.Players) != len(data.Players) {
				t.Fatalf("players: got %d want %d", len(r.Players), len(data.Players))
			}
			for i, p := range data.Players {
				lp := r.Players[i]
				if lp.ReplayPlayerID != p.PlayerID || lp.Name != p.Name || lp.Key != strings.ToLower(strings.TrimSpace(p.Name)) {
					t.Fatalf("ordinal %d: %+v vs %+v", i, lp, p)
				}
				if lp.IsObserver() != p.IsObserver || lp.IsWinner() != p.IsWinner || lp.Team != p.Team {
					t.Fatalf("ordinal %d flags mismatch", i)
				}
				if lp.Race != library.ParseRace(p.Race) || lp.Type != library.ParsePlayerType(p.Type) || lp.Type == library.PlayerTypeUnknown {
					t.Fatalf("ordinal %d race/type mismatch (%q, %q)", i, p.Race, p.Type)
				}
				if int(lp.APM) != p.APM || int(lp.EAPM) != p.EAPM {
					t.Fatalf("ordinal %d apm mismatch", i)
				}
				if blob, ok := data.HotkeyStreams[p.PlayerID]; ok && len(blob) > 0 && p.PlayerID != 255 && len(lp.HotkeyStream) == 0 {
					t.Fatalf("ordinal %d lost its hotkey stream", i)
				}
			}
			for _, fp := range data.FingerprintVectors {
				found := false
				for i := range r.Players {
					if r.Players[i].ReplayPlayerID == fp.PlayerID && r.Players[i].Fingerprint != nil && len(r.Players[i].Fingerprint.Vector) == len(fp.Vector) {
						found = true
					}
				}
				if !found {
					t.Fatalf("fingerprint for replay player %d not attached", fp.PlayerID)
				}
			}

			wantMarkers := 0
			for _, res := range refResults {
				if markers.ByPatternName(res.PatternName) != nil {
					wantMarkers++
				}
			}
			if len(r.Markers) != wantMarkers {
				t.Fatalf("markers: got %d want %d (of %d results)", len(r.Markers), wantMarkers, len(refResults))
			}
			if wantMarkers == 0 {
				t.Fatal("expected at least one marker in a corpus replay")
			}
			for _, m := range r.Markers {
				if m.Feature == 0 {
					t.Fatal("marker without feature")
				}
				if m.Player != library.NoPlayer && int(m.Player) >= len(r.Players) {
					t.Fatalf("marker ordinal %d out of range", m.Player)
				}
				switch library.Features.Name(m.Feature) {
				case "bo_z_fuzzy":
					if m.Label == 0 || !strings.HasPrefix(library.Strings.Name(m.Label), "~") {
						t.Fatalf("fuzzy label not decoded: %s", m.Payload)
					}
				case "viewport_multitasking":
					p := r.Players[m.Player]
					if !p.Flags.Has(library.PlayerHasViewport) || p.Viewport <= 0 {
						t.Fatalf("viewport not decoded for %s: %s", p.Name, m.Payload)
					}
				}
			}

			if len(r.Events) != len(refEvents) {
				t.Fatalf("events: got %d want %d", len(r.Events), len(refEvents))
			}
			for i, ev := range r.Events {
				if library.EventTypes.Name(ev.Type) != refEvents[i].EventType || int(ev.Sec) != refEvents[i].Second {
					t.Fatalf("event %d mismatch", i)
				}
				if len(ev.AttackUnits()) != len(refEvents[i].AttackUnitTypes) || len(ev.CastCounts()) != len(refEvents[i].AttackCastCounts) {
					t.Fatalf("event %d detail mismatch", i)
				}
				if refEvents[i].SourceReplayPlayerID != nil && ev.Source == library.NoPlayer {
					t.Fatalf("event %d lost its source", i)
				}
			}

			if r.Prod.Len() == 0 {
				t.Fatal("expected production events")
			}
			for i := 0; i < r.Prod.Len(); i++ {
				switch r.Prod.Kind[i] {
				case library.ProdTrain, library.ProdUnitMorph, library.ProdBuildingMorph, library.ProdBuild,
					library.ProdTech, library.ProdUpgrade, library.ProdCast:
				default:
					t.Fatalf("prod %d has kind %d", i, r.Prod.Kind[i])
				}
				if i > 0 && r.Prod.Sec[i] < r.Prod.Sec[i-1] {
					t.Fatalf("prod not sorted at %d", i)
				}
				if int(r.Prod.Player[i]) >= len(r.Players) || r.Prod.Subject[i] == 0 || r.Prod.Count[i] == 0 {
					t.Fatalf("prod %d malformed", i)
				}
			}
			wantProd, wantChat := 0, 0
			left := map[byte]bool{}
			for _, cmd := range reference.Commands {
				if cmd.Player == nil {
					continue
				}
				switch cmd.ActionType {
				case "Train", "Unit Morph", "Building Morph", "Build":
					if cmd.UnitType != nil && strings.TrimSpace(*cmd.UnitType) != "" {
						wantProd++
					}
				case "Tech":
					if cmd.TechName != nil {
						wantProd++
					}
				case "Upgrade":
					if cmd.UpgradeName != nil {
						wantProd++
					}
				case "Chat":
					if cmd.ChatMessage != nil {
						wantChat++
					}
				case "Leave Game":
					left[cmd.Player.PlayerID] = true
				}
			}
			casts := 0
			for i := 0; i < r.Prod.Len(); i++ {
				if r.Prod.Kind[i] == library.ProdCast {
					casts++
				}
			}
			if r.Prod.Len()-casts != wantProd {
				t.Fatalf("prod events: got %d (+%d casts) want %d", r.Prod.Len()-casts, casts, wantProd)
			}
			if len(r.Chat) != wantChat {
				t.Fatalf("chat: got %d want %d", len(r.Chat), wantChat)
			}
			for _, p := range r.Players {
				if p.Flags.Has(library.PlayerLeft) != left[p.ReplayPlayerID] {
					t.Fatalf("leave flag for %s: got %v", p.Name, p.Flags.Has(library.PlayerLeft))
				}
				if p.Flags.Has(library.PlayerLeft) && library.Strings.Name(p.LeaveReason) == "" {
					t.Fatalf("leave reason missing for %s", p.Name)
				}
			}

			if (r.Alliance != nil) != (len(data.AllianceSnapshots) > 0) {
				t.Fatal("alliance timeline presence mismatch")
			}
			if r.Alliance != nil {
				for _, snap := range r.Alliance.Snapshots {
					for _, team := range snap.Teams {
						for _, ord := range team {
							if int(ord) >= len(r.Players) {
								t.Fatalf("alliance ordinal %d out of range", ord)
							}
						}
					}
				}
			}

			if r.Layout == nil {
				t.Fatal("expected a map layout")
			}
			again, err := FromReplayData(parseCorpusReplay(t, path), meta)
			if err != nil {
				t.Fatal(err)
			}
			if again.Layout != r.Layout {
				t.Fatal("layout must be interned across compactions of the same map")
			}
			if prev, ok := layoutsByMap[data.Replay.MapName]; ok && prev != r.Layout {
				t.Fatalf("replays on %q do not share a layout pointer", data.Replay.MapName)
			}
			layoutsByMap[data.Replay.MapName] = r.Layout

			for i := range r.Players {
				p := &r.Players[i]
				want := referenceCadence(reference, data.Players[i].PlayerID, data.Replay.DurationSeconds)
				if want == nil {
					if p.Cadence != nil {
						t.Fatalf("unexpected cadence for %s", p.Name)
					}
					continue
				}
				if p.Cadence == nil {
					t.Fatalf("missing cadence for %s", p.Name)
				}
				assertCadence(t, p.Name, p.Cadence, want)
			}
		})
	}
	if len(layoutsByMap) == 0 {
		t.Fatal("no layouts seen")
	}
}

// referenceCadence recomputes the cadence CTE straight from the raw command
// stream, independently of the compact record.
func referenceCadence(data *models.ReplayData, pid byte, duration int) *library.Cadence {
	var player *models.Player
	for _, p := range data.Players {
		if p.PlayerID == pid {
			player = p
			break
		}
	}
	if player == nil || player.IsObserver || strings.ToLower(strings.TrimSpace(player.Type)) != "human" {
		return nil
	}
	excluded := map[string]bool{"SCV": true, "Probe": true, "Drone": true, "Overlord": true, "Observer": true, "Shuttle": true,
		"Science Vessel": true, "Medic": true, "Dropship": true, "Defiler": true, "Queen": true, "Nuclear Missile": true}
	end := int(0.8 * float64(duration))
	if end <= 420 {
		return nil
	}
	var times []int
	for _, cmd := range data.Commands {
		if cmd.Player != player || (cmd.ActionType != "Train" && cmd.ActionType != "Unit Morph") || cmd.UnitType == nil {
			continue
		}
		if strings.TrimSpace(*cmd.UnitType) == "" || excluded[*cmd.UnitType] || cmd.SecondsFromGameStart < 420 || cmd.SecondsFromGameStart > end {
			continue
		}
		times = append(times, cmd.SecondsFromGameStart)
	}
	if len(times) == 0 {
		return nil
	}
	sort.Ints(times)
	window := end - 420
	c := &library.Cadence{WindowSec: uint16(window), Units: uint16(len(times)), Gaps: uint16(len(times) - 1)}
	c.RatePerMinute = float64(len(times)) * 60 / float64(window)
	if len(times) == 1 {
		c.Score = c.RatePerMinute / 10000
		return c
	}
	var sum, sq float64
	idle := 0
	for i := 1; i < len(times); i++ {
		g := float64(times[i] - times[i-1])
		sum += g
		sq += g * g
		if g >= 20 {
			idle++
		}
	}
	n := float64(len(times) - 1)
	mean := sum / n
	variance := sq/n - mean*mean
	c.Idle20Ratio = float64(idle) / n
	if variance < 0 || mean == 0 {
		c.Score = c.RatePerMinute / 10000
		return c
	}
	cv := math.Sqrt(variance) / mean
	c.CVGap = cv
	c.Burstiness = (cv - 1) / (cv + 1)
	c.Score = c.RatePerMinute / (1 + cv)
	return c
}

func assertCadence(t *testing.T, name string, got, want *library.Cadence) {
	t.Helper()
	if got.WindowSec != want.WindowSec || got.Units != want.Units || got.Gaps != want.Gaps {
		t.Fatalf("%s counts: got %+v want %+v", name, got, want)
	}
	for label, pair := range map[string][2]float64{
		"rate": {got.RatePerMinute, want.RatePerMinute}, "cv": {got.CVGap, want.CVGap},
		"burstiness": {got.Burstiness, want.Burstiness}, "idle": {got.Idle20Ratio, want.Idle20Ratio}, "score": {got.Score, want.Score},
	} {
		if math.Abs(pair[0]-pair[1]) > 1e-9 {
			t.Fatalf("%s %s: got %v want %v", name, label, pair[0], pair[1])
		}
	}
}

func strPtr(s string) *string { return &s }

func syntheticData() *models.ReplayData {
	human := &models.Player{PlayerID: 0, Name: " Flash ", Race: "Terran", Type: "Human", Team: 1, SlotID: 0, APM: 300, EAPM: 250}
	lone := &models.Player{PlayerID: 1, Name: "Solo", Race: "Zerg", Type: "Human", Team: 2, SlotID: 1}
	bot1 := &models.Player{PlayerID: 255, Name: "Bot A", Race: "Protoss", Type: "Computer", Team: 3, SlotID: 2}
	bot2 := &models.Player{PlayerID: 255, Name: "Bot B", Race: "Protoss", Type: "Computer", Team: 4, SlotID: 3}
	rep := &models.Replay{
		ReplayDate: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), DurationSeconds: 900, GameType: "Melee", MapKind: "Regular",
		MapName: "Synthetic", Host: "Flash", Title: "gg",
	}
	cmd := func(p *models.Player, sec int, action string, unit string) *models.Command {
		c := &models.Command{Player: p, SecondsFromGameStart: sec, ActionType: action}
		if unit != "" {
			c.UnitType = strPtr(unit)
		}
		return c
	}
	commands := []*models.Command{
		nil,
		cmd(nil, 10, "Train", "Marine"),
		cmd(&models.Player{PlayerID: 9}, 10, "Train", "Marine"),
		cmd(human, 300, "Train", "Marine"),
		cmd(human, 425, "Train", "SCV"),
		cmd(human, 420, "Train", "Marine"),
		cmd(human, 430, "Train", "Marine"),
		cmd(human, 450, "Unit Morph", "Zergling"),
		cmd(human, 490, "Train", "Marine"),
		cmd(human, 800, "Train", "Marine"),
		cmd(human, 100, "Build", "Barracks"),
		cmd(human, 50, "Targeted Order", ""),
		cmd(human, 600, "Targeted Order", ""),
		cmd(human, 700, "Tech", ""),
		cmd(human, 710, "Upgrade", ""),
		cmd(human, 20, "Chat", ""),
		cmd(human, 890, "Leave Game", ""),
		cmd(human, 895, "Leave Game", ""),
		cmd(lone, 500, "Train", "Hydralisk"),
		cmd(bot1, 200, "Train", "Zealot"),
		cmd(bot2, 210, "Train", "Dragoon"),
		cmd(bot2, 220, "Right Click", ""),
	}
	commands[11].OrderName = strPtr("CastScannerSweep")
	commands[12].OrderName = strPtr("CastPsionicStorm")
	commands[13].TechName = strPtr("Stim Packs")
	commands[14].UpgradeName = strPtr("Terran Infantry Weapons")
	commands[15].ChatMessage = strPtr("glhf")
	commands[16].LeaveReason = strPtr("Dropped")
	commands[17].LeaveReason = strPtr("Quit")
	morph := cmd(human, 460, "Unit Morph", "Mutalisk")
	morph.MorphUnitCount = 3
	commands = append(commands, morph)

	return &models.ReplayData{
		Replay:   rep,
		Players:  []*models.Player{human, lone, bot1, bot2},
		Commands: commands,
		HotkeyStreams: map[byte][]byte{
			0:   {1, 2, 3},
			255: {9},
		},
		FingerprintVectors: []models.PlayerFingerprintVector{
			{PlayerID: 0, Race: "Terran", Vector: []float64{1, 2}, FeatureVersion: 3, ModelTag: "m"},
			{PlayerID: 255, Race: "Protoss", Vector: []float64{5}},
			{PlayerID: 255, Race: "Protoss", Vector: []float64{6}},
		},
		AllianceSnapshots: []models.AllianceSnapshot{
			{Sec: 0, Teams: [][]byte{{0}, {1}, {255}}},
			{Sec: 400, Teams: [][]byte{{0, 1}, {255}, {77}}, Stacking: true},
		},
		MapContext:          &models.ReplayMapContext{Layout: &models.MapContextLayout{WidthTiles: 128, HeightTiles: 128, Bases: []models.MapContextBase{{Name: "Main", Clock: 11, Polygon: []models.MapPolygonPoint{{X: 1, Y: 2}}}}}},
		PatternOrchestrator: "not an orchestrator",
	}
}

func TestFromReplayDataSynthetic(t *testing.T) {
	meta := FileMeta{Path: "/sc/Maps/Replays/AutoSave/x.rep", Size: 1, ModTime: time.Unix(1, 0), Checksum: sha256.Sum256([]byte("x"))}
	r, err := FromReplayData(syntheticData(), meta)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Flags.Has(library.FlagIsAutosave) || !r.Flags.Has(library.FlagHasComputer) || r.Flags.Has(library.FlagIsOneOnOne) {
		t.Fatalf("flags %b", r.Flags)
	}
	if r.Players[0].Key != "flash" || r.Players[0].APM != 300 || library.Strings.Name(r.Host) != "Flash" || r.Title != "gg" {
		t.Fatal("player/replay meta")
	}
	if len(r.Players[0].HotkeyStream) != 3 || len(r.Players[2].HotkeyStream) != 1 || r.Players[3].HotkeyStream != nil {
		t.Fatal("hotkey streams: shared computer id must go to the first computer only")
	}
	if r.Players[0].Fingerprint == nil || r.Players[0].Fingerprint.FeatureVersion != 3 ||
		r.Players[2].Fingerprint == nil || r.Players[2].Fingerprint.Vector[0] != 5 ||
		r.Players[3].Fingerprint == nil || r.Players[3].Fingerprint.Vector[0] != 6 || r.Players[1].Fingerprint != nil {
		t.Fatal("fingerprints must fill shared-id computers in slot order")
	}

	type prod struct {
		sec   int
		ord   uint8
		kind  library.ProdKind
		name  string
		count uint8
	}
	var got []prod
	for i := 0; i < r.Prod.Len(); i++ {
		got = append(got, prod{int(r.Prod.Sec[i]), r.Prod.Player[i], r.Prod.Kind[i], r.Prod.SubjectName(i), r.Prod.Count[i]})
	}
	want := []prod{
		{100, 0, library.ProdBuild, "Barracks", 1},
		{200, 2, library.ProdTrain, "Zealot", 1},
		{210, 3, library.ProdTrain, "Dragoon", 1},
		{300, 0, library.ProdTrain, "Marine", 1},
		{420, 0, library.ProdTrain, "Marine", 1},
		{425, 0, library.ProdTrain, "SCV", 1},
		{430, 0, library.ProdTrain, "Marine", 1},
		{450, 0, library.ProdUnitMorph, "Zergling", 1},
		{460, 0, library.ProdUnitMorph, "Mutalisk", 3},
		{490, 0, library.ProdTrain, "Marine", 1},
		{500, 1, library.ProdTrain, "Hydralisk", 1},
		{600, 0, library.ProdCast, "CastPsionicStorm", 1},
		{700, 0, library.ProdTech, "Stim Packs", 1},
		{710, 0, library.ProdUpgrade, "Terran Infantry Weapons", 1},
		{800, 0, library.ProdTrain, "Marine", 1},
	}
	if len(got) != len(want) {
		t.Fatalf("prod: got %d events %+v", len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("prod %d: got %+v want %+v", i, got[i], want[i])
		}
	}
	if cap(r.Prod.Sec) > len(r.Prod.Sec)+8 {
		t.Fatal("prod columns must be clipped")
	}

	if len(r.Chat) != 1 || r.Chat[0].Text != "glhf" || r.Chat[0].Player != 0 || r.Chat[0].Sec != 20 {
		t.Fatalf("chat %+v", r.Chat)
	}
	p := r.Players[0]
	if !p.Flags.Has(library.PlayerLeft) || !p.Flags.Has(library.PlayerDropped) || p.LeaveSec != 890 || library.Strings.Name(p.LeaveReason) != "Dropped" {
		t.Fatalf("leave: %+v", p)
	}
	if r.Players[1].Flags.Has(library.PlayerLeft) {
		t.Fatal("player without a leave command flagged")
	}

	if len(r.Markers) != 0 || len(r.Events) != 0 {
		t.Fatal("a non-orchestrator must yield no markers or events")
	}

	if r.Alliance == nil || len(r.Alliance.Snapshots) != 2 {
		t.Fatalf("alliance %+v", r.Alliance)
	}
	second := r.Alliance.Snapshots[1]
	if second.Sec != 400 || !second.Stacking || len(second.Teams) != 2 || second.Teams[1][0] != 2 {
		t.Fatalf("alliance snapshot %+v: unknown ids must be dropped and 255 must map to the first computer", second)
	}

	if r.Layout == nil || r.Layout.WidthTiles != 128 || len(r.Layout.Bases) != 1 || r.Layout.Bases[0].Clock != 11 {
		t.Fatalf("layout %+v", r.Layout)
	}
	again, _ := FromReplayData(syntheticData(), meta)
	if again.Layout != r.Layout {
		t.Fatal("identical layouts must intern to one pointer")
	}

	// Window [420, 720]: Marines at 420, 430, 490 and morphs at 450, 460
	// count; the SCV is excluded, 300 and 800 fall outside. Gaps 10, 20, 10, 30.
	c := r.Players[0].Cadence
	if c == nil {
		t.Fatal("expected cadence for the human player")
	}
	gaps := []float64{10, 20, 10, 30}
	mean := 17.5
	variance := (100 + 400 + 100 + 900) / 4.0
	variance -= mean * mean
	cv := math.Sqrt(variance) / mean
	rate := 5 * 60.0 / 300
	wantCadence := &library.Cadence{WindowSec: 300, Units: 5, Gaps: uint16(len(gaps)), RatePerMinute: rate, CVGap: cv,
		Burstiness: (cv - 1) / (cv + 1), Idle20Ratio: 0.5, Score: rate / (1 + cv)}
	assertCadence(t, "hand", c, wantCadence)
	if math.Abs(c.Score-0.6785) > 0.001 || math.Abs(c.CVGap-0.4738) > 0.001 || c.RatePerMinute != 1 {
		t.Fatalf("hand-computed check: score %.4f cv %.4f", c.Score, c.CVGap)
	}

	lone := r.Players[1].Cadence
	if lone == nil || lone.Units != 1 || lone.Gaps != 0 || lone.CVGap != 0 || lone.Burstiness != 0 ||
		math.Abs(lone.Score-(60.0/300)/10000) > 1e-12 {
		t.Fatalf("single-unit cadence must use the 9999 fallback: %+v", lone)
	}
	if r.Players[2].Cadence != nil || r.Players[3].Cadence != nil {
		t.Fatal("computers have no cadence")
	}
}

func TestCadenceEdgeCases(t *testing.T) {
	short := &library.Replay{Duration: 500}
	short.Prod.Append(430, 0, library.ProdTrain, library.Units.Intern("Marine"), 1)
	if cadenceFor(short, 0) != nil {
		t.Fatal("a window that ends before it starts has no cadence")
	}
	even := &library.Replay{Duration: 900}
	for _, sec := range []uint16{420, 440, 460, 480} {
		even.Prod.Append(sec, 0, library.ProdTrain, library.Units.Intern("Marine"), 1)
	}
	c := cadenceFor(even, 0)
	if c == nil || c.CVGap != 0 || c.Burstiness != -1 || c.Idle20Ratio != 1 || math.Abs(c.Score-c.RatePerMinute) > 1e-12 {
		t.Fatalf("zero-variance gaps must yield cv 0 and score = rate: %+v", c)
	}
	if cadenceFor(even, 1) != nil {
		t.Fatal("a player with no production has no cadence")
	}
}

func TestFromReplayDataRejectsMalformedInput(t *testing.T) {
	meta := FileMeta{Path: "/r/x.rep"}
	if _, err := FromReplayData(nil, meta); err == nil {
		t.Fatal("nil data")
	}
	if _, err := FromReplayData(&models.ReplayData{}, meta); err == nil {
		t.Fatal("nil replay")
	}
	if _, err := FromReplayData(&models.ReplayData{Replay: &models.Replay{}}, FileMeta{}); err == nil {
		t.Fatal("empty path")
	}
	if _, err := FromReplayData(&models.ReplayData{Replay: &models.Replay{}, Players: []*models.Player{nil}}, meta); err == nil {
		t.Fatal("nil player")
	}
	tooMany := make([]*models.Player, library.MaxPlayersPerReplay+1)
	for i := range tooMany {
		tooMany[i] = &models.Player{PlayerID: byte(i)}
	}
	if _, err := FromReplayData(&models.ReplayData{Replay: &models.Replay{}, Players: tooMany}, meta); err == nil {
		t.Fatal("too many players")
	}
	r, err := FromReplayData(&models.ReplayData{Replay: &models.Replay{}}, meta)
	if err != nil || r.Prod.Len() != 0 || len(r.Players) != 0 || r.Layout != nil || r.Alliance != nil {
		t.Fatalf("an empty replay must compact to an empty record: %v", err)
	}
}
