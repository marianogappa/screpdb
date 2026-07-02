package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/marianogappa/screpdb/cmd"
	"github.com/marianogappa/screpdb/internal/appdata"
	"github.com/marianogappa/screpdb/internal/crashreport"
	"github.com/marianogappa/screpdb/internal/dashboardrun"
	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/tray"
	"github.com/marianogappa/screpdb/internal/winsandbox"
	"github.com/spf13/pflag"
)

func main() {
	crashreport.SetOpenBrowser(true)
	defer crashreport.Recover(true)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// On Windows, the first (Medium-integrity) process is the launcher: it
	// prepares the Low-writable app-data dir and relaunches this binary as a
	// Low-integrity worker (issue #237). ShouldLaunch is false once we are the
	// worker, and always false off Windows.
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
	if logPath, err := appdata.Path("screpdb-dashboard.log"); err == nil {
		if logFile, err := iofacade.Create(logPath); err == nil {
			log.SetOutput(logFile)
		}
	}

	var opts dashboardrun.Options
	fs := pflag.NewFlagSet("windows-dashboard", pflag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dashboardrun.RegisterFlags(fs, &opts)
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Println(err)
		os.Exit(1)
	}

	if !tray.Supported() {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := cmd.RunDashboardWithContext(ctx, opts); err != nil {
			log.Println(err)
			os.Exit(1)
		}
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		defer crashreport.Guard()
		errCh <- cmd.RunDashboardWithContext(ctx, opts)
	}()

	go func() {
		defer crashreport.Guard()
		if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("dashboard exited with error: %v", err)
		}
		cancel()
		tray.Quit()
	}()

	if err := tray.Run(tray.Config{
		Title:   "screpdb",
		Tooltip: "screpdb dashboard",
		Icon:    tray.DefaultIcon(),
		OnQuit:  cancel,
	}); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
