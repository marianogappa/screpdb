package dashboard

import (
	"context"
	"database/sql"
	"testing"

	"github.com/marianogappa/screpdb/internal/dashboard/apigen"
	"github.com/marianogappa/screpdb/internal/hotkeystream"
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

// TestPlayerHotkeySignatureWithCards renames three ingested player rows to one
// key (via a second connection into the shared in-memory test DB), so the
// signature endpoint crosses the 3-games-per-race threshold and builds a card.
func TestPlayerHotkeySignatureWithCards(t *testing.T) {
	d := newTestDashboard(t)

	db, err := sql.Open("sqlite", dashboardTestDB)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT id, name, race FROM players WHERE hotkey_stream IS NOT NULL LIMIT 4`)
	if err != nil {
		t.Fatalf("list seedable rows: %v", err)
	}
	type original struct {
		id   int64
		name string
		race string
	}
	var originals []original
	for rows.Next() {
		var o original
		if err := rows.Scan(&o.id, &o.name, &o.race); err != nil {
			t.Fatalf("scan: %v", err)
		}
		originals = append(originals, o)
	}
	_ = rows.Close()
	if len(originals) < 3 {
		t.Fatalf("only %d seedable rows with hotkey streams", len(originals))
	}
	for _, o := range originals {
		if _, err := db.Exec(`UPDATE players SET name = 'SigCardTester', race = 'Zerg' WHERE id = ?`, o.id); err != nil {
			t.Fatalf("seed row %d: %v", o.id, err)
		}
	}
	// The shared in-memory DB outlives this test: restore the original rows so
	// later tests keep seeing the untouched corpus.
	t.Cleanup(func() {
		for _, o := range originals {
			_, _ = db.Exec(`UPDATE players SET name = ?, race = ? WHERE id = ?`, o.name, o.race, o.id)
		}
	})

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

func TestPlayerHotkeySignatureRejectsEmptyKey(t *testing.T) {
	d := newTestDashboard(t)
	if _, err := d.PlayerHotkeySignature(context.Background(), apigen.PlayerHotkeySignatureRequestObject{PlayerKey: "  "}); err == nil {
		t.Fatal("expected error for empty player key")
	}
}
