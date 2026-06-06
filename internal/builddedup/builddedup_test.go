package builddedup

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/icza/screp/rep"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/screp"
	"github.com/marianogappa/screpdb/internal/unittags"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden file instead of comparing")

var goldenBuildingOrder = []string{
	"Nexus", "Gateway", "Robotics Facility", "Stargate",
	"Command Center", "Barracks", "Factory", "Starport",
}

// TestGoldenDedup parses the committed replays end-to-end through the tag tracker
// and the dedup planner and asserts a stable, human-readable summary of what gets
// dropped. Regenerate after intentional behaviour changes with:
//
//	go test ./internal/builddedup -run TestGoldenDedup -update
func TestGoldenDedup(t *testing.T) {
	dir := "testdata/replays"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if strings.EqualFold(filepath.Ext(e.Name()), ".rep") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		t.Fatal("no .rep fixtures found in testdata/replays")
	}

	var b strings.Builder
	for _, name := range names {
		r, err := screp.ParseFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		ev := unittags.Analyze(r)
		players := playersFromRep(r)
		plan := Compute(ev, players)
		raceByPID := map[byte]string{}
		for _, p := range players {
			raceByPID[p.PlayerID] = p.Race
		}

		fmt.Fprintf(&b, "== %s ==\n", name)
		pids := make([]int, 0, len(ev.Players))
		for pid := range ev.Players {
			pids = append(pids, int(pid))
		}
		sort.Ints(pids)
		for _, pidInt := range pids {
			pid := byte(pidInt)
			pe := ev.Players[pid]
			tierA, tierB := 0, 0
			for _, builds := range pe.Builds {
				for _, bd := range builds {
					switch plan.Reason(pid, bd.Frame) {
					case "worker_one_at_a_time":
						tierA++
					case "never_produced":
						tierB++
					}
				}
			}
			fmt.Fprintf(&b, "  player %d (%s): tierA=%d tierB=%d\n", pid, raceByPID[pid], tierA, tierB)
			for _, bldg := range goldenBuildingOrder {
				builds := pe.Builds[bldg]
				if len(builds) == 0 {
					continue
				}
				dropped := 0
				for _, bd := range builds {
					if plan.Reason(pid, bd.Frame) != "" {
						dropped++
					}
				}
				fmt.Fprintf(&b, "    %-18s built=%d produced=%d dropped=%d\n",
					bldg, len(builds), len(pe.Producers[bldg]), dropped)
			}
		}
	}

	got := b.String()
	goldenPath := filepath.Join("testdata", "golden.txt")
	if *updateGolden {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", goldenPath)
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (regenerate with -update): %v", err)
	}
	if got != string(want) {
		t.Errorf("dedup output changed (regenerate with -update if intended):\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func playersFromRep(r *rep.Replay) []*models.Player {
	var ps []*models.Player
	for _, p := range r.Header.Players {
		if p == nil {
			continue
		}
		race := ""
		if p.Race != nil {
			race = p.Race.Name
		}
		ps = append(ps, &models.Player{PlayerID: p.ID, Race: race})
	}
	return ps
}
