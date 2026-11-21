package dashboard

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
	"github.com/marianogappa/screpdb/internal/library/persist"
)

func savedReplay(id int64, playedAt time.Time, saver string, others ...string) *library.Replay {
	opts := []librarytest.Option{
		librarytest.WithID(id),
		librarytest.WithChecksum(fmt.Sprintf("you-%d", id)),
		librarytest.WithDate(playedAt),
		librarytest.WithPath(fmt.Sprintf("/sc/Maps/Replays/AutoSave/%d.rep", id), playedAt),
		librarytest.WithPlayer(saver, librarytest.Team(1)),
	}
	for i, name := range others {
		opts = append(opts, librarytest.WithPlayer(name, librarytest.Team(uint8(i+2))))
	}
	r := librarytest.Replay(opts...)
	r.Flags |= library.FlagIsAutosave
	r.Players[0].Flags |= library.PlayerSaver
	return r
}

func TestYouSaversNeedTwoSavesAndNoStrangerPattern(t *testing.T) {
	now := time.Now()
	replays := []*library.Replay{
		savedReplay(1, now, "Oldie", "Foe"),
		savedReplay(2, now, "Oldie", "Foe"),
		savedReplay(3, now, "OnceOnly", "Foe"),
		savedReplay(4, now, "Shared", "Foe"),
		savedReplay(5, now, "Shared", "Foe"),
		savedReplay(6, now, "Chobo86", "Shared"),
	}
	got := youSavers(replays)
	if got["oldie"] != 2 {
		t.Errorf("oldie saves = %d, want 2", got["oldie"])
	}
	if _, ok := got["onceonly"]; ok {
		t.Error("a name that recorded a single autosave counts as the user")
	}
	if _, ok := got["shared"]; ok {
		t.Error("a name another recorder's file shows playing counts as the user")
	}
	if _, ok := got["foe"]; ok {
		t.Error("an opponent counts as the user")
	}
}

func TestYouSaversIgnoreCompleteCopiesAndOtherFolders(t *testing.T) {
	now := time.Now()
	complete := savedReplay(1, now, "CoPlayer", "Chobo86")
	complete.Flags |= library.FlagIsCompleteCopy
	complete2 := savedReplay(2, now, "CoPlayer", "Chobo86")
	complete2.Flags |= library.FlagIsCompleteCopy
	downloaded := savedReplay(3, now, "Pro", "Other")
	downloaded.Flags &^= library.FlagIsAutosave
	downloaded2 := savedReplay(4, now, "Pro", "Other")
	downloaded2.Flags &^= library.FlagIsAutosave
	if got := youSavers([]*library.Replay{complete, complete2, downloaded, downloaded2}); len(got) != 0 {
		t.Fatalf("savers = %v, want none from complete copies or files outside AutoSave", got)
	}
}

func TestReplaySaverFallsBackToTheOnlyPlayerNeverSeenLeaving(t *testing.T) {
	r := savedReplay(1, time.Now(), "Me", "Foe", "Ally")
	r.Players[0].Flags &^= library.PlayerSaver
	r.Players[1].Flags |= library.PlayerLeft
	r.Players[2].Flags |= library.PlayerLeft
	if got := replaySaver(r); got != 0 {
		t.Fatalf("replaySaver = %d, want 0", got)
	}
	r.Players[2].Flags &^= library.PlayerLeft
	if got := replaySaver(r); got != -1 {
		t.Fatalf("replaySaver = %d, want -1 when two players never left", got)
	}
}

func TestYouAccountToonsNeedTwoOwnNamesOnTheAccount(t *testing.T) {
	d := newTestDashboard(t)
	ctx := context.Background()
	seed := []persist.BnetProfile{
		{Toon: "chobo86", Gateway: 10, Found: true, AuroraID: 1, Toons: []persist.BnetToon{{Toon: "chobo86"}, {Toon: "MarineTax"}, {Toon: "Zergonomicon"}}},
		{Toon: "chobo86", Gateway: 30, Found: true, AuroraID: 2, Toons: []persist.BnetToon{{Toon: "chobo86"}, {Toon: "Stranger"}}},
	}
	for _, p := range seed {
		p.FetchedAt = time.Now().UTC()
		if err := d.dbStore.UpsertBnetProfile(ctx, p); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	d.youKeys.Store(youKeySetFromBattleTags([]string{"chobo86#1", "MarineTax#2"}))

	set := d.youSet()
	if set["zergonomicon"].Evidence != youEvidenceAccount {
		t.Errorf("zergonomicon evidence = %q, want account", set["zergonomicon"].Evidence)
	}
	if _, ok := set["stranger"]; ok {
		t.Error("a stranger's account sharing one CSettings name counts as the user")
	}
}

func TestOtherYouProfilesListsRecentProfilesWithGames(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	d := newTestDashboardWithReplays(t,
		savedReplay(1, now.Add(-time.Hour), "Chobo86", "Foe"),
		savedReplay(2, now.Add(-2*time.Hour), "Zergonomicon", "Foe"),
		savedReplay(3, now.Add(-3*time.Hour), "Zergonomicon", "Foe"),
		savedReplay(4, now.Add(-200*24*time.Hour), "Oldie", "Foe"),
		savedReplay(5, now.Add(-201*24*time.Hour), "Oldie", "Foe"),
	)
	d.youKeys.Store(youKeySetFromBattleTags([]string{"Chobo86#1234"}))

	got := d.otherYouProfiles("chobo86")
	if len(got) != 2 || got[0].PlayerName != "Zergonomicon" || got[1].PlayerName != "Oldie" {
		t.Fatalf("profiles = %+v, want Zergonomicon then Oldie", got)
	}
	if got[0].Games != 2 {
		t.Errorf("Zergonomicon games = %d, want 2", got[0].Games)
	}
	if d.otherYouProfiles("foe") != nil {
		t.Error("someone else's page lists the user's profiles")
	}
	if name := d.youDisplayName("zergonomicon", "Zergonomicon"); name != formatYouDisplayName("Zergonomicon") {
		t.Errorf("display name = %q, want the you marker", name)
	}
}
