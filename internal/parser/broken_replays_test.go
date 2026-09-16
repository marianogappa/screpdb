package parser

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	screp "github.com/icza/screp/repparser"
	"github.com/marianogappa/screpdb/internal/models"
)

// testdata/broken holds replays the StarCraft engine cannot play back. screpdb
// has no feature that uses them; this test exists so the catalogue stays
// honest — the files must keep parsing, and the anomaly that distinguishes them
// must stay measurable. See testdata/broken/README.md for what was observed
// in-game, which cannot be recovered from the file.
func TestBrokenReplaysStillParse(t *testing.T) {
	dir := filepath.Join("testdata", "broken")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	found := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".rep" {
			continue
		}
		found++
		t.Run(e.Name(), func(t *testing.T) {
			data, err := ParseReplay(filepath.Join(dir, e.Name()), &models.Replay{})
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if len(data.Players) == 0 || len(data.Commands) == 0 {
				t.Fatalf("parsed to %d players and %d commands", len(data.Players), len(data.Commands))
			}
		})
	}
	if found == 0 {
		t.Fatal("no replays in testdata/broken; delete the directory or the test")
	}
}

// The 2024-02-07 entry's signature: three of its eight players issue production
// bursts no unmodified client can produce (a player starts with 50 minerals, so
// nine seconds in one worker is affordable). Locking the measurement down means
// a future detector has a calibrated example, and that re-parsing changes cannot
// quietly erase the evidence.
//
// Measured on the RAW screp stream. screpdb's early filter already discards
// these as opening spam — which is itself corroboration, but it means the
// evidence is gone from data.Commands by the time a parse returns.
func TestBrokenReplayProductionFlood(t *testing.T) {
	const (
		path       = "testdata/broken/20240207_bgh_2v2v2v2_engine_desync.rep"
		windowSec  = 5
		openingSec = 60
		// Well above the 14-command burst the busiest player of a normal
		// 8-player game reaches, and well below this replay's 20.
		impossibleBurst = 18
	)
	rep, err := screp.ParseFile(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	names := map[byte]string{}
	for _, p := range rep.Header.Players {
		names[p.ID] = p.Name
	}
	bursts := map[string]int{}
	times := map[string][]int{}
	for _, c := range rep.Commands.Cmds {
		base := c.BaseCmd()
		if base.Type.Name != models.ActionTypeTrain && base.Type.Name != models.ActionTypeUnitMorph {
			continue
		}
		second := int(base.Frame.Duration().Seconds())
		if second > openingSec {
			continue
		}
		name, ok := names[base.PlayerID]
		if !ok {
			continue
		}
		times[name] = append(times[name], second)
	}
	for name, ts := range times {
		sort.Ints(ts)
		for i := range ts {
			n := 0
			for j := i; j < len(ts) && ts[j]-ts[i] <= windowSec; j++ {
				n++
			}
			bursts[name] = max(bursts[name], n)
		}
	}
	var flooded []string
	for name, b := range bursts {
		if b >= impossibleBurst {
			flooded = append(flooded, name)
		}
	}
	sort.Strings(flooded)
	want := []string{"Jackal.", "PainXG", "|200Spartans|"}
	if len(flooded) != len(want) {
		t.Fatalf("players with an impossible opening burst: %v, want %v (all bursts: %v)", flooded, want, bursts)
	}
	for i := range want {
		if flooded[i] != want[i] {
			t.Fatalf("players with an impossible opening burst: %v, want %v", flooded, want)
		}
	}
}
