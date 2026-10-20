package library

import (
	"reflect"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/models"
)

func TestRaceParseAndString(t *testing.T) {
	cases := map[string]Race{"zerg": RaceZerg, " Terran ": RaceTerran, "PROTOSS": RaceProtoss, "random": RaceUnknown, "": RaceUnknown}
	for in, want := range cases {
		if got := ParseRace(in); got != want {
			t.Errorf("ParseRace(%q) = %d, want %d", in, got, want)
		}
	}
	for _, r := range []Race{RaceZerg, RaceTerran, RaceProtoss} {
		if ParseRace(r.String()) != r {
			t.Errorf("%d does not round trip through %q", r, r.String())
		}
	}
	if RaceUnknown.String() != "" {
		t.Errorf("unknown race reads %q", RaceUnknown.String())
	}
}

func TestPlayerTypeParseAndString(t *testing.T) {
	for name, want := range playerTypeNames {
		if got := ParsePlayerType("  " + name + " "); got != want {
			t.Errorf("ParsePlayerType(%q) = %d, want %d", name, got, want)
		}
		if got := want.String(); got != name {
			t.Errorf("%d.String() = %q, want %q", want, got, name)
		}
	}
	if ParsePlayerType("Human") != PlayerTypeHuman || ParsePlayerType("Computer Controlled") != PlayerTypeComputerControlled {
		t.Error("parsing is not case-insensitive")
	}
	if ParsePlayerType("alien") != PlayerTypeUnknown || PlayerTypeUnknown.String() != "" {
		t.Error("unknown player type does not map to the zero value")
	}
	computers := map[PlayerType]bool{PlayerTypeComputer: true, PlayerTypeComputerControlled: true}
	for _, pt := range playerTypeNames {
		if pt.IsComputer() != computers[pt] {
			t.Errorf("%s IsComputer = %v", pt, pt.IsComputer())
		}
	}
}

func TestMapKindParseAndString(t *testing.T) {
	for _, k := range []MapKind{MapKindRegular, MapKindMoney, MapKindUseMapSettings} {
		if got := ParseMapKind(" " + k.String() + " "); got != k {
			t.Errorf("%q parses as %d, want %d", k.String(), got, k)
		}
	}
	if ParseMapKind("money") != MapKindUnknown {
		t.Error("map kinds are case-sensitive")
	}
	if MapKindUnknown.String() != "" {
		t.Errorf("unknown map kind reads %q", MapKindUnknown.String())
	}
}

func TestReplayPathAccessors(t *testing.T) {
	var empty Replay
	if empty.Path() != "" || empty.FileName() != "." || !empty.ModTime().IsZero() {
		t.Fatalf("empty replay: path %q name %q mod %v", empty.Path(), empty.FileName(), empty.ModTime())
	}
	mod := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	r := Replay{Paths: []FileRef{{Path: "/replays/sub/game.rep", ModTime: mod}, {Path: "/old/game.rep"}}}
	if r.Path() != "/replays/sub/game.rep" || r.FileName() != "game.rep" || !r.ModTime().Equal(mod) {
		t.Fatalf("path %q name %q mod %v", r.Path(), r.FileName(), r.ModTime())
	}
}

func TestFingerprintEligible(t *testing.T) {
	cases := []struct {
		kind  MapKind
		flags Flags
		want  bool
	}{
		{MapKindRegular, FlagIsOneOnOne, true},
		{MapKindUnknown, FlagIsOneOnOne | FlagIsAutosave, true},
		{MapKindMoney, FlagIsOneOnOne, false},
		{MapKindRegular, FlagTeamStacking, false},
		{MapKindRegular, 0, false},
	}
	for _, c := range cases {
		r := Replay{MapKind: c.kind, Flags: c.flags}
		if got := r.FingerprintEligible(); got != c.want {
			t.Errorf("kind %s flags %b: got %v, want %v", c.kind, c.flags, got, c.want)
		}
	}
}

func TestPlayerOutcomes(t *testing.T) {
	cases := []struct {
		flags        PlayerFlags
		player, team models.Outcome
	}{
		{0, models.OutcomeUnknown, models.OutcomeUnknown},
		{PlayerResultWon | TeamResultWon, models.OutcomeWon, models.OutcomeWon},
		{PlayerResultLost | TeamResultWon, models.OutcomeLost, models.OutcomeWon},
		{PlayerResultDisconnected | TeamResultLost, models.OutcomeDisconnected, models.OutcomeLost},
		{PlayerResultNotScored | TeamResultWon, models.OutcomeNotScored, models.OutcomeNotScored},
		{PlayerResultNotScored | PlayerResultDisconnected, models.OutcomeNotScored, models.OutcomeNotScored},
	}
	for _, c := range cases {
		p := Player{Flags: c.flags}
		if got := p.PlayerOutcome(); got != c.player {
			t.Errorf("flags %b: player outcome %v, want %v", c.flags, got, c.player)
		}
		if got := p.TeamOutcome(); got != c.team {
			t.Errorf("flags %b: team outcome %v, want %v", c.flags, got, c.team)
		}
	}
}

