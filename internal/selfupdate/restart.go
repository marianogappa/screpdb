package selfupdate

import "os"

const restartEnv = "SCREPDB_SELFUPDATE_RESTART"

// IsRestart reports whether this process was launched by Restart after a
// successful self-update. Startup uses it to suppress a duplicate browser tab and
// to wait briefly for the previous process to release the listening port.
func IsRestart() bool { return os.Getenv(restartEnv) == "1" }

// Restart relaunches the just-swapped binary in place, inheriting the original
// arguments. On Unix it replaces the current process image; on Windows it spawns
// the new binary and exits. It does not return on success.
func Restart() error {
	self, err := executablePath()
	if err != nil {
		return err
	}
	env := append(os.Environ(), restartEnv+"=1")
	args := append([]string{self}, os.Args[1:]...)
	return reexec(self, args, env)
}
