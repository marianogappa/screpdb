package db

import (
	"context"
	"testing"

	"github.com/marianogappa/screpdb/internal/botdetect"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
)

func TestLibStoreBotGamesByPlayerKey(t *testing.T) {
	bot := librarytest.Bot(botdetect.SignalFarJumps)
	s := newTestLibStore(t,
		librarytest.Replay(librarytest.WithPlayer("Flash"), librarytest.WithPlayer("ComfyUI", bot)),
		librarytest.Replay(librarytest.WithPlayer("Bisu"), librarytest.WithPlayer("comfyui", bot)),
		librarytest.Replay(librarytest.WithPlayer("Flash"), librarytest.WithPlayer("ComfyUI")),
	)
	got, err := s.BotGamesByPlayerKey(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got["comfyui"] != 2 || len(got) != 1 {
		t.Fatalf("got %v, want only comfyui with 2 games", got)
	}
}

func TestLibStoreGamePlayerRowsCarryBotSignals(t *testing.T) {
	r := librarytest.Replay(
		librarytest.WithPlayer("Flash"),
		librarytest.WithPlayer("ComfyUI", librarytest.Bot(botdetect.SignalSameFrame|botdetect.SignalFarJumps)),
	)
	s := newTestLibStore(t, r)
	ctx := context.Background()

	listRows, err := s.ListReplayPlayers(ctx, []int64{r.ID})
	if err != nil {
		t.Fatal(err)
	}
	detailRows, err := s.ListReplayPlayersForDetail(ctx, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]botdetect.Signal{"Flash": 0, "ComfyUI": botdetect.SignalSameFrame | botdetect.SignalFarJumps}
	for _, row := range listRows {
		if row.BotSignals != want[row.Name] {
			t.Errorf("games list %s: %04b", row.Name, row.BotSignals)
		}
	}
	for _, row := range detailRows {
		if row.BotSignals != want[row.Name] {
			t.Errorf("detail %s: %04b", row.Name, row.BotSignals)
		}
	}
}
