package parser

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
)

// scenario builds a two-sided game whose endgame has the given shape, in
// seconds before the game ends.
type scenario struct {
	durationSec  int
	credSilence  int // credited side's last action
	credDrought  int // credited side's last Build/Train/Upgrade
	rivalSilence int
	rivalDrought int
	saverIsCred  bool
}

func (s scenario) build() ([]*models.Player, []*models.Command, *byte) {
	cred := &models.Player{PlayerID: 1, Name: "cred", Team: 1, Type: models.PlayerTypeHuman, TeamOutcome: models.OutcomeWon}
	rival := &models.Player{PlayerID: 2, Name: "rival", Team: 2, Type: models.PlayerTypeHuman}
	players := []*models.Player{cred, rival}

	at := func(p *models.Player, before int, action string) *models.Command {
		sec := max(s.durationSec-before, 0)
		return &models.Command{Player: p, SecondsFromGameStart: sec, ActionType: action}
	}
	commands := []*models.Command{
		at(cred, s.durationSec, "Build"),
		at(rival, s.durationSec, "Build"),
		at(cred, s.credDrought, "Train"),
		at(rival, s.rivalDrought, "Train"),
		at(cred, s.credSilence, "Right Click"),
		at(rival, s.rivalSilence, "Right Click"),
	}
	saver := rival.PlayerID
	if s.saverIsCred {
		saver = cred.PlayerID
	}
	return players, commands, &saver
}

func creditedNames(players []*models.Player) []string {
	var out []string
	for _, p := range players {
		if p.TeamOutcome == models.OutcomeWon {
			out = append(out, p.Name)
		}
	}
	return out
}

// Every hand-labelled game, reduced to the endgame shape the correction reads.
// want is the verdict from watching the replay: OVERTURN and KEEP discriminate,
// EITHER means both credits were judged defensible, UNDET that the game never
// resolved and AMBIG that the watcher could not tell.
func TestCorrectEliminatedWinners_LabelledShapes(t *testing.T) {
	cases := []struct {
		name                                         string
		want                                         string
		credSil, rivalSil, credDrought, rivalDrought int
		saverIsCred                                  bool
		fires                                        bool
	}{
		{"wr01", "OVERTURN", 5, 1, 60, 3, false, true},
		{"wr02", "OVERTURN", 230, 2, 237, 3, false, true},
		{"wr03", "OVERTURN", 55, 3, 78, 3, false, true},
		{"wr04", "KEEP", 2, 4, 17, 12, true, false},
		{"wr05", "OVERTURN", 10, 1, 59, 18, false, true},
		{"wr06", "OVERTURN", 7, 1, 23, 5, false, true},
		{"wr07", "OVERTURN", 15, 5, 80, 5, false, true},
		{"wr08", "KEEP", 3, 7, 3, 7, true, false},
		{"wr09", "KEEP", 1, 12, 62, 34, true, false},
		{"wr10", "KEEP", 5, 5, 65, 46, false, false},
		{"wr12", "KEEP", 1, 12, 3, 127, true, false},
		{"wr13", "KEEP", 2, 1, 2, 6, false, false},
		{"zvp5win", "OVERTURN", 13, 2, 24, 4, false, true},
		{"zvp6win", "OVERTURN", 8, 3, 29, 3, false, true},
		{"zvp7win", "OVERTURN", 9, 1, 46, 2, false, true},
		{"zvz3win", "OVERTURN", 6, 2, 18, 6, false, false},
		{"zvz4win", "OVERTURN", 7, 0, 44, 0, false, true},
		{"zvzwinvsb", "OVERTURN", 5, 2, 14, 11, false, false},
		{"zvp9loss", "KEEP", 1, 3, 49, 3, false, false},
		{"zvt4lose", "KEEP", 0, 2, 8, 5, false, false},
		{"wx01", "KEEP", 4, 6, 22, 9, true, false},
		{"wx02", "AMBIG", 12, 1, 12, 1, false, false},
		{"wx03", "KEEP", 2, 0, 9, 2, false, false},
		{"wx04", "KEEP", 2, 0, 30, 13, false, false},
		{"wx05", "KEEP", 0, 9, 3, 36, false, false},
		{"wx06", "KEEP", 0, 1, 2, 26, false, false},
		{"wx07", "KEEP", 6, 12, 15, 19, false, false},
		{"wx08", "KEEP", 3, 8, 5, 20, false, false},
		{"wx09", "KEEP", 10, 9, 15, 38, false, false},
		{"wx10", "KEEP", 2, 0, 31, 32, false, false},
		{"wx11", "KEEP", 2, 2, 32, 2, false, false},
		{"wx12", "KEEP", 3, 3, 15, 9, true, false},
		{"wx13", "KEEP", 4, 7, 7, 7, false, false},
		{"wx14", "KEEP", 1, 3, 1, 4, false, false},
		{"wx15", "KEEP", 2, 1, 9, 32, false, false},
		{"wx16", "KEEP", 2, 2, 68, 7, false, false},
		{"wx17", "UNDET", 0, 1, 4, 1, false, false},
		{"wx18", "KEEP", 1, 4, 89, 4, false, false},
		{"wx19", "KEEP", 0, 1, 0, 22, false, false},
		{"wx20", "KEEP", 3, 12, 116, 45, true, false},
		{"wx21", "EITHER", 3, 256, 45, 285, true, false},
		{"wx22", "EITHER", 14, 6, 80, 6, false, true},
		{"wx23", "KEEP", 107, 23, 107, 33, true, false},
		{"wz01", "KEEP", 1, 2, 6, 2, false, false},
		{"wz02", "UNDET", 0, 0, 0, 10, false, false},
		{"wz03", "KEEP", 0, 0, 5, 1, false, false},
		{"wz04", "OVERTURN", 4, 3, 73, 15, false, false},
		{"wz05", "KEEP", 4, 247, 24, 251, true, false},
		{"wz06", "KEEP", 2, 53, 32, 78, true, false},
		{"wz07", "KEEP", 1, 4643, 9, 4690, true, false},
		{"wz08", "EITHER", 15, 1, 36, 3, false, true},
		{"wz09", "AMBIG", 1, 0, 10, 6, false, false},
		{"wz10", "EITHER", 25, 1, 39, 14, false, true},
		{"wz11", "EITHER", 31, 8, 31, 8, false, true},
		{"wz12", "AMBIG", 5, 6, 32, 9, false, false},
		{"wz13", "EITHER", 6, 0, 23, 1, false, true},
		{"wz14", "EITHER", 4, 1, 16, 4, false, false},
		{"wz15", "UNDET", 7, 1, 23, 10, false, false},
		{"wz16", "EITHER", 5, 1, 12, 2, false, false},
	}

	var overturnHit, overturnTotal, keepFalsePositives, keepTotal int
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := scenario{
				durationSec: 5000, credSilence: c.credSil, credDrought: c.credDrought,
				rivalSilence: c.rivalSil, rivalDrought: c.rivalDrought, saverIsCred: c.saverIsCred,
			}
			players, commands, saver := s.build()
			if got, _ := CorrectEliminatedWinners(players, commands, s.durationSec, nil, saver); got != c.fires {
				t.Fatalf("fired=%v, want %v", got, c.fires)
			}
		})
		switch c.want {
		case "OVERTURN":
			overturnTotal++
			if c.fires {
				overturnHit++
			}
		case "KEEP":
			keepTotal++
			if c.fires {
				keepFalsePositives++
			}
		}
	}

	// The property the rule is allowed to trade against: it may miss, but it must
	// never turn a correct credit into a wrong one.
	if keepFalsePositives != 0 {
		t.Errorf("false positives: %d of %d games the credited team really won", keepFalsePositives, keepTotal)
	}
	if overturnHit != 10 || overturnTotal != 13 {
		t.Errorf("overturns: %d of %d, want 10 of 13", overturnHit, overturnTotal)
	}
}

