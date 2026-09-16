package spec

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/marianogappa/screpdb/internal/parser"
)

func init() {
	registerGameResultEvidence()
	registerGameResultProcedure()
	registerEliminationGates()
}

// registerGameResultEvidence states what each source of truth can and cannot
// answer. Everything downstream follows from this table, so it comes first.
func registerGameResultEvidence() {
	Register(Section{
		Key:   "30-game-result-evidence",
		Title: "Who won: what the evidence can answer",
		Intro: "Every game has two results per player, not one: what happened to the " +
			"player, and what happened to their side. They coincide only in 1v1; anywhere " +
			"else a player can lose a game their team goes on to win, by leaving before " +
			"it ends. screpdb derives both. The personal result is available far more " +
			"often — 84% of a 3,364-replay corpus against 64% for the team result — " +
			"because it needs only that player's own exit, while the team question needs " +
			"every coalition resolved. StarCraft never records a defeat. A replay is a command log, and being " +
			"destroyed issues no command, so \"who lost\" is never written down — only " +
			"\"who chose to leave\". Two further gaps follow from that. The replay saver's " +
			"own departure is not recorded either, because the recording ends at that " +
			"moment; and any game that outlives the recording has an outcome the file " +
			"cannot contain. Battle.net fills some of the gap, but it answers a different " +
			"question: it reports what happened to one account, not who won the game. " +
			"Each row below is a fact screpdb either has, infers, or must decline to state.",
		Columns: []string{"Fact", "Recorded in the replay", "How screpdb gets it"},
		Rows: func() [][]string {
			return [][]string{
				{"A player quit", "Yes — a Leave Game command", "Read directly"},
				{"When they quit", "Yes — the command's frame", "Read directly"},
				{"A player was destroyed", "No — elimination issues no command", "Inferred: an eliminated side stops acting and stops producing while its rival does neither"},
				{"The replay saver quit", "No — the recording ends at that instant", "Assumed, but never used as evidence about their team: they are excluded from every survivor count"},
				{"The saver's own outcome", "No", "Battle.net replay_result, for that account only"},
				{"Who won, after the recording ends", "No", "Battle.net, only when some account reports a win"},
				{"Who won, in a game nobody finished", "No", "Not derivable — no winner is credited"},
				{"That a player lost, in a game with no winner", "No", "Inferred: they quit, or the recording ended at their exit, while a rival was still in the game"},
				{"That a player lost the connection", "Partly — their leave carries a Dropped reason", "Read directly, or inferred for the saver from the phantom leave cluster their drop writes"},
				{"Who won, after the saver dropped", "No", "Not derivable — the game continues off the recording"},
			}
		},
		Verify: func() error {
			// The inference in row 3 is real code, not aspiration: assert the gates
			// that implement it exist and are ordered sanely.
			if parser.EliminatedActionLeadSec <= 0 || parser.EliminatedProductionLeadSec <= 0 {
				return fmt.Errorf("elimination inference gates must be positive")
			}
			if parser.EliminatedProductionMinSec < parser.EliminatedProductionLeadSec {
				return fmt.Errorf("production floor (%d) must not be below the production lead (%d)",
					parser.EliminatedProductionMinSec, parser.EliminatedProductionLeadSec)
			}
			return nil
		},
	})
}

// registerGameResultProcedure is the decision procedure itself, in the order
// the code applies it.
func registerGameResultProcedure() {
	Register(Section{
		Key:   "31-game-result-procedure",
		Title: "Who won: the decision procedure",
		Intro: "Applied in order; the first step that yields an answer wins, and reaching " +
			"the end means no winner is credited. \"Coalition\" is the end-of-game " +
			"alliance clique in a melee game with more than two active players, and the " +
			"static team otherwise — winners may therefore span two displayed teams. The " +
			"replay saver is excluded from every survivor count: StarCraft records no " +
			"Leave Game for them because the recording ends at their exit, so their " +
			"departure is evidence about them and never about their team. screpdb does " +
			"not use screp's WinnerTeam, whose \"largest remaining team wins\" credits a " +
			"side whenever that unrecorded exit leaves the tally uneven. Declining to " +
			"answer is a valid outcome and a common one — about 40% of a 3,364-replay " +
			"corpus — because a game the recording does not see the end of has no winner " +
			"to report. A disconnect is not a loss and not an unknown: it is its own " +
			"outcome, and the only one a dropped saver's recording establishes.",
		Columns: []string{"Step", "Condition", "Outcome"},
		Rows: func() [][]string {
			return [][]string{
				{"1", "Only one side, because every opponent is a computer, or none at all", "Nobody wins — there is nothing to compare. An end-of-game alliance is the exception: StarCraft will not start a one-sided game, so a single coalition there means the survivors allied into it"},
				{"2", "Exactly one coalition still holds a player who never left", "That coalition wins — including the allied case, where it holds everyone left in the game"},
				{"3", "No coalition does — everyone but the saver quit", "The saver's coalition wins: they were the last player in the game"},
				{"4", "...and no saver is known either, typically an observer-saved replay", "The last leaver's coalition wins"},
				{"5", "Two or more coalitions still hold a player, and all but one show the elimination signature below", "The remaining coalition wins — a destroyed player issues no Leave Game"},
				{"6", "A coalition credited by 2, 3 or 4 itself shows that signature, and does not contain the saver", "Overturned: the last-acting rival's coalition wins instead"},
				{"7", "None of the above", "Nobody wins"},
				{"last", "The recording ends in a cluster of leaves caused by the saver losing the connection", "Overrides everything above: the saver's own result is the disconnect, and nobody's is known — the game went on and resolved, just not on this recording"},
			}
		},
		Verify: func() error {
			// Steps 6 and 7 rest on measuring an endgame, so they must refuse to
			// evaluate games too short to have one.
			if parser.EliminatedMinDurationSec <= 0 {
				return fmt.Errorf("the elimination signature must have a minimum duration guard")
			}
			return nil
		},
	})
}

