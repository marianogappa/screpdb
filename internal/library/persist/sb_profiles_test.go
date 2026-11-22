package persist

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/sbfacade"
)

func TestSBProfilesRoundTrip(t *testing.T) {
	root := t.TempDir()
	store := NewSBProfiles(root)
	if _, ok, err := store.Get(17185); ok || err != nil {
		t.Fatalf("empty store: %v %v", ok, err)
	}
	fetched := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	want := SBProfile{
		Profile: sbfacade.Profile{
			UserID:   17185,
			Name:     "K_BishOp",
			Ladder:   map[string]sbfacade.LadderStats{"1v1": {Rank: 4, Rating: 2363.05, ByRace: map[string]sbfacade.WinLoss{"t": {Wins: 123, Losses: 56}}}},
			Lifetime: map[string]sbfacade.WinLoss{"z": {Wins: 15, Losses: 1}},
			RecentGames: []sbfacade.RecentGame{{
				ID:         "g",
				GameLength: 719502 * time.Millisecond,
				Players:    []sbfacade.RecentPlayer{{UserID: 17185, Race: "t", Result: sbfacade.ResultWin}},
			}},
		},
		FetchedAt: fetched,
	}
	if err := store.Put(17185, want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := NewSBProfiles(root).Get(17185)
	if err != nil || !ok || !got.FetchedAt.Equal(fetched) {
		t.Fatalf("got %+v %v %v", got, ok, err)
	}
	p := got.Profile
	if p.Name != "K_BishOp" || p.Ladder["1v1"].ByRace["t"].Wins != 123 || p.Lifetime["z"].Losses != 1 ||
		p.RecentGames[0].GameLength != 719502*time.Millisecond || p.RecentGames[0].Players[0].Result != sbfacade.ResultWin {
		t.Fatalf("profile %+v", p)
	}
}

func TestSBProfilesDamagedFileStartsEmpty(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, SBGamesDirName, sbProfilesFile)
	if err := iofacade.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := iofacade.WriteFile(path, []byte("{trunc"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewSBProfiles(root)
	if _, ok, err := store.Get(1); ok || err != nil {
		t.Fatalf("damaged file: %v %v", ok, err)
	}
	if err := store.Put(1, SBProfile{Profile: sbfacade.Profile{Name: "x"}}); err != nil {
		t.Fatal(err)
	}
	if got, ok, _ := NewSBProfiles(root).Get(1); !ok || got.Profile.Name != "x" {
		t.Fatalf("rewrite: %+v %v", got, ok)
	}
}
