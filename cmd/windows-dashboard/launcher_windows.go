//go:build windows

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/marianogappa/screpdb/internal/appdata"
	"github.com/marianogappa/screpdb/internal/crashreport"
	"github.com/marianogappa/screpdb/internal/dashboardrun"
	"github.com/marianogappa/screpdb/internal/selfupdate"
	"github.com/marianogappa/screpdb/internal/tray"
	"github.com/marianogappa/screpdb/internal/winsandbox"
	"github.com/pkg/browser"
	"github.com/spf13/pflag"
)

// runLauncher is the Medium-integrity parent process (issue #237). It prepares
// the Low-writable app-data dir, serves the watch-me/open-url broker, hosts the
// system-tray icon, and supervises the Low-integrity worker: on a normal exit it
// exits too; on the update exit code it performs the (Medium-only) self-update
// swap and relaunches the worker. It returns the process exit code. The worker
// inherits os.Args verbatim.
//
// The tray must live here, not in the worker: a Low-integrity process cannot
// register a notification-area icon — UIPI drops its Shell_NotifyIcon call up to
// the (Medium) Explorer — so the icon silently never appears. The launcher, at
// Medium, also opens the browser directly for "Open dashboard".
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

	// The worker re-parses flags authoritatively; the launcher only needs the
	// port and headless flag to wire the tray's "Open dashboard" action.
	var opts dashboardrun.Options
	fs := pflag.NewFlagSet("screpdb-launcher", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dashboardrun.RegisterFlags(fs, &opts)
	_ = fs.Parse(os.Args[1:])
	serverURL := fmt.Sprintf("http://localhost:%d", opts.Port)

	// Supervise the worker off the main goroutine; systray owns the main thread.
	exitCode := make(chan int, 1)
	workerDone := make(chan struct{})
	go func() {
		defer crashreport.Guard()
		exitCode <- superviseWorker(ctx, self, dir)
		close(workerDone)
		tray.Quit() // worker exited on its own → drop the tray so the launcher exits
	}()

	err = tray.Run(tray.Config{
		Title:   "screpdb",
		Tooltip: "screpdb dashboard",
		Icon:    tray.DefaultIcon(),
		OnOpen: func() {
			if opts.Headless {
				return
			}
			if err := browser.OpenURL(serverURL); err != nil {
				log.Printf("launcher: failed to open dashboard: %v", err)
			}
		},
		OnQuit: func() {
			cancel()     // terminate the worker (SpawnWorkerLow honors ctx)
			<-workerDone // hold the tray open until the worker is actually gone
		},
	})
	if err != nil {
		log.Printf("launcher: tray error: %v", err)
	}
	cancel() // tray gone → make sure the worker is torn down before we exit
	return <-exitCode
}

// superviseWorker runs (and re-runs, across self-updates) the Low-integrity
// worker until it exits for good, and returns the launcher's exit code. It
// returns 0 when ctx was cancelled (an intentional tray "Quit").
func superviseWorker(ctx context.Context, self, dir string) int {
	extraEnv := []string{winsandbox.WorkerEnv + "=1"}
	for {
		// SCREPDB_WORKER=1 tells the child to run the dashboard rather than
		// relaunch itself; the working dir is the Low-writable app-data root.
		code, err := winsandbox.SpawnWorkerLow(ctx, self, os.Args[1:], extraEnv, dir)
		if err != nil {
			log.Printf("launcher: failed to spawn Low-integrity worker: %v", err)
			return 1
		}
		if ctx.Err() != nil {
			return 0 // intentional shutdown: the tray "Quit" cancelled ctx.
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
