package db

import (
	"context"
	"testing"
	"time"
)

func TestBnetProfileRoundtrip(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	row, err := store.GetBnetProfile(ctx, "ByuN", 30)
	if err != nil {
		t.Fatalf("GetBnetProfile (missing): %v", err)
	}
	if row != nil {
		t.Fatalf("expected nil for missing row, got %+v", row)
	}

	fetchedAt := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	want := BnetProfileRow{
		Toon:        "ByuN",
		Gateway:     30,
		Found:       true,
		AuroraID:    42,
		BattleTag:   "ByuN#123",
		CountryCode: "KR",
		Payload:     `{"aurora_id":42}`,
		FetchedAt:   fetchedAt,
	}
	if err := store.UpsertBnetProfile(ctx, want); err != nil {
		t.Fatalf("UpsertBnetProfile: %v", err)
	}

	got, err := store.GetBnetProfile(ctx, "ByuN", 30)
	if err != nil {
		t.Fatalf("GetBnetProfile: %v", err)
	}
	if got == nil || *got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}

	updated := want
	updated.Found = false
	updated.AuroraID = 0
	updated.Payload = `{"aurora_id":0}`
	updated.FetchedAt = fetchedAt.Add(time.Hour)
	if err := store.UpsertBnetProfile(ctx, updated); err != nil {
		t.Fatalf("UpsertBnetProfile (update): %v", err)
	}
	got, err = store.GetBnetProfile(ctx, "ByuN", 30)
	if err != nil {
		t.Fatalf("GetBnetProfile after update: %v", err)
	}
	if got == nil || *got != updated {
		t.Errorf("after update: got %+v, want %+v", got, updated)
	}

	other, err := store.GetBnetProfile(ctx, "ByuN", 20)
	if err != nil {
		t.Fatalf("GetBnetProfile other gateway: %v", err)
	}
	if other != nil {
		t.Errorf("gateway is part of the key; expected nil for gateway 20, got %+v", other)
	}
}
