// Throwaway tool: measure the drop in team-stacking detections between the
// pre-change "no inactivity filter" logic and the new effective-player
// logic. Parses .rep files via screp directly (no DB access). Compares,
// per replay:
//
//   - oldFlag = AnalyzeAlliances with an empty Activity (no leave/stop
//     filtering). Equivalent to the legacy behavior because effectiveTeamsAt
//     returns teams unchanged when Activity is empty, so isStacking sees
//     the raw alliance topology.
//
//   - newFlag = AnalyzeAlliances with ComputeActivity-built maps.
//
// Run:
//
//	go run ./scripts/measure_stacking_drop --dir <path-to-rep-collection>
//	go run ./scripts/measure_stacking_drop --dir <path> --verbose
//
// This is deliberately minimal — read-only, no DB, no CLI registration.
// Delete after the measurement is no longer interesting.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/parser"
)

func main() {
	var (
		dir     = flag.String("dir", "", "directory to walk for .rep files (recursively)")
		verbose = flag.Bool("verbose", false, "print per-replay diffs in the old-only bucket")
		workers = flag.Int("workers", 8, "parser worker goroutines")
	)
	flag.Parse()
	if *dir == "" {
		fmt.Fprintln(os.Stderr, "usage: measure_stacking_drop --dir <path> [--verbose]")
		os.Exit(2)
	}

	var paths []string
	walkErr := filepath.WalkDir(*dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(p), ".rep") {
			return nil
		}
		paths = append(paths, p)
		return nil
	})
	if walkErr != nil {
		fmt.Fprintf(os.Stderr, "walk %s: %v\n", *dir, walkErr)
		os.Exit(1)
	}

	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "no .rep files found")
		os.Exit(1)
	}

	type result struct {
		path    string
		oldFlag bool
		newFlag bool
		skipped bool
	}

	var (
		scanned  int64
		gated    int64
		oldOnly  int64
		newOnly  int64
		both     int64
		neither  int64
		errCount int64
	)
	var oldOnlyMu sync.Mutex
	var oldOnlyPaths []string

	jobs := make(chan string, *workers*2)
	results := make(chan result, *workers*2)

	var wg sync.WaitGroup
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				r := analyzeOne(p)
				results <- r
			}
		}()
	}

	go func() {
		for _, p := range paths {
			jobs <- p
		}
		close(jobs)
	}()

	done := make(chan struct{})
	go func() {
		for r := range results {
			atomic.AddInt64(&scanned, 1)
			if r.skipped {
				if r.oldFlag {
					// repurpose oldFlag=true as "errored" sentinel; harmless
					atomic.AddInt64(&errCount, 1)
				} else {
					atomic.AddInt64(&gated, 1)
				}
				continue
			}
			switch {
			case r.oldFlag && r.newFlag:
				atomic.AddInt64(&both, 1)
			case r.oldFlag && !r.newFlag:
				atomic.AddInt64(&oldOnly, 1)
				if *verbose {
					oldOnlyMu.Lock()
					oldOnlyPaths = append(oldOnlyPaths, r.path)
					oldOnlyMu.Unlock()
				}
			case !r.oldFlag && r.newFlag:
				atomic.AddInt64(&newOnly, 1)
			default:
				atomic.AddInt64(&neither, 1)
			}
		}
		close(done)
	}()

	wg.Wait()
	close(results)
	<-done

	oldTotal := both + oldOnly
	newTotal := both + newOnly
	delta := newTotal - oldTotal

	fmt.Printf("scanned:    %d files (%d errored, %d skipped — non-melee or ≤2 active humans)\n",
		scanned, errCount, gated)
	fmt.Printf("eligible:   %d melee replays (>2 active human players)\n", both+oldOnly+newOnly+neither)
	fmt.Printf("old=true:   %d\n", oldTotal)
	fmt.Printf("new=true:   %d\n", newTotal)
	fmt.Printf("delta:      %+d  (old-only: %d, new-only: %d, both: %d, neither: %d)\n",
		delta, oldOnly, newOnly, both, neither)
	if newOnly > 0 {
		fmt.Println("WARNING: new-only > 0 — the new logic should be a strict relaxation of the old.")
	}

	if *verbose && len(oldOnlyPaths) > 0 {
		fmt.Println()
		fmt.Println("old-only (flagged before, not after):")
		for _, p := range oldOnlyPaths {
			fmt.Printf("  %s\n", p)
		}
	}
}

func analyzeOne(path string) (out struct {
	path    string
	oldFlag bool
	newFlag bool
	skipped bool
}) {
	out.path = path
	rep := &models.Replay{FilePath: path}
	data, err := parser.ParseReplay(path, rep)
	if err != nil {
		out.skipped = true
		out.oldFlag = true // sentinel: errored
		return
	}

	if data.Replay.GameType != "Melee" {
		out.skipped = true
		return
	}
	active := 0
	for _, p := range data.Players {
		if p == nil || p.IsObserver || p.Type == "Computer" {
			continue
		}
		active++
	}
	if active <= 2 {
		out.skipped = true
		return
	}

	dur := data.Replay.DurationSeconds

	// Legacy behavior: empty Activity → effectiveTeamsAt is identity →
	// isStacking sees the raw alliance topology.
	emptyAct := parser.Activity{
		StoppedSecByPID: map[byte]int{},
		LeaveSecByPID:   map[byte]int{},
	}
	oldRes := parser.AnalyzeAlliances(data.Players, data.Commands, dur, emptyAct)

	// New behavior: full activity, monotonic inactivity excluded.
	activity := parser.ComputeActivity(data.Players, data.Commands, dur)
	newRes := parser.AnalyzeAlliances(data.Players, data.Commands, dur, activity)

	out.oldFlag = oldRes.TeamStackingFlag
	out.newFlag = newRes.TeamStackingFlag
	return
}
