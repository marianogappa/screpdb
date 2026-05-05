//go:build ignore

// dump_ownership prints the engine's per-polygon ownership timeline for a
// replay so we can see exactly where each "claim" / "expansion" /
// "takeover" / "timeout" transition lands and why one we expected is
// missing. Spot-check by base name.
package main

import (
	"fmt"
	"os"

	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/parser"
	"github.com/marianogappa/screpdb/internal/patterns/worldstate"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: dump_ownership <replay-path>")
		os.Exit(1)
	}
	path := os.Args[1]

	replay := &models.Replay{FilePath: path}
	data, err := parser.ParseReplay(path, replay)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse: %v\n", err)
		os.Exit(1)
	}

	engine := worldstate.NewEngine(data.Replay, data.Players, data.MapContext)
	for _, cmd := range data.Commands {
		engine.ProcessCommand(cmd)
	}
	engine.Finalize()

	bases, startBaseByPID, naturalBaseByPID, _ := engine.DebugSnapshot()
	pidToName := map[byte]string{}
	for _, p := range data.Players {
		pidToName[p.PlayerID] = p.Name
	}

	fmt.Println("Bases:")
	for i, b := range bases {
		fmt.Printf("  [%d] %s (kind=%s, clock=%d, isStarting=%v) center=(%d,%d)\n",
			i, b.Name, b.Kind, b.Clock, b.IsStarting, int(b.CenterX), int(b.CenterY))
	}
	fmt.Println()
	fmt.Println("startBaseByPID:")
	for pid, idx := range startBaseByPID {
		fmt.Printf("  pid=%d (%s) → base[%d] %s\n", pid, pidToName[pid], idx, bases[idx].Name)
	}
	fmt.Println("naturalBaseByPID:")
	for pid, idx := range naturalBaseByPID {
		fmt.Printf("  pid=%d (%s) → base[%d] %s\n", pid, pidToName[pid], idx, bases[idx].Name)
	}
	fmt.Println()

	// Dump ownership transitions from replay events.
	events := engine.ReplayEvents()
	fmt.Println("All events with location info:")
	for _, ev := range events {
		if ev.LocationBaseType == nil {
			continue
		}
		actor := ""
		if ev.SourceReplayPlayerID != nil {
			actor = pidToName[*ev.SourceReplayPlayerID]
		}
		fmt.Printf("  sec=%d type=%s actor=%s ", ev.Second, ev.EventType, actor)
		if ev.LocationBaseType != nil {
			fmt.Printf("type=%s ", *ev.LocationBaseType)
		}
		if ev.LocationBaseOclock != nil {
			fmt.Printf("clock=%d ", *ev.LocationBaseOclock)
		}
		if ev.LocationNaturalOfClock != nil {
			fmt.Printf("natural-of=%d ", *ev.LocationNaturalOfClock)
		}
		fmt.Println()
	}
}
