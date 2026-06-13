package unittags

import (
	"path/filepath"
	"testing"

	"github.com/icza/screp/rep"
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/screp"
)

func TestMatchTagsToBuilds_GreedyEarliestFirst(t *testing.T) {
	// Two producing tags; the earlier producer claims the earlier build.
	firstSec := map[uint16]int{
		0xA: 100, // produced first
		0xB: 60,  // produced even earlier
	}
	builds := []Build{
		{Sec: 50, X: 10, Y: 10},  // earliest build
		{Sec: 80, X: 20, Y: 20},  // later build
		{Sec: 200, X: 30, Y: 30}, // built after both first-events: claimable by neither
	}
	got := matchTagsToBuilds(firstSec, builds)
	if got[0xB] != (Build{Sec: 50, X: 10, Y: 10}) {
		t.Errorf("tag B (earliest producer) should claim earliest build, got %+v", got[0xB])
	}
	if got[0xA] != (Build{Sec: 80, X: 20, Y: 20}) {
		t.Errorf("tag A should claim the next build placed before its first event, got %+v", got[0xA])
	}
	if _, ok := got[0xA]; len(got) != 2 || !ok {
		t.Errorf("only the two tags should match; the sec=200 build precedes no first-event")
	}
}

func TestResolveTagLocations_TownHallFallsBackToStart(t *testing.T) {
	a := newPlayerAcc()
	// One Hatchery build (an expansion) and two producing town-hall tags: the
	// frame-0 hatch (no build, earliest) and the expansion.
	a.builds[models.GeneralUnitHatchery] = []Build{{Sec: 300, X: 50, Y: 60}}
	a.firstSec[models.GeneralUnitHatchery] = map[uint16]int{
		0x1: 10,  // main: produced from the start, before any build → fallback
		0x2: 320, // expansion: produced after the build → matches it
	}
	start := point{X: 50*32 + 16, Y: 70*32 + 16} // pixels

	loc := resolveTagLocations(a, start)
	if got := loc[ttKey{models.GeneralUnitHatchery, 0x1}]; got != (point{X: 50, Y: 70}) {
		t.Errorf("unmatched town-hall tag should fall back to start tile, got %+v", got)
	}
	if got := loc[ttKey{models.GeneralUnitHatchery, 0x2}]; got != (point{X: 50, Y: 60}) {
		t.Errorf("expansion tag should match its Build tile, got %+v", got)
	}
}

func TestResolveTagLocations_NonTownHallNoFallback(t *testing.T) {
	a := newPlayerAcc()
	a.firstSec[models.GeneralUnitBarracks] = map[uint16]int{0x9: 100} // no Build for it
	loc := resolveTagLocations(a, point{X: 100, Y: 100})
	if got, ok := loc[ttKey{models.GeneralUnitBarracks, 0x9}]; ok {
		t.Errorf("a non-town-hall tag with no matching Build must not resolve, got %+v", got)
	}
}

func TestResolveTagLocations_RecycledTagKeptDistinct(t *testing.T) {
	// BW recycles unit tags: tag 0x7 names a Barracks early, then a Factory
	// later. Each must resolve to its own building, not collapse to one.
	a := newPlayerAcc()
	a.builds[models.GeneralUnitBarracks] = []Build{{Sec: 100, X: 10, Y: 10}}
	a.builds[models.GeneralUnitFactory] = []Build{{Sec: 500, X: 80, Y: 90}}
	a.firstSec[models.GeneralUnitBarracks] = map[uint16]int{0x7: 110}
	a.firstSec[models.GeneralUnitFactory] = map[uint16]int{0x7: 510}

	loc := resolveTagLocations(a, point{})
	if got := loc[ttKey{models.GeneralUnitBarracks, 0x7}]; got != (point{10, 10}) {
		t.Errorf("Barracks incarnation of recycled tag: got %+v", got)
	}
	if got := loc[ttKey{models.GeneralUnitFactory, 0x7}]; got != (point{80, 90}) {
		t.Errorf("Factory incarnation of recycled tag: got %+v", got)
	}

	// A CancelTrain at sec 520 must bind to the Factory (latest incarnation).
	if got := a.latestTypeForTag(0x7, 520); got != models.GeneralUnitFactory {
		t.Errorf("cancel at 520 should bind to Factory, got %q", got)
	}
	// A CancelTrain at sec 200 must bind to the Barracks.
	if got := a.latestTypeForTag(0x7, 200); got != models.GeneralUnitBarracks {
		t.Errorf("cancel at 200 should bind to Barracks, got %q", got)
	}
}

