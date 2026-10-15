// engine-probe runs the engine pass on a list of replays and prints, per replay, the pass time, the
// encoded digest size and the drift, then percentiles. It answers the engine pass's measuring questions
// on any machine with StarCraft: Remastered installed: go run ./scripts/engine-probe -list replays.txt
// OPENBW_DATA=<dir> uses already extracted game data instead of the install.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"time"

	"github.com/marianogappa/screpdb/internal/engine"
)

type result struct {
	path   string
	pass   time.Duration
	size   int
	digest *engine.Digest
	err    error
}

func main() {
	list := flag.String("list", "", "file with one replay path per line")
	workers := flag.Int("workers", max(1, runtime.NumCPU()/2-1), "parallel engine passes")
	flag.Parse()
	paths, err := readList(*list)
	if err != nil {
		log.Fatal(err)
	}
	data, err := loadData()
	if err != nil {
		log.Fatal(err)
	}
	start := time.Now()
	var results []result
	for r := range runAll(paths, data, *workers) {
		results = append(results, r)
		printResult(r)
	}
	summarize(results, time.Since(start), *workers)
}

func loadData() (*engine.Data, error) {
	if dir := os.Getenv("OPENBW_DATA"); dir != "" {
		return engine.LoadData(dir)
	}
	return engine.OpenGameData()
}

func readList(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var paths []string
	s := bufio.NewScanner(f)
	for s.Scan() {
		if line := s.Text(); line != "" {
			paths = append(paths, line)
		}
	}
	return paths, s.Err()
}

func runAll(paths []string, data *engine.Data, workers int) <-chan result {
	jobs := make(chan string)
	out := make(chan result)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for p := range jobs {
				out <- run(p, data)
			}
		})
	}
	go func() {
		for _, p := range paths {
			jobs <- p
		}
		close(jobs)
		wg.Wait()
		close(out)
	}()
	return out
}

func run(path string, data *engine.Data) result {
	r := result{path: path}
	b, err := os.ReadFile(path)
	if err != nil {
		r.err = err
		return r
	}
	t := time.Now()
	r.digest, r.err = engine.Pass(b, data)
	r.pass = time.Since(t)
	if r.err != nil {
		return r
	}
	enc, err := engine.Encode(r.digest)
	if err != nil {
		r.err = err
		return r
	}
	r.size = len(enc)
	return r
}

func printResult(r result) {
	name := filepath.Base(r.path)
	if r.err != nil {
		fmt.Printf("%s\terror=%v\n", name, r.err)
		return
	}
	d := r.digest
	fmt.Printf("%s\tpass=%v\tframes=%d\tbytes=%d\tplayers=%d\tconfidence=%d\ttrusted_until=%d\tmissing=%d/%d\tengine_error=%q\n",
		name, r.pass.Round(time.Millisecond), d.LastFrame, r.size, len(d.Players), d.Confidence(), d.TrustedUntil(),
		d.Drift.MissingSelects, d.Drift.Selects, d.Drift.EngineError)
}

func summarize(results []result, wall time.Duration, workers int) {
	var sizes, millis []int
	var frames, passMillis, failed, low int
	for _, r := range results {
		if r.err != nil {
			failed++
			continue
		}
		if r.digest.Confidence() == engine.ConfidenceLow {
			low++
		}
		sizes = append(sizes, r.size)
		millis = append(millis, int(r.pass.Milliseconds()))
		frames += int(r.digest.LastFrame)
		passMillis += int(r.pass.Milliseconds())
	}
	fmt.Printf("\nreplays=%d failed=%d low_confidence=%d workers=%d wall=%v\n", len(results), failed, low, workers, wall.Round(time.Second))
	if len(sizes) == 0 {
		return
	}
	slices.Sort(sizes)
	slices.Sort(millis)
	fmt.Printf("digest bytes: p50=%d p90=%d max=%d total=%d\n", pct(sizes, .5), pct(sizes, .9), sizes[len(sizes)-1], sum(sizes))
	fmt.Printf("pass ms: p50=%d p90=%d max=%d; frames/s per worker=%.0f\n", pct(millis, .5), pct(millis, .9), millis[len(millis)-1], float64(frames)/(float64(max(passMillis, 1))/1000))
}

func pct(sorted []int, q float64) int { return sorted[int(float64(len(sorted)-1)*q)] }

func sum(xs []int) (s int) {
	for _, x := range xs {
		s += x
	}
	return s
}
