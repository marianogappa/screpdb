//go:build windows && integration

// This integration test asserts the core containment invariant of issue #237 on
// a real Windows kernel: a worker spawned via SpawnWorkerLow runs at Low
// integrity, can write into the Low-labeled app-data dir, and is refused by the
// OS when it tries to write anywhere else. It is gated behind the `integration`
// build tag so it only runs on the windows-latest CI job (`go test -tags
// integration ./internal/winsandbox`), never in the default `go test ./...`.
//
// The worker is this very test binary, re-executed with WINSANDBOX_SELFTEST set;
// TestMain dispatches to the worker self-check in that case.
package winsandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

const selfTestEnv = "WINSANDBOX_SELFTEST"

func TestMain(m *testing.M) {
	if appDir := os.Getenv(selfTestEnv); appDir != "" {
		os.Exit(workerSelfCheck(appDir))
	}
	os.Exit(m.Run())
}

func TestLowIntegrityContainment(t *testing.T) {
	appDir := t.TempDir()
	if err := SetLowLabel(appDir); err != nil {
		t.Fatalf("SetLowLabel: %v", err)
	}

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("Executable: %v", err)
	}
	code, err := SpawnWorkerLow(self, nil, []string{selfTestEnv + "=" + appDir, WorkerEnv + "=1"}, appDir)
	if err != nil {
		t.Fatalf("SpawnWorkerLow: %v", err)
	}
	if code != 0 {
		t.Fatalf("Low-integrity worker self-check failed with exit code %d (see stderr above)", code)
	}
}

// workerSelfCheck runs inside the Low-integrity child. It returns 0 on success;
// any non-zero code (with a message on stderr) fails the parent test.
func workerSelfCheck(appDir string) int {
	sid, err := currentIntegritySID()
	if err != nil {
		fmt.Fprintf(os.Stderr, "worker: read integrity level: %v\n", err)
		return 2
	}
	if sid != lowIntegritySID {
		fmt.Fprintf(os.Stderr, "worker: integrity SID = %s, want %s (not running at Low)\n", sid, lowIntegritySID)
		return 3
	}

	// A write inside the Low-labeled app-data dir must succeed.
	inside := filepath.Join(appDir, "worker-write-probe.txt")
	if err := os.WriteFile(inside, []byte("ok"), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "worker: write inside app-data failed (should succeed): %v\n", err)
		return 4
	}

	// A write outside it (into the Medium-labeled parent) must be refused.
	outside := filepath.Join(filepath.Dir(appDir), "worker-escape-probe.txt")
	if err := os.WriteFile(outside, []byte("escape"), 0o644); err == nil {
		_ = os.Remove(outside)
		fmt.Fprintf(os.Stderr, "worker: write outside app-data SUCCEEDED (containment breached): %s\n", outside)
		return 5
	}
	return 0
}

// currentIntegritySID returns the integrity-level SID string of the current
// process token (e.g. "S-1-16-4096" for Low).
func currentIntegritySID() (string, error) {
	var tok windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &tok); err != nil {
		return "", err
	}
	defer tok.Close()

	var size uint32
	// First call sizes the buffer.
	err := windows.GetTokenInformation(tok, windows.TokenIntegrityLevel, nil, 0, &size)
	if err != nil && err != windows.ERROR_INSUFFICIENT_BUFFER {
		return "", err
	}
	buf := make([]byte, size)
	if err := windows.GetTokenInformation(tok, windows.TokenIntegrityLevel, &buf[0], size, &size); err != nil {
		return "", err
	}
	tml := (*windows.Tokenmandatorylabel)(unsafe.Pointer(&buf[0]))
	return tml.Label.Sid.String(), nil
}