func TestCorrectEliminatedWinners_Gates(t *testing.T) {
	// A shape comfortably past every threshold.
	fires := scenario{durationSec: 600, credSilence: 20, credDrought: 90, rivalSilence: 1, rivalDrought: 2}

	tests := []struct {
		name string
		mut  func(*scenario)
		want bool
	}{
		{"clear elimination", func(*scenario) {}, true},
		{"saver on credited side", func(s *scenario) { s.saverIsCred = true }, false},
		{"action lead one short", func(s *scenario) { s.credSilence = s.rivalSilence + 2 }, false},
		{"action lead exactly met", func(s *scenario) { s.credSilence = s.rivalSilence + 3 }, true},
		{"production lead one short", func(s *scenario) { s.rivalDrought = 10; s.credDrought = 24 }, false},
		{"production lead exactly met", func(s *scenario) { s.rivalDrought = 10; s.credDrought = 25 }, true},
		{"production floor one short", func(s *scenario) { s.credDrought = 19; s.rivalDrought = 1 }, false},
		{"production floor exactly met", func(s *scenario) { s.credDrought = 20; s.rivalDrought = 1 }, true},
		{"game too short", func(s *scenario) { s.durationSec = 119 }, false},
		{"rival idled longer", func(s *scenario) { s.rivalSilence = 30; s.rivalDrought = 120 }, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := fires
			tc.mut(&s)
			players, commands, saver := s.build()
			if got, _ := CorrectEliminatedWinners(players, commands, s.durationSec, nil, saver); got != tc.want {
				t.Fatalf("fired=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestCorrectEliminatedWinners_ReassignsToSurvivors(t *testing.T) {
	s := scenario{durationSec: 600, credSilence: 20, credDrought: 90, rivalSilence: 1, rivalDrought: 2}
	players, commands, saver := s.build()
	if fired, _ := CorrectEliminatedWinners(players, commands, s.durationSec, nil, saver); !fired {
		t.Fatal("expected the correction to fire")
	}
	got := creditedNames(players)
	if len(got) != 1 || got[0] != "rival" {
		t.Fatalf("credited %v, want [rival]", got)
	}
}

// Running the correction on its own output must change nothing: the side it
// credits is the one that was still acting, so the gates can no longer pass.
func TestCorrectEliminatedWinners_Idempotent(t *testing.T) {
	s := scenario{durationSec: 600, credSilence: 20, credDrought: 90, rivalSilence: 1, rivalDrought: 2}
	players, commands, saver := s.build()
	if fired, _ := CorrectEliminatedWinners(players, commands, s.durationSec, nil, saver); !fired {
		t.Fatal("expected the correction to fire")
	}
	if fired, _ := CorrectEliminatedWinners(players, commands, s.durationSec, nil, saver); fired {
		t.Fatal("fired a second time on its own output")
	}
}

func TestCorrectEliminatedWinners_IgnoresChatAndLeave(t *testing.T) {
	// An eliminated player can still type and still quit; neither may count as
	// evidence that their side was alive.
	s := scenario{durationSec: 600, credSilence: 20, credDrought: 90, rivalSilence: 1, rivalDrought: 2}
	players, commands, saver := s.build()
	cred := players[0]
	commands = append(commands,
		&models.Command{Player: cred, SecondsFromGameStart: s.durationSec, ActionType: "Chat"},
		&models.Command{Player: cred, SecondsFromGameStart: s.durationSec, ActionType: "Leave Game"},
	)
	if fired, _ := CorrectEliminatedWinners(players, commands, s.durationSec, nil, saver); !fired {
		t.Fatal("chat or leave was treated as proof the credited side was alive")
	}
}

func TestCorrectEliminatedWinners_NoOpCases(t *testing.T) {
	s := scenario{durationSec: 600, credSilence: 20, credDrought: 90, rivalSilence: 1, rivalDrought: 2}

	// With nobody credited the procedure declined, but a corpse still issues no
	// Leave Game: the same evidence resolves it, crediting the side that is not
	// production-dead.
	t.Run("credits the survivor when the game was left undecided", func(t *testing.T) {
		players, commands, saver := s.build()
		players[0].TeamOutcome = models.OutcomeUnknown
		if fired, _ := CorrectEliminatedWinners(players, commands, s.durationSec, nil, saver); !fired {
			t.Fatal("did not resolve an undecided game with a clear elimination")
		}
		if got := creditedNames(players); len(got) != 1 || got[0] != "rival" {
			t.Fatalf("credited %v, want [rival]", got)
		}
	})

	t.Run("leaves an undecided game alone when neither side is dead", func(t *testing.T) {
		even := scenario{durationSec: 600, credSilence: 2, credDrought: 5, rivalSilence: 1, rivalDrought: 4}
		players, commands, saver := even.build()
		players[0].TeamOutcome = models.OutcomeUnknown
		if fired, _ := CorrectEliminatedWinners(players, commands, even.durationSec, nil, saver); fired {
			t.Fatal("credited a winner with no elimination evidence")
		}
	})

	t.Run("no rival activity", func(t *testing.T) {
		players, commands, saver := s.build()
		var credOnly []*models.Command
		for _, c := range commands {
			if c.Player.PlayerID == players[0].PlayerID {
				credOnly = append(credOnly, c)
			}
		}
		if fired, _ := CorrectEliminatedWinners(players, credOnly, s.durationSec, nil, saver); fired {
			t.Fatal("fired with no rival to compare against")
		}
	})

	t.Run("unknown saver", func(t *testing.T) {
		players, commands, _ := s.build()
		if fired, _ := CorrectEliminatedWinners(players, commands, s.durationSec, nil, nil); !fired {
			t.Fatal("a nil saver should not block the correction")
		}
	})

	t.Run("computers are not evidence", func(t *testing.T) {
		players, commands, saver := s.build()
		bot := &models.Player{PlayerID: 3, Name: "bot", Team: 3, Type: "Computer"}
		players = append(players, bot)
		commands = append(commands, &models.Command{
			Player: bot, SecondsFromGameStart: s.durationSec, ActionType: "Train",
		})
		// The bot out-lives everyone, but only human liveness decides the comparison.
		if fired, _ := CorrectEliminatedWinners(players, commands, s.durationSec, nil, saver); !fired {
			t.Fatal("a computer's activity changed the verdict")
		}
	})

	t.Run("observers are untouched", func(t *testing.T) {
		players, commands, saver := s.build()
		obs := &models.Player{PlayerID: 4, Name: "obs", Team: 4, Type: models.PlayerTypeHuman, IsObserver: true}
		players = append(players, obs)
		if fired, _ := CorrectEliminatedWinners(players, commands, s.durationSec, nil, saver); !fired {
			t.Fatal("expected the correction to fire")
		}
		if obs.TeamOutcome == models.OutcomeWon {
			t.Fatal("credited an observer")
		}
	})
}
