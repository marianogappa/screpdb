package library_test

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
)

func twoHumans(opts ...librarytest.Option) *library.Replay {
	all := append([]librarytest.Option{
		librarytest.WithPlayer("Flash", librarytest.Team(1)),
		librarytest.WithPlayer("Jaedong", librarytest.Team(2)),
	}, opts...)
	return librarytest.Replay(all...)
}

func TestFilterConfigMatches(t *testing.T) {
	melee := twoHumans()
	ums := twoHumans(librarytest.WithMapKind(library.MapKindUseMapSettings))
	short := twoHumans(librarytest.WithDuration(60))
	exactlyShortFloor := twoHumans(librarytest.WithDuration(120))
	withComputer := librarytest.Replay(
		librarytest.WithPlayer("Flash", librarytest.Team(1)),
		librarytest.WithPlayer("AI", librarytest.Team(2), librarytest.Computer()),
	)
	withComputerControlled := librarytest.Replay(
		librarytest.WithPlayer("Flash", librarytest.Team(1)),
		librarytest.WithPlayer("AI", librarytest.Team(2), librarytest.Type(library.PlayerTypeComputerControlled)),
	)
	observerComputer := twoHumans(librarytest.WithPlayer("AI", librarytest.Computer(), librarytest.Observer()))
	topVsBottom := twoHumans(librarytest.WithGameType(" Top vs Bottom "))
	ffa := twoHumans(librarytest.WithGameType("Free For All"))
	sameTeam := librarytest.Replay(
		librarytest.WithPlayer("A", librarytest.Team(1)),
		librarytest.WithPlayer("B", librarytest.Team(1)),
		librarytest.WithGameType("Team Melee"),
	)
	threePlayers := librarytest.Replay(
		librarytest.WithPlayer("A", librarytest.Team(1)),
		librarytest.WithPlayer("B", librarytest.Team(2)),
		librarytest.WithPlayer("C", librarytest.Team(3)),
		librarytest.WithGameType("Team Melee"),
	)
	oneOnOneWithObserver := librarytest.Replay(
		librarytest.WithPlayer("A", librarytest.Team(1)),
		librarytest.WithPlayer("B", librarytest.Team(2)),
		librarytest.WithPlayer("Obs", librarytest.Team(3), librarytest.Observer()),
		librarytest.WithGameType("Team Melee"),
	)
	money := twoHumans(librarytest.WithMapKind(library.MapKindMoney))
	unknownKind := twoHumans(librarytest.WithMapKind(library.MapKindUnknown))

	cases := []struct {
		name    string
		cfg     library.FilterConfig
		wantIn  []*library.Replay
		wantOut []*library.Replay
	}{
		{
			name:    "no filters keeps only the hardcoded UMS exclusion",
			cfg:     library.FilterConfig{},
			wantIn:  []*library.Replay{melee, short, withComputer, topVsBottom, ffa, sameTeam, money, unknownKind},
			wantOut: []*library.Replay{ums},
		},
		{
			name:    "short games and computers",
			cfg:     library.FilterConfig{ExcludeShortGames: true, ExcludeComputers: true},
			wantIn:  []*library.Replay{melee, exactlyShortFloor, observerComputer},
			wantOut: []*library.Replay{short, withComputer, withComputerControlled},
		},
		{
			name:    "game type predicate melee",
			cfg:     library.FilterConfig{GameTypes: []string{library.GameTypeMelee}},
			wantIn:  []*library.Replay{melee},
			wantOut: []*library.Replay{topVsBottom, ffa, sameTeam},
		},
		{
			name:    "game type predicates OR together and normalise case",
			cfg:     library.FilterConfig{GameTypes: []string{library.GameTypeTopVsBottom, library.GameTypeFreeForAll}},
			wantIn:  []*library.Replay{topVsBottom, ffa},
			wantOut: []*library.Replay{melee, sameTeam},
		},
		{
			name:    "one on one counts non-observer players on two teams",
			cfg:     library.FilterConfig{GameTypes: []string{library.GameTypeOneOnOne}},
			wantIn:  []*library.Replay{melee, oneOnOneWithObserver, withComputer},
			wantOut: []*library.Replay{sameTeam, threePlayers},
		},
		{
			name:    "map kinds translate to storage casing",
			cfg:     library.FilterConfig{MapKinds: []string{library.MapKindFilterRegular, library.MapKindFilterMoney}},
			wantIn:  []*library.Replay{melee, money},
			wantOut: []*library.Replay{unknownKind, ums},
		},
		{
			name:    "single map kind",
			cfg:     library.FilterConfig{MapKinds: []string{library.MapKindFilterMoney}},
			wantIn:  []*library.Replay{money},
			wantOut: []*library.Replay{melee},
		},
		{
			name:    "unknown enum values are dropped",
			cfg:     library.FilterConfig{GameTypes: []string{"bogus"}, MapKinds: []string{"bogus"}},
			wantIn:  []*library.Replay{melee, topVsBottom, sameTeam, money, unknownKind},
			wantOut: []*library.Replay{ums},
		},
		{
			name:    "defaults",
			cfg:     library.DefaultFilterConfig(),
			wantIn:  []*library.Replay{melee, topVsBottom, ffa, money, oneOnOneWithObserver},
			wantOut: []*library.Replay{short, withComputer, ums, sameTeam, threePlayers},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, r := range tc.wantIn {
				if !tc.cfg.Matches(r) {
					t.Errorf("expected %s (game type %q) to pass", r.FileName(), library.Strings.Name(r.GameType))
				}
			}
			for _, r := range tc.wantOut {
				if tc.cfg.Matches(r) {
					t.Errorf("expected %s (game type %q) to be excluded", r.FileName(), library.Strings.Name(r.GameType))
				}
			}
		})
	}
	if (library.FilterConfig{}).Matches(nil) {
		t.Fatal("nil replay must not match")
	}
}

func TestFilterConfigNormalize(t *testing.T) {
	cases := []struct {
		name    string
		in      library.FilterConfig
		want    library.FilterConfig
		wantErr bool
	}{
		{
			name: "trims lowercases and dedups",
			in:   library.FilterConfig{GameTypes: []string{" Melee", "melee", "", "ONE_ON_ONE"}, MapKinds: []string{"Regular", "regular"}},
			want: library.FilterConfig{GameTypes: []string{"melee", "one_on_one"}, MapKinds: []string{"regular"}},
		},
		{
			name: "empty lists stay empty",
			in:   library.FilterConfig{ExcludeShortGames: true},
			want: library.FilterConfig{GameTypes: []string{}, MapKinds: []string{}, ExcludeShortGames: true},
		},
		{name: "rejects unknown game type", in: library.FilterConfig{GameTypes: []string{"bogus"}}, wantErr: true},
		{name: "rejects unknown map kind", in: library.FilterConfig{MapKinds: []string{"ums"}}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.in.Normalize()
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !got.Equal(tc.want) {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
		})
	}
}
