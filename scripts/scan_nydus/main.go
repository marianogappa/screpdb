package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/parser"
	"github.com/marianogappa/screpdb/internal/patterns"
)

// scan_nydus walks replay paths/dirs, runs the full parse + worldstate
// pipeline, and reports any nydus_attack events. Used to surface curatable
// offensive-nydus examples for the golden suite (issue #193).
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: scan_nydus <file-or-dir> [more...]")
		os.Exit(2)
	}
	var paths []string
	for _, arg := range os.Args[1:] {
		info, err := os.Stat(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", arg, err)
			continue
		}
		if info.IsDir() {
			_ = filepath.Walk(arg, func(p string, fi os.FileInfo, err error) error {
				if err == nil && !fi.IsDir() && filepath.Ext(p) == ".rep" {
					paths = append(paths, p)
				}
				return nil
			})
			continue
		}
		paths = append(paths, arg)
	}

	hits := 0
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		replay := parser.CreateReplayFromFileInfo(path, filepath.Base(path), info.Size(), "")
		data, err := parser.ParseReplay(path, replay)
		if err != nil {
			continue
		}
		orch, ok := data.PatternOrchestrator.(*patterns.Orchestrator)
		if !ok {
			continue
		}
		if os.Getenv("EVENTS") != "" {
			counts := map[string]int{}
			for _, ev := range orch.ReplayEvents() {
				counts[ev.EventType]++
			}
			fmt.Printf("EVENTS %s map=%s fmt=%s %v\n", filepath.Base(path), replay.MapName, replay.TeamFormat, counts)
		}
		if os.Getenv("DEBUG") != "" {
			exits, enters := 0, 0
			for _, c := range data.Commands {
				if c.OrderName == nil {
					continue
				}
				switch *c.OrderName {
				case "BuildNydusExit":
					exits++
					ec, ok := cmdenrich.Classify(c)
					fmt.Printf("  [exit ] p%d @%d:%02d at=%q (%v,%v) classify: kind=%d ok=%v\n", c.PlayerID, c.SecondsFromGameStart/60, c.SecondsFromGameStart%60, c.ActionType, ptr(c.X), ptr(c.Y), ec.Kind, ok)
				case "EnterNydusCanal":
					enters++
					fmt.Printf("  [enter] p%d @%d:%02d (%v,%v)\n", c.PlayerID, c.SecondsFromGameStart/60, c.SecondsFromGameStart%60, ptr(c.X), ptr(c.Y))
				}
			}
			fmt.Printf("DEBUG %s: exits=%d enters=%d fmt=%s map=%s\n", filepath.Base(path), exits, enters, replay.TeamFormat, replay.MapName)
		}
		for _, ev := range orch.ReplayEvents() {
			if ev.EventType != "nydus_attack" {
				continue
			}
			hits++
			payload := ""
			if ev.Payload != nil {
				payload = *ev.Payload
			}
			src, tgt := byteVal(ev.SourceReplayPlayerID), byteVal(ev.TargetReplayPlayerID)
			oclock := -1
			if ev.LocationBaseOclock != nil {
				oclock = *ev.LocationBaseOclock
			}
			fmt.Printf("%s | map=%s fmt=%s | @%d:%02d src=p%d tgt=p%d oclock=%d units=%s payload=%s\n",
				filepath.Base(path), replay.MapName, replay.TeamFormat,
				ev.Second/60, ev.Second%60, src, tgt, oclock, jsonStr(ev.AttackUnitTypes), payload)
		}
	}
	fmt.Printf("\nscanned %d replays, %d nydus_attack events\n", len(paths), hits)
}

func ptr(p *int) int {
	if p == nil {
		return -1
	}
	return *p
}

func byteVal(p *byte) int {
	if p == nil {
		return -1
	}
	return int(*p)
}

func jsonStr(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
