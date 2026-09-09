package library_test

import (
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
)

func TestCompletePathHelpers(t *testing.T) {
	if got := library.CompletePathFor("/replays/game.rep"); got != "/replays/game-complete.rep" {
		t.Fatalf("CompletePathFor: %q", got)
	}
	base, ok := library.CompleteBasePath("/replays/game-complete.rep")
	if !ok || base != "/replays/game.rep" {
		t.Fatalf("CompleteBasePath: %q %v", base, ok)
	}
	for _, path := range []string{"/replays/game.rep", "/replays/-complete.rep", "/replays/complete.rep"} {
		if library.IsCompletePath(path) {
			t.Fatalf("IsCompletePath(%q) = true", path)
		}
	}
	if !library.IsCompletePath("/replays/game-complete.rep") {
		t.Fatal("IsCompletePath missed a complete path")
	}
}

func pairFixtures(date time.Time) (own, complete *library.Replay) {
	own = librarytest.Replay(
		librarytest.WithDate(date),
		librarytest.WithPath("/replays/game.rep", date),
		librarytest.WithPlayer("Flash"),
		librarytest.WithPlayer("Bisu"),
		librarytest.WithChat(0, 10, "gl hf"),
		librarytest.WithChat(1, 12, "u2"),
	)
	complete = librarytest.Replay(
		librarytest.WithDate(date),
		librarytest.WithPath("/replays/game-complete.rep", date),
		librarytest.WithPlayer("Bisu"),
		librarytest.WithPlayer("Flash"),
		librarytest.WithChat(0, 12, "u2"),
		librarytest.WithChat(1, 900, "gg"),
	)
	return own, complete
}

func TestCompletePairSupersedesAndMergesChat(t *testing.T) {
	for name, order := range map[string][2]int{"own first": {0, 1}, "complete first": {1, 0}} {
		t.Run(name, func(t *testing.T) {
			lib := newTestLibrary(t, library.Options{})
			own, complete := pairFixtures(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))
			pair := [2]*library.Replay{own, complete}
			lib.Add(0, pair[order[0]])
			lib.Flush()
			lib.Add(0, pair[order[1]])
			lib.Flush()

			snap := lib.Snapshot()
			ownRec, ok := snap.ByPath("/replays/game.rep")
			if !ok {
				t.Fatal("own record missing from snapshot")
			}
			completeRec, ok := snap.ByPath("/replays/game-complete.rep")
			if !ok {
				t.Fatal("complete record missing from snapshot")
			}
			if !ownRec.Flags.Has(library.FlagSuperseded) {
				t.Fatal("own record not superseded")
			}
			if !completeRec.Flags.Has(library.FlagIsCompleteCopy) {
				t.Fatal("complete record missing FlagIsCompleteCopy")
			}
			if completeRec.Flags.Has(library.FlagSuperseded) {
				t.Fatal("complete record must not be superseded")
			}

			view := lib.View()
			if view.Contains(ownRec.ID) {
				t.Fatal("superseded record visible through the filter")
			}
			if !view.Contains(completeRec.ID) {
				t.Fatal("complete record hidden by the filter")
			}

			// Chat is the union of both files, re-attributed by name: the
			// complete file lists Bisu first, so Flash's "gl hf" maps to
			// ordinal 1 and the shared "u2" line dedupes.
			want := []library.ChatLine{
				{Player: 1, Sec: 10, Text: "gl hf"},
				{Player: 0, Sec: 12, Text: "u2"},
				{Player: 1, Sec: 900, Text: "gg"},
			}
			if len(completeRec.Chat) != len(want) {
				t.Fatalf("chat lines: got %v want %v", completeRec.Chat, want)
			}
			for i, line := range want {
				if completeRec.Chat[i] != line {
					t.Fatalf("chat[%d]: got %+v want %+v", i, completeRec.Chat[i], line)
				}
			}
			if len(ownRec.Chat) != 2 {
				t.Fatalf("own record chat mutated: %v", ownRec.Chat)
			}
		})
	}
}

func TestCompletePairBreaksOnRemoval(t *testing.T) {
	lib := newTestLibrary(t, library.Options{})
	own, complete := pairFixtures(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))
	lib.Add(0, own, complete)
	lib.Flush()

	lib.Remove(0, "/replays/game-complete.rep")
	lib.Flush()

	snap := lib.Snapshot()
	ownRec, ok := snap.ByPath("/replays/game.rep")
	if !ok {
		t.Fatal("own record missing")
	}
	if ownRec.Flags.Has(library.FlagSuperseded) {
		t.Fatal("own record still superseded after the complete copy was removed")
	}
	if !lib.View().Contains(ownRec.ID) {
		t.Fatal("own record not reinstated in the view")
	}
	if _, stillThere := snap.ByPath("/replays/game-complete.rep"); stillThere {
		t.Fatal("complete record not removed")
	}
}

func TestCompletePairIdenticalContentDoesNotSelfPair(t *testing.T) {
	lib := newTestLibrary(t, library.Options{})
	date := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	own := librarytest.Replay(
		librarytest.WithChecksum("same-bytes"),
		librarytest.WithDate(date),
		librarytest.WithPath("/replays/game.rep", date),
	)
	lib.Add(0, own)
	lib.Flush()
	lib.Alias(0, library.FileRef{Path: "/replays/game-complete.rep", Size: 100_000, ModTime: date}, own.Checksum)
	lib.Flush()

	snap := lib.Snapshot()
	rec, ok := snap.ByPath("/replays/game.rep")
	if !ok {
		t.Fatal("record missing")
	}
	if rec.Flags.Has(library.FlagSuperseded) {
		t.Fatal("record superseded by its own alias")
	}
	if len(rec.Paths) != 2 {
		t.Fatalf("paths: %v", rec.Paths)
	}
}

func TestMergeChatIdempotent(t *testing.T) {
	own, complete := pairFixtures(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))
	merged, changed := library.MergeChat(complete, own)
	if !changed {
		t.Fatal("first merge reported no change")
	}
	complete.Chat = merged
	if _, changed = library.MergeChat(complete, own); changed {
		t.Fatal("second merge reported a change")
	}
}
