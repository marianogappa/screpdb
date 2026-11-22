package persist

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/iofacade"
)

func TestSBGamesRoundTrip(t *testing.T) {
	root := t.TempDir()
	store := NewSBGames(root)
	if _, ok, err := store.Get("g"); ok || err != nil {
		t.Fatalf("empty store: %v %v", ok, err)
	}
	want := SBGame{Results: map[uint32]string{1: "win", 2: "loss"}, FetchedAt: time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)}
	if err := store.Put("g", want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := NewSBGames(root).Get("g")
	if err != nil || !ok || got.Results[1] != "win" || got.Results[2] != "loss" || !got.FetchedAt.Equal(want.FetchedAt) {
		t.Fatalf("got %+v %v %v", got, ok, err)
	}
}

func TestSBGamesDamagedFileStartsEmpty(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, SBGamesDirName, sbGamesFile)
	if err := iofacade.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := iofacade.WriteFile(path, []byte("{trunc"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewSBGames(root)
	if _, ok, err := store.Get("g"); ok || err != nil {
		t.Fatalf("damaged file: %v %v", ok, err)
	}
	if err := store.Put("g", SBGame{Canceled: true}); err != nil {
		t.Fatal(err)
	}
	if got, ok, _ := NewSBGames(root).Get("g"); !ok || !got.Canceled {
		t.Fatalf("rewrite: %+v %v", got, ok)
	}
}
