package library_test

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
	"github.com/marianogappa/screpdb/internal/models"
)

type want struct{ player, team models.Outcome }

func outcomesOf(r *library.Replay) []want {
	var out []want
	for i := range r.Players {
		out = append(out, want{r.Players[i].PlayerOutcome(), r.Players[i].TeamOutcome()})
	}
	return out
}

func assertOutcomes(t *testing.T, r *library.Replay, expected ...want) {
	t.Helper()
	got := outcomesOf(r)
	if len(got) != len(expected) {
		t.Fatalf("got %d players, want %d", len(got), len(expected))
	}
	for i := range got {
		if got[i] != expected[i] {
			t.Errorf("%s: got %v/%v, want %v/%v (%s)", r.Players[i].Name, got[i].player, got[i].team, expected[i].player, expected[i].team, r.Players[i].PlayerOutcomeReason)
		}
	}
}

func setEngine(t *testing.T, lib *library.Library, r *library.Replay, data *library.EngineData) *library.Replay {
	t.Helper()
	lib.SetEngineData(r.Checksum, data)
	lib.Flush()
	got, _ := lib.Snapshot().ByChecksum(r.Checksum)
	return got
}

var (
	won     = want{models.OutcomeWon, models.OutcomeWon}
	lost    = want{models.OutcomeLost, models.OutcomeLost}
	unknown = want{models.OutcomeUnknown, models.OutcomeUnknown}
)

// The recording ends at the victory screen: the parser cannot see the last enemy die and calls the
// game undecided, with the saver conceding. The engine saw the defeat and the win.
func TestEngineResultsSettleAGameTheRecordingCutOff(t *testing.T) {
	lib := newTestLibrary(t, library.Options{})
	r := librarytest.Replay(
		librarytest.WithPlayer("saver", librarytest.Team(1), func(p *library.Player) { p.Flags |= library.PlayerResultLost }),
		librarytest.WithPlayer("ally", librarytest.Team(1)),
		librarytest.WithPlayer("enemy", librarytest.Team(2)),
		librarytest.WithPlayer("quitter", librarytest.Team(2), func(p *library.Player) { p.Flags |= library.PlayerResultLost }),
	)
	r.TeamOutcomeReason = "no winner"
	lib.Add(0, r)
	lib.Flush()

	got := setEngine(t, lib, r, &library.EngineData{ResultsKnown: true, Results: []library.EngineResult{
		{ReplayPlayerID: 3, Kind: library.EngineLeft, Frame: 4000},
		{ReplayPlayerID: 2, Kind: library.EngineDefeated, Frame: 25112},
		{ReplayPlayerID: 0, Kind: library.EngineWon, Frame: 25143},
		{ReplayPlayerID: 1, Kind: library.EngineWon, Frame: 25143},
	}})
	assertOutcomes(t, got, won, won, lost, lost)
	if got.TeamOutcomeReason != "the engine saw saver, ally win" {
		t.Errorf("reason %q", got.TeamOutcomeReason)
	}

	lib.ClearEngineData()
	lib.Flush()
	back, _ := lib.Snapshot().ByChecksum(r.Checksum)
	assertOutcomes(t, back, want{models.OutcomeLost, models.OutcomeUnknown}, unknown, unknown, want{models.OutcomeLost, models.OutcomeUnknown})
	if back.TeamOutcomeReason != "no winner" {
		t.Errorf("switching the engine off kept reason %q", back.TeamOutcomeReason)
	}
}

func TestEngineResultsIgnoredUnlessTrusted(t *testing.T) {
	lib := newTestLibrary(t, library.Options{})
	r := librarytest.Replay(librarytest.WithPlayer("a"), librarytest.WithPlayer("b"))
	lib.Add(0, r)
	lib.Flush()
	got := setEngine(t, lib, r, &library.EngineData{LowConfidence: true, Results: []library.EngineResult{{ReplayPlayerID: 0, Kind: library.EngineWon}}})
	assertOutcomes(t, got, unknown, unknown)
}

// The saver's exit is never recorded, so when the engine saw no winner the parser's concession stands.
func TestEngineKeepsTheParsersAnswerWhileSidesRemain(t *testing.T) {
	lib := newTestLibrary(t, library.Options{})
	r := librarytest.Replay(
		librarytest.WithPlayer("saver", librarytest.Loser()),
		librarytest.WithPlayer("rival", librarytest.Winner()),
	)
	r.TeamOutcomeReason = "the saver conceded"
	lib.Add(0, r)
	lib.Flush()
	got := setEngine(t, lib, r, &library.EngineData{ResultsKnown: true})
	assertOutcomes(t, got, lost, won)
	if got.TeamOutcomeReason != "the saver conceded" {
		t.Errorf("reason %q", got.TeamOutcomeReason)
	}
}

