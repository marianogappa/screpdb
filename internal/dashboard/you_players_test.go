package dashboard

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/marianogappa/screpdb/internal/engine/moments"
	"github.com/marianogappa/screpdb/internal/library"
)

func TestSetReplayDirRereadsYouFromTheNewFolder(t *testing.T) {
	dash := newTestDashboard(t)
	if keys := dash.loadYouKeys(); len(keys) != 0 {
		t.Fatalf("you keys = %v before switching, want none for a folder outside StarCraft", keys)
	}

	starcraft := t.TempDir()
	replays := filepath.Join(starcraft, "Maps", "Replays")
	if err := os.MkdirAll(replays, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(starcraft, "CSettings.json"), []byte(`{"General settings":{"account":"Alice#1234"}}`), 0o644); err != nil {
		t.Fatalf("write CSettings: %v", err)
	}
	corpus := testCorpusDir(t)
	entries, err := os.ReadDir(corpus)
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(corpus, entries[0].Name()))
	if err != nil {
		t.Fatalf("read replay: %v", err)
	}
	if err := os.WriteFile(filepath.Join(replays, entries[0].Name()), data, 0o644); err != nil {
		t.Fatalf("write replay: %v", err)
	}

	if err := dash.setReplayDir(context.Background(), replays); err != nil {
		t.Fatalf("setReplayDir: %v", err)
	}
	keys := dash.loadYouKeys()
	if _, ok := keys["alice"]; !ok {
		t.Fatalf("you keys = %v after switching to the StarCraft folder, want alice", keys)
	}
}

func TestCSettingsNamesNeverClaimShieldBatteryPlayers(t *testing.T) {
	dash := newTestDashboard(t)
	dash.youKeys.Store(youKeySetFromBattleTags([]string{"Chobo86#1234"}))
	rec := &library.Replay{Players: []library.Player{
		{Slot: 0, Name: "Chobo86", Key: library.SBKeyPrefix + "999"},
		{Slot: 1, Name: "Someone", Key: library.SBKeyPrefix + "1000"},
	}}
	if got := dash.youSlot(rec); got != moments.NoPlayer {
		t.Fatalf("youSlot = %d, want NoPlayer: a Battle.net name from CSettings must not claim a ShieldBattery account", got)
	}
	if dash.isYouKey(rec.Players[0].Key) {
		t.Fatal("ShieldBattery key named like a CSettings account reads as the user")
	}
	if !dash.isYouKey("chobo86") {
		t.Fatal("the Battle.net key of the same name stopped reading as the user")
	}
}
