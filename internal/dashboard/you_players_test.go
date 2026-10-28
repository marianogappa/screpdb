package dashboard

import (
	"context"
	"os"
	"path/filepath"
	"testing"
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