func TestEngineOverturnsAWinnerItSawDefeated(t *testing.T) {
	lib := newTestLibrary(t, library.Options{})
	r := librarytest.Replay(
		librarytest.WithPlayer("credited", librarytest.Winner()),
		librarytest.WithPlayer("survivor", librarytest.Loser()),
		librarytest.WithPlayer("third", librarytest.Loser()),
	)
	lib.Add(0, r)
	lib.Flush()
	got := setEngine(t, lib, r, &library.EngineData{ResultsKnown: true, Results: []library.EngineResult{
		{ReplayPlayerID: 0, Kind: library.EngineDefeated, Frame: 2000},
	}})
	assertOutcomes(t, got, lost, want{models.OutcomeLost, models.OutcomeUnknown}, want{models.OutcomeLost, models.OutcomeUnknown})
}

func TestEngineLeavesSaverDisconnectsAndComputerGamesAlone(t *testing.T) {
	lib := newTestLibrary(t, library.Options{})
	disconnect := librarytest.Replay(librarytest.WithFlags(library.FlagSaverDisconnect), librarytest.WithPlayer("a"), librarytest.WithPlayer("b"))
	computer := librarytest.Replay(
		librarytest.WithPlayer("a", func(p *library.Player) { p.Flags |= library.PlayerResultNotScored }),
		librarytest.WithPlayer("cpu", librarytest.Computer(), func(p *library.Player) { p.Flags |= library.PlayerResultNotScored }),
	)
	lib.Add(0, disconnect, computer)
	lib.Flush()
	left := &library.EngineData{ResultsKnown: true, Results: []library.EngineResult{{ReplayPlayerID: 1, Kind: library.EngineLeft}}}
	assertOutcomes(t, setEngine(t, lib, disconnect, left), unknown, unknown)
	notScored := want{models.OutcomeNotScored, models.OutcomeNotScored}
	assertOutcomes(t, setEngine(t, lib, computer, left), notScored, notScored)
}

func TestEngineDoesNotScoreAGameOfOneSide(t *testing.T) {
	lib := newTestLibrary(t, library.Options{})
	r := librarytest.Replay(librarytest.WithPlayer("alone"), librarytest.WithPlayer("Watcher", librarytest.Observer()))
	lib.Add(0, r)
	lib.Flush()
	got := setEngine(t, lib, r, &library.EngineData{ResultsKnown: true, Results: []library.EngineResult{{ReplayPlayerID: 0, Kind: library.EngineWon}}})
	assertOutcomes(t, got, unknown, unknown)
}

func TestBnetSettlesOnlyUndecidedGames(t *testing.T) {
	lib := newTestLibrary(t, library.Options{})
	undecided := librarytest.Replay(
		librarytest.WithPlayer("me", librarytest.Team(1), func(p *library.Player) { p.Flags |= library.PlayerResultLost }),
		librarytest.WithPlayer("ally", librarytest.Team(1)),
		librarytest.WithPlayer("foe", librarytest.Team(2)),
	)
	decided := librarytest.Replay(librarytest.WithPlayer("me", librarytest.Loser()), librarytest.WithPlayer("foe", librarytest.Winner()))
	lib.Add(0, undecided, decided)
	lib.Flush()
	results := []library.BnetResult{{Key: "me", Outcome: models.OutcomeWon}}
	lib.SetBnetResults(undecided.Checksum, results, "game 1")
	lib.SetBnetResults(decided.Checksum, results, "game 2")
	lib.Flush()

	got, _ := lib.Snapshot().ByChecksum(undecided.Checksum)
	assertOutcomes(t, got, won, won, lost)
	if got.BnetOutcomeSource != "game 1" || got.TeamOutcomeReason != "Battle.net names me as winning" {
		t.Errorf("source %q reason %q", got.BnetOutcomeSource, got.TeamOutcomeReason)
	}
	kept, _ := lib.Snapshot().ByChecksum(decided.Checksum)
	assertOutcomes(t, kept, lost, won)

	// The engine outranks Battle.net: once it settles the game, the Battle.net layer has nothing to add.
	engine := setEngine(t, lib, undecided, &library.EngineData{ResultsKnown: true, Results: []library.EngineResult{
		{ReplayPlayerID: 2, Kind: library.EngineWon, Frame: 100},
	}})
	assertOutcomes(t, engine, lost, lost, won)
	if engine.BnetOutcomeSource != "" {
		t.Errorf("source %q", engine.BnetOutcomeSource)
	}
}

func TestBnetLossNamesTheWinnerOfAOneOnOne(t *testing.T) {
	lib := newTestLibrary(t, library.Options{})
	r := librarytest.Replay(librarytest.WithPlayer("me"), librarytest.WithPlayer("foe"))
	lib.Add(0, r)
	lib.SetBnetResults(r.Checksum, []library.BnetResult{{Key: "me", Outcome: models.OutcomeLost}}, "game")
	lib.Flush()
	got, _ := lib.Snapshot().ByChecksum(r.Checksum)
	assertOutcomes(t, got, lost, won)
}
