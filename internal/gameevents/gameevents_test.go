package gameevents

import "testing"

func TestBudgetGrowsWithLengthAndPlayersUpToACap(t *testing.T) {
	short := Budget(Game{DurationSeconds: 300, Players: 2})
	long := Budget(Game{DurationSeconds: 1500, Players: 2})
	crowded := Budget(Game{DurationSeconds: 1500, Players: 8})
	huge := Budget(Game{DurationSeconds: 5000, Players: 8})
	if !(short < long && long < crowded) || huge != 22 || short < 4 {
		t.Fatalf("budgets short=%d long=%d crowded=%d huge=%d", short, long, crowded, huge)
	}
}

func TestSelectKeepsMandatoryAndPrefersWeightyVariedEvents(t *testing.T) {
	var cs []Candidate
	cs = append(cs, Candidate{Type: "bo_openers"}, Candidate{Type: "leave_game", Second: 1200, Actor: 2})
	for i := range 20 {
		cs = append(cs, Candidate{Type: "expansion", Second: 100 + i*50, Actor: int64(1 + i%2)})
	}
	for i := range 10 {
		cs = append(cs, Candidate{Type: "attack", Second: 400 + i*60, Actor: 1, Magnitude: Magnitude{Traded: 200}})
	}
	cs = append(cs,
		Candidate{Type: "attack", Second: 900, Actor: 2, Magnitude: Magnitude{Traded: 4000, TownHall: true}},
		Candidate{Type: "zergling_rush", Second: 150, Actor: 2},
	)
	Number(cs)
	key := Select(cs, Game{DurationSeconds: 1200, Players: 2})
	count := map[string]int{}
	total := 0
	for i, k := range key {
		if k {
			count[cs[i].Type]++
			total++
		}
	}
	if !key[0] || !key[1] {
		t.Fatal("mandatory events must be key")
	}
	if !key[len(cs)-1] || !key[len(cs)-2] {
		t.Fatal("the rush and the base kill must be key")
	}
	if picked := total - 2; picked > Budget(Game{DurationSeconds: 1200, Players: 2}) || picked < 6 {
		t.Fatalf("picked %d non-mandatory events", total-2)
	}
	if count["expansion"] > 6 || count["attack"] > 6 {
		t.Fatalf("one type crowds the list: %v", count)
	}
}

func TestNumberCountsPerActorAndType(t *testing.T) {
	cs := []Candidate{
		{Type: "expansion", Second: 300, Actor: 1},
		{Type: "expansion", Second: 100, Actor: 1},
		{Type: "expansion", Second: 200, Actor: 2},
	}
	Number(cs)
	if cs[0].Magnitude.Nth != 2 || cs[1].Magnitude.Nth != 1 || cs[2].Magnitude.Nth != 1 {
		t.Fatalf("nth %+v", cs)
	}
}
