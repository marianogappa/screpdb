package mcp

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/marianogappa/screpdb/internal/netfacade"
)

// PortScanRange mirrors the dashboard's single-instance policy: starting at
// the preferred port, a small range is probed before giving up.
const PortScanRange = 10

const (
	probeTimeout = 2 * time.Second
	// listenTimeout bounds how long a spawned server may take to answer at
	// all; it binds and accepts before it reads a single replay.
	listenTimeout = 30 * time.Second
	// corpusTimeout bounds the read of the replay folder that follows. It is
	// generous because it is a cold read of a folder of unknown size, and
	// giving up early would answer the first question from half a corpus.
	corpusTimeout = 5 * time.Minute
	pollInterval  = 250 * time.Millisecond
)

type ConnectOptions struct {
	// Port is where the probe starts; PortScanRange ports are tried.
	Port int
	// AutoStart spawns `screpdb dashboard --headless` when no running
	// screpdb answers on the scanned range.
	AutoStart bool
	// ReplayDir is passed to a spawned server; empty means its saved folder.
	ReplayDir string
	// WaitForCorpus holds startup until the spawned server has read its replay
	// folder, so the first question isn't answered from a half-loaded corpus.
	WaitForCorpus bool
}

// Connect returns a client for a screpdb API, together with a stop function
// that shuts down a server this call started (and does nothing otherwise).
//
// It attaches to an already-running screpdb first: an open dashboard serves
// the same API, so the common case costs nothing and reads the corpus the user
// is already looking at. Only when nothing answers does it start its own
// headless server.
func Connect(ctx context.Context, opts ConnectOptions) (*Client, func(), error) {
	if opts.Port <= 0 {
		opts.Port = 8000
	}
	if addr, ok := findRunning(opts.Port); ok {
		log.Printf("Using the screpdb already serving on %s", addr)
		return NewClient(addr), func() {}, nil
	}
	if !opts.AutoStart {
		return nil, nil, fmt.Errorf(
			"no screpdb server found on localhost ports %d-%d.\n"+
				"Start one and try again:\n"+
				"    screpdb dashboard --headless -p %d\n"+
				"An open screpdb dashboard serves the same API, so launching the app works too.\n"+
				"Alternatively, drop --no-auto-start and screpdb mcp will start its own headless server.",
			opts.Port, opts.Port+PortScanRange-1, opts.Port)
	}
	return start(ctx, opts)
}

func findRunning(preferred int) (string, bool) {
	for p := preferred; p < preferred+PortScanRange; p++ {
		addr := "localhost:" + strconv.Itoa(p)
		if netfacade.IsLocalScrepdb(addr, probeTimeout) {
			return addr, true
		}
	}
	return "", false
}

// execCommand builds the child process. It is a variable so the tests can put
// a stand-in in place of the real binary and exercise the whole spawn, wait
// and shutdown path without a dashboard.
var execCommand = func(exe string, args ...string) *exec.Cmd { return exec.Command(exe, args...) }

func start(ctx context.Context, opts ConnectOptions) (*Client, func(), error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, nil, fmt.Errorf("could not locate the screpdb binary to start a server: %w", err)
	}
	port, err := freePort(opts.Port)
	if err != nil {
		return nil, nil, err
	}

	args := []string{"dashboard", "--headless", "-p", strconv.Itoa(port)}
	if opts.ReplayDir != "" {
		args = append(args, "--replay-dir", opts.ReplayDir)
	}
	log.Printf("No screpdb server found; starting one: %s %v", exe, args)

	cmd := execCommand(exe, args...)
	// The MCP transport owns stdin/stdout, so the child gets neither; its log
	// lines go to our stderr, where the MCP client shows them.
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("could not start a headless screpdb server: %w", err)
	}
	// One owner of Wait: the goroutine. It closes exited rather than sending,
	// so both the startup wait and stop can observe it, and a child that dies
	// on its own is noticed while we are still waiting for it rather than
	// after the whole startup budget has burned down.
	var exitErr error
	exited := make(chan struct{})
	go func() { exitErr = cmd.Wait(); close(exited) }()
	stop := func() {
		_ = cmd.Process.Kill()
		<-exited
	}

	client, err := waitForServer(ctx, port, opts.WaitForCorpus, exited, &exitErr)
	if err != nil {
		stop()
		return nil, nil, err
	}
	return client, stop, nil
}

func freePort(preferred int) (int, error) {
	for p := preferred; p < preferred+PortScanRange; p++ {
		if netfacade.LocalPortAvailable("localhost:" + strconv.Itoa(p)) {
			return p, nil
		}
	}
	return 0, fmt.Errorf("no free port in range %d-%d to start a screpdb server on",
		preferred, preferred+PortScanRange-1)
}

// waitForServer polls until the spawned server answers, and — when asked —
// until it has finished reading its replay folder. A child that exits first
// fails the wait immediately, carrying its exit status.
func waitForServer(ctx context.Context, port int, waitForCorpus bool, exited <-chan struct{}, exitErr *error) (*Client, error) {
	addr := "localhost:" + strconv.Itoa(port)
	client := NewClient(addr)

	health, err := poll(ctx, exited, exitErr, listenTimeout, func() (Health, bool) {
		h, err := client.Health(ctx)
		return h, err == nil
	})
	if err != nil {
		return nil, fmt.Errorf("the headless screpdb server on %s never answered: %w", addr, err)
	}
	if waitForCorpus && !health.Library.Complete {
		log.Printf("Waiting for the headless screpdb server on %s to read %s", addr, health.Library.ReplayDir)
		health, err = poll(ctx, exited, exitErr, corpusTimeout, func() (Health, bool) {
			h, err := client.Health(ctx)
			return h, err == nil && h.Library.Complete
		})
		if err != nil {
			return nil, fmt.Errorf("the headless screpdb server on %s did not finish reading the replay folder: %w", addr, err)
		}
	}
	log.Printf("Headless screpdb ready on %s with %d replays from %s",
		addr, health.TotalReplays, health.Library.ReplayDir)
	return client, nil
}

// poll calls probe until it reports success, the context ends, the child exits,
// or the budget runs out.
func poll(ctx context.Context, exited <-chan struct{}, exitErr *error, budget time.Duration, probe func() (Health, bool)) (Health, error) {
	deadline := time.Now().Add(budget)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return Health{}, ctx.Err()
		case <-exited:
			return Health{}, fmt.Errorf("it exited first: %w", *exitErr)
		case <-time.After(pollInterval):
		}
		if health, ok := probe(); ok {
			return health, nil
		}
	}
	return Health{}, fmt.Errorf("gave up after %s", budget)
}
