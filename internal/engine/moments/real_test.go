package moments

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/engine"
	"github.com/marianogappa/screpdb/internal/engine/macro"
)

// Logs the moments of a real game for eyeballing; it asserts only that a long PvZ has fights and expansions.
func TestRealGameMoments(t *testing.T) {
	if os.Getenv("OPENBW_DATA") == "" {
		t.Skip("OPENBW_DATA is not set")
	}
	data, err := engine.LoadData(os.Getenv("OPENBW_DATA"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "replays", "SomaTyson.rep"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := engine.Pass(context.Background(), b, data)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[Kind]int{}
	for _, m := range Find(d, macro.Compute(d)) {
		counts[m.Kind]++
		line := fmt.Sprintf("%2d:%02d %-12s actor %d target %d", m.Second/60, m.Second%60, m.Kind, m.Actor, m.Target)
		for _, s := range m.Sides {
			line += fmt.Sprintf(" | p%d lost %d killed %d", s.Owner, s.Lost.Value(), s.Killed.Value())
		}
		if m.UnitType != 0 {
			line += " " + repcmd.UnitByID(m.UnitType).Name
		}
		t.Log(line)
	}
	if counts[KindFight] == 0 || counts[KindExpansion] == 0 {
		t.Fatalf("counts %v", counts)
	}
	snap := SnapshotAt(d, 900)
	t.Logf("at 15:00: %d buildings, %d army units (army sample %ds)", len(snap.Buildings), len(snap.Units), snap.ArmySecond)
}
