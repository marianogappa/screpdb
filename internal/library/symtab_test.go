package library

import (
	"sync"
	"testing"
)

func TestTableInternRoundTrip(t *testing.T) {
	tbl := NewTable()
	cases := []struct {
		name   string
		wantID uint16
	}{
		{"", 0},
		{"Zergling", 1},
		{"Marine", 2},
		{"Zergling", 1},
		{"", 0},
	}
	for _, tc := range cases {
		if got := tbl.Intern(tc.name); got != tc.wantID {
			t.Fatalf("Intern(%q) = %d, want %d", tc.name, got, tc.wantID)
		}
		if got := tbl.Name(tc.wantID); got != tc.name {
			t.Fatalf("Name(%d) = %q, want %q", tc.wantID, got, tc.name)
		}
	}
	if tbl.Len() != 3 {
		t.Fatalf("Len = %d, want 3", tbl.Len())
	}
	if got := tbl.Name(999); got != "" {
		t.Fatalf("unknown id should read as empty, got %q", got)
	}
	if id, ok := tbl.Lookup("Marine"); !ok || id != 2 {
		t.Fatalf("Lookup(Marine) = %d,%v", id, ok)
	}
	if _, ok := tbl.Lookup("Hydralisk"); ok {
		t.Fatal("Lookup must not intern")
	}
}

func TestTableConcurrentInternIsStable(t *testing.T) {
	tbl := NewTable()
	names := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	var wg sync.WaitGroup
	results := make([][]uint16, 16)
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			ids := make([]uint16, len(names))
			for i, n := range names {
				ids[i] = tbl.Intern(n)
			}
			results[g] = ids
		}(g)
	}
	wg.Wait()
	for g := 1; g < 16; g++ {
		for i := range names {
			if results[g][i] != results[0][i] {
				t.Fatalf("goroutine %d interned %q as %d, goroutine 0 as %d", g, names[i], results[g][i], results[0][i])
			}
		}
	}
	if tbl.Len() != len(names)+1 {
		t.Fatalf("Len = %d, want %d", tbl.Len(), len(names)+1)
	}
}

func TestTableOverflowReturnsEmpty(t *testing.T) {
	tbl := NewTable()
	for i := 1; i < maxTableEntries; i++ {
		tbl.Intern(string(rune(0x10000 + i)))
	}
	if got := tbl.Intern("one too many"); got != 0 {
		t.Fatalf("overflow Intern = %d, want 0", got)
	}
}

func TestProdKindTables(t *testing.T) {
	cases := map[ProdKind]*Table{
		ProdTrain: Units, ProdUnitMorph: Units, ProdBuildingMorph: Units, ProdBuild: Units,
		ProdTech: Techs, ProdUpgrade: Upgrades, ProdCast: Orders,
	}
	for kind, want := range cases {
		if kind.Table() != want {
			t.Fatalf("%s resolves the wrong table", kind)
		}
	}
}