// enrichableActionIDs are the raw command types Coordinates may enrich.
var enrichableActionIDs = map[byte]bool{
	repcmd.TypeIDTrain: true, repcmd.TypeIDUnitMorph: true,
	repcmd.TypeIDTech: true, repcmd.TypeIDUpgrade: true,
	repcmd.TypeIDBuildingMorph: true, repcmd.TypeIDCancelTrain: true,
}

func playersWithStarts(r *rep.Replay) []*models.Player {
	var ps []*models.Player
	for i, p := range r.Header.Players {
		if p == nil {
			continue
		}
		race := ""
		if p.Race != nil {
			race = p.Race.Name
		}
		mp := &models.Player{PlayerID: p.ID, Race: race}
		if r.Computed != nil && i < len(r.Computed.PlayerDescs) {
			if sl := r.Computed.PlayerDescs[i].StartLocation; sl != nil {
				x, y := int(sl.X), int(sl.Y)
				mp.StartLocationX, mp.StartLocationY = &x, &y
			}
		}
		ps = append(ps, mp)
	}
	return ps
}

func TestCoordinates_RealReplays(t *testing.T) {
	fixtures := map[string]string{
		"SomaTyson.rep":       filepath.Join("..", "testdata", "replays", "SomaTyson.rep"),
		"SomaJyJ.rep":         filepath.Join("..", "testdata", "replays", "SomaJyJ.rep"),
		"bgh.rep":             filepath.Join("..", "testdata", "replays", "bgh.rep"),
		"Somavssnow.rep":      filepath.Join("..", "testdata", "replays", "Somavssnow.rep"),
		"bo_forge_expand.rep": filepath.Join("..", "builddedup", "testdata", "replays", "bo_forge_expand.rep"),
	}
	for name, path := range fixtures {
		t.Run(name, func(t *testing.T) {
			r, err := screp.ParseFile(path)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			players := playersWithStarts(r)
			coords := Coordinates(r, players)
			if len(coords) == 0 {
				t.Fatal("expected some inferred coordinates")
			}

			maxX, maxY := int(r.Header.MapWidth)*32, int(r.Header.MapHeight)*32
			for idx, c := range coords {
				if c.X < 0 || c.X > maxX || c.Y < 0 || c.Y > maxY {
					t.Errorf("idx %d: coord %+v out of map bounds %dx%d", idx, c, maxX, maxY)
				}
				if id := r.Commands.Cmds[idx].BaseCmd().Type.ID; !enrichableActionIDs[id] {
					t.Errorf("idx %d: enriched a non-enrichable command type %#x", idx, id)
				}
			}

			// Train commands for produced units (Protoss/Terran) should be
			// enriched at a high rate — the producing building is single-selected.
			var trains, trainsWithCoords int
			for idx, c := range r.Commands.Cmds {
				tc, ok := c.(*repcmd.TrainCmd)
				if !ok || tc.Unit == nil {
					continue
				}
				if _, isProducer := producerBuilding[tc.Unit.Name]; !isProducer {
					continue
				}
				trains++
				if _, has := coords[idx]; has {
					trainsWithCoords++
				}
			}
			if trains > 0 && trainsWithCoords*2 < trains {
				t.Errorf("expected most producer-building trains enriched, got %d/%d", trainsWithCoords, trains)
			}

			// Determinism: repeated runs yield the identical map. Looping
			// exercises Go's randomized map iteration so a tag/build firstSec
			// tie can't silently flip a coordinate between ingests.
			for n := 0; n < 50; n++ {
				again := Coordinates(r, players)
				if len(again) != len(coords) {
					t.Fatalf("non-deterministic: %d vs %d entries", len(again), len(coords))
				}
				for idx, c := range coords {
					if again[idx] != c {
						t.Fatalf("non-deterministic at idx %d: %+v vs %+v", idx, c, again[idx])
					}
				}
			}
		})
	}
}
