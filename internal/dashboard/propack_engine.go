//go:build !js

package dashboard

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/marianogappa/screpdb/internal/engine"
	"github.com/marianogappa/screpdb/internal/enginepass"
	"github.com/marianogappa/screpdb/internal/iofacade"
)

// RunEngine runs the engine pass over every replay of the corpus, caching digests at cachePath, so Metrics
// reads Skill and engine-attributed hotkeys as the dashboard does. It returns once every replay has engine
// data or the engine gave up on it.
func (c *ProCorpus) RunEngine(ctx context.Context, data *engine.Data, cachePath string, logf func(string, ...any)) error {
	if err := iofacade.AllowDir(filepath.Dir(cachePath)); err != nil {
		return err
	}
	runner := enginepass.New(enginepass.Options{
		Library:   c.lib,
		CachePath: cachePath,
		OpenData:  func() (*engine.Data, error) { return data, nil },
		Logf:      logf,
	})
	c.dash.enginePass.runner = runner
	c.dash.library = &libraryRuntime{lib: c.lib}
	runner.Start()
	last, lastChange := -1, time.Now()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		status := runner.Status()
		if status.State == enginepass.StateOff || status.State == enginepass.StateFailed {
			return fmt.Errorf("engine pass: %s", status.Reason)
		}
		done, total := 0, 0
		for _, rec := range c.lib.Snapshot().Replays {
			total++
			if rec.EngineData != nil || runner.Unavailable(rec.ID) {
				done++
			}
		}
		if status.State == enginepass.StateRunning && done == total {
			return nil
		}
		if done != last {
			last, lastChange = done, time.Now()
			logf("engine pass: %d/%d replays", done, total)
		} else if time.Since(lastChange) > 10*time.Minute {
			return fmt.Errorf("engine pass stalled at %d/%d replays", done, total)
		}
		time.Sleep(2 * time.Second)
	}
}
