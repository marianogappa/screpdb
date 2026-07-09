package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/marianogappa/screpdb/cmd"
	"github.com/marianogappa/screpdb/internal/appdata"
	"github.com/marianogappa/screpdb/internal/crashreport"
	"github.com/marianogappa/screpdb/internal/dashboardrun"
	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/winsandbox"
	"github.com/spf13/pflag"
)

func main() {
	crashreport.SetOpenBrowser(true)
	defer crashreport.Recover(true)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// On Windows, the first (Medium-integrity) process is the launcher: it
	// prepares the Low-writable app-data dir, hosts the tray, opens the browser,
	// and relaunches this binary as a Low-integrity worker (issue #237).
	// ShouldLaunch is false once we are the worker, and always false off Windows.
	if winsandbox.ShouldLaunch() {
		if logPath, err := appdata.Path("screpdb-launcher.log"); err == nil {
			if logFile, err := iofacade.Create(logPath); err == nil {
				log.SetOutput(logFile)
			}
		}
		os.Exit(runLauncher())
	}

	// The GUI binary has no attached console, so route diagnostics to a log file
	// under the app-data root (issue #237) — the single Low-writable directory
	// where crash reports also land. Best-effort: fall back to default logging.
	if logPath, err := appdata.Path("screpdb-gui.log"); err == nil {
		if logFile, err := iofacade.Create(logPath); err == nil {
			log.SetOutput(logFile)
		}
	}

	// The Low-integrity worker cannot open a browser itself (issue #237), so the
	// crash reporter's "open the prefilled issue" step is routed to the Medium
	// launcher via the broker, same as the dashboard auto-open.
	if winsandbox.IsWorker() {
		crashreport.SetOpener(func(issueURL string) error {
			dir, err := appdata.Dir()
			if err != nil {
				return err
			}
			return winsandbox.BrokerOpenURL(dir, issueURL)
		})
	}

	var opts dashboardrun.Options
	fs := pflag.NewFlagSet("windows-dashboard", pflag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dashboardrun.RegisterFlags(fs, &opts)
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Println(err)
		os.Exit(1)
	}

	// The tray lives in the Medium launcher; the worker only serves the dashboard.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := cmd.RunDashboardWithContext(ctx, opts); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
