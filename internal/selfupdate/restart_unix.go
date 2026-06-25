//go:build !windows

package selfupdate

import "syscall"

// reexec replaces the current process image with the new binary. The kernel
// closes the (CLOEXEC) listening socket, so the relaunched process can re-bind
// the same port; the PID is preserved.
func reexec(self string, args, env []string) error {
	return syscall.Exec(self, args, env)
}
