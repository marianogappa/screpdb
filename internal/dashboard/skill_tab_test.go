package dashboard

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/skill"
)

func TestPlayersOutBeforeEightMinutesAreNotRated(t *testing.T) {
	model, err := skill.Load()
	if err != nil {
		t.Fatal(err)
	}
	rec := &library.Replay{Duration: 1200, MapKind: library.MapKindRegular, EngineData: &library.EngineData{}}
	for slot, seconds := range []uint16{1200, 1200, 1200, 287} {
		rec.Players = append(rec.Players, library.Player{Slot: uint8(slot), Race: library.RaceProtoss, Type: library.PlayerTypeHuman, APM: 150, EAPM: 120})
		rec.EngineData.Players = append(rec.EngineData.Players, library.EnginePlayer{Slot: uint8(slot), Seconds: seconds})
	}
	g := rateGame(model, rec)
	if g.status != skillStatusOK {
		t.Fatalf("status %q", g.status)
	}
	if _, ok := g.players[3]; ok {
		t.Fatal("a player eliminated at 4:47 must not be rated: the model only knows players who played 8:00")
	}
	if len(g.players) != 3 {
		t.Fatalf("rated %d players, want the 3 who played the whole game", len(g.players))
	}
}