func TestOutcomeFlagsRoundTrip(t *testing.T) {
	for _, o := range []models.Outcome{models.OutcomeUnknown, models.OutcomeWon, models.OutcomeLost, models.OutcomeDisconnected, models.OutcomeNotScored} {
		p := Player{Flags: OutcomeFlags(o, PlayerResultWon, PlayerResultLost)}
		if got := p.PlayerOutcome(); got != o {
			t.Errorf("player %v round trips as %v", o, got)
		}
	}
	team := map[models.Outcome]PlayerFlags{
		models.OutcomeWon:          TeamResultWon,
		models.OutcomeLost:         TeamResultLost,
		models.OutcomeDisconnected: 0,
		models.OutcomeNotScored:    0,
		models.OutcomeUnknown:      0,
	}
	for o, want := range team {
		if got := OutcomeFlags(o, TeamResultWon, TeamResultLost); got != want {
			t.Errorf("team %v: got %b, want %b", o, got, want)
		}
	}
	if got := OutcomeFlags(models.Outcome(99), PlayerResultWon, PlayerResultLost); got != 0 {
		t.Errorf("unknown outcome encodes as %b", got)
	}
}

func TestPlayerKindPredicates(t *testing.T) {
	human := Player{Type: PlayerTypeHuman}
	ai := Player{Type: PlayerTypeComputerControlled}
	if !human.IsHuman() || human.IsComputer() || ai.IsHuman() || !ai.IsComputer() {
		t.Fatal("human/computer predicates disagree with the player type")
	}
}

func TestHasNonObserverComputer(t *testing.T) {
	r := Replay{Players: []Player{{Type: PlayerTypeHuman}, {Type: PlayerTypeComputer, Flags: PlayerObserver}}}
	if r.HasNonObserverComputer() {
		t.Fatal("an observing computer counted")
	}
	r.Players = append(r.Players, Player{Type: PlayerTypeComputer})
	if !r.HasNonObserverComputer() {
		t.Fatal("a playing computer did not count")
	}
}

func TestGameEventDetailAccessors(t *testing.T) {
	var bare GameEvent
	if bare.AttackUnits() != nil || bare.CastCounts() != nil || bare.Payload() != nil {
		t.Fatal("an event without detail returned detail")
	}
	e := GameEvent{Detail: &EventDetail{AttackUnits: []uint16{1, 2}, CastCounts: []CastCount{{Order: 3, Count: 4}}, Payload: []byte("x")}}
	if !reflect.DeepEqual(e.AttackUnits(), []uint16{1, 2}) || !reflect.DeepEqual(e.CastCounts(), []CastCount{{Order: 3, Count: 4}}) || string(e.Payload()) != "x" {
		t.Fatalf("detail accessors: %v %v %q", e.AttackUnits(), e.CastCounts(), e.Payload())
	}
}

func TestProdKindString(t *testing.T) {
	want := map[ProdKind]string{
		ProdTrain: "Train", ProdUnitMorph: "Unit Morph", ProdBuildingMorph: "Building Morph", ProdBuild: "Build",
		ProdTech: "Tech", ProdUpgrade: "Upgrade", ProdCast: "Targeted Order", 0: "", 99: "",
	}
	for k, s := range want {
		if k.String() != s {
			t.Errorf("%d.String() = %q, want %q", k, k.String(), s)
		}
	}
}

func TestProdColumnsSortClipAndSubjectName(t *testing.T) {
	tech := Techs.Intern("types_more_test tech")
	unit := Units.Intern("types_more_test unit")
	var p ProdColumns
	p.Append(30, 1, ProdTech, tech, 1)
	p.Append(10, 2, ProdTrain, unit, 2)
	p.Append(30, 3, ProdTrain, unit, 3)
	p.Append(10, 4, ProdTrain, unit, 4)
	p.SortBySecond()
	if !reflect.DeepEqual(p.Sec, []uint16{10, 10, 30, 30}) || !reflect.DeepEqual(p.Player, []uint8{2, 4, 1, 3}) || !reflect.DeepEqual(p.Count, []uint8{2, 4, 1, 3}) {
		t.Fatalf("sort is not stable by second: %+v", p)
	}
	before := p
	p.Clip()
	if p.Len() != 4 || !reflect.DeepEqual(p, before) {
		t.Fatalf("clip changed the data: %+v", p)
	}
	if &p.Sec[0] == &before.Sec[0] || &p.Player[0] == &before.Player[0] || &p.Kind[0] == &before.Kind[0] || &p.Subject[0] == &before.Subject[0] || &p.Count[0] == &before.Count[0] {
		t.Fatal("clip kept a column's backing array")
	}
	if p.SubjectName(2) != "types_more_test tech" || p.SubjectName(0) != "types_more_test unit" {
		t.Fatalf("subject names %q %q", p.SubjectName(2), p.SubjectName(0))
	}
}

func TestClamps(t *testing.T) {
	u16 := map[int]uint16{-5: 0, 0: 0, 1234: 1234, 0xFFFF: 0xFFFF, 0x10000: 0xFFFF, 1 << 40: 0xFFFF}
	for in, want := range u16 {
		if got := ClampU16(in); got != want {
			t.Errorf("ClampU16(%d) = %d, want %d", in, got, want)
		}
	}
	u8 := map[int]uint8{-1: 0, 7: 7, 255: 255, 256: 255, 100000: 255}
	for in, want := range u8 {
		if got := ClampU8(in); got != want {
			t.Errorf("ClampU8(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestOneOnOneMore(t *testing.T) {
	obs := Player{Flags: PlayerObserver, Team: 9}
	cases := []struct {
		name    string
		players []Player
		want    bool
	}{
		{"two teams", []Player{{Team: 1}, {Team: 2}}, true},
		{"observers ignored", []Player{{Team: 1}, obs, {Team: 2}, obs}, true},
		{"same team", []Player{{Team: 1}, {Team: 1}}, false},
		{"three players", []Player{{Team: 1}, {Team: 2}, {Team: 3}}, false},
		{"one player", []Player{{Team: 1}}, false},
	}
	for _, c := range cases {
		r := Replay{Players: c.players}
		if got := r.OneOnOne(); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
