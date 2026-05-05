// debug_phases dumps the inputs phases.Compute sees for a single replay
// — first-occurrence seconds for every KindMakeUnit subject, every
// KindTech subject, and every KindUpgrade occurrence — followed by the
// (earlyEnd, midEnd) result. Useful when a per-game pill bins a unit
// to the wrong phase: the listing immediately shows whether a tech /
// upgrade subject is missing from the stream or whether the algorithm
// fell back to a later signal.
//
// Usage:
//
//	go run ./scripts/debug_phases /path/to/replay.rep
package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/parser"
	"github.com/marianogappa/screpdb/internal/patterns"
	"github.com/marianogappa/screpdb/internal/patterns/phases"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: debug-phases <replay-path>")
		os.Exit(1)
	}
	path := os.Args[1]
	info, err := os.Stat(path)
	if err != nil {
		panic(err)
	}
	replay := parser.CreateReplayFromFileInfo(path, info.Name(), info.Size(), "")
	data, err := parser.ParseReplay(path, replay)
	if err != nil {
		panic(err)
	}
	orch, ok := data.PatternOrchestrator.(*patterns.Orchestrator)
	if !ok {
		fmt.Println("no orchestrator")
		return
	}
	ws := orch.WorldStateEngine()
	if ws == nil {
		fmt.Println("no worldstate")
		return
	}
	stream := ws.EnrichedStream()

	// First-second-per-unit-name in the stream that phases.Compute sees.
	firstByName := map[string]int{}
	firstByTech := map[string]int{}
	firstByUpgrade := map[string]int{}
	upgradeOccurrences := map[string][]int{}

	for _, f := range stream {
		switch f.Kind {
		case cmdenrich.KindMakeUnit:
			if cur, ok := firstByName[f.Subject]; !ok || f.Second < cur {
				firstByName[f.Subject] = f.Second
			}
		case cmdenrich.KindTech:
			if cur, ok := firstByTech[f.Subject]; !ok || f.Second < cur {
				firstByTech[f.Subject] = f.Second
			}
		case cmdenrich.KindUpgrade:
			if cur, ok := firstByUpgrade[f.Subject]; !ok || f.Second < cur {
				firstByUpgrade[f.Subject] = f.Second
			}
			upgradeOccurrences[f.Subject] = append(upgradeOccurrences[f.Subject], f.Second)
		}
	}

	fmt.Println("=== KindMakeUnit first-occurrences (stream as seen by phases.Compute) ===")
	type kv struct {
		k string
		v int
	}
	pairs := []kv{}
	for k, v := range firstByName {
		pairs = append(pairs, kv{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].v < pairs[j].v })
	for _, p := range pairs {
		fmt.Printf("  %5d %s\n", p.v, p.k)
	}

	fmt.Println()
	fmt.Println("=== KindTech first-occurrences ===")
	pairs = pairs[:0]
	for k, v := range firstByTech {
		pairs = append(pairs, kv{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].v < pairs[j].v })
	for _, p := range pairs {
		fmt.Printf("  %5d %s\n", p.v, p.k)
	}

	fmt.Println()
	fmt.Println("=== KindUpgrade occurrences (showing all occurrences per subject) ===")
	upgradeKeys := []string{}
	for k := range upgradeOccurrences {
		upgradeKeys = append(upgradeKeys, k)
	}
	sort.Strings(upgradeKeys)
	for _, k := range upgradeKeys {
		secs := upgradeOccurrences[k]
		sort.Ints(secs)
		fmt.Printf("  %s: %v\n", k, secs)
	}

	fmt.Println()
	earlyEnd, midEnd := phases.Compute(stream)
	fmt.Printf("=== phases.Compute result ===\n  earlyEnd = %d\n  midEnd   = %d\n", earlyEnd, midEnd)
}
