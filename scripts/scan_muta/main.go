// scan_muta runs unittags.DetectMutaHarass over replays and prints the raw
// hit-n-run windows it finds — an inspection tool for the #194 detector. NOTE
// these are the raw detection windows; the shipped "Muta hit-n-run" marker
// applies a stricter per-game-player confidence bar on top (see
// internal/patterns/worldstate/muta_harass_pass.go).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/screp"
	"github.com/marianogappa/screpdb/internal/unittags"
)

func main() {
	var paths []string
	for _, arg := range os.Args[1:] {
		info, err := os.Stat(arg)
		if err != nil {
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
	sort.Strings(paths)

	mm := func(s int) string { return fmt.Sprintf("%d:%02d", s/60, s%60) }
	total := 0
	for _, path := range paths {
		r, err := screp.ParseFile(path)
		if err != nil {
			continue
		}
		var players []*models.Player
		for _, p := range r.Header.Players {
			if p == nil {
				continue
			}
			players = append(players, &models.Player{PlayerID: p.ID, Race: p.Race.String(), Name: p.Name})
		}
		eps := unittags.DetectMutaHarass(r, players)
		if len(eps) == 0 {
			continue
		}
		fmt.Printf("\n%s\n", filepath.Base(path))
		for _, e := range eps {
			total++
			fmt.Printf("  P%d harass %s-%s (%ds) cycles=%d grp~%d path=%d\n",
				e.PlayerID, mm(e.StartSec), mm(e.EndSec), e.EndSec-e.StartSec, e.Cycles, e.GroupSize, len(e.Path))
		}
	}
	fmt.Printf("\ntotal episodes: %d across %d replays\n", total, len(paths))
}
