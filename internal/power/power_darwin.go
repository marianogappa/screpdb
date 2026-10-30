package power

import (
	"bytes"
	"runtime"

	"golang.org/x/sys/unix"
)

func starCraftRunning() bool {
	procs, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return false
	}
	for i := range procs {
		if isStarCraftProcess(cString(procs[i].Proc.P_comm[:])) {
			return true
		}
	}
	return false
}

func cString(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
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
