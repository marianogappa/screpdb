package dashboard

import (
	"context"
	"fmt"
	"testing"

	"github.com/marianogappa/screpdb/internal/dashboard/apigen"
	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
	"github.com/marianogappa/screpdb/internal/hotkeystream"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
)

func TestGameHotkeysAcrossTestCorpus(t *testing.T) {
	d := newTestDashboard(t)
	r := d.setupRouter()
	saw := 0
	sawBuilding := false
	for _, replayID := range listTestGames(t, r) {
		payload, err := d.GameHotkeys(context.Background(), apigen.GameHotkeysRequestObject{ReplayID: apigen.ReplayID(replayID)})
		if err != nil {
			t.Fatalf("GameHotkeys(%d): %v", replayID, err)
		}
		p, ok := payload.(hotkeyTimelinePayload)
		if !ok {
			t.Fatalf("GameHotkeys(%d): unexpected payload type %T", replayID, payload)
		}
		if p.DurationSeconds <= 0 {
			t.Fatalf("GameHotkeys(%d): non-positive duration", replayID)
		}
		for _, player := range p.Players {
			saw += len(player.Events)
			if player.Legacy {
				t.Fatalf("freshly ingested stream flagged legacy: %+v", player)
			}
			for _, e := range player.Events {
				if e[1] == int(hotkeystream.TypeAssignBuilding) {
					sawBuilding = true
					if p.Buildings[byte(e[4])] == "" {
						t.Fatalf("building ID %d missing from names map", e[4])
					}
				}
			}
		}
	}
	if saw == 0 || !sawBuilding {
		t.Fatalf("corpus produced events=%d buildingAssigns=%v", saw, sawBuilding)
	}
}

func TestIsLegacyStream(t *testing.T) {
	v2 := hotkeystream.Encode([]hotkeystream.Event{{Sec: 1, Type: hotkeystream.TypeSelect, Group: 1}})
	if isLegacyStream(v2) {
		t.Fatal("v2 blob flagged legacy")
	}
	if !isLegacyStream([]byte{0x43, 0x10}) {
		t.Fatal("v1 blob not flagged legacy")
	}
	if isLegacyStream(nil) {
		t.Fatal("empty blob flagged legacy")
	}
}

// TestPlayerHotkeySignatureWithCards gives one player three long Zerg games
// with hotkey streams, which is the threshold the signature endpoint needs
// before it will build a card.
func TestPlayerHotkeySignatureWithCards(t *testing.T) {
	source := newTestDashboard(t)
	streams := corpusHotkeyStreams(t, source, 3)

	replays := make([]*library.Replay, 0, len(streams))
	for i, blob := range streams {
		replays = append(replays, librarytest.Replay(
			librarytest.WithID(int64(9000+i)),
			librarytest.WithChecksum(fmt.Sprintf("sigcard-%d", i)),
			librarytest.WithDuration(1800),
			librarytest.WithPlayer("SigCardTester", librarytest.Race(library.RaceZerg)),
			librarytest.WithPlayer("Opponent", librarytest.Race(library.RaceTerran)),
			librarytest.WithHotkeyStream(0, blob),
		))
	}
	d := newTestDashboardWithReplays(t, replays...)

	payload, err := d.PlayerHotkeySignature(context.Background(), apigen.PlayerHotkeySignatureRequestObject{PlayerKey: "sigcardtester"})
	if err != nil {
		t.Fatalf("PlayerHotkeySignature: %v", err)
	}
	p, ok := payload.(hotkeySignaturePayload)
	if !ok {
		t.Fatalf("unexpected payload type %T", payload)
	}
	if len(p.Cards) == 0 {
		t.Fatalf("expected a signature card, got games_by_race=%v", p.GamesByRace)
	}
	card := p.Cards[0]
	if card.Race != "Zerg" || card.Games < 3 {
		t.Fatalf("unexpected card: race=%s games=%d", card.Race, card.Games)
	}
	if len(card.Keys) == 0 || card.Prose == "" {
		t.Fatalf("card missing keys or prose: %+v", card)
	}
	if card.TemporalScore <= 0 || card.TemporalScore > 1 {
		t.Fatalf("temporal score out of range: %f", card.TemporalScore)
	}
}

// corpusHotkeyStreams borrows real hotkey blobs from the committed replays, so
// the synthetic games carry streams the decoder actually accepts.
func corpusHotkeyStreams(t *testing.T, d *Dashboard, want int) [][]byte {
	t.Helper()
	ctx := context.Background()
	games, err := d.dbStore.ListGames(ctx, dashboarddb.GamesQuery{}, 50, 0)
	if err != nil {
		t.Fatalf("ListGames: %v", err)
	}
	var out [][]byte
	for _, game := range games {
		streams, err := d.dbStore.ListReplayPlayerHotkeyStreams(ctx, game.ReplayID)
		if err != nil {
			t.Fatalf("ListReplayPlayerHotkeyStreams: %v", err)
		}
		for _, stream := range streams {
			if len(stream.HotkeyStream) == 0 {
				continue
			}
			out = append(out, stream.HotkeyStream)
			if len(out) == want {
				return out
			}
		}
	}
	t.Skipf("only %d hotkey streams in the committed corpus, need %d", len(out), want)
	return nil
}

func TestPlayerHotkeySignatureRejectsEmptyKey(t *testing.T) {
	d := newTestDashboard(t)
	if _, err := d.PlayerHotkeySignature(context.Background(), apigen.PlayerHotkeySignatureRequestObject{PlayerKey: "  "}); err == nil {
		t.Fatal("expected error for empty player key")
	}
}
