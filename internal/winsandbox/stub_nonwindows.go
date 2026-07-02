//go:build !windows

package winsandbox

import (
	"context"
	"errors"
)

// errUnsupported is returned by the Windows-only primitives on other platforms.
// Callers gate real use behind ShouldLaunch()/IsWorker(), which are false here,
// so these are never reached in practice — they exist only to keep the codebase
// cross-platform.
var errUnsupported = errors.New("winsandbox: Low-integrity sandbox is Windows-only")

// ShouldLaunch reports whether this process should act as the Medium-integrity
// launcher. Always false off Windows (no launcher/worker split).
func ShouldLaunch() bool { return false }

// SetLowLabel is a no-op stub off Windows.
func SetLowLabel(string) error { return errUnsupported }

// SpawnWorkerLow is a no-op stub off Windows.
func SpawnWorkerLow(string, []string, []string, string) (int, error) { return 0, errUnsupported }

// StartBroker is a no-op stub off Windows.
func StartBroker(context.Context, string) (func(), error) { return func() {}, errUnsupported }

// BrokerSeeReplay is a no-op stub off Windows; the worker path that calls it
// only runs on Windows (guarded by IsWorker()).
func BrokerSeeReplay(string, string, string) (string, error) { return "", errUnsupported }
