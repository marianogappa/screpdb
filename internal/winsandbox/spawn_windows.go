//go:build windows

package winsandbox

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// lowIntegritySID is the well-known Low mandatory integrity level SID. Lowering
// a duplicated primary token to this level is what makes the spawned worker
// unable to write outside directories explicitly labeled Low.
const lowIntegritySID = "S-1-16-4096"

// ShouldLaunch reports whether this process should act as the Medium-integrity
// launcher: true on Windows unless we are already the Low worker child.
func ShouldLaunch() bool { return !IsWorker() }

// SpawnWorkerLow launches exePath (with args) as a Low-integrity child of this
// Medium process, waits for it to exit, and returns its exit code. extraEnv
// entries ("KEY=VALUE") are added to the child's environment (WorkerEnv is set
// by the caller). workDir becomes the child's working directory — set it to the
// Low-writable app-data root so the worker's cwd-relative bootstrap never tries
// to write into the Medium install directory. Cancelling ctx terminates the
// worker (the tray "Quit" path): a spawned Low process is not a child job, so it
// would otherwise outlive the launcher.
func SpawnWorkerLow(ctx context.Context, exePath string, args, extraEnv []string, workDir string) (int, error) {
	var procToken windows.Token
	if err := windows.OpenProcessToken(
		windows.CurrentProcess(),
		windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY|windows.TOKEN_ASSIGN_PRIMARY|windows.TOKEN_ADJUST_DEFAULT|windows.TOKEN_ADJUST_SESSIONID,
		&procToken,
	); err != nil {
		return 0, fmt.Errorf("open process token: %w", err)
	}
	defer procToken.Close()

	var lowToken windows.Token
	if err := windows.DuplicateTokenEx(procToken, windows.MAXIMUM_ALLOWED, nil, windows.SecurityImpersonation, windows.TokenPrimary, &lowToken); err != nil {
		return 0, fmt.Errorf("duplicate token: %w", err)
	}
	defer lowToken.Close()

	if err := setTokenIntegrityLow(lowToken); err != nil {
		return 0, err
	}

	env := append(os.Environ(), extraEnv...)
	envBlock, err := makeEnvBlock(env)
	if err != nil {
		return 0, fmt.Errorf("build environment block: %w", err)
	}

	cmdLine := windows.ComposeCommandLine(append([]string{exePath}, args...))
	cmdLinePtr, err := windows.UTF16PtrFromString(cmdLine)
	if err != nil {
		return 0, fmt.Errorf("encode command line: %w", err)
	}
	var workDirPtr *uint16
	if strings.TrimSpace(workDir) != "" {
		if workDirPtr, err = windows.UTF16PtrFromString(workDir); err != nil {
			return 0, fmt.Errorf("encode working dir: %w", err)
		}
	}

	si := &windows.StartupInfo{}
	si.Cb = uint32(unsafe.Sizeof(*si))
	var pi windows.ProcessInformation

	// The GUI worker has no console; do not force stdio inheritance. It routes
	// diagnostics to the app-data log file itself.
	if err := windows.CreateProcessAsUser(
		lowToken,
		nil, // appName: taken from the command line's argv[0]
		cmdLinePtr,
		nil, nil,
		false, // inheritHandles
		windows.CREATE_UNICODE_ENVIRONMENT,
		&envBlock[0],
		workDirPtr,
		si,
		&pi,
	); err != nil {
		return 0, fmt.Errorf("create low-integrity process: %w", err)
	}
	defer windows.CloseHandle(pi.Thread)
	defer windows.CloseHandle(pi.Process)

	// Terminate the worker when the launcher cancels ctx (tray "Quit"). We join
	// this goroutine (wg.Wait) before the deferred CloseHandle runs, so it can
	// never fire a TerminateProcess against an already-closed, possibly recycled
	// handle.
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-ctx.Done():
			_ = windows.TerminateProcess(pi.Process, 1)
		case <-done:
		}
	}()

	_, waitErr := windows.WaitForSingleObject(pi.Process, windows.INFINITE)
	close(done)
	wg.Wait()
	if waitErr != nil {
		return 0, fmt.Errorf("wait for worker: %w", waitErr)
	}
	var code uint32
	if err := windows.GetExitCodeProcess(pi.Process, &code); err != nil {
		return 0, fmt.Errorf("get worker exit code: %w", err)
	}
	return int(code), nil
}

// setTokenIntegrityLow sets tok's mandatory integrity level to Low.
func setTokenIntegrityLow(tok windows.Token) error {
	sid, err := windows.StringToSid(lowIntegritySID)
	if err != nil {
		return fmt.Errorf("parse low integrity SID: %w", err)
	}
	tml := windows.Tokenmandatorylabel{
		Label: windows.SIDAndAttributes{
			Sid:        sid,
			Attributes: windows.SE_GROUP_INTEGRITY,
		},
	}
	if err := windows.SetTokenInformation(tok, windows.TokenIntegrityLevel, (*byte)(unsafe.Pointer(&tml)), tml.Size()); err != nil {
		return fmt.Errorf("lower token integrity: %w", err)
	}
	return nil
}

// makeEnvBlock builds a UTF-16, double-null-terminated environment block from
// "KEY=VALUE" strings, as CreateProcessAsUser expects with
// CREATE_UNICODE_ENVIRONMENT.
func makeEnvBlock(env []string) ([]uint16, error) {
	var block []uint16
	for _, e := range env {
		if e == "" {
			continue
		}
		u, err := windows.UTF16FromString(e)
		if err != nil {
			return nil, err
		}
		block = append(block, u...) // u already includes its terminating NUL
	}
	block = append(block, 0) // final NUL terminating the block
	return block, nil
}
