package power

import (
	"os/exec"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"
)

func onBattery() bool {
	out, err := exec.Command("/usr/bin/pmset", "-g", "batt").Output()
	return err == nil && strings.Contains(string(out), "'Battery Power'")
}

const (
	prioDarwinThread = 3
	prioDarwinBG     = 0x1000
)

// LowerThreadPriority locks the goroutine to its OS thread and moves that thread to the background band,
// which macOS also throttles for disk I/O. The goroutine must not unlock: the thread then dies with it.
func LowerThreadPriority() {
	runtime.LockOSThread()
	_ = unix.Setpriority(prioDarwinThread, 0, prioDarwinBG)
}
