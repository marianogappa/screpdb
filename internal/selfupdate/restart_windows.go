//go:build windows

package selfupdate

import (
	"os"
	"os/exec"
)

// reexec spawns the new binary and exits the current process. Windows has no
// exec-in-place; the child waits briefly on startup (see IsRestart handling) for
// this process to exit and release the listening port.
func reexec(self string, args, env []string) error {
	cmd := exec.Command(self, args[1:]...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		return err
	}
	os.Exit(0)
	return nil
}
