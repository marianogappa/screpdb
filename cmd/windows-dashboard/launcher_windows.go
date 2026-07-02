//go:build windows

package main

import (
	"context"
	"log"
	"os"

	"github.com/marianogappa/screpdb/internal/appdata"
	"github.com/marianogappa/screpdb/internal/selfupdate"
	"github.com/marianogappa/screpdb/internal/winsandbox"
)

// runLauncher is the Medium-integrity parent process (issue #237). It prepares
// the Low-writable app-data dir, serves the watch-me broker, and supervises the
// Low-integrity worker: on a normal exit it exits too; on the update exit code
// it performs the (Medium-only) self-update swap and relaunches the worker.
// It returns the process exit code. The worker inherits os.Args verbatim.
func runLauncher() int {
	dir, err := appdata.Dir()
	if err != nil {
		log.Printf("launcher: cannot resolve app-data dir: %v", err)
		return 1
	}
	if err := winsandbox.SetLowLabel(dir); err != nil {
		// Non-fatal: without the label the worker can't write and will surface
		// its own errors, but we still want the app to come up for diagnosis.
		log.Printf("launcher: failed to set Low integrity label on %s: %v", dir, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopBroker, err := winsandbox.StartBroker(ctx, dir)
	if err != nil {
		log.Printf("launcher: watch-me broker unavailable: %v", err)
	}
	defer stopBroker()

	self, err := os.Executable()
	if err != nil {
		log.Printf("launcher: cannot resolve own path: %v", err)
		return 1
	}

	extraEnv := []string{winsandbox.WorkerEnv + "=1"}
	for {
		// SCREPDB_WORKER=1 tells the child to run the dashboard rather than
		// relaunch itself; the working dir is the Low-writable app-data root.
		code, err := winsandbox.SpawnWorkerLow(self, os.Args[1:], extraEnv, dir)
		if err != nil {
			log.Printf("launcher: failed to spawn Low-integrity worker: %v", err)
			return 1
		}
		if code != winsandbox.ExitCodeUpdate {
			return code
		}

		// The worker asked us to self-update. Only the Medium launcher can
		// overwrite the install-dir binary; the worker has already exited and
		// released the port.
		log.Printf("launcher: worker requested self-update; applying")
		selfupdate.CleanupOldBinary()
		extraEnv = []string{winsandbox.WorkerEnv + "=1"}
		newVersion, applyErr := selfupdate.Apply(ctx)
		if applyErr != nil {
			log.Printf("launcher: self-update failed (%v); relaunching current version", applyErr)
			continue
		}
		log.Printf("launcher: self-update applied (%s); relaunching worker", newVersion)
		extraEnv = append(extraEnv, selfupdate.RestartEnvKV())
	}
}
