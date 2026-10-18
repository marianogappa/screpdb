package moments

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/openbw-go"
	"github.com/marianogappa/screpdb/internal/engine"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/compact"
	"github.com/marianogappa/screpdb/internal/parser"
)

// Logs the curated events of real games for review. MOMENTS_LIST names a file with one replay per line.
func TestRealGameEvents(t *testing.T) {
	list := os.Getenv("MOMENTS_LIST")
	if list == "" || os.Getenv("OPENBW_DATA") == "" {
		t.Skip("MOMENTS_LIST and OPENBW_DATA are not set")
	}
	data, err := engine.LoadData(os.Getenv("OPENBW_DATA"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(list)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		path := s.Text()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		rec, err := libraryReplay(path, b)
		if err != nil {
			fmt.Printf("%s: %v\n", filepath.Base(path), err)
			continue
		}
		d, err := engine.Pass(context.Background(), b, data)
		if err != nil {
			fmt.Printf("%s: %v\n", filepath.Base(path), err)
			continue
		}
		enc, _ := engine.Encode(d)
		bases, players := basesAndPlayers(rec)
		names := map[int8]string{}
		for _, p := range rec.Players {
			names[int8(p.Slot)] = p.Name
		}
		fmt.Printf("\n%s  %ds  %d KB digest  confidence %d  unloads %d teleports %d casts %d  bases %d\n", filepath.Base(path), d.LastFrame/24, len(enc)/1024, d.Confidence(), len(d.Unloads), len(d.Teleports), len(d.Casts), len(bases))
		for _, e := range Detect(d, bases, players) {
			var units []string
			for _, u := range e.Units {
				units = append(units, repcmd.UnitByID(u).Name)
			}
			kind := ""
			if e.Base != NoBase {
				kind = string(bases[e.Base].Kind)
			}
			fmt.Printf("  %2d:%02d %-15s %-16s -> %-16s %-10s %s\n", e.Second/60, e.Second%60, e.Type, names[e.Actor], names[e.Target], kind, strings.Join(units, ", "))
		}
	}
}

func libraryReplay(path string, b []byte) (*library.Replay, error) {
	replay := parser.CreateReplayFromFileInfo(path, filepath.Base(path), int64(len(b)), "")
	parsed, err := parser.ParseReplayWithOptions(path, replay, parser.Options{})
	if err != nil {
		return nil, err
	}
	var sum [32]byte
	return compact.FromReplayData(parsed, compact.FileMeta{Path: path, Size: int64(len(b)), Checksum: sum})
}

func basesAndPlayers(rec *library.Replay) ([]Base, []Player) {
	var bases []Base
	naturals := map[string]bool{}
	if rec.Layout != nil {
		for _, b := range rec.Layout.Bases {
			if strings.EqualFold(b.Kind, "start") || strings.EqualFold(b.Kind, "starting") {
				naturals[b.NaturalExpansion] = true
			}
		}
		for _, b := range rec.Layout.Bases {
			kind := BaseExpansion
			switch {
			case strings.EqualFold(b.Kind, "start") || strings.EqualFold(b.Kind, "starting"):
				kind = BaseStart
			case naturals[b.Name]:
				kind = BaseNatural
			}
			var poly [][2]int32
			for _, p := range b.Polygon {
				poly = append(poly, [2]int32{p.X, p.Y})
			}
			bases = append(bases, Base{Kind: kind, X: b.CenterX, Y: b.CenterY, Polygon: poly})
		}
	}
	var players []Player
	for _, p := range rec.Players {
		race := openbw.RaceRandom
		switch p.Race {
		case library.RaceZerg:
			race = openbw.RaceZerg
		case library.RaceTerran:
			race = openbw.RaceTerran
		case library.RaceProtoss:
			race = openbw.RaceProtoss
		}
		players = append(players, Player{Slot: int8(p.Slot), Race: race, Human: p.IsHuman() && !p.IsObserver()})
	}
	return bases, players
}
