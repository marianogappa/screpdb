package persist

import (
	"path/filepath"
	"testing"
	"time"
)

func TestEnrichQueueRoundTrip(t *testing.T) {
	root := filepath.Join(t.TempDir(), "appdata")
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)

	q := NewEnrichQueue(root)
	entry := EnrichEntry{
		MD5:        "abc",
		Path:       "/replays/bgh.rep",
		SizeBytes:  100_000,
		ReplayDate: now.Add(-2 * time.Hour),
		UserLeftAt: now.Add(-90 * time.Minute),
		Toon:       "me",
		CreatedAt:  now,
		Status:     EnrichNeedGameID,
	}
	if err := q.Upsert(entry); err != nil {
		t.Fatal(err)
	}
	if err := q.CountCall(now); err != nil {
		t.Fatal(err)
	}

	reloaded := NewEnrichQueue(root)
	got, ok, err := reloaded.Get("abc")
	if err != nil || !ok {
		t.Fatalf("Get after reload: ok=%v err=%v", ok, err)
	}
	if got.Path != entry.Path || got.Status != EnrichNeedGameID || got.UpdatedAt.IsZero() {
		t.Fatalf("entry: %+v", got)
	}
	live, err := reloaded.Live()
	if err != nil || len(live) != 1 {
		t.Fatalf("Live: %v %v", live, err)
	}
	paths, err := reloaded.KnownPaths()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := paths["/replays/bgh.rep"]; !ok {
		t.Fatalf("paths: %v", paths)
	}
	if calls, _ := reloaded.Calls(now); calls != 1 {
		t.Fatalf("calls: %d", calls)
	}
	// The daily counter resets across UTC days.
	if calls, _ := reloaded.Calls(now.Add(24 * time.Hour)); calls != 0 {
		t.Fatalf("next-day calls: %d", calls)
	}
}

func TestEnrichQueueTerminalAndPrune(t *testing.T) {
	root := filepath.Join(t.TempDir(), "appdata")
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	q := NewEnrichQueue(root)

	for _, status := range []string{EnrichDone, EnrichNotBetter, EnrichUnavailable, EnrichExpired} {
		if !(EnrichEntry{Status: status}).Terminal() {
			t.Errorf("%s should be terminal", status)
		}
		if err := q.Upsert(EnrichEntry{MD5: status, Path: "/replays/" + status, Status: status}); err != nil {
			t.Fatal(err)
		}
	}
	for _, status := range []string{EnrichNeedGameID, EnrichWaiting} {
		if (EnrichEntry{Status: status}).Terminal() {
			t.Errorf("%s should be live", status)
		}
	}

	live, err := q.Live()
	if err != nil || len(live) != 0 {
		t.Fatalf("Live: %v %v", live, err)
	}
	// Terminal entries survive as durable stop conditions inside retention...
	if err := q.Prune(now); err != nil {
		t.Fatal(err)
	}
	if paths, _ := q.KnownPaths(); len(paths) != 4 {
		t.Fatalf("pruned too early: %v", paths)
	}
	// ...and go away past it. Upsert stamps UpdatedAt with the wall clock, so
	// prune relative to the actual wall clock, not the fixture's base time.
	if err := q.Prune(time.Now().UTC().Add(enrichRetainTerminal + time.Hour)); err != nil {
		t.Fatal(err)
	}
	if paths, _ := q.KnownPaths(); len(paths) != 0 {
		t.Fatalf("not pruned: %v", paths)
	}
}
