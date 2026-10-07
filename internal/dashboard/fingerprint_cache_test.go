package dashboard

import (
	"context"
	"testing"
	"time"

	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
	"github.com/marianogappa/screpdb/internal/library/persist"
)

// A player's fingerprint outcome must follow the corpus. The cache used to
// survive every replay the watcher or the streaming load added, so a player
// first seen with two games kept "not enough games" until the folder or the
// filter changed.
func TestFingerprintOutcome_RefreshesWhenGamesArrive(t *testing.T) {
	lib := library.New(library.Options{CoalesceRecords: 1, CoalesceDelay: time.Millisecond})
	t.Cleanup(lib.Close)
	lib.Reset(1)
	root := t.TempDir()
	if err := iofacade.AllowDir(root); err != nil {
		t.Fatal(err)
	}
	settings, err := dashboarddb.NewFileSettings(root, lib)
	if err != nil {
		t.Fatal(err)
	}
	d := &Dashboard{ctx: context.Background(), libraryHub: newLibraryHub()}
	d.dbStore = dashboarddb.NewLibStore(lib, persist.NewBnetCache(root), settings)

	game := func() *library.Replay {
		r := librarytest.Replay(
			librarytest.WithPlayer("Regular", librarytest.Team(1)),
			librarytest.WithPlayer("Stranger", librarytest.Team(2)),
			librarytest.WithFingerprint(0, []float64{1, 2}),
		)
		r.Flags |= library.FlagIsOneOnOne
		return r
	}

	lib.Add(1, game(), game())
	lib.Flush()
	outcome, err := d.fingerprintOutcomeFor("regular", 1)
	if err != nil || outcome.reason != fingerprintNoMatchNotEnoughGames {
		t.Fatalf("two games: outcome %+v err %v, want not enough games", outcome, err)
	}

	lib.Add(1, game())
	lib.Flush()
	outcome, err = d.fingerprintOutcomeFor("regular", 1)
	if err == nil && outcome.reason == fingerprintNoMatchNotEnoughGames {
		t.Fatal("a third game arrived but the cached two-game answer was served")
	}
}