// registerEliminationGates publishes the elimination signature's thresholds from the live
// constants, so the document cannot drift from the code that applies them.
func registerEliminationGates() {
	Register(Section{
		Key:   "32-elimination-gates",
		Title: "Who won: the elimination signature",
		Intro: "Steps 5 and 6 of the procedure. A coalition that was wiped out cannot act after " +
			"its last unit dies, and cannot Build, Train or Upgrade once its production " +
			"buildings are gone — while the coalition that beat it keeps doing both until " +
			"the recording stops. All four gates must hold. Times are seconds, measured " +
			"back from the end of the recording, and each side is scored by its liveliest " +
			"member. Chat and Leave Game are excluded from \"acting\": a player with " +
			"nothing left on the map can still type, and can still quit. Calibrated on 59 " +
			"hand-labelled games; it is tuned to miss rather than to misfire, so it never " +
			"replaces a correct credit with a wrong one.",
		Columns: []string{"Gate", "Requirement", "Value"},
		Rows: func() [][]string {
			return [][]string{
				{"Saver provenance", "When overturning (step 6) the credited coalition must not contain the replay saver — the recording ends at their exit, so their silence measures their own departure, not their team's. Step 5 needs no such guard: its evidence is the losers being provably dead, never the winner looking alive", "—"},
				{"Action lead", "The credited coalition stopped acting at least this long before the last surviving rival did", strconv.Itoa(parser.EliminatedActionLeadSec)},
				{"Production lead", "...and stopped producing at least this long before that rival did", strconv.Itoa(parser.EliminatedProductionLeadSec)},
				{"Production floor", "...and had produced nothing at all for at least this long", strconv.Itoa(parser.EliminatedProductionMinSec)},
				{"Minimum duration", "Games shorter than this are not evaluated; the endgame is too compressed for either signal to separate", strconv.Itoa(parser.EliminatedMinDurationSec)},
				{"Counts as production", sortedProductionTypes(), "—"},
			}
		},
		Verify: func() error {
			if parser.EliminatedActionLeadSec != 3 {
				return fmt.Errorf("action lead is %d, spec documents 3", parser.EliminatedActionLeadSec)
			}
			if parser.EliminatedProductionLeadSec != 15 {
				return fmt.Errorf("production lead is %d, spec documents 15", parser.EliminatedProductionLeadSec)
			}
			if parser.EliminatedProductionMinSec != 20 {
				return fmt.Errorf("production floor is %d, spec documents 20", parser.EliminatedProductionMinSec)
			}
			if parser.EliminatedMinDurationSec != 120 {
				return fmt.Errorf("minimum duration is %d, spec documents 120", parser.EliminatedMinDurationSec)
			}
			for _, required := range []string{"Build", "Train", "Train Fighter", "Upgrade", "Tech", "Unit Morph", "Building Morph", "Land"} {
				if _, ok := parser.ProductionActionTypes[required]; !ok {
					return fmt.Errorf("%q is missing from the production action types", required)
				}
			}
			if n := len(parser.ProductionActionTypes); n != 8 {
				return fmt.Errorf("production action types has %d entries, spec documents 8", n)
			}
			return nil
		},
	})
}

func sortedProductionTypes() string {
	names := make([]string, 0, len(parser.ProductionActionTypes))
	for name := range parser.ProductionActionTypes {
		names = append(names, name)
	}
	sort.Strings(names)
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}
