// Package winsandbox contains screpdb's Windows Low-integrity containment
// (issue #237): a Medium-integrity launcher relaunches the real worker at Low
// integrity so the OS confines all of screpdb's writes to the single app-data
// directory. Even a compromised replay/map parser cannot write elsewhere.
//
// This package makes raw golang.org/x/sys/windows syscalls (token duplication,
// integrity-level lowering, CreateProcessAsUser, SetNamedSecurityInfo) and runs
// a small file-drop broker so the Medium launcher can perform the one legitimate
// write into the read-only replays folder on the Low worker's behalf. It is a
// documented, sanctioned exception to the iofacade enforcement test, alongside
// internal/selfupdate.
//
// All exported entry points are safe to call on any OS: the non-Windows build
// provides no-op / error stubs so the rest of the codebase stays cross-platform.
package winsandbox

import "os"

const (
	// WorkerEnv marks a process spawned as the Low-integrity worker child. The
	// launcher sets it when relaunching; IsWorker reads it.
	WorkerEnv = "SCREPDB_WORKER"

	// ExitCodeUpdate is the exit code the Low worker exits with to ask the
	// Medium launcher to perform a self-update and relaunch it. Chosen well clear
	// of the 0/1/2 codes the app and crash handler already use.
	ExitCodeUpdate = 90
)

// IsWorker reports whether this process is the Low-integrity worker child.
func IsWorker() bool { return os.Getenv(WorkerEnv) == "1" }
